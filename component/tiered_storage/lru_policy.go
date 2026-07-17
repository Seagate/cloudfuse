package tiered_storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
)

type lruNode struct {
	prev *lruNode
	next *lruNode
	name string
}

//upload 50 files and then check

type lruQueue struct {
	mu sync.Mutex

	nodeMap sync.Map

	wg            sync.WaitGroup // tracks capacityChecker only
	workerWg      sync.WaitGroup // tracks worker goroutines
	numWorkers    int
	activeWorkers int32

	head *lruNode
	tail *lruNode

	uploadChan   chan string
	doneChan     chan struct{}
	hallPassChan chan bool

	cachePath         string
	maxCacheSize      float64
	totalUploadedSize int64

	threshold   float64
	targetRatio float64

	tickerUnit time.Duration

	fileLocks *common.LockMap // uses object name (common.JoinUnixFilepath)

	//Functions to wire later into tiered_storage package
	//upload function from tiered storage WIRE THIS LATER in tiered storage because we are using a function from there
	uploadFn func(name string) error

	//this function is just used for testing
	FileHasOpenFileHandle func(name string) bool

	//policy.isFileInUse = func(name string) bool {
	//	return c.fileLocks.Get(name).Count() > 0
	//}

	//we also wire a cleanup function to delete from FileMap and Local
	cleanupFn func(name string) error

	// //delete locally
	// 		localPath := filepath.Join(c.tmpPath, options.Name)
	// 		c.mu.Lock()
	// 		delete(c.fileMap, options.Name)
	// 		c.mu.Unlock()
	// 		os.Remove(localPath)

}

func (q *lruQueue) StartPolicy() error {
	if q.uploadFn == nil {
		return fmt.Errorf("lruQueue: upload function not set")
	}
	if q.numWorkers <= 0 {
		return fmt.Errorf("lruQueue: numWorkers must be > 0")
	}
	//initialize queue
	q.head = nil
	q.tail = nil
	//channels
	q.doneChan = make(chan struct{})
	q.hallPassChan = make(chan bool, 1)
	q.hallPassChan <- true

	//timer
	//go routines
	q.wg.Add(1)
	go q.capacityChecker()

	return nil
}

func (q *lruQueue) StopPolicy() error {
	close(q.doneChan)
	// Wait for capacityChecker to exit — its deferred close(uploadChan) fires here,
	// signalling workers that no more jobs are coming.
	q.wg.Wait()
	// Now wait for workers to drain whatever remains in uploadChan.
	q.workerWg.Wait()
	return nil
}

func (q *lruQueue) Touch(name string) {
	q.Enqueue(name)
}

func (q *lruQueue) Enqueue(name string) {
	//Maybe have a duplicate , that touches essentially

	//lock earlier
	q.mu.Lock()
	defer q.mu.Unlock()

	//create node
	newNode := &lruNode{name: name}
	val, found := q.nodeMap.LoadOrStore(name, newNode)
	node := val.(*lruNode)

	if found {
		// touch
		q.extractNode(node)
	} else {
		// brand new node — update tail if list was empty
		if q.tail == nil {
			q.tail = node
		}
	}
	q.setHead(node)
}

func (q *lruQueue) Dequeue(name string) {
	log.Trace("lruPolicy::removeNode : %s", name)

	q.mu.Lock()
	defer q.mu.Unlock()

	val, found := q.nodeMap.LoadAndDelete(name)
	if !found || val == nil {
		return
	}

	node := val.(*lruNode)

	q.extractNode(node)
}

func (q *lruQueue) setHead(node *lruNode) {
	// insert node at the head
	node.prev = nil
	node.next = q.head
	if q.head != nil {
		q.head.prev = node
	}
	q.head = node
}

