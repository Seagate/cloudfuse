package tiered_storage

import (
	"sync"
	"time"

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

	threshold   float64
	targetRatio float64
}

func (q *lruQueue) Touch(name string) {}

func (q *lruQueue) Enqueue(name string) {
	//Maybe have a duplicate checker

	//create node
	node := &lruNode{name: name}
	//Add node to map
	q.nodeMap.Store(name, node)
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.head == nil {
		q.head = node
		q.tail = node
	} else {
		q.setHead(node)
	}
}

func (q *lruQueue) Dequeue(name string) {
	log.Trace("lruPolicy::removeNode : %s", name)

	var node *lruNode = nil

	val, found := q.nodeMap.LoadAndDelete(name)
	if !found || val == nil {
		return
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	node = val.(*lruNode)

	q.extractNode(node)
}

func (q *lruQueue) setHead(node *lruNode) {
	// insert node at the head
	node.prev = nil
	node.next = q.head
	q.head.prev = node
	q.head = node
}

func worker() {}

func (q *lruQueue) capacityChecker() {
	defer q.wg.Done()
	defer close(q.uploadChan)

	for {
		select {
		case <-time.After(2 * time.Minute):
			// eviction

		case <-q.doneChan:
			return
		}
	}
}
