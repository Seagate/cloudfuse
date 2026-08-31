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

const (
	// uploadQueueDepthPerWorker sizes the job channel. It only needs enough
	// slack that the dispatcher is not serialised against the workers.
	uploadQueueDepthPerWorker = 16
)

// nodeState records whether the queue or an eviction worker owns a node.
type nodeState uint8

const (
	// nodeQueued: linked into the list and eligible for eviction.
	nodeQueued nodeState = iota
	// nodeEvicting: unlinked and owned by a worker. The node deliberately stays
	// in nodeMap so a rename or delete can still find it and cancel the
	// eviction. Dropping it from the map here instead would leave a renamed
	// object in no queue at all, never to be uploaded or evicted again.
	nodeEvicting
	// nodeCancelled: the object was deleted or renamed while a worker owned it.
	// The worker must skip the upload and drop the node.
	nodeCancelled
)

type lruNode struct {
	prev *lruNode
	next *lruNode
	name string

	state nodeState
}

// uploadJob is one nominated object. The waitgroup belongs to the eviction
// pass that nominated it, so a pass waits only for its own batch.
type uploadJob struct {
	name string
	wg   *sync.WaitGroup
}

// lruQueue decides which local-only objects move to cloud storage.
//
// Locking: callers may hold a file lock and then take mu. The reverse is
// forbidden - see the lock ordering note in tiered_storage.go. Nomination
// therefore consults handle counts (which are atomic) instead of taking file
// locks, and workers use TryLock so they never block on user I/O.
type lruQueue struct {
	// mu guards head, tail and every field of every lruNode.
	mu   sync.Mutex
	head *lruNode
	tail *lruNode
	// nodeMap indexes nodes by object name. It is only written under mu; it is
	// a sync.Map so callers can test membership without taking mu.
	nodeMap sync.Map

	// evictMu serialises eviction cycles so two cycles cannot nominate the same
	// objects or interleave their batches.
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
	// eviction starts above threshold*maxCacheSize and runs until usage reaches
	// targetRatio*maxCacheSize, both fractions in (0,1]
	threshold    float64
	targetRatio  float64
	numWorkers   int
	maxEviction  uint32
	pollInterval time.Duration

	// uploads the object to cloud storage and removes the local copy.
	// Supplied by TieredStorage; called with the object's file lock held.
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

	q.head = nil
	q.tail = nil
	q.doneChan = make(chan struct{})
	q.uploadChan = make(chan uploadJob, q.numWorkers*uploadQueueDepthPerWorker)

	q.workerWg.Add(q.numWorkers)
	for range q.numWorkers {
		go q.worker()
	}

	q.checkerWg.Add(1)
	go q.capacityChecker()

	return nil
}

// StopPolicy shuts the policy down and waits for uploads already in flight. It
// does not drain the queue: local-only data is authoritative, so whatever has
// not been evicted stays in the cache for the next mount.
func (q *lruQueue) StopPolicy() error {
	q.stopOnce.Do(func() {
		if q.doneChan == nil {
			return
		}
		close(q.doneChan)
		q.checkerWg.Wait()

		// Block new eviction cycles before retiring the workers, so a
		// concurrent EvictNow cannot send on a closed channel.
		q.evictMu.Lock()
		close(q.uploadChan)
		q.evictMu.Unlock()

		q.workerWg.Wait()
	})
	return nil
}

func (q *lruQueue) Touch(name string) {
	q.Enqueue(name)
}

// Enqueue makes name the most recently used object, adding it if it is new.
// A node that a worker owns is left alone; the worker re-queues it itself if
// the upload does not happen.
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
	if node.state != nodeQueued {
		return
	}
	q.extractNode(node)
	q.setHead(node)
}

