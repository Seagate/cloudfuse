//go:build unix

package size_tracker

import (
	"syscall"
)

// exclusiveLock obtains an exclusive advisory lock on the underlying file descriptor.
func exclusiveLock(fd uintptr) error {
	return syscall.Flock(int(fd), syscall.LOCK_EX)
}

// unlock releases any advisory lock on the file descriptor.
func unlock(fd uintptr) error {
	return syscall.Flock(int(fd), syscall.LOCK_UN)
}
