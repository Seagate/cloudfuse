# FUSE request behavior in cloudfuse

This document describes the request patterns cloudfuse can expect from its FUSE
hosts and the requirements those patterns place on the implementation. It is
intended as a reference for design, debugging, and code review.

The current stack is:

- Linux: the Linux VFS and FUSE kernel driver, with cgofuse adapting requests to
  `CgofuseFS`
- Windows: WinFSP's FUSE compatibility layer, with cgofuse adapting requests to
  the same `CgofuseFS` interface
- cloudfuse: the component named `libfuse` (implemented with cgofuse), handle
  map, optional attribute cache, and storage components

The relevant contract is therefore the high-level callback interface exposed by
cgofuse. Lower-level FUSE protocol details are omitted unless they explain an
observable callback pattern.

## Reading the tables

The expectation column uses four terms:

- **Always**: required by the current host contract during a normal mounted
  session.
- **Usually**: expected in ordinary application workloads, but caching, flags,
  or a different syscall sequence can change it.
- **May**: valid behavior that handlers must tolerate.
- **Never**: excluded by the current contract during a normal session. Forced
  unmount, process termination, and host failure are called out separately.

The provider column identifies where the behavior comes from. If that layer is
replaced or reconfigured, the expectation must be checked again.

## Request lifecycle at a glance

| Area | Expectation | Provider | Requirement for cloudfuse |
|---|---|---|---|
| Initialization | `Init` runs before normal filesystem callbacks. | cgofuse host | Global callback state may be initialized there. |
| Clean shutdown | `Destroy` runs after request processing has stopped. | cgofuse host | It may release process-wide resources, but must not be the only cleanup path. |
| Forced shutdown | `Destroy`, `Release`, and `Releasedir` may be absent. | OS and host failure behavior | Correctness must not depend on callbacks after process termination or forced unmount. |
| Successful file open | `Open` or `Create` returns one handle. Reads, writes, flushes, and release for that opening use that handle. | cgofuse high-level interface | Store all state needed after namespace changes on the handle. |
| Final file close | One `Release` follows each successful `Open` or `Create` in a normal session. | libfuse on Linux; WinFSP close mapping on Windows; exposed by cgofuse | Attempt backend close, then remove local handle state regardless of the backend result. |
| Successful directory open | One `Releasedir` follows each successful `Opendir` in a normal session. | libfuse/WinFSP through cgofuse | Directory listing state may be scoped to the directory handle and freed in `Releasedir`. |
| Path metadata | `Getattr` is common during path resolution, but need not immediately precede another callback. | VFS/WinFSP caches and cgofuse | No operation may require a recent `Getattr` callback for correctness. |
| Concurrency | Callbacks for different operations, and sometimes the same path, may overlap. | Multithreaded host loop | Protect shared state explicitly; callback order is not a lock. |

### Typical traces, not callback contracts

The following sequences are useful when reading logs, but each can be shortened,
repeated, or interleaved with another operation:

| Application activity | Typical callbacks | Common variations |
|---|---|---|
| Inspect a path | `Getattr` | No callback on a cache hit; repeated `Getattr` after expiration or during Windows metadata queries |
| Open and read a file | `Getattr`, `Open`, one or more `Read`, `Flush`, `Release` | `Getattr` may be absent; reads may be served by the kernel cache; `Flush` may be absent or repeated |
| Create, write, and close a file | `Create`, one or more `Write`, `Flush`, `Release` | Writes may be split, combined, or delayed; `Fsync` appears only when requested by the application |
| List a directory | `Opendir`, one or more `Readdir`, `Releasedir` | Enumeration may restart at offset zero; attributes returned by readdir-plus may avoid per-entry `Getattr` |
| Rename a path | Metadata queries followed by `Rename` | Cached metadata can remove the queries; a no-op rename may be resolved before `Rename` reaches cloudfuse |
| Remove an open file | `Unlink`, continued handle operations, then `Release` | The interval can be long; Windows behavior depends on WinFSP POSIX unlink support |

