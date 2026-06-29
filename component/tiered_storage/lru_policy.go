package tiered_storage

import (
	"fmt"
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
			//1. check if we need eviction
			curSize, err := common.GetUsage(q.cachePath)
			if err != nil {
				log.Err("lruPolicy::capacityChecker : failed to get usage: %v", err)
				continue
			}
			if curSize/q.maxCacheSize <= q.threshold {
				break
			}
			for curSize/q.maxCacheSize > q.targetRatio {
				if !q.eviction() {
					break
				}
				curSize, err = common.GetUsage(q.cachePath)
				if err != nil {
					log.Err(
						"lruPolicy::capacityChecker : failed to get usage after eviction: %v",
						err,
					)
					continue
				}
			}

		case <-q.doneChan:
			return
		}
	}
}

func (q *lruQueue) eviction() bool {
	q.mu.Lock()
	nodeToEvict := q.tail
	if nodeToEvict == nil {
		return false
	}

	//remove node from queue and map
	q.extractNode(nodeToEvict)
	q.nodeMap.Delete(nodeToEvict.name)
	q.mu.Unlock()

	//send node to channel
	q.uploadChan <- nodeToEvict.name
	return true
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
