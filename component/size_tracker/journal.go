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
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
)

type MountSize struct {
	size              atomic.Uint64
	lastJournaledSize atomic.Uint64
	journalPath       string
	flushTicker       *time.Ticker
	stopCh            chan struct{}
	wg                sync.WaitGroup
}

func CreateSizeJournal(filename string) (*MountSize, error) {
	err := common.CreateDefaultDirectory()
	if err != nil {
		return nil, fmt.Errorf("failed to create default work dir [%s]", err.Error())
	}

	root, err := os.OpenRoot(common.ExpandPath(common.DefaultWorkDir))
	if err != nil {
		return nil, err
	}
	defer root.Close()

	// Short-lived handle to read initial size under lock
	f, err := root.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	if err := exclusiveLock(f.Fd()); err != nil {
		_ = f.Close()
		return nil, err
	}
	var initialSize uint64
	{
		fileInfo, statErr := f.Stat()
		if statErr == nil && fileInfo.Size() >= 8 {
			var buf [8]byte
			if _, readErr := f.ReadAt(buf[:], 0); readErr == nil {
				initialSize = binary.BigEndian.Uint64(buf[:])
			}
		}
	}
	_ = f.Sync()
	_ = unlock(f.Fd())
	_ = f.Close()

	ms := &MountSize{
		journalPath: filename,
	}
	ms.size.Store(initialSize)
	ms.lastJournaledSize.Store(initialSize)

	return ms, nil
}

func (ms *MountSize) runJournalWriter() {
	defer ms.wg.Done()
	for {
		select {
		case <-ms.flushTicker.C:
			if err := ms.flushIfChanged(); err != nil {
				log.Err("SizeTracker::runJournalWriter : flush error: %v", err)
			}
		case <-ms.stopCh:
			_ = ms.flushIfChanged()
			return
		}
	}
}

func (ms *MountSize) GetSize() uint64 {
	return ms.size.Load()
}

func (ms *MountSize) Add(delta uint64) uint64 {
	newVal := ms.size.Add(delta)
	// Persist delta safely across processes
	if err := ms.applyDeltaLocked(int64(delta)); err != nil {
		log.Err("SizeTracker::Add : delta persist failed: %v", err)
	}
	return newVal
}

func (ms *MountSize) Subtract(delta uint64) uint64 {
	for {
		old := ms.size.Load()
		var newVal uint64
		if old < delta {
			newVal = 0
		} else {
			newVal = old - delta
		}
		if ms.size.CompareAndSwap(old, newVal) {
			// Persist delta (negative) safely across processes
			var apply uint64
			if old < delta {
				apply = old
			} else {
				apply = delta
			}
			if err := ms.applyDeltaLocked(-int64(apply)); err != nil {
				log.Err("SizeTracker::Subtract : delta persist failed: %v", err)
			}
			return newVal
		}
	}
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

// applyDeltaLocked performs a cross-process safe read-modify-write of the 8-byte total.
// NOTE: Uses Unix advisory locks (flock). For Windows portability, an OS-specific lock
// implementation should be added.
func (ms *MountSize) applyDeltaLocked(delta int64) error {
	// Open the journal file via OpenRoot to match CreateSizeJournal's path handling
	root, err := os.OpenRoot(common.ExpandPath(common.DefaultWorkDir))
	if err != nil {
		return err
	}
	defer root.Close()

	// Short-lived handle and lock it exclusively
	f, err := root.OpenFile(ms.journalPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := exclusiveLock(f.Fd()); err != nil {
		return err
	}
	defer unlock(f.Fd())

	var buf [8]byte
	// Read current total (treat missing/short as zero)
	if _, err := f.ReadAt(buf[:], 0); err != nil && err != os.ErrNotExist {
		// If file empty treat as zero
		if err != os.ErrNotExist {
			return err
		}
	}
	current := binary.BigEndian.Uint64(buf[:])

	// Apply delta with saturation at 0
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

	binary.BigEndian.PutUint64(buf[:], updated)
	if _, err := f.WriteAt(buf[:], 0); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	ms.lastJournaledSize.Store(updated)
	return nil
}

// flushIfChanged writes current absolute size only if different from last flushed.
// This is a fallback periodic sync (updates are already durable on each delta).
func (ms *MountSize) flushIfChanged() error {
	cur := ms.size.Load()
	last := ms.lastJournaledSize.Load()
	if cur == last {
		return nil
	}
	// Use locked overwrite via delta to avoid racing with other processes doing RMW cycles.
	return ms.applyDeltaLocked(int64(cur) - int64(last))
}

func minUint64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
