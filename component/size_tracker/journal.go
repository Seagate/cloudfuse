/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates

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
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
)

type MountSize struct {
	size         atomic.Uint64 // cached copy of size from file
	pendingDelta atomic.Int64  // net change not yet written to the file
	journalPath  string
	flushTicker  *time.Ticker
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

func CreateSizeJournal(filename string) (*MountSize, error) {
	err := common.CreateDefaultDirectory()
	if err != nil {
		return nil, fmt.Errorf("failed to create default work dir [%s]", err.Error())
	}
	ms := &MountSize{
		journalPath: filename,
	}
	if err := ms.sync(); err != nil {
		return nil, err
	}

	return ms, nil
}

func (ms *MountSize) runJournalWriter() {
	defer ms.wg.Done()
	for {
		select {
		case <-ms.flushTicker.C:
			_ = ms.flushIfChanged()
		case <-ms.stopCh:
			if err := ms.flushIfChanged(); err != nil {
				log.Err("SizeTracker::runJournalWriter : Failed to sync when stopping.")
			}
			return
		}
	}
}

func (ms *MountSize) GetSize() uint64 {
	return ms.size.Load()
}

func (ms *MountSize) Add(delta int64) int64 {
	return ms.pendingDelta.Add(delta)
}

func (ms *MountSize) Start() {
	// create stop signal
	ms.stopCh = make(chan struct{})
	// start ticker
	ms.flushTicker = time.NewTicker(10 * time.Second)
	// start listening for the flush ticker
	// Use a wait group to ensure that the background close finishes before the go routine ends
	ms.wg.Add(1)
	go ms.runJournalWriter()
}

func (ms *MountSize) Stop() error {
	close(ms.stopCh)
	ms.flushTicker.Stop()
	ms.wg.Wait()
	return nil
}

// safely read from and update the size file
func (ms *MountSize) sync() error {
	// Open the journal's root
	root, err := os.OpenRoot(common.ExpandPath(common.DefaultWorkDir))
	if err != nil {
		return err
	}
	defer root.Close()

	// Get a short-lived handle and lock it exclusively
	f, err := root.OpenFile(ms.journalPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := exclusiveLock(f.Fd()); err != nil {
		return err
	}
	defer unlock(f.Fd())

	// Read current total (treat missing/short as zero)
	var current uint64
	var buf [8]byte
	_, err = f.ReadAt(buf[:], 0)
	switch err {
	case nil:
		// update local copy
		current = binary.BigEndian.Uint64(buf[:])
	case io.EOF:
		// Treat empty and short read as zero
	default:
		// return I/O errors without changing ms.size
		return err
	}
	ms.size.Store(current)

	// get pending delta
	delta := ms.pendingDelta.Load()
	if delta == 0 {
		return nil
	}

	// bottom out at zero
	var updated uint64
	if delta < 0 {
		dec := uint64(-delta)
		if current < dec {
			updated = 0
		} else {
			updated = current - dec
		}
	} else {
		updated = current + uint64(delta)
	}

	// write updated size to file
	binary.BigEndian.PutUint64(buf[:], updated)
	if _, err := f.WriteAt(buf[:], 0); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}

	// update state
	ms.size.Store(updated)
	// clear / update pendingDelta
	ms.pendingDelta.Add(-delta)

	return nil
}

// flushIfChanged writes current absolute size only if different from last flushed.
// This is a fallback periodic sync (updates are already durable on each delta).
func (ms *MountSize) flushIfChanged() error {
	log.Err("SizeTracker::flushIfChanged : syncing delta: %u.", ms.pendingDelta.Load())
	if ms.pendingDelta.Load() == 0 {
		return nil
	}
	// Use locked overwrite via delta to avoid racing with other processes doing RMW cycles.
	err := ms.sync()
	if err != nil {
		log.Err(
			"SizeTracker::flushIfChanged : sync failed (with delta %u). Here's why: %v",
			ms.pendingDelta.Load(),
			err,
		)
	}
	return err
}
