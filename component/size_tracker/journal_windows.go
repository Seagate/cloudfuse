//go:build windows

package size_tracker

import (
	"errors"
	"time"

	"golang.org/x/sys/windows"
)

// exclusiveLock acquires an exclusive byte-range lock on the file handle.
// On Windows, LockFileEx returns immediately if the region is already locked.
// We convert that into a blocking retry loop so callers transparently wait
// instead of receiving spurious ERROR_LOCK_VIOLATION failures under contention.
func exclusiveLock(fd uintptr) error {
	h := windows.Handle(fd)
	var ov windows.Overlapped
	// Simple bounded backoff: start at 5ms, cap at 200ms
	backoff := 5 * time.Millisecond
	for {
		err := windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &ov)
		if err == nil {
			return nil
		}
		// Only retry on lock contention; propagate all other errors.
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			time.Sleep(backoff)
			// Exponential backoff with cap
			backoff *= 2
			if backoff > 200*time.Millisecond {
				backoff = 200 * time.Millisecond
			}
			continue
		}
		return err
	}
}

// unlock releases the previously acquired byte-range lock.
func unlock(fd uintptr) error {
	h := windows.Handle(fd)
	var ov windows.Overlapped
	return windows.UnlockFileEx(h, 0, 1, 0, &ov)
}
