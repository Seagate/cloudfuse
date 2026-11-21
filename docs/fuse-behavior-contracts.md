# FUSE request behavior in cloudfuse

This document describes the callback patterns cloudfuse can expect from its
FUSE hosts and what those patterns require of the implementation. It is a
reference for design, debugging, and code review.

On Linux, requests come from the kernel FUSE driver through libfuse; on
Windows, they come from WinFSP's FUSE layer. In both cases cgofuse adapts them
to the same `CgofuseFS` callbacks, so this document is written in terms of the
[cgofuse interface](https://github.com/winfsp/cgofuse/blob/v1.6.0/fuse/fsop.go).

## Request lifecycle

| Area | Expectation | Requirement for cloudfuse |
|---|---|---|
| File handles | `Open` or `Create` returns a handle that later reads, writes, flushes, and release use. | Keep any state needed after a rename or unlink on the handle. |
| Final close | Exactly one `Release` follows each successful `Open` or `Create`. | Attempt backend close, then remove local handle state regardless of the result. |
| Directory handles | Exactly one `Releasedir` follows each successful `Opendir`. | Listing state may live on the directory handle. |
| Metadata | `Getattr` is common but need not precede any other callback. | No operation may rely on a recent `Getattr`. |
| Concurrency | Callbacks may overlap, including on the same path. | Protect shared state explicitly; callback order is not a lock. |
| Forced shutdown | A killed process or forced unmount may skip `Release`, `Releasedir`, and `Destroy`. | Correctness must not depend on those callbacks running. |

### Typical traces

These are useful when reading logs, but any of them can be shortened, repeated,
or interleaved with other operations.

| Application activity | Typical callbacks | Common variations |
|---|---|---|
| Inspect a path | `Getattr` | Skipped on a cache hit; repeated on Windows |
| Read a file | `Getattr`, `Open`, `Read`…, `Flush`, `Release` | Reads may be served from the kernel cache; `Flush` may be absent or repeated |
| Write a file | `Create`, `Write`…, `Flush`, `Release` | Writes may be split or combined; `Fsync` only if the application asks |
| List a directory | `Opendir`, `Readdir`…, `Releasedir` | Enumeration may restart at offset zero |
| Rename | `Getattr`…, `Rename` | Some no-op renames never reach cloudfuse |
| Remove an open file | `Unlink` (or `Rename`, see [below](#unlink-while-open)), more handle I/O, `Release` | The file may stay open for a long time |

## Metadata and caching

`Getattr` is a query, not a prerequisite or a synchronization point. The host
may skip it on a cache hit, reject a path from its negative cache, or get
attributes from readdir-plus (which cloudfuse enables). `Open`, `Rename`,
`Unlink`, and similar operations must validate what they need themselves.
When a callback has a valid file handle, prefer the handle over the path.

Metadata is cached in two places that act as one policy:

1. the host cache (`attribute-expiration-sec`, `entry-expiration-sec`,
   `negative-entry-expiration-sec`), and
2. the optional `attr_cache` component (`attr_cache.timeout-sec`).

The staleness of changes made outside cloudfuse is roughly the longer of the
two. A shorter host timeout just adds callbacks that return the same cached
data, so keep the two close together. A long negative-entry timeout delays
visibility of files created by other clients. Direct I/O zeroes the host
timeouts but not `attr_cache`, so it does not guarantee fresh metadata on its
own.

Long timeouts are fine when cloudfuse is the only writer. Workloads that need
to see external changes promptly should use short, aligned timeouts.

## Reads and writes

- Reads and writes always arrive with a handle; an unknown handle returns
  `EBADF`.
- A successful write must report exactly the bytes accepted.
- `Flush` often follows a write, but not always. `Fsync` appears only when the
  application requests it.
- With writeback caching, Linux may read from a write-only handle and reshape
  writes. Use the callback arguments, not assumptions about the original
  syscalls.

## Flush, fsync, and release

**`Flush`** runs when a file descriptor closes, not when the handle ends. `dup`
and `fork` can cause several flushes per open, and some closes cause none. It
must be safe to repeat, may report pending write errors, and must not free the
handle or be the only path that persists data.

**`Fsync`** is the explicit durability request. If a backend cannot provide it,
document the limitation rather than approximating it.

**`Release`** ends the handle and happens once per open. Neither Linux nor
WinFSP delivers a `Release` error to the application, and there is no retry. So
the handler must detach the handle, attempt the backend release, and clean up
local state regardless of the outcome.

## Rename

POSIX rename replaces the destination atomically, removes the old name, treats a
rename onto itself as a no-op, and leaves open handles pointing at the renamed
object. cloudfuse handles identical paths as a no-op itself so that it does not
depend on the host filtering them out.

A preceding `Getattr` does not make the rename atomic. Atomicity must come from
the storage component or a cloudfuse transaction.

The host does give some serialization. While `Rename`, `Unlink`, or `Rmdir`
runs, the host holds back other callbacks on the affected paths (Linux) or all
creates and opens on the volume (Windows). So a component may list a directory
inside `RenameDir` and rely on nothing new appearing beneath it;
`tiered_storage` relies on this. The lock lasts only for the callback and does
not cover cloudfuse's own background work, such as eviction or upload
goroutines. It also depends on configuration. See
[Appendix A](#appendix-a-host-serialization-during-rename) for details.

## Unlink while open

An open handle must stay usable after its path is unlinked. A component can do
this with a local file, deferred deletion, or a stable object identity.
Resolving the deleted path again on each read is not enough.

**Linux:** cloudfuse does not enable libfuse's `hard_remove`. As a result,
libfuse turns an unlink of an open file, or a rename over one, into a `Rename`
to a hidden `.fuse_hidden…` name. It then calls `Unlink` on that name after the
last `Release`. Components therefore see a rename, not an unlink, and the hidden
name shows up in listings until the file closes. See
[Appendix B](#appendix-b-observed-unlink-while-open-trace).

**Windows:** WinFSP's default POSIX unlink support removes the name while open
handles keep working. The `LegacyUnlinkRename` option would change this, and
cloudfuse does not support it.

## Directory enumeration

`Readdir` may be called repeatedly with continuation offsets, stopped early, or
restarted at offset zero. If the directory changes during enumeration, entries
may be missing or stale, but unrelated entries must not be skipped or
duplicated. Integer offsets are safe only over a stable snapshot or stable
backend continuation tokens.

## Concurrency

Callbacks run concurrently. Reads on one handle can overlap, I/O can overlap
`Getattr`, and listings can overlap namespace changes. Callbacks on paths being
renamed or removed are the exception (see [Rename](#rename)). Shared maps, dirty
state, and cached listings need locks or immutable state.

## Errors

Return the specific errno where possible; unknown backend errors become `EIO`.
Errors from final `Release` and from WinFSP cleanup never reach the
application, so local cleanup there must be unconditional and idempotent. That
does not justify dropping errors from `Open`, `Write`, `Flush`, `Fsync`,
`Rename`, or `Unlink`.

## Guarantees provided by the cloudfuse handler

The sections above describe host behavior. The
[`CgofuseFS` handler](../component/libfuse/libfuse2_handler.go) adds the
following. Final storage semantics still depend on the downstream components.

- **Paths:** Namespace callbacks strip the leading slash, normalize the name,
  and reject paths over 4096 bytes or components over 255 bytes. Handle-based
  callbacks use the handle's stored path instead of the callback's path.
- **Errors:** Known Go and syscall errors map to specific errnos; the rest
  become `EIO`.
- **Attributes:** uid and gid come from the mount configuration. Inodes are
  hashes of the path, so they change on rename.
- **I/O:** `Read`, `Write`, `Flush`, `Fsync`, and `Release` require a known
  handle or return `EBADF`. `Flush` skips clean handles. `Release` removes the
  handle only if `ReleaseFile` succeeds.
- **Directories:** `Opendir` creates a handle with a listing cache without
  checking the path. `Readdir` at offset zero resets the cache and returns
  attributes with every entry. `Rmdir` checks `IsDirEmpty` before `DeleteDir`.
- **Rename:** Identical paths are a no-op. The handler checks source attributes
  to choose file or directory rename. Directory rename rejects a nondirectory or
  nonempty destination first.
- **No-ops:** `Chown`, `Utimens`, and `Chmod` on Windows report success without
  persisting anything. Access checks, hard links, xattrs, and `Mknod` return
  `ENOSYS`.

The handler does not make replace-rename atomic, keep unlinked content alive,
or serialize operations on one path. Those are up to the storage components.

### Areas for future inquiry

These are open questions, not known bugs.

- **Release after backend failure:** `Release` leaves the handle and count in
  place if `ReleaseFile` fails, but no second release will come.
- **`Opendir` validation:** Missing or nondirectory paths fail at `Readdir`
  instead of `Opendir`.
- **Handle-aware `Getattr`:** `Getattr` ignores the handle, which may matter
  after rename or unlink.
- **Create flags:** `Create` does not pass its flags to `CreateFile`.
- **Check-then-act races:** `Rmdir` and directory rename check the destination
  separately from the mutation.
- **`IsDirEmpty` errors:** A failure to inspect a directory surfaces as
  `ENOTEMPTY`.
- **Concurrent `Readdir` on one handle:** The listing cache is not locked while
  it is populated and served.
- **Directory cookies:** Confirm that every `StreamDir` implementation
  continues stably under concurrent changes.
- **`datasync`:** `Fsync` and `Fsyncdir` ignore it.
- **Silent no-ops:** Decide whether `Chown`, `Utimens`, and Windows `Chmod`
  should be documented limitations.
- **Symlink targets:** Confirm that normalizing targets preserves relative,
  absolute, and `..` targets.
- **Inode stability:** Check whether any target application needs inodes to
  survive a rename.
- **Hidden files on Linux:** Components may perform a cloud rename to
  `.fuse_hidden…`, and a crash can leave it behind. Either enable `hard_remove`
  (and rely on components keeping unlinked files usable) or handle hidden names
  explicitly.
- **Errno coverage:** Add mappings when applications need errors that currently
  collapse to `EIO`.

## Configuration assumptions

This document assumes:

- cgofuse uses libfuse's high-level path API on Linux, without `nullpath_ok`
  or `hard_remove`;
- WinFSP uses its default POSIX unlink/rename support;
- readdir-plus is enabled;
- host and `attr_cache` timeouts are chosen together.

Changing any of these changes the behavior described above and should come with
focused mount tests.

## Review checklist

When changing a filesystem operation, check:

1. Does it work without a recent `Getattr`?
2. Does correctness depend on cache timeouts or the absence of external writers?
3. Does it still work after the path is renamed or unlinked?
4. Are repeated `Flush`, `Readdir`, and metadata calls harmless?
5. Are local handles and counters cleaned up after backend failure?
6. Is shared state protected against overlapping callbacks?

## Appendix A: Host serialization during rename

This comes from the host layer, not from cgofuse or `CgofuseFS`.

**Linux (libfuse high-level API).** libfuse keeps a lock for each path node.
`Rename`, `Unlink`, and `Rmdir` write-lock the node they change, and `Rename`
also locks an existing destination. Every other callback read-locks its target
and that target's ancestors. While a directory rename runs, nothing beneath the
source or destination can be looked up, created, or opened. Handle I/O beneath
it (`Read`, `Write`, `Flush`, `Release`) also waits, because cgofuse does not
enable `nullpath_ok`, so libfuse locks a path even for handle operations.
Parent directories are only read-locked, so listing a parent can overlap the
rename. Enabling `nullpath_ok` or switching to the low-level API removes this
guarantee.

**Windows (WinFSP).** A rename takes the volume's rename resource exclusively,
and every create or open takes it shared. During a rename, nothing on the
volume can be created or opened, but I/O on existing handles continues. WinFSP
also refuses to rename a directory that has open handles beneath it, unless it
is a POSIX rename and every opener shares delete. NTFS has a similar rule, so a
component backed by local files cannot rename a directory while files beneath
it are open. Short-lived handles that cloudfuse itself opens, for example while
listing a directory, can cause such a rename to fail spuriously.

## Appendix B: Observed unlink-while-open trace

On Linux with the `libfuse` and `loopbackfs` components, opening `victim.txt`,
unlinking it, and then closing it produced these callbacks:

1. `Getattr` `.fuse_hidden0000000200000001` → not found
2. `Rename` `victim.txt` → `.fuse_hidden0000000200000001`
3. `Flush`, `Release` on the hidden name
4. `Unlink` `.fuse_hidden0000000200000001`

The hidden file existed in the backing store until the close.

## Sources

- [cgofuse filesystem interface](https://github.com/winfsp/cgofuse/blob/v1.6.0/fuse/fsop.go)
- [libfuse high-level operations](https://libfuse.github.io/doxygen/structfuse__operations.html)
- [libfuse high-level implementation, including path locking and hidden files](https://github.com/libfuse/libfuse/blob/master/lib/fuse.c)
- [WinFSP rename locking: create](https://github.com/winfsp/winfsp/blob/master/src/sys/create.c) and [set-information](https://github.com/winfsp/winfsp/blob/master/src/sys/fileinfo.c)
- [WinFSP open-descendant rename check (`FspFileNodeRenameCheck`)](https://github.com/winfsp/winfsp/blob/master/src/sys/file.c)
- [Windows rules for renaming open files](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information)
- [Linux `rename(2)`](https://man7.org/linux/man-pages/man2/rename.2.html)
- [Linux `unlink(2)`](https://man7.org/linux/man-pages/man2/unlink.2.html)
- [WinFSP delete redesign](https://github.com/winfsp/winfsp/blob/master/doc/archive/WinFsp-Delete-Redesign.asciidoc)
- [cloudfuse FUSE configuration](../component/libfuse/libfuse.go)
- [cloudfuse attribute cache](../component/attr_cache/attr_cache.go)
