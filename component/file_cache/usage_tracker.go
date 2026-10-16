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

package file_cache

import (
	"errors"
	"io/fs"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/Seagate/cloudfuse/common"
)

// usageTracker keeps the apparent size of every file in the local cache in memory,
// so usage checks on the I/O path cost a map lookup instead of walking the cache directory.
// Every component path that changes a cached file's size reports it here; reconcile walks the
// cache directory in the background to correct any drift.
type usageTracker struct {
	mu    sync.Mutex
	sizes map[string]int64 // apparent size of each file
	total atomic.Int64     // sum of diskSize over all files
	// files are counted in whole blocks of this size (1 counts exact bytes)
	blockSize int64
}

func newUsageTracker() *usageTracker {
	blockSize := int64(1)
	if runtime.GOOS == "windows" {
		// matches the sector rounding common.GetUsage uses on Windows
		blockSize = common.SectorSize
	}
	return &usageTracker{sizes: make(map[string]int64), blockSize: blockSize}
}

// diskSize returns the space counted for a file of the given apparent size
func (u *usageTracker) diskSize(size int64) int64 {
	if u.blockSize <= 1 || size <= 0 {
		return size
	}
	return (size + u.blockSize - 1) / u.blockSize * u.blockSize
}

// bytes returns the total space used by all tracked files
func (u *usageTracker) bytes() int64 {
	return u.total.Load()
}

// size returns the tracked size of a file, and whether it is tracked
func (u *usageTracker) size(path string) (int64, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	size, ok := u.sizes[path]
	return size, ok
}

// set records the current size of a file
func (u *usageTracker) set(path string, size int64) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.total.Add(u.diskSize(size) - u.diskSize(u.sizes[path]))
	u.sizes[path] = size
}

// grow records that a file now extends at least to end (e.g. after a write)
func (u *usageTracker) grow(path string, end int64) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if current, ok := u.sizes[path]; !ok || end > current {
		u.total.Add(u.diskSize(end) - u.diskSize(current))
		u.sizes[path] = end
	}
}

// remove forgets a deleted file
func (u *usageTracker) remove(path string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if size, ok := u.sizes[path]; ok {
		u.total.Add(-u.diskSize(size))
		delete(u.sizes, path)
	}
}

// rename moves a file's size to its new path, replacing any file already there.
// It returns false if src was not tracked, in which case dst's size is unknown.
func (u *usageTracker) rename(src, dst string) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if src == dst {
		_, ok := u.sizes[src]
		return ok
	}
	// dst's previous contents are replaced either way
	u.total.Add(-u.diskSize(u.sizes[dst]))
	delete(u.sizes, dst)
	size, ok := u.sizes[src]
	if !ok {
		return false
	}
	delete(u.sizes, src)
	u.sizes[dst] = size
	return true
}

// reconcile rebuilds the size table from the files under root.
// It runs in the background (at startup and periodically).
// If the walk fails, the current table is kept.
func (u *usageTracker) reconcile(root string) (int64, error) {
	sizes := make(map[string]int64)
	var total int64
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// files can be removed while we walk, but the root must be readable
			if path != root && errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		} else if err != nil {
			return err
		}
		sizes[path] = info.Size()
		total += u.diskSize(info.Size())
		return nil
	})
	if err != nil {
		return u.bytes(), err
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.sizes = sizes
	u.total.Store(total)
	return total, nil
}
