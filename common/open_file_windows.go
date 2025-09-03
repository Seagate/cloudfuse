//go:build windows

// The following code is copied and modified from the Go source
// code and is covered by the copyright below.

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// https://github.com/golang/go/blob/master/LICENSE

package common

import (
	"os"
	"sync"
	"syscall"
	"unsafe"
)

// This file adds all the changed needed to replace the os.OpenFile call
// with one that allows for renaming and deleting an open file on
// Windows.

const (
	_FILE_WRITE_EA = 0x00000010
)

var getwdCache struct {
	sync.Mutex
	dir string
}

// Open opens the named file for reading. If successful, methods on
// the returned file can be used for reading; the associated file
// descriptor has mode O_RDONLY.
// If there is an error, it will be of type *PathError.
func Open(name string) (*os.File, error) {
	return OpenFile(name, syscall.O_RDONLY, 0)
}

// OpenFile is the generalized open call; most users will use Open
// or Create instead. It opens the named file with specified flag
// ([O_RDONLY] etc.). If the file does not exist, and the [O_CREATE] flag
// is passed, it is created with mode perm (before umask);
// the containing directory must exist. If successful,
// methods on the returned File can be used for I/O.
// If there is an error, it will be of type [*PathError].
// Copied from https://cs.opensource.google/go/go/+/refs/tags/go1.25.0:src/os/file.go;l=410
func OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	f, err := openFileNolog(name, flag, perm)
	if err != nil {
		return nil, err
	}
	//f.appendMode = flag&O_APPEND != 0

	return f, nil
}

// OpenFile is the generalized open call; most users will use Open
// or Create instead. It opens the named file with specified flag
// (O_RDONLY etc.). If the file does not exist, and the O_CREATE flag
// is passed, it is created with mode perm (before umask). If successful,
// methods on the returned File can be used for I/O.
// If there is an error, it will be of type *PathError.
// We modify this function to allow renaming and deleting an open file
// on Windows.
// Copied from https://cs.opensource.google/go/go/+/refs/tags/go1.25.0:src/os/file_windows.go;l=113
func openFileNolog(name string, flag int, perm os.FileMode) (*os.File, error) {
	if name == "" {
		return nil, &os.PathError{Op: "open", Path: name, Err: syscall.ENOENT}
	}
	path := fixLongPath(name)
	r, err := open(path, flag|syscall.O_CLOEXEC, syscallMode(perm))
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: err}
	}
	// syscall.Open always returns a non-blocking handle.
	return os.NewFile(uintptr(r), name), nil
}

