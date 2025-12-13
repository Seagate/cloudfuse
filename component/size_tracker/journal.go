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
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
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
	epoch        atomic.Uint64 // tracks mount epoch to detect mismatches
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
	log.Trace("SizeTracker::Add : %d", delta)
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

func (ms *MountSize) IncrementEpoch() {
	ms.epoch.Add(1)
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

	// Read current values from INI-style text file. Treat missing values as defaults.
	// version=1
	// epoch=<uint64>
	// size_bytes=<uint64>
	// updated_unix_ms=<int64>
	var currentSize, fileEpoch uint64
	// Read entire file
	stat, statErr := f.Stat()
	if statErr != nil {
		return statErr
	}
	var data []byte
	if stat.Size() > 0 {
		// Legacy single-value journal: if file size is 8 bytes and we're initializing (epoch==0), read it.
		if stat.Size() == 8 && ms.epoch.Load() == 0 {
			var buf8 [8]byte
			if _, err := f.ReadAt(buf8[:], 0); err != nil && err != io.EOF {
				return err
			}
			currentSize = binary.BigEndian.Uint64(buf8[:])
		} else {
			data = make([]byte, stat.Size())
			if _, err := f.ReadAt(data, 0); err != nil && err != io.EOF {
				return err
			}
			// Parse simple key=value lines
			lines := bytes.Split(data, []byte("\n"))
			for _, ln := range lines {
				ln = bytes.TrimSpace(ln)
				if len(ln) == 0 ||
					bytes.HasPrefix(ln, []byte("#")) ||
					bytes.HasPrefix(ln, []byte(";")) {
					continue
				}
				kv := bytes.SplitN(ln, []byte("="), 2)
				if len(kv) != 2 {
					continue
				}
				key := string(bytes.TrimSpace(kv[0]))
				val := string(bytes.TrimSpace(kv[1]))
				switch key {
				case "version":
					// currently unused beyond presence; could validate == 1
				case "epoch":
					if u, perr := parseUint64(val); perr == nil {
						fileEpoch = u
					}
				case "size_bytes":
					if u, perr := parseUint64(val); perr == nil {
						currentSize = u
					}
				case "updated_unix_ms":
					// parsed but not used currently; skip
				}
			}
		}
	}

	myEpoch := ms.epoch.Load()
	delta := ms.pendingDelta.Load()
	ms.size.Store(currentSize)
	if myEpoch < fileEpoch {
		// Epoch changed externally: discard our delta and adopt the new epoch.
		if delta != 0 {
			log.Debug(
				"SizeTracker::sync : epoch changed (local=%d -> file=%d) — discarding delta %d.",
				myEpoch,
				fileEpoch,
				delta,
			)
			ms.pendingDelta.Add(-delta)
			delta = 0
		}
		ms.epoch.Store(fileEpoch)
	}

	// make sure epoch is nonzero
	ms.epoch.CompareAndSwap(0, 1)

	// Compute updated size (bottom out at zero)
	var updated uint64
	if delta < 0 {
		dec := uint64(-delta)
		if currentSize < dec {
			updated = 0
		} else {
			updated = currentSize - dec
		}
	} else {
		updated = currentSize + uint64(delta)
	}

	// Prepare new file contents
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "version=1\n")
	fmt.Fprintf(buf, "epoch=%d\n", ms.epoch.Load())
	fmt.Fprintf(buf, "size_bytes=%d\n", updated)
	fmt.Fprintf(buf, "updated_unix_ms=%d\n", time.Now().UnixMilli())

	// Overwrite file atomically-ish: seek to start and write new contents, then truncate
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		return err
	}
	if err := f.Truncate(int64(buf.Len())); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}

	// update state
	ms.size.Store(updated)
	// clear / update pendingDelta
	if delta != 0 {
		ms.pendingDelta.Add(-delta)
	}

	return nil
}

// parse helpers
func parseUint64(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// flushIfChanged writes current absolute size only if different from last flushed.
// This is a fallback periodic sync (updates are already durable on each delta).
func (ms *MountSize) flushIfChanged() error {
	log.Debug("SizeTracker::flushIfChanged : syncing delta: %d.", ms.pendingDelta.Load())
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
