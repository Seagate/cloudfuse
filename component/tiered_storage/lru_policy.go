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
}

//upload 50 files and then check

type lruQueue struct {
	mu sync.Mutex

	nodeMap sync.Map

	wg         sync.WaitGroup
	numWorkers int

	head *lruNode
	tail *lruNode

	uploadChan chan string
	doneChan   chan struct{}

	cachePath    string
	maxCacheSize float64

	threshold   float64
	targetRatio float64

	//upload function from tiered storage WIRE THIS LATER in tiered storage because we are using a function from there
	upload func(name string) error
}

func (q *lruQueue) StartPolicy() error {
	if q.upload == nil {
		return fmt.Errorf("lruQueue: upload function not set")
	}
	if q.numWorkers <= 0 {
		return fmt.Errorf("lruQueue: numWorkers must be > 0")
	}
	//initialize queue
	q.head = nil
	q.tail = nil
	//channels
	q.uploadChan = make(chan string, 1000)
	q.doneChan = make(chan struct{})
	//timer
	//go routines
	q.wg.Add(1)
	go q.capacityChecker()

	q.wg.Add(q.numWorkers)
	for i := 0; i < q.numWorkers; i++ {
		go q.worker()
	}
	return nil
}

func (q *lruQueue) StopPolicy() error {
	close(q.doneChan)
	q.wg.Wait()
	return nil
}

func (q *lruQueue) Touch(name string) {
	q.Enqueue(name)
}

func (q *lruQueue) Enqueue(name string) {
	//Maybe have a duplicate , that touches essentially

	//create node
	newNode := &lruNode{name: name}
	val, found := q.nodeMap.LoadOrStore(name, newNode)
	node := val.(*lruNode)

	q.mu.Lock()
	defer q.mu.Unlock()

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

	val, found := q.nodeMap.LoadAndDelete(name)
	if !found || val == nil {
		return
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	node := val.(*lruNode)

	q.extractNode(node)
}

func (q *lruQueue) setHead(node *lruNode) {
	// insert node at the head
	node.prev = nil
	node.next = q.head
	q.head.prev = node
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

	for {
		select {
		case <-time.After(2 * time.Minute):
			// eviction

			//check du , do stat file before, based on difference between DU and

			//1. check if we need eviction
			curSize, err := common.GetUsage(q.cachePath)
			if err != nil {
				log.Err("lruPolicy::capacityChecker : failed to get usage: %v", err)
				continue
			}
			if curSize/q.maxCacheSize <= q.threshold {
				break
			}

			//targetRatio should always be less than thresholdRatio

			//find difference to evict down to 60%
			difference := curSize - q.maxCacheSize*q.targetRatio
			curEvictedSpace := 0
			for curEvictedSpace < int(difference) {

				nodeName, evicted := q.eviction()
				if !evicted {
					break
				}

				localPath := filepath.Join(q.cachePath, nodeName)

				fileInfo, err := os.Stat(localPath)
				if err != nil {
					log.Err("lruPolicy::capacityChecker : failed to stat file: %v", err)
					break
				}
				curEvictedSpace += int(fileInfo.Size())
			}

		case <-q.doneChan:
			return
		}
	}
}

func (q *lruQueue) eviction() (string, bool) {
	q.mu.Lock()
	nodeToEvict := q.tail
	if nodeToEvict == nil {
		return "", false
	}

	//ok we have to add in handle checkers/logic

	//remove node from queue and map
	q.extractNode(nodeToEvict)
	q.nodeMap.Delete(nodeToEvict.name)
	q.mu.Unlock()

	//send node to channel
	q.uploadChan <- nodeToEvict.name
	return nodeToEvict.name, true
}

func (q *lruQueue) worker() {
	defer q.wg.Done()
	for fileName := range q.uploadChan {
		err := q.upload(fileName)
		if err != nil {
			log.Err("lruPolicy::worker : failed to upload file %s: %v", fileName, err)
		}
	}
}
