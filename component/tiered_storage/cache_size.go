/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2026 Seagate Technology LLC and/or its Affiliates

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

package tiered_storage

import (
	"sync/atomic"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
)

// driftWarnBytes is how far the running total may diverge from a measurement
// before the correction is worth a log line.
const driftWarnBytes = 16 * common.MbToBytes

// cacheSizeTracker tracks how many bytes of local storage the component is
// using.
//
// The total is maintained incrementally, because measuring it costs a du(1)
// subprocess on Linux and a full tree walk on Windows - far too expensive to do
// per write or per eviction tick. Incremental accounting drifts (directory
// entries, sparse files, anything that touches the cache directory behind our
// back), so Reconcile periodically replaces the total with a real measurement.
//
// Sizes are apparent bytes, matching `du -sb` on Linux. Windows measures
// allocated sectors instead, so the two platforms disagree slightly for small
// files; Reconcile is what keeps that from accumulating.
type cacheSizeTracker struct {
	path              string
	used              atomic.Int64
	reconcileInterval time.Duration
	lastReconcile     atomic.Int64 // unix nanoseconds
}

func newCacheSizeTracker(path string, reconcileInterval time.Duration) *cacheSizeTracker {
	t := &cacheSizeTracker{path: path, reconcileInterval: reconcileInterval}
	t.lastReconcile.Store(time.Now().UnixNano())
	return t
}

// Used returns the current cache usage in bytes.
func (t *cacheSizeTracker) Used() int64 {
	return t.used.Load()
}

// Add applies a signed change in bytes. The total is clamped at zero: drift
// could otherwise drive it negative and hide a full cache.
func (t *cacheSizeTracker) Add(delta int64) {
	if delta == 0 {
		return
	}
	for {
		current := t.used.Load()
		next := max(current+delta, 0)
		if t.used.CompareAndSwap(current, next) {
			return
		}
	}
}

// Resize records one cached file changing from oldSize to newSize bytes.
func (t *cacheSizeTracker) Resize(oldSize, newSize int64) {
	t.Add(newSize - oldSize)
}

// Refresh measures the cache directory and replaces the running total.
// Changes made while the measurement is in flight are lost, which is inherent
// to reconciling against a moving target and is why it is not the hot path.
func (t *cacheSizeTracker) Refresh() {
	t.lastReconcile.Store(time.Now().UnixNano())

	usage, err := common.GetUsage(t.path)
	if err != nil {
		log.Err("cacheSizeTracker::Refresh : failed to measure %s [%v]", t.path, err)
		return
	}

	measured := int64(usage)
	previous := t.used.Swap(measured)
	if drift := measured - previous; drift > driftWarnBytes || drift < -driftWarnBytes {
		log.Info(
			"cacheSizeTracker::Refresh : corrected usage by %d bytes (was %d, now %d)",
			drift,
			previous,
			measured,
		)
	}
}

// Reconcile refreshes the running total, but no more often than
// reconcileInterval. An interval of zero refreshes on every call.
func (t *cacheSizeTracker) Reconcile() {
	if time.Since(time.Unix(0, t.lastReconcile.Load())) < t.reconcileInterval {
		return
	}
	t.Refresh()
}