func (q *lruQueue) extractNode(node *lruNode) {
	// remove the node from its position in the list

	// head case
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

func (q *lruQueue) capacityChecker() {
	defer q.wg.Done()
	defer close(q.uploadChan)

	ticker := time.NewTicker(q.tickerUnit)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// eviction
			select {
			case <-q.hallPassChan:

				//1. Check if we need eviction
				curSize, err := common.GetUsage(q.cachePath)
				if err != nil {
					log.Err("lruPolicy::capacityChecker : failed to get usage: %v", err)
					q.hallPassChan <- true
					continue
				}
				if curSize/q.maxCacheSize <= q.threshold {
					q.hallPassChan <- true
					break
				}
				//targetRatio should always be less than thresholdRatio

				//2. Find difference to evict down to target ratio
				difference := curSize - q.maxCacheSize*q.targetRatio
				curEvictedSpace := 0
				actualEvictedSpace := 0
				atomic.StoreInt64(&q.totalUploadedSize, 0)

				//3. LRU Eviction to match difference
				for actualEvictedSpace < int(difference) {

					//initialize channel
					q.uploadChan = make(chan string, 1000)

					//start workers here
					q.workerWg.Add(q.numWorkers)
					for i := 0; i < q.numWorkers; i++ {
						go q.worker()
					}

					//populate upload channel for workers
					for curEvictedSpace < int(difference) {
						nodeSize, evicted := q.eviction()
						if !evicted {
							break
						}
						curEvictedSpace += int(nodeSize)
					}

					//close upload chan and wait for workers to process all remaining jobs
					close(q.uploadChan)
					q.workerWg.Wait()

					//update the actual evicted space here
					actualEvictedSpace = int(q.totalUploadedSize)
				}
				//give hall pass back when actual evicted space is satisfied
				q.hallPassChan <- true

			case <-q.doneChan:
				return

			}
		case <-q.doneChan:
			return
		}
	}
}

func (q *lruQueue) eviction() (int64, bool) {
	q.mu.Lock()
	nodeToEvict := q.tail
	if nodeToEvict == nil {
		q.mu.Unlock()
		return 0, false
	}

	//1. loop through and find the first applicable node
	for nodeToEvict != nil {
		prevNode := nodeToEvict.prev

		flock := q.fileLocks.Get(nodeToEvict.name)
		flock.RLock()
		handleCount := flock.Count()
		flock.RUnlock()

		if handleCount == 0 {
			break
		}

		//node has open handles touch node
		q.extractNode(nodeToEvict)
		q.setHead(nodeToEvict)
		nodeToEvict = prevNode
	}

	//all files are in use
	if nodeToEvict == nil {
		q.mu.Unlock()
		return 0, false
	}
	name := nodeToEvict.name

	//2. Remove file from queue so not accidentally chosen again
	q.extractNode(nodeToEvict)
	q.nodeMap.Delete(name)

	q.mu.Unlock()

	//3. Get the node size that we evict
	localPath := filepath.Join(q.cachePath, name)

	fileInfo, err := os.Stat(localPath)
	if err != nil {
		log.Err("lruPolicy::capacityChecker : failed to stat file: %v", err)
		return 0, false
	}
	nodeSize := fileInfo.Size()

	//4. Send node to channel to be uploaded by workers
	select {
	case q.uploadChan <- name:
	case <-q.doneChan:
		return 0, false
	}

	return nodeSize, true
}

func (q *lruQueue) worker() {
	defer q.workerWg.Done()
	for fileName := range q.uploadChan {

		//1. Get handle count and file size
		flock := q.fileLocks.Get(fileName)
		flock.Lock()
		handleCount := flock.Count()

		localPath := filepath.Join(q.cachePath, fileName)
		fileInfo, err := os.Stat(localPath)
		if err != nil {
			log.Err("lruPolicy::capacityChecker : failed to stat file: %v", err)
		}

		fileSize := fileInfo.Size()

		//2. Check if file eligible to upload
		if handleCount == 0 {
			err := q.uploadandCleanFn(fileName)
			flock.Unlock()
			if err != nil {
				log.Err("lruPolicy::worker : failed to upload file %s: %v", fileName, err)
				//if upload fails we have to put the file back to the queue to retry later
				q.Touch(fileName)

			} else {
				atomic.AddInt64(&q.totalUploadedSize, fileSize)
			}
			//file is in use skip upload touch file back to top of queue
		} else {
			flock.Unlock()
			q.Touch(fileName)
		}
	}
}
