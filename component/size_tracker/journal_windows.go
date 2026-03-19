//go:build windows

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2026 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

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
