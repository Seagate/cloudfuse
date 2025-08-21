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
	file              *os.File
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

	f, err := root.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	fileInfo, err := f.Stat()
	if err != nil {
		return nil, err
	}

	var initialSize uint64

	if fileInfo.Size() >= 8 {
		buf := make([]byte, 8)
		if _, err := f.ReadAt(buf, 0); err != nil {
			_ = f.Close()
			return nil, err
		}

		initialSize = binary.BigEndian.Uint64(buf)
	}

	ms := &MountSize{
		file:        f,
		flushTicker: time.NewTicker(10 * time.Second),
		stopCh:      make(chan struct{}),
	}
	ms.size.Store(initialSize)

	// Use a wait group to ensure that the background close finishes before the go routine ends
	ms.wg.Add(1)
	go ms.runJournalWriter()

	return ms, nil
}

func (ms *MountSize) runJournalWriter() {
	defer ms.wg.Done()
	for {
		select {
		case <-ms.flushTicker.C:
			if err := ms.writeSizeToFile(); err != nil {
				log.Err("SizeTracker::runJournalWriter : Unable to journal size. Error: %v", err)
			}
		case <-ms.stopCh:
			if err := ms.writeSizeToFile(); err != nil {
				log.Err(
					"SizeTracker::runJournalWriter : Unable to journal final size before closing channel. Error: %v",
					err,
				)
			}
			return
		}
	}
}

func (ms *MountSize) GetSize() uint64 {
	return ms.size.Load()
}

func (ms *MountSize) Add(delta uint64) uint64 {
	return ms.size.Add(delta)
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
			return newVal
		}
	}
}

func (ms *MountSize) CloseFile() error {
	close(ms.stopCh)
	ms.flushTicker.Stop()
	ms.wg.Wait()
	return ms.file.Close()
}

func (ms *MountSize) writeSizeToFile() error {
	old_size := ms.lastJournaledSize.Load()
	currentSize := ms.size.Load()
	if old_size == currentSize {
		// No change in size, no need to write
		return nil
	}

	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], currentSize)

	if _, err := ms.file.WriteAt(buf[:], 0); err != nil {
		return err
	}

	if err := ms.file.Sync(); err != nil {
		return err
	}
	ms.lastJournaledSize.Store(currentSize)

	return nil
}