// copied from https://cs.opensource.google/go/go/+/refs/tags/go1.25.0:src/syscall/syscall_windows.go;l=365
func open(name string, flag int, perm uint32) (fd syscall.Handle, err error) {
	if len(name) == 0 {
		return syscall.InvalidHandle, syscall.ERROR_FILE_NOT_FOUND
	}
	namep, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return syscall.InvalidHandle, err
	}
	accessFlags := flag & (syscall.O_RDONLY | syscall.O_WRONLY | syscall.O_RDWR)
	var access uint32
	switch accessFlags {
	case syscall.O_RDONLY:
		access = syscall.GENERIC_READ
	case syscall.O_WRONLY:
		access = syscall.GENERIC_WRITE
	case syscall.O_RDWR:
		access = syscall.GENERIC_READ | syscall.GENERIC_WRITE
	}
	if flag&syscall.O_CREAT != 0 {
		access |= syscall.GENERIC_WRITE
	}
	if flag&syscall.O_APPEND != 0 {
		// Remove GENERIC_WRITE unless O_TRUNC is set, in which case we need it to truncate the file.
		// We can't just remove FILE_WRITE_DATA because GENERIC_WRITE without FILE_WRITE_DATA
		// starts appending at the beginning of the file rather than at the end.
		if flag&syscall.O_TRUNC == 0 {
			access &^= syscall.GENERIC_WRITE
		}
		// Set all access rights granted by GENERIC_WRITE except for FILE_WRITE_DATA.
		access |= syscall.FILE_APPEND_DATA | syscall.FILE_WRITE_ATTRIBUTES | _FILE_WRITE_EA | syscall.STANDARD_RIGHTS_WRITE | syscall.SYNCHRONIZE
	}
	// We add the FILE_SHARE_DELETE flag which allows the open file to be renamed and deleted before being closed.
	// This is not enabled in Go.
	sharemode := uint32(syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE | syscall.FILE_SHARE_DELETE)
	var sa *syscall.SecurityAttributes
	if flag&syscall.O_CLOEXEC == 0 {
		sa = makeInheritSa()
	}
	var attrs uint32 = syscall.FILE_ATTRIBUTE_NORMAL
	if perm&syscall.S_IWRITE == 0 {
		attrs = syscall.FILE_ATTRIBUTE_READONLY
	}
	switch accessFlags {
	case syscall.O_WRONLY, syscall.O_RDWR:
		// Unix doesn't allow opening a directory with O_WRONLY
		// or O_RDWR, so we don't set the flag in that case,
		// which will make CreateFile fail with ERROR_ACCESS_DENIED.
		// We will map that to EISDIR if the file is a directory.
	default:
		// We might be opening a directory for reading,
		// and CreateFile requires FILE_FLAG_BACKUP_SEMANTICS
		// to work with directories.
		attrs |= syscall.FILE_FLAG_BACKUP_SEMANTICS
	}
	if flag&syscall.O_SYNC != 0 {
		const _FILE_FLAG_WRITE_THROUGH = 0x80000000
		attrs |= _FILE_FLAG_WRITE_THROUGH
	}
	// We don't use CREATE_ALWAYS, because when opening a file with
	// FILE_ATTRIBUTE_READONLY these will replace an existing file
	// with a new, read-only one. See https://go.dev/issue/38225.
	//
	// Instead, we ftruncate the file after opening when O_TRUNC is set.
	var createmode uint32
	switch {
	case flag&(syscall.O_CREAT|syscall.O_EXCL) == (syscall.O_CREAT | syscall.O_EXCL):
		createmode = syscall.CREATE_NEW
		attrs |= syscall.FILE_FLAG_OPEN_REPARSE_POINT // don't follow symlinks
	case flag&syscall.O_CREAT == syscall.O_CREAT:
		createmode = syscall.OPEN_ALWAYS
	default:
		createmode = syscall.OPEN_EXISTING
	}
	h, err := syscall.CreateFile(namep, access, sharemode, sa, createmode, attrs, 0)
	if h == syscall.InvalidHandle {
		if err == syscall.ERROR_ACCESS_DENIED && (attrs&syscall.FILE_FLAG_BACKUP_SEMANTICS == 0) {
			// We should return EISDIR when we are trying to open a directory with write access.
			fa, e1 := syscall.GetFileAttributes(namep)
			if e1 == nil && fa&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
				err = syscall.EISDIR
			}
		}
		return h, err
	}
	// Ignore O_TRUNC if the file has just been created.
	if flag&syscall.O_TRUNC == syscall.O_TRUNC &&
		(createmode == syscall.OPEN_EXISTING || (createmode == syscall.OPEN_ALWAYS && err == syscall.ERROR_ALREADY_EXISTS)) {
		err = syscall.Ftruncate(h, 0)
		if err != nil {
			syscall.CloseHandle(h)
			return syscall.InvalidHandle, err
		}
	}
	return h, nil
}

// Copied from https://cs.opensource.google/go/go/+/master:src/syscall/syscall_windows.go;drc=964985362b4d8702a16bce08c7a825488ccb9601;l=317
func makeInheritSa() *syscall.SecurityAttributes {
	var sa syscall.SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1
	return &sa
}

// fixLongPath returns the extended-length (\\?\-prefixed) form of
// path when needed, in order to avoid the default 260 character file
// path limit imposed by Windows. If the path is short enough or already
// has the extended-length prefix, fixLongPath returns path unmodified.
// If the path is relative and joining it with the current working
// directory results in a path that is too long, fixLongPath returns
// the absolute path with the extended-length prefix.
//
// See https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file#maximum-path-length-limitation
// Copied from https://cs.opensource.google/go/go/+/refs/tags/go1.25.0:src/os/path_windows.go;l=100
func fixLongPath(path string) string {
	// TODO: Apparently in later version of Windows we don't need to call this function
	// as it can use longer path names. See this issue https://groups.google.com/g/golang-checkins/c/2Lv2xYuo_h0
	// if windows.CanUseLongPaths {
	// 	return path
	// }
	return addExtendedPrefix(path)
}