// Dequeue removes name from the queue. If a worker is currently evicting the
// object then the eviction is cancelled instead, and the worker drops the node
// once it notices.
func (q *lruQueue) Dequeue(name string) {
	log.Trace("lruQueue::Dequeue : %s", name)

	q.mu.Lock()
	defer q.mu.Unlock()

	val, found := q.nodeMap.Load(name)
	if !found {
		return
	}

	node := val.(*lruNode)
	if node.state == nodeEvicting {
		node.state = nodeCancelled
		return
	}

	q.extractNode(node)
	q.nodeMap.Delete(name)
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
	//tail case
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

func (q *lruQueue) stopping() bool {
	select {
	case <-q.doneChan:
		return true
	default:
		return false
	}
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

// EvictNow synchronously makes room for growBytes more bytes of local data and
// reports whether the cache has room afterwards. Callers may hold a file lock:
// workers never block on file locks, so an object cannot deadlock against its
// own eviction.
func (q *lruQueue) EvictNow(growBytes int64) bool {
	target := min(int64(q.maxCacheSize*q.targetRatio), int64(q.maxCacheSize)-growBytes)
	q.evictDownTo(target)
	return float64(q.size.Used()+growBytes) <= q.maxCacheSize
}

// evictDownTo runs one eviction pass, nominating enough least-recently-used
// objects to reach targetUsed. If the pass does not bring usage below the high
// threshold, a later capacity check may try again.
func (q *lruQueue) evictDownTo(targetUsed int64) {
	q.evictMu.Lock()
	defer q.evictMu.Unlock()

	if q.stopping() {
		return
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

// runBatch hands every name to the worker pool and waits for that batch to
// finish.
func (q *lruQueue) runBatch(names []string) {
	var wg sync.WaitGroup

	wg.Add(len(names))
	for i, name := range names {
		select {
		case q.uploadChan <- uploadJob{name: name, wg: &wg}:
		case <-q.doneChan:
			// shutting down - hand back everything we could not dispatch
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

// nominate detaches least-recently-used nodes until their combined size covers
// bytesNeeded, marking each as owned by a worker. Objects with open handles are
// promoted to the head instead: they are by definition in use, and promoting
// them stops the next pass from considering them again.
//
// Each candidate is measured here, under mu. That is a syscall per nominated
// object while the queue is locked, but the count is bounded by the deficit
// being covered, and it keeps nomination from having to reach back into the
// component's file map.
func (q *lruQueue) nominate(bytesNeeded int64, limit int) []string {
	q.mu.Lock()
	defer q.mu.Unlock()

	var names []string
	var busy []*lruNode
	var total int64

	for node := q.tail; node != nil && len(names) < limit && total < bytesNeeded; {
		// remember where to go next before the node is unlinked
		prev := node.prev

		switch {
		case node.state != nodeQueued:
			// a worker owns it; it should not be in the list at all

		case q.fileLocks.Get(node.name).Count() > 0:
			busy = append(busy, node)

		default:
			info, err := os.Stat(filepath.Join(q.cachePath, node.name))
			if err != nil {
				// there is no local copy left to evict
				log.Warn("lruQueue::nominate : dropping %s [%v]", node.name, err)
				q.extractNode(node)
				q.nodeMap.Delete(node.name)
				break
			}
			q.extractNode(node)
			node.state = nodeEvicting
			names = append(names, node.name)
			total += info.Size()
		}

		node = prev
	}

	// Promote busy nodes after the walk, not during it: re-linking a node while
	// traversing can lead the walk back over nodes it already visited. The list
	// is walked oldest-first, so promoting in that order preserves their
	// relative recency.
	for _, node := range busy {
		q.extractNode(node)
		q.setHead(node)
	}

	return names
}

// claimed reports whether the worker may still upload name.
func (q *lruQueue) claimed(name string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	val, found := q.nodeMap.Load(name)
	return found && val.(*lruNode).state == nodeEvicting
}

// requeue returns a nominated node to the queue. A failed upload goes to the
// tail for a future eviction cycle. An object that was merely busy goes to the
// head, because being in use is what the head of an LRU means.
func (q *lruQueue) requeue(name string, failed bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// get the node
	val, found := q.nodeMap.Load(name)
	if !found {
		return
	}
	node := val.(*lruNode)

	// was the file deleted or renamed?
	if node.state == nodeCancelled {
		q.nodeMap.Delete(name)
		return
	}

	node.state = nodeQueued
	if failed {
		q.setTail(node)
		return
	}
	q.setHead(node)
}

// drop forgets a node whose local copy no longer exists.
func (q *lruQueue) drop(name string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if val, found := q.nodeMap.Load(name); found {
		q.extractNode(val.(*lruNode))
	}
	q.nodeMap.Delete(name)
}
func (q *lruQueue) worker() {
	defer q.workerWg.Done()
	for job := range q.uploadChan {
		q.evictOne(job)
	}
}

// evictOne uploads one object and removes its local copy. It never blocks on a
// file lock: an object that user I/O has picked up in the meantime goes back
// into the queue and is reconsidered on a later pass.
func (q *lruQueue) evictOne(job uploadJob) {
	defer job.wg.Done()

	flock := q.fileLocks.Get(job.name)
	if !flock.TryLock() {
		q.requeue(job.name, false)
		return
	}
	defer flock.Unlock()

	// The object may have been deleted or renamed while the job sat in the
	// channel. Uploading it now would resurrect data the user removed.
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
