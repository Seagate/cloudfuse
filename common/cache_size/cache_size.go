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

package cache_size

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
)

// driftWarnBytes is how far the running total may diverge from a measurement
// before the correction is worth a log line.
const driftWarnBytes = 16 * common.MbToBytes

// Tracker tracks how many bytes of local storage a caching component is using.
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
type Tracker struct {
	paths             []string
	used              atomic.Int64
	reconcileInterval time.Duration
	lastReconcile     atomic.Int64 // unix nanoseconds
}

// NewTracker tracks the combined size of paths. The total starts at zero; call
// Refresh to seed it with a measurement.
func NewTracker(reconcileInterval time.Duration, paths ...string) *Tracker {
	t := &Tracker{paths: paths, reconcileInterval: reconcileInterval}
	t.lastReconcile.Store(time.Now().UnixNano())
	return t
}

// Used returns the current cache usage in bytes.
func (t *Tracker) Used() int64 {
	return t.used.Load()
}

// Add applies a signed change in bytes.
func (t *Tracker) Add(delta int64) {
	t.used.Add(delta)
}

// Refresh measures the tracked paths and replaces the running total.
// Changes made while the measurement is in flight are lost, which is inherent
// to reconciling against a moving target.
func (t *Tracker) Refresh() error {
	// record the time of this attempt; don't retry on failure.
	t.lastReconcile.Store(time.Now().UnixNano())
	// check the counter
	beforeMeasurement := t.used.Load()
	// measure
	var measured int64
	for _, path := range t.paths {
		usage, err := common.GetUsage(path)
		if err != nil {
			return fmt.Errorf("failed to measure %s: %w", path, err)
		}
		measured += int64(usage)
	}
	// swap the measured value in, and note the intermediate delta
	beforeSwap := t.used.Swap(measured)
	// log if the drift is significant
	if drift := measured - beforeMeasurement; drift > driftWarnBytes || drift < -driftWarnBytes {
		log.Info(
			"Tracker::Refresh : corrected usage by %d bytes (was %d, now %d)",
			drift,
			beforeMeasurement,
			measured,
		)
	}
	// did a lot of data get added during measurement?
	deltaDuringMeasurement := beforeSwap - beforeMeasurement
	if deltaDuringMeasurement > driftWarnBytes {
		log.Info(
			"Tracker::Refresh : %d bytes were added during measurement. Re-adding...",
			deltaDuringMeasurement,
		)
		t.used.Add(deltaDuringMeasurement)
	}
	return nil
}

// Reconcile refreshes the running total, but no more often than reconcileInterval.
func (t *Tracker) Reconcile() {
	if time.Since(time.Unix(0, t.lastReconcile.Load())) < t.reconcileInterval {
		return
	}
	if err := t.Refresh(); err != nil {
		log.Err("Tracker::Reconcile : %v", err)
	}
}
