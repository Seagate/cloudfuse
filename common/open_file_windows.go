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
	"syscall"
	"unsafe"
)

// This file adds all the changed needed to replace the os.OpenFile call
// with one that allows for renaming and deleting an open file on
// Windows.

const (
	_ERROR_BAD_NETPATH = syscall.Errno(53)
)

// Open opens the named file for reading. If successful, methods on
// the returned file can be used for reading; the associated file
// descriptor has mode O_RDONLY.
// If there is an error, it will be of type *PathError.
func Open(name string) (*os.File, error) {
	return OpenFile(name, syscall.O_RDONLY, 0)
}

// OpenFile is the generalized open call; most users will use Open
// or Create instead. It opens the named file with specified flag
// (O_RDONLY etc.). If the file does not exist, and the O_CREATE flag
// is passed, it is created with mode perm (before umask). If successful,
// methods on the returned File can be used for I/O.
// If there is an error, it will be of type *PathError.
// We modify this function to allow renaming and deleting an open file
// on Windows.
// Copied from https://cs.opensource.google/go/go/+/refs/tags/go1.20.2:src/os/file_windows.go;drc=0f0aa5d8a6a0253627d58b3aa083b24a1091933f;l=164
func OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	if name == "" {
		return nil, &os.PathError{Op: "open", Path: name, Err: syscall.ENOENT}
	}
	path := fixLongPath(name)

	r, e := open(path, flag|syscall.O_CLOEXEC, syscallMode(perm))
	if e != nil {
		// We should return EISDIR when we are trying to open a directory with write access.
		if e == syscall.ERROR_ACCESS_DENIED && (flag&os.O_WRONLY != 0 || flag&os.O_RDWR != 0) {
			pathp, e1 := syscall.UTF16PtrFromString(path)
			if e1 == nil {
				var fa syscall.Win32FileAttributeData
				e1 = syscall.GetFileAttributesEx(
					pathp,
					syscall.GetFileExInfoStandard,
					(*byte)(unsafe.Pointer(&fa)),
				)
				if e1 == nil && fa.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
					e = syscall.EISDIR
				}
			}
		}
		return nil, &os.PathError{Op: "open", Path: name, Err: e}
	}
	f, e := os.NewFile(uintptr(r), name), nil
	if e != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: e}
	}

	// TODO: Not sure how to handle this
	// f.appendMode = flag&os.O_APPEND != 0

	return f, nil
}

// copied from https://cs.opensource.google/go/go/+/master:src/syscall/syscall_windows.go;drc=964985362b4d8702a16bce08c7a825488ccb9601;l=324
func open(path string, mode int, perm uint32) (fd syscall.Handle, err error) {
	if len(path) == 0 {
		return syscall.InvalidHandle, syscall.ERROR_FILE_NOT_FOUND
	}
	pathp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return syscall.InvalidHandle, err
	}
	var access uint32
	switch mode & (syscall.O_RDONLY | syscall.O_WRONLY | syscall.O_RDWR) {
	case syscall.O_RDONLY:
		access = syscall.GENERIC_READ
	case syscall.O_WRONLY:
		access = syscall.GENERIC_WRITE
	case syscall.O_RDWR:
		access = syscall.GENERIC_READ | syscall.GENERIC_WRITE
	}
	if mode&syscall.O_CREAT != 0 {
		access |= syscall.GENERIC_WRITE
	}
	if mode&syscall.O_APPEND != 0 {
		access &^= syscall.GENERIC_WRITE
		access |= syscall.FILE_APPEND_DATA
	}
	// We add the FILE_SHARE_DELETE flag which allows the open file to be renamed and deleted before being closed.
	// This is not enabled in Go.
	sharemode := uint32(
		syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE | syscall.FILE_SHARE_DELETE,
	)
	var sa *syscall.SecurityAttributes
	if mode&syscall.O_CLOEXEC == 0 {
		sa = makeInheritSa()
	}
	var createmode uint32
	switch {
	case mode&(syscall.O_CREAT|syscall.O_EXCL) == (syscall.O_CREAT | syscall.O_EXCL):
		createmode = syscall.CREATE_NEW
	case mode&(syscall.O_CREAT|syscall.O_TRUNC) == (syscall.O_CREAT | syscall.O_TRUNC):
		createmode = syscall.CREATE_ALWAYS
	case mode&syscall.O_CREAT == syscall.O_CREAT:
		createmode = syscall.OPEN_ALWAYS
	case mode&syscall.O_TRUNC == syscall.O_TRUNC:
		createmode = syscall.TRUNCATE_EXISTING
	default:
		createmode = syscall.OPEN_EXISTING
	}
	var attrs uint32 = syscall.FILE_ATTRIBUTE_NORMAL
	if perm&syscall.S_IWRITE == 0 {
		attrs = syscall.FILE_ATTRIBUTE_READONLY
		if createmode == syscall.CREATE_ALWAYS {
			// We have been asked to create a read-only file.
			// If the file already exists, the semantics of
			// the Unix open system call is to preserve the
			// existing permissions. If we pass CREATE_ALWAYS
			// and FILE_ATTRIBUTE_READONLY to CreateFile,
			// and the file already exists, CreateFile will
			// change the file permissions.
			// Avoid that to preserve the Unix semantics.
			h, e := syscall.CreateFile(
				pathp,
				access,
				sharemode,
				sa,
				syscall.TRUNCATE_EXISTING,
				syscall.FILE_ATTRIBUTE_NORMAL,
				0,
			)
			switch e {
			case syscall.ERROR_FILE_NOT_FOUND, _ERROR_BAD_NETPATH, syscall.ERROR_PATH_NOT_FOUND:
				// File does not exist. These are the same
				// errors as Errno.Is checks for ErrNotExist.
				// Carry on to create the file.
			default:
				// Success or some different error.
				return h, e
			}
		}
	}
	if createmode == syscall.OPEN_EXISTING && access == syscall.GENERIC_READ {
		// Necessary for opening directory handles.
		attrs |= syscall.FILE_FLAG_BACKUP_SEMANTICS
	}
	return syscall.CreateFile(pathp, access, sharemode, sa, createmode, attrs, 0)
}

