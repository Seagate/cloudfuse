
package tiered_storage

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
)

type lruNode struct{
	prev *lruNode  
	next *lruNode
	name string 
	
}
//upload 50 files and then check

type lruQueue struct{
	sync.Mutex

	wg sync.WaitGroup
	numWorkers int 

	head *lruNode
	tail *lruNode 

	uploadChan chan string
	doneChan chan struct{}

	threshold float64
	targetRatio float64


}


func(q *lruQueue) Touch(){}

func(q *lruQueue) Add(){}

func(q *lruQueue) Remove(){}

//this will

func worker(){}


func (q *lruQueue) capacityChecker() {
    defer q.wg.Done()
    defer close(q.uploadChan) 

    for {
        select {
        case <-time.After(//some time idk):
            // eviction

        case <-q.doneChan:
            return
        }
    }
}