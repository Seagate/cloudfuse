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
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
)

type lruNode struct {
	prev *lruNode
	next *lruNode
	name string

	evicting bool
}

type uploadJob struct {
	name string
	wg   *sync.WaitGroup
}

// lruQueue moves local-only objects to cloud storage.
// File locks may be held before mu, never after it.
type lruQueue struct {
	// mu guards the list and its nodes.
	mu   sync.Mutex
	head *lruNode
	tail *lruNode
	// nodeMap is written under mu, but may be read without it.
	nodeMap sync.Map

	// evictMu serializes eviction cycles.
	evictMu sync.Mutex

	checkerWg sync.WaitGroup
	workerWg  sync.WaitGroup
	stopOnce  sync.Once

	uploadChan chan uploadJob
	doneChan   chan struct{}

	cachePath string
	fileLocks *common.LockMap // uses object name (common.JoinUnixFilepath)
	size      *cacheSizeTracker

	maxCacheSize float64
	// threshold and targetRatio are fractions of maxCacheSize.
	threshold    float64
	targetRatio  float64
	numWorkers   int
	maxEviction  uint32
	pollInterval time.Duration

	// Called with the object's file lock held.
	uploadandCleanFn func(name string) error
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
		return fmt.Errorf("lruQueue: cache size tracker not set")
	}
	if q.pollInterval <= 0 {
		q.pollInterval = capacityPollInterval
	}
	if q.maxEviction == 0 {
		q.maxEviction = defaultMaxEviction
	}

	q.doneChan = make(chan struct{})
	q.uploadChan = make(chan uploadJob)

	q.workerWg.Add(q.numWorkers)
	for range q.numWorkers {
		go q.worker()
	}

	q.checkerWg.Add(1)
	go q.capacityChecker()

	return nil
}

// StopPolicy waits for in-flight uploads but leaves queued files local.
func (q *lruQueue) StopPolicy() error {
	q.stopOnce.Do(func() {
		if q.doneChan == nil {
			return
		}
		close(q.doneChan)
		q.checkerWg.Wait()

		// Prevent EvictNow from sending while uploadChan is closed.
		q.evictMu.Lock()
		close(q.uploadChan)
		q.evictMu.Unlock()

		q.workerWg.Wait()
	})
	return nil
}

// Enqueue adds or promotes an object unless a worker owns it.
func (q *lruQueue) Enqueue(name string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	val, found := q.nodeMap.Load(name)
	if !found {
		node := &lruNode{name: name}
		q.nodeMap.Store(name, node)
		q.setHead(node)
		return
	}

	node := val.(*lruNode)
	if node.evicting {
		return
	}
	q.extractNode(node)
	q.setHead(node)
}

// Dequeue removes an object and cancels any in-flight eviction.
func (q *lruQueue) Dequeue(name string) {
	log.Trace("lruQueue::Dequeue : %s", name)

	q.mu.Lock()
	defer q.mu.Unlock()

	val, found := q.nodeMap.LoadAndDelete(name)
	if !found {
		return
	}

	node := val.(*lruNode)
	if !node.evicting {
		q.extractNode(node)
	}
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
func (q *lruQueue) setTail(node *lruNode) {
	node.next = nil
	node.prev = q.tail
	if q.tail != nil {
		q.tail.next = node
	}
	q.tail = node
	if q.head == nil {
		q.head = node
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

func (q *lruQueue) capacityChecker() {
	defer q.checkerWg.Done()

	ticker := time.NewTicker(q.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-q.doneChan:
			return
		case <-ticker.C:
			q.size.Reconcile()
			if float64(q.size.Used()) <= q.maxCacheSize*q.threshold {
				continue
			}
			q.evictDownTo(int64(q.maxCacheSize * q.targetRatio))
		}
	}
}

// EvictNow synchronously makes room for growBytes.
func (q *lruQueue) EvictNow(growBytes int64) bool {
	target := min(int64(q.maxCacheSize*q.targetRatio), int64(q.maxCacheSize)-growBytes)
	q.evictDownTo(target)
	return float64(q.size.Used()+growBytes) <= q.maxCacheSize
}

// evictDownTo runs one eviction pass toward targetUsed.
func (q *lruQueue) evictDownTo(targetUsed int64) {
	q.evictMu.Lock()
	defer q.evictMu.Unlock()

	select {
	case <-q.doneChan:
		return
	default:
	}

	used := q.size.Used()
	if used <= targetUsed {
		return
	}

	names := q.nominate(used-targetUsed, int(q.maxEviction))
	if len(names) == 0 {
		log.Info("lruQueue::evictDownTo : nothing is eligible for eviction")
		return
	}

	q.runBatch(names)
	if usedAfter := q.size.Used(); float64(usedAfter) > q.maxCacheSize*q.threshold {
		log.Err(
			"lruQueue::evictDownTo : usage remains above high threshold after eviction (using %d, high threshold %.0f)",
			usedAfter,
			q.maxCacheSize*q.threshold,
		)
	}
}

func (q *lruQueue) runBatch(names []string) {
	var wg sync.WaitGroup

	wg.Add(len(names))
	for i, name := range names {
		select {
		case q.uploadChan <- uploadJob{name: name, wg: &wg}:
		case <-q.doneChan:
			for _, undelivered := range names[i:] {
				q.requeue(undelivered, false)
				wg.Done()
			}
			wg.Wait()
			return
		}
	}
	wg.Wait()
}

// nominate claims enough least-recently-used files to cover bytesNeeded.
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
		} else {
			info, err := os.Stat(filepath.Join(q.cachePath, node.name))
			if err != nil {
				log.Warn("lruQueue::nominate : dropping %s [%v]", node.name, err)
				q.extractNode(node)
				q.nodeMap.Delete(node.name)
			} else {
				q.extractNode(node)
				node.evicting = true
				names = append(names, node.name)
				total += info.Size()
			}
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

func (q *lruQueue) claimed(name string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	val, found := q.nodeMap.Load(name)
	return found && val.(*lruNode).evicting
}

// Failed uploads return to the tail; busy files return to the head.
func (q *lruQueue) requeue(name string, failed bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	val, found := q.nodeMap.Load(name)
	if !found {
		return
	}
	node := val.(*lruNode)

	node.evicting = false
	if failed {
		q.setTail(node)
		return
	}
	q.setHead(node)
}

func (q *lruQueue) drop(name string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.nodeMap.Delete(name)
}

func (q *lruQueue) worker() {
	defer q.workerWg.Done()
	for job := range q.uploadChan {
		q.evictOne(job)
	}
}

func (q *lruQueue) evictOne(job uploadJob) {
	defer job.wg.Done()

	flock := q.fileLocks.Get(job.name)
	if !flock.TryLock() {
		q.requeue(job.name, false)
		return
	}
	defer flock.Unlock()

	// Dequeue may cancel the job while it waits for a worker.
	if !q.claimed(job.name) {
		q.drop(job.name)
		return
	}

	if flock.Count() > 0 {
		q.requeue(job.name, false)
		return
	}

	_, err := os.Stat(filepath.Join(q.cachePath, job.name))
	if err != nil {
		log.Warn("lruQueue::evictOne : %s has no local copy, dropping it [%v]", job.name, err)
		q.drop(job.name)
		return
	}

	if err := q.uploadandCleanFn(job.name); err != nil {
		log.Err("lruQueue::evictOne : %s upload failed [%v]", job.name, err)
		q.requeue(job.name, true)
		return
	}

	q.drop(job.name)
}
