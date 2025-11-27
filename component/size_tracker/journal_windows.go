//go:build windows

package size_tracker

import (
	"golang.org/x/sys/windows"
)

// exclusiveLock acquires an exclusive byte-range lock on the file handle.
// Locking one byte is sufficient to serialize writers across processes.
func exclusiveLock(fd uintptr) error {
	h := windows.Handle(fd)
	var ov windows.Overlapped
	return windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &ov)
}

// unlock releases the previously acquired byte-range lock.
func unlock(fd uintptr) error {
	h := windows.Handle(fd)
	var ov windows.Overlapped
	return windows.UnlockFileEx(h, 0, 1, 0, &ov)
}
