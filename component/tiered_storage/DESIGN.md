# Tiered Storage Design

This document is the implementation reference for the `tiered_storage` component.
It describes the target design. Where the current code differs, the code should
be changed to match this document.

## Purpose

`tiered_storage` lets applications keep data on local storage without backing it
up to the cloud. When local capacity runs low, the least recently used files are
moved (evicted) to cloud storage. Evicted files remain accessible: opening one
downloads a temporary copy, which is discarded when the file is closed.

## Guiding principles

- **Simplicity over perfect correctness.** The component has few users. Prefer a
  small, understandable state machine over exhaustive bookkeeping.
- **One authority per file.** At any moment, either the local copy or the cloud
  copy of a file is authoritative, never both.
- **Authority is encoded in the filesystem layout**, not in metadata. Recovery
  after a crash requires no persisted state.
- **Fail early.** Check everything that can be checked cheaply before changing
  anything, and order steps so that the hardest-to-undo step runs last.

## Assumptions and non-goals

- A single mounted instance owns the cloud namespace. Concurrent writers to the
  same bucket/container are unsupported; a stale local file may overwrite a newer
  cloud object.
- Local-owned data is intentionally **not** protected against loss of the local
  disk. Keeping data local is the purpose of the component. The one exception
  is [evict on close](#evict-on-close): a file that was modified after coming
  from the cloud is uploaded and evicted after its last close, so the cloud
  does not keep serving outdated content.
- Every recovery edge case need not be perfect. Rare failures (for example, a
  failed revert of a local rename) are logged and otherwise ignored.
- The tiered root is a dedicated directory owned by cloudfuse. It is never
  exposed to users or listed directly.

## On-disk layout

```text
<tiered_root>/            # configured by tiered_storage.path
  local/                  # authoritative local files (local-owned)
  cache/                  # disposable copies of cloud-owned files
```

Object `dir/file` maps to `<tiered_root>/local/dir/file` or
`<tiered_root>/cache/dir/file`. A file exists in at most one of the two trees.

Both trees are on the same filesystem, so moving a file between them is an
atomic rename.

There is no staging directory and no partial-download suffix. `cache/` is
disposable by definition, so a partially downloaded file in `cache/` needs the
same recovery as a complete one: deletion at startup. The per-file lock
serializes `OpenFile`, so no other opener can observe a partial download.

There is no state snapshot.

## Ownership states

| State | Local location | Authoritative copy | Cloud copy |
|---|---|---|---|
| Local-owned | `local/` | Local | Absent or stale |
| Cloud-owned, open | `cache/` | Cloud | Current |
| Cloud-owned, closed | none | Cloud | Current |

Transitions:

| From | To | Trigger | Mechanism |
|---|---|---|---|
| (none) | Local-owned | Create | Create file in `local/` |
| Cloud-owned, closed | Cloud-owned, open | Open | Download into `cache/` |
| Cloud-owned, closed | Local-owned | Open with `O_TRUNC` | Create empty file in `local/`; no download |
| Cloud-owned, open | Local-owned (`cloudStale`) | First mutation (write, truncate) | Rename `cache/x` → `local/x` before acknowledging |
| Cloud-owned, open | Cloud-owned, closed | Last close | Delete `cache/x` |
| Local-owned | Cloud-owned, closed | Eviction | Upload, then delete `local/x` |
| Local-owned (`cloudStale`) | Cloud-owned, closed | Last close | Evict on close: same as eviction, but not limited by thresholds |

Consequences:

- Files in `cache/` are always clean. Any mutation promotes them first.
- Files in `local/` are always treated as needing upload before deletion.
- Location alone decides ownership. No per-file `cloudBacked`, `isDirty`,
  `modeDirty`, or `ownerDirty` flags are needed. The only per-file hint,
  `cloudStale`, is in memory only. Losing it is safe: the file stays
  local-owned and is uploaded at normal eviction.

## In-memory state

- **Per-object file locks** (`common.LockMap`), with open-handle counts.
- **LRU queue** of local-owned files, used for eviction.
- **Cache size tracker**, covering both `local/` and `cache/`.
- **`cloudStale` flag** for each local-owned file, stored alongside its
  open-handle count. It means a cloud copy is known to exist and to be
  outdated, so the file should be [evicted on close](#evict-on-close).
  - Set on promotion (`cache/x` → `local/x`) and on an `O_TRUNC` open of a
    cloud-owned file. Both of these already hold the file lock, and the cloud
    copy is known to exist at that moment, so no network call is needed.
  - Cleared when the file leaves `local/` (eviction or delete), or when a cloud
    rename returns `ENOENT`.
  - Not persisted or recovered after a restart.

Nothing else is required. File size, mode, and owner are read from the local
file when needed.

## Startup

1. Ensure `<tiered_root>` exists.
2. If `<tiered_root>` contains entries other than `local/` and `cache/`, fail
   to start with a clear error. This prevents silently misinterpreting data from
   an older layout or a misconfigured path.
3. `RemoveAll(cache/)`, then recreate `local/` and `cache/`.
4. Walk `local/`. Every regular file is local-owned. Enqueue them in the LRU,
   ordered by modification time (oldest at the tail).
5. Measure cache size.
6. Start the eviction policy.

## Shutdown

Stop the eviction policy and wait for in-flight uploads, including
evict-on-close goroutines (tracked with a `sync.WaitGroup`). Evict-on-close
goroutines that have not started uploading exit without doing anything. Leave
`local/` in place.
Nothing is persisted. `cache/` contents are discarded on next start.

## Cloud connectivity rule

Operations that change the namespace of something which might exist in the
cloud (delete, rename) check `NextComponent().CloudConnected()` **after taking
locks and before making any change**. If the cloud is not connected, they check
`NextComponent().GetAttr()` as well. If there's no connection, and GetAttr returns
anything other than a simple error satisfying os.IsNotExist(err), they fail immediately.
No network call is made by these checks.

Rationale:

- Fails fast instead of holding locks while SDK retries and timeouts elapse.
- Avoids exposing transient mixed state to listings and lookups, which do not
  take file locks.
- Avoids rename-then-revert churn visible to watchers, Explorer, and antivirus.

The flag is a hint. Operations still handle cloud failures after the check.

Writing to and creating local files does not require connectivity.

## File operations

### CreateFile / OpenFile with O_CREATE on a nonexistent object

1. Take the file lock.
2. Fail with `EEXIST` if the object exists locally or in the cloud.
3. `MkdirAll` the parent in `local/`; create the file in `local/`.
4. Return a handle.

### OpenFile on an existing object

1. Take the file lock.
2. If `local/x` exists: open it.
3. Else if `cache/x` exists (another handle has it open): open it.
4. Else (cloud-owned, closed):
   - With `O_TRUNC`: create an empty `local/x` and set `cloudStale`. The file
     is now local-owned.
   - Otherwise: download directly into `cache/x`. On failure (including
     `ENOSPC` from the local filesystem), delete `cache/x` and return the
     error.
5. Increment the open count and return a handle.

### WriteFile / TruncateFile

1. If the handle's file is in `cache/`, take the file lock and promote: rename
   `cache/x` → `local/x`, set `cloudStale`, then enqueue it in the LRU. Open
   handles remain valid (see [Windows semantics](#windows-semantics)).
2. Write/truncate the local file. There is no size pre-check. If the local
   filesystem returns `ENOSPC`, return it to the caller.

Promotion happens before the write is acknowledged, so an acknowledged write is
always in `local/` and survives a crash.

### FlushFile / SyncFile

Sync the local file. No upload.

Uploading on flush would only protect local-owned data from local disk loss,
which is a non-goal. Modified cloud files are uploaded after the last close
instead (see [Evict on close](#evict-on-close)), so files that are flushed
repeatedly while open are uploaded at most once per close.

### ReleaseFile

1. Take the file lock, decrement the open count, close the file.
2. If other handles remain open, return.
3. If the file is in `cache/`: delete it.
4. If the file is in `local/`: enqueue it in the LRU. If `cloudStale` is set
   and `evict-on-close` is enabled, start an
   [evict on close](#evict-on-close) goroutine.

### ReadInBuffer

Read from the handle's file. No locking or state changes.

### GetAttr

1. If `local/x` exists, return its attributes. No cloud call.
2. Otherwise return the cloud attributes.

`cache/` copies are identical to the cloud object, so they never need to be
consulted.

### DeleteFile

1. Take the file lock.
2. Check cloud connectivity; fail early if disconnected.
3. Delete in the cloud. Ignore `ENOENT` (a local-owned file may never have been
   uploaded). Return any other error. Nothing has changed locally.
4. Delete `local/x` or `cache/x` if present. Open handles remain valid.
5. Remove the object from the LRU.

The cloud delete runs first so a stale cloud copy can never resurface after the
local copy is gone.

### RenameFile

1. Take both file locks in lexical order.
2. Check cloud connectivity; fail early if disconnected.
3. Fail early if the destination exists and replacing it is not permitted.
4. If the source is in `local/` or `cache/`: `MkdirAll` the destination parent
   and rename the local file.
5. Rename in the cloud.
   - `ENOENT` is acceptable if the source was in `local/` (never uploaded).
   - On any other failure, rename the local file back and return the error. If
     the revert fails, log an error naming both paths and stating that local and
     cloud have diverged, then return the original error.
6. Update in-memory state: LRU entry, open-handle paths, lock counts. The
   `cloudStale` flag moves with the file. If the cloud rename returned
   `ENOENT`, clear it.

A stale cloud copy of a local-owned file is renamed along with it, which is
harmless: evict on close or normal eviction will overwrite it.

### Chmod

- `local/x` exists: chmod the local file only. The mode is applied to the cloud
  object when the file is uploaded.
- `cache/x` exists: chmod the cloud object (authoritative), then the local copy.
- Neither: chmod the cloud object.

### Chown

Same pattern as Chmod. On upload, owner and group are read from the local file
(Linux only). If the process cannot chown the local file, the change is not
preserved for local-owned files. This loss is accepted.

## Directory operations

Directories are not tracked by ownership. A directory exists in the virtual
namespace if it exists in the cloud or in `local/`.

### CreateDir

1. Fail with `EEXIST` if the directory exists in either place.
2. Create it in `local/` and in the cloud (ignore cloud `EEXIST`).

### DeleteDir

1. Check cloud connectivity; fail early if disconnected.
2. Delete in the cloud (ignore `ENOENT`).
3. Remove `local/<dir>` and `cache/<dir>` if present.
4. Return `ENOENT` only if it existed nowhere.

### IsDirEmpty

Empty only if `local/<dir>` is empty or missing **and** the cloud directory is
empty.

### StreamDir

Merge cloud entries with `local/<dir>` entries. For files present in both,
local attributes win. `cache/` is never listed. No name filtering is needed,
because the tiered root is never exposed.

### RenameDir

1. Take locks for every affected object. Collect affected names **after**
   locking (or hold a coarse directory-rename lock), so files created during the
   operation cannot be missed.
2. Check cloud connectivity; fail early if disconnected.
3. Cheap pre-checks: source exists, destination does not exist, destination
   parent can be created.
4. **Rename locally first**: `local/src` → `local/dst`, then `cache/src` →
   `cache/dst`, for whichever exist. If a local rename fails, revert any earlier
   local rename and return the error. Map Windows access-denied errors to
   `EBUSY`.
5. **Rename in the cloud last.** On failure, revert the local renames in
   reverse order and return the error. If a revert fails, log an error naming
   both paths and stating that local and cloud have diverged.
6. Update in-memory state: LRU entries, open-handle paths, lock counts, and
   `cloudStale` flags, which move with their files.

Why local first:

| Step | Can fail unpredictably | Cheap to undo |
|---|---|---|
| Local rename | Yes (Windows, open files) | Yes |
| Cloud rename | Yes (S3 is copy+delete per object; partial failure possible) | No |

The hardest-to-undo step runs last. A partial cloud failure leaves the cloud in
a mixed state regardless of ordering; local-first does not make that worse.

No Windows-specific open-file pre-check is needed. On Windows, NTFS refuses to
rename a directory while any file beneath it is open, even with
`FILE_SHARE_DELETE`. Because the local rename runs first, that refusal is the
early failure, and it also covers handles held by other processes (antivirus,
indexer, Explorer) that a handle-count check would miss. Linux keeps its native
behavior of allowing the rename.

## Eviction

Eviction is always asynchronous. It runs only in the background LRU policy,
which triggers when usage exceeds `high-threshold` and evicts until usage is
below `low-threshold`. File operations never evict, and never wait for eviction.

Rationale: synchronous eviction makes a write wait on cloud uploads, holds the
writer's file lock across network calls, and adds a second eviction entry point
with its own locking and target calculation. Letting writes fail with the
local filesystem's `ENOSPC` is simpler and predictable. Users size
`max-size-mb` and the thresholds so the background policy keeps headroom for
their write rate.

For each candidate, with its file lock held:

1. Skip it if it has open handles.
2. Upload `local/x` to the cloud, including mode (and owner/group on Linux) read
   from the local file.
3. On upload failure: leave the file in place and keep it eligible for later
   eviction.
4. On success: delete `local/x` and remove it from the LRU.

Empty directories left in `local/` are not pruned. They are harmless and also
exist in the cloud namespace.

### Evict on close

A local-owned file with `cloudStale` set is evicted as soon as it is closed
for the last time, whatever the usage thresholds say. A file that came from
the cloud therefore returns to the cloud once it is no longer in use. The cloud
copy does not stay stale until LRU eviction reaches the file, which may never
happen for a frequently used file. This is controlled by
`tiered_storage.evict-on-close` (default `true`). It only affects files with
`cloudStale` set.

`ReleaseFile` starts a goroutine for the path and returns. It never uploads
inline. The goroutine:

1. Takes `evictMu`, then the file lock (the normal lock order).
2. Returns if the component is shutting down, the file is no longer in
   `local/`, or `cloudStale` is no longer set. A file that was reopened is
   skipped by the eviction steps, because it has open handles. Its next close
   will start another goroutine.
3. Runs the [eviction](#eviction) steps for that file.
4. On upload failure: logs the error and returns. The file stays local-owned
   with `cloudStale` set. It is retried at its next close or by normal
   eviction. There is no retry loop.

No queue or deduplication is needed. Extra goroutines for the same file find
nothing to do after step 2. Holding `evictMu` means only one eviction upload
runs at a time, whether it was started by a close or by a threshold pass.
- No connectivity pre-check is made. If the cloud is disconnected, the upload
  fails and the file stays local.

Files without `cloudStale` set are not affected. Data created locally, and
any file after a restart, stays local until threshold-driven eviction.

Trade-off: a cloud file that is modified often is downloaded again on each
reopen, as unmodified cloud files already are.

## Capacity accounting

- There is no space reservation. Writes, truncates, and downloads proceed
  without checking size. `max-size-mb` is not a hard limit; it is only the
  basis for `high-threshold` and `low-threshold`. Usage can temporarily exceed
  it while background eviction catches up.
- The only hard limit is the free space of the local filesystem. When it is
  full, the filesystem's `ENOSPC` is passed up the call stack unchanged. This
  can happen even though older files could have been evicted. This is a known
  limitation, documented with the thresholds in `setup/baseConfig.yaml` and
  `setup/advancedConfig.yaml`. The application may retry once background
  eviction has freed space.
- Usage is tracked incrementally after each write, truncate, download, and
  delete, and periodically reconciled by measuring `<tiered_root>`, which
  includes both `local/` and `cache/`. Only the eviction policy reads it.

## Locking

- Per-object file locks serialize open, release, promotion, delete, rename, and
  eviction for that object.
- `evictMu` serializes all eviction: threshold passes and evict-on-close
  goroutines. File operations never take it, and never block on eviction.
  `ReleaseFile` starts an evict-on-close goroutine but does not wait for it.
- `cloudStale` is only read or changed with the file lock held.
- Lock order: `evictMu`, then file lock(s), then the LRU queue mutex. Never take
  a file lock while holding the LRU queue mutex.
- Multi-object operations take file locks in lexical order.
- `GetAttr` and `StreamDir` do not take file locks. They rely on atomic renames
  and on the connectivity pre-check to keep mixed-state windows short.

## Windows semantics

`common.OpenFile` opens files on Windows with
`FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE`. This gives Linux-like
behavior and allows a single code path:

- **Promotion** renames an open file from `cache/` to `local/`; handles stay
  valid.
- **Delete while open** removes the file immediately; handles stay valid. No
  delete-on-last-close tracking is needed.
- **`replaceFile` is unnecessary.** Go's `os.Rename` on Windows already replaces
  an existing destination. Use `os.Rename` directly.

Limits:

- Only handles opened through `common.OpenFile` have `FILE_SHARE_DELETE`. Always
  use it for files under the tiered root (not `os.Open` or `os.CreateTemp`).
  Other processes can still hold blocking handles, so delete/rename failures
  during eviction and cleanup must be treated as retryable.
- Directory renames fail while any file beneath is open (handled by
  local-first ordering in `RenameDir`).
- On older Windows delete semantics, a deleted-but-open file keeps its name
  reserved until the last handle closes, blocking re-creation and parent
  directory removal. Verify on target Windows versions.

## Crash recovery

No recovery bookkeeping exists. Startup clears `cache/` and treats all of
`local/` as authoritative.

| Crash point | Outcome after restart |
|---|---|
| During download into `cache/` | Partial file deleted; cloud remains authoritative |
| While a cloud file is open, unmodified | Cache copy deleted; cloud remains authoritative |
| After promotion, before the write | Unchanged file in `local/`; one redundant upload on eviction |
| After an acknowledged write | Data is in `local/`; preserved |
| Before or during evict on close | `cloudStale` is lost; the file stays local-owned and the cloud copy stays stale until normal eviction |
| During eviction upload | Local copy still authoritative; eviction retries |
| After upload, before local delete | Redundant re-upload on next eviction |
| During delete, after cloud delete | Local copy remains; user-visible as existing; delete can be retried |
| During file/directory rename, between local and cloud steps | Local and cloud names diverge; accepted as rare |

## Required tests

- Startup clears `cache/`, recovers `local/` into the LRU by mtime, and fails on
  unexpected root entries.
- Open of a cloud file downloads into `cache/`; last close deletes it.
- First write to a cached file promotes it to `local/` while handles remain
  valid.
- `O_TRUNC` open of a cloud file creates an empty local-owned file without
  downloading.
- Delete and rename fail early when the cloud is disconnected, with no local
  change.
- `RenameFile` and `RenameDir` revert local renames when the cloud rename fails.
- `RenameDir` collects affected names after taking locks.
- Eviction skips open files, keeps files on upload failure, and deletes them on
  success.
- `cloudStale` is set by promotion and by `O_TRUNC` of a cloud file, is not
  set by create, and is unset after a restart.
- Last close of a `cloudStale` file evicts it (upload, then delete `local/x`)
  regardless of thresholds. It is not evicted if it was reopened, deleted, or
  renamed first, or if `evict-on-close` is false.
- Last close of a locally created file does not evict it.
- A failed evict on close leaves the file in `local/` with `cloudStale` set,
  and does not retry until the next close or normal eviction.
- Writes and downloads are not size-checked: usage may exceed `max-size-mb`,
  and an `ENOSPC` from the local filesystem is returned to the caller
  unchanged (for downloads, after deleting the partial `cache/x`).
- Windows: renaming a directory containing a file opened via `common.OpenFile`
  fails cleanly, before any change, and is reported as `EBUSY`.
- Windows: promotion and delete of open files succeed with handles remaining
  usable.

## Code removed by this design

- State snapshot (`persistence.go` snapshot read/write, LRU order persistence).
- Partial-download suffix and the interim `.cloudfuse-downloads` staging
  directory.
- `replaceFile`.
- Synchronous eviction and space reservation (`EvictNow`, `reserveSpace`, and
  their call sites in `OpenFile`, `WriteFile`, and `TruncateFile`).
- `FileNode` flags: `cloudBacked`, `isDirty`, `modeDirty`, `ownerDirty`, and
  cached `mode`/`owner`/`group`. These are replaced by the single in-memory
  `cloudStale` flag.
- Name filtering of internal files in `StreamDir` and reserved-name checks in
  `localPath`.

## Backlog - do not implement

Ideas deferred to keep the current design simple. They are recorded here so
they are not lost. Do not implement any of them until they are moved into the
design above.

- **Hard limit.** An option (like `file_cache`'s `hard-limit`) that enforces
  `max-size-mb`. Writes, truncates, and downloads that would exceed it fail
  with `ENOSPC` before touching the local filesystem, instead of relying on the
  filesystem running out of space.
