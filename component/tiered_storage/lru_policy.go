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
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/cache_size"
	"github.com/Seagate/cloudfuse/common/log"
)

type lruNode struct {
	prev *lruNode
	next *lruNode
	name string
}

// lruQueueConfig holds the fields needed to construct an lruQueue.
type lruQueueConfig struct {
	localRoot string          // local/ beneath the tiered root
	fileLocks *common.LockMap // uses object name (common.JoinUnixFilepath)
	size      *cache_size.Tracker

	maxSize float64
	// threshold and targetRatio are fractions of maxSize.
	threshold    float64
	targetRatio  float64
	numWorkers   int
	maxEviction  uint32
	pollInterval time.Duration

	// Called with the object's file lock held.
	uploadandCleanFn func(name string) error
}

// lruQueue moves local-only objects to cloud storage. Threshold passes and
// evict-on-close each run on their own goroutine, which uploads up to
// numWorkers files at a time.
// File locks may be held before mu, never after it.
type lruQueue struct {
	mu         sync.Mutex
	nodes      map[string]*lruNode
	head, tail *lruNode
	// closed holds evict-on-close requests not yet handled.
	closed map[string]struct{}

	wake chan struct{}
	stop chan struct{}
	wg   sync.WaitGroup

	lruQueueConfig
}

func newLRUQueue(cfg lruQueueConfig) *lruQueue {
	return &lruQueue{
		nodes:          make(map[string]*lruNode),
		closed:         make(map[string]struct{}),
		wake:           make(chan struct{}, 1),
		lruQueueConfig: cfg,
	}
}

func (q *lruQueue) StartPolicy() error {
	if q.uploadandCleanFn == nil {
		return fmt.Errorf("lruQueue: upload function not set")
	}
	if q.numWorkers <= 0 {
		return fmt.Errorf("lruQueue: numWorkers must be > 0")
	}
	if q.fileLocks == nil {
		return fmt.Errorf("lruQueue: file locks not set")
	}
	if q.size == nil {
		return fmt.Errorf("lruQueue: disk usage tracker not set")
	}
	if q.pollInterval <= 0 {
		q.pollInterval = capacityPollInterval
	}
	if q.maxEviction == 0 {
		q.maxEviction = defaultMaxEviction
	}

	q.stop = make(chan struct{})

	// start eviction goroutines
	q.wg.Go(q.watchCapacity)
	q.wg.Go(q.watchCloses)

	return nil
}

// StopPolicy waits for in-flight uploads but leaves queued files local.
func (q *lruQueue) StopPolicy() error {
	if q.stop == nil {
		return nil
	}
	close(q.stop)
	q.wg.Wait()
	return nil
}

func (q *lruQueue) stopping() bool {
	select {
	case <-q.stop:
		return true
	default:
		return false
	}
}

func (q *lruQueue) watchCapacity() {
	ticker := time.NewTicker(q.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-q.stop:
			return
		case <-ticker.C:
			if err := q.size.Reconcile(); err != nil {
				log.Err("lruQueue::watchCapacity : failed to measure disk usage [%v]", err)
			}
			if float64(q.size.Used()) > q.maxSize*q.threshold {
				q.evictDownTo(int64(q.maxSize * q.targetRatio))
			}
		}
	}
}

func (q *lruQueue) watchCloses() {
	for {
		select {
		case <-q.stop:
			return
		case <-q.wake:
			q.evictClosed()
		}
	}
}

// EvictOnClose asks for name to be evicted ahead of its LRU turn, if it is
// still closed and cloudStale when its turn comes. It does not block.
func (q *lruQueue) EvictOnClose(name string) {
	q.mu.Lock()
	q.closed[name] = struct{}{}
	q.mu.Unlock()

	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *lruQueue) evictClosed() {
	q.mu.Lock()
	names := slices.Collect(maps.Keys(q.closed))
	q.mu.Unlock()

	q.evictAll(names, true)
}

// evictDownTo runs one eviction pass toward targetUsed.
func (q *lruQueue) evictDownTo(targetUsed int64) {
	used := q.size.Used()
	if used <= targetUsed {
		return
	}

	names := q.nominate(used-targetUsed, int(q.maxEviction))
	if len(names) == 0 {
		log.Info("lruQueue::evictDownTo : nothing is eligible for eviction")
		return
	}

	q.evictAll(names, false)
	if usedAfter := q.size.Used(); float64(usedAfter) > q.maxSize*q.threshold {
		log.Err(
			"lruQueue::evictDownTo : usage remains above high threshold after eviction (using %d, high threshold %.0f)",
			usedAfter,
			q.maxSize*q.threshold,
		)
	}
}