### What should not happen in a normal session

Subject to a correctly functioning host and a mount that is not forcibly
terminated:

- `Release` is not issued more than once for one successful `Open` or `Create`.
- `Releasedir` is not issued more than once for one successful `Opendir`.
- A Linux application does not receive an error returned by final `Release`.
- `Flush` does not establish that the handle has no remaining references.
- After POSIX unlink succeeds, the removed name does not continue to identify
  the unlinked object, although another object may subsequently be created at
  the same name.

## Metadata and path resolution

### What usually happens

Applications perform many operations that require the host to resolve a path or
inspect its type, permissions, size, or timestamps. This commonly produces
`Getattr` calls near `Open`, `Rename`, directory traversal, and existence
checks. Windows tends to request more metadata than Linux because file opening,
security checks, and directory enumeration need Windows file information.

These calls are frequent enough to matter for performance, but their exact
placement is not part of the cloudfuse contract.

### What may happen

- A cached positive entry may let the host proceed without calling `Getattr`.
- A cached negative entry may reject a path without calling cloudfuse again.
- A file-handle operation may proceed using state established by `Open` or
  `Create`.
- `Readdir` with attributes can avoid a separate `Getattr` for each entry.
  cloudfuse enables cgofuse's readdir-plus capability.
- An expired kernel entry can cause another `Getattr`, while the pipeline
  attribute cache still returns its cached value.

### What cloudfuse must provide

`Getattr` is a query, not a prerequisite or synchronization point. `Open`,
`Rename`, `Unlink`, and other operations must validate the state they actually
need or rely on an atomic backend operation. They must not assume that a
previous `Getattr` result is current.

When a callback includes a valid file handle, handle state is generally a
better identity than the path. The path may have been renamed or unlinked since
the handle was opened.

## Metadata cache layers

Metadata can be cached in two places:

1. The host cache, controlled by `attribute-expiration-sec`,
   `entry-expiration-sec`, and `negative-entry-expiration-sec`.
2. The optional cloudfuse `attr_cache` component, controlled by
   `attr_cache.timeout-sec` and whether attributes are cached during listing.

These timeouts form one effective cache policy:

- If a host timeout is shorter than the pipeline attribute-cache timeout, the
  host calls cloudfuse again but may receive the same cached metadata. This adds
  callback and pipeline overhead without improving freshness.
- If a host timeout is longer than the pipeline attribute-cache timeout, the
  host may continue using metadata after the pipeline entry would have expired.
  The shorter pipeline timeout does not help until the host asks again.
- Consequently, the observable staleness for changes made outside cloudfuse is
  approximately the longest cache interval that can answer the request.

For a predictable configuration, keep corresponding host and pipeline timeouts
close to one another. A host timeout at least as long as the pipeline timeout
avoids revalidation that cannot produce fresher data; equal values make the
freshness and performance tradeoff easiest to explain.

Negative caching deserves separate attention. A long
`negative-entry-expiration-sec` delays the visibility of files created by
another client, even if positive attribute caching is short.

When direct I/O is enabled, cloudfuse sets the host attribute, entry, and
negative-entry timeouts to zero. A nonzero `attr_cache.timeout-sec` can still
return cached metadata, so direct I/O alone does not imply fresh metadata.

### Recommended operating boundary

Long metadata timeouts are appropriate when cloudfuse is the only writer, or
when applications accept delayed visibility of external changes. Workloads
that require prompt visibility of changes made through another client should
use short, aligned host and pipeline timeouts. This is a configuration boundary
rather than a case every filesystem operation can repair locally.

## Open, create, read, and write

