//go:build linux

package common

import (
	"os"
	"syscall"
)

// Open opens the named file for reading. If successful, methods on
// the returned file can be used for reading; the associated file
// descriptor has mode O_RDONLY.
// If there is an error, it will be of type *PathError.
func Open(name string) (*os.File, error) {
	return OpenFile(name, syscall.O_RDONLY, 0)
}

// OpenFile on linux passes the call to the os. This file is needed so we can
// easily compile the code on both windows and linux.
func OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}