// addExtendedPrefix adds the extended path prefix (\\?\) to path.
// Copied from https://cs.opensource.google/go/go/+/refs/tags/go1.25.0:src/os/path_windows.go;l=108
func addExtendedPrefix(path string) string {
	if len(path) >= 4 {
		if path[:4] == `\??\` {
			// Already extended with \??\
			return path
		}
		if os.IsPathSeparator(path[0]) && os.IsPathSeparator(path[1]) && path[2] == '?' && os.IsPathSeparator(path[3]) {
			// Already extended with \\?\ or any combination of directory separators.
			return path
		}
	}

	// Do nothing (and don't allocate) if the path is "short".
	// Empirically (at least on the Windows Server 2013 builder),
	// the kernel is arbitrarily okay with < 248 bytes. That
	// matches what the docs above say:
	// "When using an API to create a directory, the specified
	// path cannot be so long that you cannot append an 8.3 file
	// name (that is, the directory name cannot exceed MAX_PATH
	// minus 12)." Since MAX_PATH is 260, 260 - 12 = 248.
	//
	// The MSDN docs appear to say that a normal path that is 248 bytes long
	// will work; empirically the path must be less then 248 bytes long.
	pathLength := len(path)
	if IsAbs(path) {
		// If the path is relative, we need to prepend the working directory
		// plus a separator to the path before we can determine if it's too long.
		// We don't want to call syscall.Getwd here, as that call is expensive to do
		// every time fixLongPath is called with a relative path, so we use a cache.
		// Note that getwdCache might be outdated if the working directory has been
		// changed without using os.Chdir, i.e. using syscall.Chdir directly or cgo.
		// This is fine, as the worst that can happen is that we fail to fix the path.
		getwdCache.Lock()
		if getwdCache.dir == "" {
			// Init the working directory cache.
			getwdCache.dir, _ = syscall.Getwd()
		}
		pathLength += len(getwdCache.dir) + 1
		getwdCache.Unlock()
	}

	if pathLength < 248 {
		// Don't fix. (This is how Go 1.7 and earlier worked,
		// not automatically generating the \\?\ form)
		return path
	}

	var isUNC, isDevice bool
	if len(path) >= 2 && os.IsPathSeparator(path[0]) && os.IsPathSeparator(path[1]) {
		if len(path) >= 4 && path[2] == '.' && os.IsPathSeparator(path[3]) {
			// Starts with //./
			isDevice = true
		} else {
			// Starts with //
			isUNC = true
		}
	}
	var prefix []uint16
	if isUNC {
		// UNC path, prepend the \\?\UNC\ prefix.
		prefix = []uint16{'\\', '\\', '?', '\\', 'U', 'N', 'C', '\\'}
	} else if isDevice {
		// Don't add the extended prefix to device paths, as it would
		// change its meaning.
	} else {
		prefix = []uint16{'\\', '\\', '?', '\\'}
	}

	p, err := syscall.UTF16FromString(path)
	if err != nil {
		return path
	}
	// Estimate the required buffer size using the path length plus the null terminator.
	// pathLength includes the working directory. This should be accurate unless
	// the working directory has changed without using os.Chdir.
	n := uint32(pathLength) + 1
	var buf []uint16
	for {
		buf = make([]uint16, n+uint32(len(prefix)))
		n, err = syscall.GetFullPathName(&p[0], n, &buf[len(prefix)], nil)
		if err != nil {
			return path
		}
		if n <= uint32(len(buf)-len(prefix)) {
			buf = buf[:n+uint32(len(prefix))]
			break
		}
	}
	if isUNC {
		// Remove leading \\.
		buf = buf[2:]
	}
	copy(buf, prefix)
	return syscall.UTF16ToString(buf)
}

// IsAbs reports whether the path is absolute.
// Copied from https://cs.opensource.google/go/go/+/refs/tags/go1.25.0:src/internal/filepathlite/path_windows.go;l=184
func IsAbs(path string) (b bool) {
	l := volumeNameLen(path)
	if l == 0 {
		return false
	}
	// If the volume name starts with a double slash, this is an absolute path.
	if os.IsPathSeparator(path[0]) && os.IsPathSeparator(path[1]) {
		return true
	}
	path = path[l:]
	if path == "" {
		return false
	}
	return os.IsPathSeparator(path[0])
}

// volumeNameLen returns length of the leading volume name on Windows.
// It returns 0 elsewhere.
//
// See:
// https://learn.microsoft.com/en-us/dotnet/standard/io/file-path-formats
// https://googleprojectzero.blogspot.com/2016/02/the-definitive-guide-on-win32-to-nt.html
// Copied from https://cs.opensource.google/go/go/+/refs/tags/go1.25.0:src/internal/filepathlite/path_windows.go;l=206
func volumeNameLen(path string) int {
	switch {
	case len(path) >= 2 && path[1] == ':':
		// Path starts with a drive letter.
		//
		// Not all Windows functions necessarily enforce the requirement that
		// drive letters be in the set A-Z, and we don't try to here.
		//
		// We don't handle the case of a path starting with a non-ASCII character,
		// in which case the "drive letter" might be multiple bytes long.
		return 2

	case len(path) == 0 || !os.IsPathSeparator(path[0]):
		// Path does not have a volume component.
		return 0

	case pathHasPrefixFold(path, `\\.\UNC`):
		// We're going to treat the UNC host and share as part of the volume
		// prefix for historical reasons, but this isn't really principled;
		// Windows's own GetFullPathName will happily remove the first
		// component of the path in this space, converting
		// \\.\unc\a\b\..\c into \\.\unc\a\c.
		return uncLen(path, len(`\\.\UNC\`))

	case pathHasPrefixFold(path, `\\.`) ||
		pathHasPrefixFold(path, `\\?`) || pathHasPrefixFold(path, `\??`):
		// Path starts with \\.\, and is a Local Device path; or
		// path starts with \\?\ or \??\ and is a Root Local Device path.
		//
		// We treat the next component after the \\.\ prefix as
		// part of the volume name, which means Clean(`\\?\c:\`)
		// won't remove the trailing \. (See #64028.)
		if len(path) == 3 {
			return 3 // exactly \\.
		}
		_, rest, ok := cutPath(path[4:])
		if !ok {
			return len(path)
		}
		return len(path) - len(rest) - 1

	case len(path) >= 2 && os.IsPathSeparator(path[1]):
		// Path starts with \\, and is a UNC path.
		return uncLen(path, 2)
	}
	return 0
}

// pathHasPrefixFold tests whether the path s begins with prefix,
// ignoring case and treating all path separators as equivalent.
// If s is longer than prefix, then s[len(prefix)] must be a path separator.
// Copied from https://cs.opensource.google/go/go/+/refs/tags/go1.25.0:src/internal/filepathlite/path_windows.go;l=257
func pathHasPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		if os.IsPathSeparator(prefix[i]) {
			if !os.IsPathSeparator(s[i]) {
				return false
			}
		} else if toUpper(prefix[i]) != toUpper(s[i]) {
			return false
		}
	}
	if len(s) > len(prefix) && !os.IsPathSeparator(s[len(prefix)]) {
		return false
	}
	return true
}

// uncLen returns the length of the volume prefix of a UNC path.
// prefixLen is the prefix prior to the start of the UNC host;
// for example, for "//host/share", the prefixLen is len("//")==2.
// Copied from https://cs.opensource.google/go/go/+/refs/tags/go1.25.0:src/internal/filepathlite/path_windows.go;l=279
func uncLen(path string, prefixLen int) int {
	count := 0
	for i := prefixLen; i < len(path); i++ {
		if os.IsPathSeparator(path[i]) {
			count++
			if count == 2 {
				return i
			}
		}
	}
	return len(path)
}

// cutPath slices path around the first path separator.
// Copied from https://cs.opensource.google/go/go/+/refs/tags/go1.25.0:src/internal/filepathlite/path_windows.go;l=293
func cutPath(path string) (before, after string, found bool) {
	for i := range path {
		if os.IsPathSeparator(path[i]) {
			return path[:i], path[i+1:], true
		}
	}
	return path, "", false
}

// Copied from https://cs.opensource.google/go/go/+/refs/tags/go1.25.0:src/internal/filepathlite/path_windows.go;l=176
func toUpper(c byte) byte {
	if 'a' <= c && c <= 'z' {
		return c - ('a' - 'A')
	}
	return c
}

// syscallMode returns the syscall-specific mode bits from Go's portable mode bits.
// Copied from https://cs.opensource.google/go/go/+/refs/tags/go1.25.0:src/os/file_posix.go;l=60
func syscallMode(i os.FileMode) (o uint32) {
	o |= uint32(i.Perm())
	if i&os.ModeSetuid != 0 {
		o |= syscall.S_ISUID
	}
	if i&os.ModeSetgid != 0 {
		o |= syscall.S_ISGID
	}
	if i&os.ModeSticky != 0 {
		o |= syscall.S_ISVTX
	}
	// No mapping for Go's ModeTemporary (plan9 only).
	return
}