| Behavior | Expectation | Provider | Notes |
|---|---|---|---|
| `Create` returns an opened handle | Always after successful `Create`. | cgofuse interface | The handle participates in the same lifecycle as one returned by `Open`. |
| Reads and writes use a successful handle | Always with the current implementation. | cgofuse and cloudfuse | Missing handles are internal errors or stale requests and return `EBADF`. |
| Read size equals the requested size | Usually, except at EOF or on error. | FUSE read contract | With buffered I/O, the kernel may fill or cache data around application requests. |
| Write size equals the requested size | Usually on success. | FUSE write contract | Short writes must be reported accurately; success must not claim bytes that were not accepted. |
| A write is followed by `Flush` | Often, but not always. | Application close sequence and host | Persistence and cleanup cannot depend on it. |
| A write is followed by `Fsync` | Only when the application requests synchronization. | Application | Applications that do not call `fsync` have not requested a durability boundary. |

With writeback caching, Linux may issue reads on a handle originally opened
write-only and may transform write patterns. Handlers should use the callback
arguments and handle state rather than reconstructing the application's
original syscall sequence.

## Flush, fsync, and release

These callbacks have distinct purposes.

### `Flush`

`Flush` is associated with closing a file descriptor, not with the lifetime of
the underlying open handle. Duplicating a descriptor with `dup`, inheriting it
across `fork`, or host-specific close handling can produce multiple `Flush`
calls for one opening. Some paths produce no `Flush`.

Therefore:

- `Flush` must be safe to call more than once.
- `Flush` may report pending write errors.
- `Flush` must not free the handle.
- `Flush` must not be the only path that uploads or preserves required state.
- A `Flush` failure does not cancel the later `Release`.

### `Fsync`

`Fsync` is the explicit durability request. A nonzero `datasync` value allows
the implementation to synchronize file data without all metadata. If cloudfuse
cannot provide the requested durability through the configured backend, that
limitation should be documented rather than approximated with close behavior.

### `Release`

`Release` ends the open-handle lifetime. During a normal mount, it occurs once
for each successful `Open` or `Create`.

Linux does not return a `Release` error to the `close` or `munmap` operation
that caused the final release. On Windows, WinFSP's close path similarly uses
release for final FUSE-handle disposal. A failed backend release therefore does
not create another opportunity to release the same local handle.

The handler must:

1. find and detach the handle from the handle map;
2. attempt backend release and log or return its error as appropriate;
3. clean local resources and accounting regardless of that error.

This is a cloudfuse requirement, not merely a common request pattern.

## Rename

### Application-visible requirements

POSIX rename has several important properties:

- replacing an existing destination is atomic;
- after success, the old name is gone and the new name identifies the object;
- renaming a path to the same object succeeds without changing it;
- open handles continue to refer to the opened object across a rename.

The Linux VFS may resolve some no-op cases without sending a `Rename` callback.
The Windows path through WinFSP may produce a different callback sequence.
Handling an identical source and destination as a successful no-op remains
inexpensive and makes the high-level handler independent of that distinction.

### What cloudfuse can and cannot infer

A preceding `Getattr` is common but does not make a later rename atomic. The
atomicity must come from the storage component or from a cloudfuse transaction
that preserves the visible rename contract. Checks performed before a
non-atomic copy/delete sequence are vulnerable to concurrent changes.

cgofuse itself does not serialize rename against other operations. The host
below it does, for the paths a rename touches, as described next. Ordering
between unrelated paths, and between a check and a later mutation inside one
callback, still needs cloudfuse locks or atomic backend primitives.

### Serialization provided by the host

Callbacks are generally concurrent, but a rename, unlink, or rmdir excludes
other callbacks on the paths it changes for as long as the callback runs.
This comes from the host layer, not from cgofuse or `CgofuseFS`.

| Host | Mechanism | Effect for cloudfuse |
|---|---|---|
| Linux, libfuse high-level API | libfuse keeps a lock per in-memory path node. `Rename`, `Unlink`, and `Rmdir` write-lock the node they change; `Rename` also write-locks an existing destination node. Every other callback read-locks its target node and each ancestor. A callback that meets a write-locked node is queued until it is released, and a writer waits for readers already in progress. | While `Rename` of a directory runs, nothing can create, open, look up, or change a path beneath the source or destination. Callbacks on open handles beneath it (`Read`, `Write`, `Flush`, `Release`) also wait, because cgofuse does not enable `nullpath_ok`, so libfuse resolves and locks a path for handle operations too. Parent directories are only read-locked, so listing a parent can overlap the rename. |
| Windows, WinFSP | The volume has a rename resource. A rename acquires it exclusively; every create or open acquires it shared. | While `Rename` runs, no file can be created or opened anywhere on the volume. This resource does not cover reads and writes on handles that are already open. |