// Coped from https://cs.opensource.google/go/go/+/master:src/syscall/syscall_windows.go;drc=964985362b4d8702a16bce08c7a825488ccb9601;l=317
func makeInheritSa() *syscall.SecurityAttributes {
	var sa syscall.SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1
	return &sa
}

// fixLongPath returns the extended-length (\\?\-prefixed) form of
// path when needed, in order to avoid the default 260 character file
// path limit imposed by Windows. If path is not easily converted to
// the extended-length form (for example, if path is a relative path
// or contains .. elements), or is short enough, fixLongPath returns
// path unmodified.
//
// See https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx#maxpath
// Copied from https://cs.opensource.google/go/go/+/refs/tags/go1.20.2:src/os/path_windows.go;drc=af725f42864c8fb56afcf3ba76d2df7d372534e4;l=143
func fixLongPath(path string) string {
	// TODO: Apparently in later version of Windows we don't need to call this function
	// as it can use longer path names. See this issue https://groups.google.com/g/golang-checkins/c/2Lv2xYuo_h0
	// if canUseLongPaths {
	// 	return path
	// }

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
	if len(path) < 248 {
		// Don't fix. (This is how Go 1.7 and earlier worked,
		// not automatically generating the \\?\ form)
		return path
	}

	// The extended form begins with \\?\, as in
	// \\?\c:\windows\foo.txt or \\?\UNC\server\share\foo.txt.
	// The extended form disables evaluation of . and .. path
	// elements and disables the interpretation of / as equivalent
	// to \. The conversion here rewrites / to \ and elides
	// . elements as well as trailing or duplicate separators. For
	// simplicity it avoids the conversion entirely for relative
	// paths or paths containing .. elements. For now,
	// \\server\share paths are not converted to
	// \\?\UNC\server\share paths because the rules for doing so
	// are less well-specified.
	if len(path) >= 2 && path[:2] == `\\` {
		// Don't canonicalize UNC paths.
		return path
	}
	if !isAbs(path) {
		// Relative path
		return path
	}

	const prefix = `\\?`

	pathbuf := make([]byte, len(prefix)+len(path)+len(`\`))
	copy(pathbuf, prefix)
	n := len(path)
	r, w := 0, len(prefix)
	for r < n {
		switch {
		case os.IsPathSeparator(path[r]):
			// empty block
			r++
		case path[r] == '.' && (r+1 == n || os.IsPathSeparator(path[r+1])):
			// /./
			r++
		case r+1 < n && path[r] == '.' && path[r+1] == '.' && (r+2 == n || os.IsPathSeparator(path[r+2])):
			// /../ is currently unhandled
			return path
		default:
			pathbuf[w] = '\\'
			w++
			for ; r < n && !os.IsPathSeparator(path[r]); r++ {
				pathbuf[w] = path[r]
				w++
			}
		}
	}
	// A drive's root directory needs a trailing \
	if w == len(`\\?\c:`) {
		pathbuf[w] = '\\'
		w++
	}
	return string(pathbuf[:w])
}

// Copied from https://cs.opensource.google/go/go/+/master:src/os/path_windows.go;drc=af725f42864c8fb56afcf3ba76d2df7d372534e4;l=42
func isAbs(path string) (b bool) {
	v := volumeName(path)
	if v == "" {
		return false
	}
	path = path[len(v):]
	if path == "" {
		return false
	}
	return os.IsPathSeparator(path[0])
}

// Copied from https://cs.opensource.google/go/go/+/master:src/os/path_windows.go;drc=af725f42864c8fb56afcf3ba76d2df7d372534e4;l=54
func volumeName(path string) (v string) {
	if len(path) < 2 {
		return ""
	}
	// with drive letter
	c := path[0]
	if path[1] == ':' &&
		('0' <= c && c <= '9' || 'a' <= c && c <= 'z' ||
			'A' <= c && c <= 'Z') {
		return path[:2]
	}
	// is it UNC
	if l := len(path); l >= 5 && os.IsPathSeparator(path[0]) && os.IsPathSeparator(path[1]) &&
		!os.IsPathSeparator(path[2]) && path[2] != '.' {
		// first, leading `\\` and next shouldn't be `\`. its server name.
		for n := 3; n < l-1; n++ {
			// second, next '\' shouldn't be repeated.
			if os.IsPathSeparator(path[n]) {
				n++
				// third, following something characters. its share name.
				if !os.IsPathSeparator(path[n]) {
					if path[n] == '.' {
						break
					}
					for ; n < l; n++ {
						if os.IsPathSeparator(path[n]) {
							break
						}
					}
					return path[:n]
				}
				break
			}
		}
	}
	return ""
}

// Copied from https://cs.opensource.google/go/go/+/master:src/os/file_posix.go;drc=a2baae6851a157d662dff7cc508659f66249698a;l=62
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