// evictAll evicts names, numWorkers at a time. Once stopping, it starts no
// more uploads, but waits for those already running.
func (q *lruQueue) evictAll(names []string, onClose bool) {
	var wg sync.WaitGroup
	// use the channel buffer to limit concurrent evictions
	sem := make(chan struct{}, q.numWorkers)
	for _, name := range names {
		sem <- struct{}{}
		// stop may have been called while we waited for a slot
		if q.stopping() {
			break
		}
		wg.Go(func() {
			q.evict(name, onClose)
			<-sem
		})
	}
	wg.Wait()
}

// evict uploads and removes local/name unless it is open. An evict-on-close
// also requires cloudStale. On failure, the file stays queued.
func (q *lruQueue) evict(name string, onClose bool) {
	flock := q.fileLocks.Get(name)
	flock.Lock()
	defer flock.Unlock()

	if onClose {
		q.mu.Lock()
		delete(q.closed, name)
		q.mu.Unlock()
		if !flock.CloudStale {
			return
		}
	}
	if flock.Count() > 0 {
		return
	}
	if _, err := os.Stat(filepath.Join(q.localRoot, name)); err != nil {
		return
	}

	if err := q.uploadandCleanFn(name); err != nil {
		log.Err("lruQueue::evict : %s upload failed [%v]", name, err)
		return
	}
	q.Dequeue(name)
}

// nominate picks least recently used closed files covering bytesNeeded.
// Open files are promoted instead.
func (q *lruQueue) nominate(bytesNeeded int64, limit int) []string {
	q.mu.Lock()
	defer q.mu.Unlock()

	var names []string
	var busy []*lruNode
	var total int64

	for node := q.tail; node != nil && len(names) < limit && total < bytesNeeded; {
		prev := node.prev

		if q.fileLocks.Get(node.name).Count() > 0 {
			busy = append(busy, node)
		} else if info, err := os.Stat(filepath.Join(q.localRoot, node.name)); err != nil {
			log.Warn("lruQueue::nominate : dropping %s [%v]", node.name, err)
			q.extractNode(node)
			delete(q.nodes, node.name)
		} else {
			names = append(names, node.name)
			total += info.Size()
		}

		node = prev
	}

	// Relink after walking so traversal cannot revisit promoted nodes.
	for _, node := range busy {
		q.extractNode(node)
		q.setHead(node)
	}

	return names
}

// Enqueue adds or promotes an object.
func (q *lruQueue) Enqueue(name string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	node, found := q.nodes[name]
	if found {
		q.extractNode(node)
	} else {
		node = &lruNode{name: name}
		q.nodes[name] = node
	}
	q.setHead(node)
}

// Dequeue removes an object.
func (q *lruQueue) Dequeue(name string) {
	log.Trace("lruQueue::Dequeue : %s", name)

	q.mu.Lock()
	defer q.mu.Unlock()

	if node, found := q.nodes[name]; found {
		q.extractNode(node)
		delete(q.nodes, name)
	}
}

// Rename moves src's entry to dst, keeping its LRU position. Any dst entry is
// discarded.
func (q *lruQueue) Rename(src, dst string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if node, found := q.nodes[dst]; found {
		q.extractNode(node)
		delete(q.nodes, dst)
	}
	node, found := q.nodes[src]
	if !found {
		return
	}
	delete(q.nodes, src)
	node.name = dst
	q.nodes[dst] = node
}

// mu must be held
func (q *lruQueue) setHead(node *lruNode) {
	node.prev = nil
	node.next = q.head
	if q.head != nil {
		q.head.prev = node
	}
	q.head = node
	if q.tail == nil {
		q.tail = node
	}
}

// mu must be held
func (q *lruQueue) extractNode(node *lruNode) {
	if node == q.head {
		q.head = node.next
	}
	if node == q.tail {
		q.tail = node.prev
	}

	if node.next != nil {
		node.next.prev = node.prev
	}
	if node.prev != nil {
		node.prev.next = node.next
	}
	node.prev = nil
	node.next = nil
}