Consequences:

- A storage component may list the contents of a directory inside `RenameDir`
  and rely on no new name appearing beneath it before the callback returns.
  `tiered_storage` relies on this instead of holding its own lock.
- The guarantee depends on configuration. Switching to libfuse's low-level
  API, or enabling `nullpath_ok`, removes it on Linux. A component that relies
  on it must then provide its own lock.
- The lock covers only the duration of the callback. Checks a component makes
  inside the callback are safe from other FUSE callbacks, but not from
  background work inside cloudfuse, such as eviction or upload goroutines,
  which needs cloudfuse locks.

## Unlink while open

### Linux

After a successful unlink, the name no longer resolves, but existing handles
continue to access the opened file until their own lifetimes end. The final
`Release` may occur much later than `Unlink`. The handle must therefore contain
enough state to operate without resolving the original path again.

cloudfuse does not enable libfuse's `hard_remove` option. In that
configuration, libfuse handles an unlink of an open file, or a rename over an
open destination, by renaming the file to a hidden `.fuse_hidden…` name with the
filesystem's `Rename` callback. It removes that name with `Unlink` after the
last `Release`. Components therefore see a `Rename` to a hidden name, not an
`Unlink`, for files that are still open.

### Windows

Current WinFSP FUSE mode enables POSIX unlink/rename support by default. In that
mode, WinFSP can remove the name while other handles remain open, and those
handles continue to operate. The legacy `LegacyUnlinkRename` option disables
that behavior and restores Windows-style delayed deletion.

cloudfuse does not currently expose legacy delete behavior as a supported
configuration. If that option or the WinFSP capability changes, unlink,
rename, and open-handle tests must be rerun because the guarantee is provided
by WinFSP, not by `CgofuseFS`.

WinFSP's `.fuse_hidden...` names are an implementation detail used for selected
FUSE compatibility operations such as replacing reparse points. They are not
the general contract for unlinking an open file.

### Requirement for storage components

An open handle must remain usable after its path is unlinked. A storage
component may implement this with a local file, an immutable object identity,
deferred deletion, or another mechanism. Re-resolving the deleted path on each
read or write is not sufficient.

## Directory enumeration

`Opendir` establishes a directory handle, `Readdir` may be called repeatedly
with continuation offsets, and `Releasedir` ends that handle's lifetime.

Usually, an application reads a directory forward until the host's buffer is
full and then resumes from the supplied offset. It may also stop early, restart
at offset zero, or issue another enumeration while entries are changing.

The listing may include entries that were removed after enumeration began and
may omit entries created afterward. It must not skip or duplicate unrelated
entries merely because the directory changed. An integer position is safe only
when the directory handle refers to a stable snapshot or the backend
continuation token provides equivalent stability.

With readdir-plus enabled, attributes returned with directory entries reduce
follow-up `Getattr` calls. This is an optimization, not a requirement that every
enumeration include complete attributes.

## Concurrency and ordering

The host runs callbacks concurrently unless explicitly configured otherwise.
The following are all possible:

