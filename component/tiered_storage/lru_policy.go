package tiered_storage

import (
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

	cachePath string
	maxCacheSize float64

	threshold   float64
	targetRatio float64
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

func worker() {}

func (q *lruQueue) capacityChecker() {
	defer q.wg.Done()
	defer close(q.uploadChan)

	for {
		select {
		case <-time.After(2 * time.Minute):
			// eviction
			//1. check if we need eviction
			curSize, err := common.GetUsage(q.cachePath)
			if err != nil{
				log.Err("lruPolicy::capacityChecker : failed to get usage: %v", err)
				continue
			}
			if curSize/q.maxCacheSize <= q.threshold{
				break
			}
			for curSize/q.maxCacheSize > q.targetRatio{
				if !q.eviction() {
					break
				}
				curSize, err = common.GetUsage(q.cachePath)
				if err != nil {
					log.Err("lruPolicy::capacityChecker : failed to get usage after eviction: %v", err)
					return
				}
			}
		}

			//seperate function for eviction

		case <-q.doneChan:
			return
		}
}

func (q *lruQueue) eviction() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	nodeToEvict := q.tail
	if nodeToEvict == nil {
		return false
	}

	//remove node from queue and map
	q.extractNode(nodeToEvict)
	q.nodeMap.Delete(nodeToEvict)

	//send node to channel
	q.uploadChan <- nodeToEvict.name
	return true
}