- reads on one handle overlapping;
- a read or write overlapping `Getattr`;
- operations on different handles for the same path;
- rename or unlink racing with metadata queries on other paths (callbacks on
  the paths being changed are excluded by the host; see
  [Serialization provided by the host](#serialization-provided-by-the-host));
- directory enumeration overlapping namespace changes.

Sequential syscalls from one thread often produce an intuitive callback order,
which is useful when reading traces, but that order is not a cross-request
synchronization mechanism. Shared maps, dirty state, cached listings, and
rename/unlink coordination need explicit locking or immutable state.

## Errors and uncommon termination paths

Handlers should return the documented errno for the application operation when
possible. Unknown backend errors map to `EIO`; permission, existence, type, and
path errors should retain their specific meaning.

Some lifecycle errors cannot be delivered usefully:

- Linux ignores errors returned from final `Release`.
- WinFSP cleanup has no failure result at the Windows API boundary.
- A killed process or forced unmount may skip all remaining cleanup callbacks.

These cases justify local cleanup that is unconditional and idempotent. They do
not justify silently discarding errors from operations whose callers can act on
them, such as `Open`, `Write`, `Flush`, `Fsync`, `Rename`, or `Unlink`.

## Guarantees provided by the cloudfuse handler

The preceding sections describe behavior supplied by the operating system,
cgofuse, and WinFSP. This section describes the additional behavior currently
provided by cloudfuse's [`CgofuseFS`
handler](../component/libfuse/libfuse2_handler.go).

These are interface-level guarantees only. When the handler delegates an
operation to the next pipeline component, the final storage semantics still
depend on that component.

### Common path, attribute, and error handling

| Handler behavior | Current guarantee |
|---|---|
| Path normalization | Implemented namespace callbacks remove the leading FUSE slash, normalize the object name, reject paths longer than 4096 bytes, and reject path components longer than 255 bytes. |
| Error translation | Recognized Go filesystem and syscall errors are translated to specific FUSE errors. Unrecognized errors become `EIO`. |
| Root attributes | `Getattr` synthesizes the root as a directory owned by the configured uid and gid, with mode `0777`, two links, and size 4096. |
| Object attributes | Returned uid and gid come from mount configuration. File type and permission bits come from `ObjAttr`; configured defaults are used when the mode is marked default. Directories report two links and size 4096. |
| Inode values | Inodes are deterministic hashes of object paths. This keeps values stable for one path but does not preserve an inode across rename. |
| Filesystem statistics | `Statfs` uses storage-provided values when available. Otherwise it reports configured display capacity and synthetic file-count values. |
| Metrics | Successful create, delete, rename, truncate, link, sync, and chmod operations update the corresponding libfuse statistics. Failed operations do not. |

Path normalization is applied before namespace operations are sent down the
pipeline. Handle-based `Read`, `Write`, `Flush`, `Fsync`, and `Release` instead
use the path and identity stored on the handle; they do not re-resolve the path
argument supplied with the callback.

### File handles and I/O

| Handler | Current guarantee |
|---|---|
| `Create` | Delegates creation, adds the returned handle to the global handle map, increments the open-handle count, and returns that handle to cgofuse. |
| `Open` | Delegates opening with the received flags and configured file mode, then records and counts the returned handle. |
| `Read` | Requires a known handle or returns `EBADF`. Cached handles use positional `ReadAt`; other handles delegate to `ReadInBuffer`. EOF is returned as a successful short read. |
| `Write` | Requires a known handle or returns `EBADF`, then delegates the supplied buffer and offset. It returns the exact byte count reported by the next component. |
| `Flush` | Requires a known handle. A clean handle succeeds without calling the next component; a dirty handle delegates to `FlushFile`. It does not remove the handle. |
| `Truncate` | Passes both the normalized path and an optional matching handle to the next component. It does not require a valid handle for path-based truncation. |
| `Fsync` | Requires a nonzero, known handle and delegates to `SyncFile`. |
| `Release` | Requires a known handle and calls `ReleaseFile`. On success it removes the handle and decrements the open-handle count. |

The handler therefore guarantees that normal read and write callbacks use a
cloudfuse handle rather than resolving the callback path again. Whether that
handle remains usable after rename or unlink depends on the components that
created and subsequently operate on it.

### Directories

| Handler | Current guarantee |
|---|---|
| `Opendir` | Creates a directory handle and per-handle listing cache after path validation. It appends a trailing slash to non-root directory paths. |
| `Readdir` | Requires an existing directory handle and cache. Offset zero resets the cache with `.` and `..`; later calls resume through cached entries and backend continuation tokens. Attributes are supplied with every returned entry. |
| Listing memory | The per-handle cache is replaced when extending it would exceed `MaxDirListCount`, while offsets continue from the requested position. |
| `Releasedir` | Atomically removes the handle from the handle map and calls its local cleanup. A repeated or unknown release returns `EBADF`. |
| `Rmdir` | Checks `IsDirEmpty` before delegating to `DeleteDir`; a nonempty result becomes `ENOTEMPTY`. |

### Namespace and metadata operations

| Handler | Current guarantee |
|---|---|
| `Mkdir` | Preserves permission and special mode bits when delegating. On Windows it performs an additional destination existence check so an existing name returns `EEXIST`. |
| `Unlink` | Normalizes the path and delegates to `DeleteFile`. It does not alter open handles itself. |
| `Rename` | Rejects an empty source or destination, treats identical normalized paths as a successful no-op, and reads source attributes to choose file or directory handling. |
| File rename | Delegates directly to `RenameFile`. If that fails, it queries the destination to refine directory and permission errors. |
| Directory rename | Rejects an existing nondirectory destination and a nonempty destination directory before delegating to `RenameDir`. |
| `Symlink` | Normalizes the link name and target before delegating to `CreateLink`. |
| `Readlink` | Makes a best-effort attribute query for link size, then delegates to `ReadLink`; failure of the preliminary query does not prevent the read. |
| `Chmod` | Delegates mode changes on Linux. On Windows it reports success without persisting the mode because WinFSP synthesizes it from Windows attributes. |
| `Chown` | Reports success without changing stored ownership. |
| `Utimens` | Reports success without changing stored timestamps, allowing common `touch` workflows to proceed. |
| Unsupported operations | Access checks, hard links, extended attributes, and `Mknod` return `ENOSYS`. |

The handler performs useful type and existence checks, but successful storage
semantics remain the responsibility of the called component. In particular,
the handler alone does not make replace-rename atomic, keep unlinked content
alive, or serialize operations on the same path.

### Areas for future inquiry

The following behaviors are visible at the handler boundary and deserve
focused investigation. They are not claims that the current implementation is
incorrect for existing workloads.

- **Release after backend failure.** `Release` currently leaves the handle and
  open-handle count in place when `ReleaseFile` returns an error. The host does
  not provide a second final-release callback, so local cleanup probably needs
  to occur regardless of that result.
- **Directory-open validation.** `Opendir` creates a handle without consulting
  the next component. A missing path or nondirectory may therefore fail during
  `Readdir` instead of `Opendir`.
- **Handle-aware attributes.** `Getattr` currently ignores the supplied file
  handle and always queries by path. Attribute requests for an open file after
  rename or unlink may need to use handle state instead.
- **Create flags.** `Create` receives open flags but does not pass them to
  `CreateFile`, whereas `Open` passes its flags to `OpenFile`. Confirm which
  create-time flags cgofuse has already handled and which lower components need
  to see.
- **Directory removal races.** `Rmdir` checks emptiness separately from
  `DeleteDir`, and directory rename separately checks the destination before
  `RenameDir`. Their final correctness depends on the storage component
  detecting changes between the check and mutation.
- **Directory-check errors.** `IsDirEmpty` returns only a boolean at this
  boundary. A failure to inspect a directory may be indistinguishable from a
  nonempty directory and surface as `ENOTEMPTY`.
- **Concurrent `Readdir` on one handle.** The listing cache is mutable and is
  not held under the handle lock while it is populated and served. Confirm
  whether cgofuse can issue overlapping enumeration calls for one directory
  handle; otherwise add synchronization.
- **Stable directory cookies.** The handler exposes numerical offsets over
  paged results. Confirm that every `StreamDir` implementation provides stable
  continuation semantics under concurrent directory changes.
- **`datasync` semantics.** `Fsync` and `Fsyncdir` currently ignore the
  `datasync` argument. Determine whether any component can or should distinguish
  data-only synchronization from full metadata synchronization.
- **Successful no-ops.** `Chown` and `Utimens`, plus `Chmod` on Windows, report
  success without persisting the requested metadata. This is convenient for
  compatibility but should be an explicit supported limitation.
- **Symlink target normalization.** The handler normalizes the target as an
  object name. Confirm that relative targets, absolute targets, `..`, and
  platform-specific separators retain the intended symlink meaning.
- **Rename identity.** Inodes are derived from paths, so a rename changes the
  inode reported by later `Getattr` calls. Confirm whether target applications
  require stable inode identity across rename.
- **Unlink of open files on Linux.** Because `hard_remove` is not enabled,
  libfuse turns an unlink of an open file into a `Rename` to a `.fuse_hidden…`
  name, followed by an `Unlink` after the final `Release`. Storage components
  may perform a cloud rename to that hidden name, and a crash before release
  can leave it behind. Decide whether to enable `hard_remove`, which relies on
  components keeping unlinked files usable, or to handle hidden names
  explicitly. This was found by reading libfuse source and has not yet been
  observed in a trace.
- **Specific errno preservation.** Errors outside the current translation table
  collapse to `EIO`. Add mappings when applications need to distinguish another
  documented filesystem error.

## Configuration expectations

The following configurations match the assumptions in this document:

- cgofuse is the only FUSE adapter used by cloudfuse;
- cgofuse uses libfuse's high-level path API on Linux, without `nullpath_ok`
  or `hard_remove`;
- WinFSP uses its default POSIX unlink/rename support;
- readdir-plus remains enabled;
- host and `attr_cache` metadata timeouts are chosen as one cache policy;
- applications accept the configured delay for changes made outside
  cloudfuse;
- direct I/O configurations also consider the pipeline attribute cache when
  metadata freshness matters.

Changing one of these is possible, but it changes request likelihoods or
semantics and should be accompanied by focused mount tests.

## Review checklist

When changing a filesystem operation, check:

1. Can the callback arrive without a recent `Getattr`?
2. Does correctness depend on a cache timeout or on external writers being
   absent?
3. Does the operation still work after the path has been renamed or unlinked?
4. Are repeated `Flush`, `Readdir`, or metadata callbacks harmless?
5. Are local handles and counters cleaned up after backend failure?
6. Is shared state protected against overlapping callbacks?
7. Is the behavior provided by cloudfuse, cgofuse, the Linux VFS, or WinFSP?
8. If it depends on configuration, is that configuration part of the supported
   operating boundary?

## Sources

- [cgofuse filesystem interface](https://github.com/winfsp/cgofuse/blob/v1.6.0/fuse/fsop.go)
- [libfuse high-level operations](https://libfuse.github.io/doxygen/structfuse__operations.html)
- [libfuse high-level implementation, including path locking and hidden files](https://github.com/libfuse/libfuse/blob/master/lib/fuse.c)
- [WinFSP rename locking: create](https://github.com/winfsp/winfsp/blob/master/src/sys/create.c) and [set-information](https://github.com/winfsp/winfsp/blob/master/src/sys/fileinfo.c)
- [Linux `rename(2)`](https://man7.org/linux/man-pages/man2/rename.2.html)
- [Linux `unlink(2)`](https://man7.org/linux/man-pages/man2/unlink.2.html)
- [WinFSP native API and FUSE comparison](https://github.com/winfsp/winfsp/wiki/Native-API-vs-FUSE)
- [WinFSP delete redesign](https://github.com/winfsp/winfsp/blob/master/doc/archive/WinFsp-Delete-Redesign.asciidoc)
- [WinFSP FUSE implementation](https://github.com/winfsp/winfsp/tree/master/src/dll/fuse)
- [cloudfuse FUSE configuration](../component/libfuse/libfuse.go)
- [cloudfuse attribute cache](../component/attr_cache/attr_cache.go)
