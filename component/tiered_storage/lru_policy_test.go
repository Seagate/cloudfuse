/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2026 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.

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
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type lruPolicyTestSuite struct {
	suite.Suite
	assert *assert.Assertions
	policy *lruQueue
}

var cache_path = filepath.Join(home_dir, "file_cache"+randomString(8))

func (suite *lruPolicyTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
	suite.assert = assert.New(suite.T())

	err = os.Mkdir(cache_path, fs.FileMode(0777))
	suite.assert.NoError(err)

	suite.setupTestHelper(cache_path, 1, 0.8, 0.6, 8)
}

// setupTestHelper creates and starts an lruQueue for testing.
func (suite *lruPolicyTestSuite) setupTestHelper(
	cachePath string, maxCacheMB float64, threshold float64, targetRatio float64, numWorkers int,
) {
	suite.policy = &lruQueue{
		cachePath:    cachePath,
		maxCacheSize: maxCacheMB * 1024 * 1024, // convert MB to bytes
		threshold:    threshold,
		targetRatio:  targetRatio,
		numWorkers:   numWorkers,
		tickerUnit:   time.Millisecond,
		fileLocks:    common.NewLockMap(), // required by eviction() and worker()

		uploadandCleanFn: func(name string) error {
			return nil
		},
	}

	err := suite.policy.StartPolicy()
	suite.assert.NoError(err)
}

func (suite *lruPolicyTestSuite) cleanupTest() {
	err := suite.policy.StopPolicy()
	suite.assert.NoError(err)

	err = os.RemoveAll(cache_path)
	suite.assert.NoError(err)
}

// Test
// 1. Touch
func (suite *lruPolicyTestSuite) TestTouch() {
	defer suite.cleanupTest()
	//put one file in
	name := "file1"
	fileName := filepath.Join(cache_path, name)
	suite.policy.Touch(fileName)
	suite.assert.Equal(fileName, suite.policy.head.name)
	suite.assert.Equal(fileName, suite.policy.tail.name)

	//put another file in
	name2 := "file2"
	fileName2 := filepath.Join(cache_path, name2)
	suite.policy.Touch(fileName2)
	suite.assert.Equal(fileName2, suite.policy.head.name)
	suite.assert.Equal(fileName, suite.policy.tail.name)

	//touch file1 back to top
	suite.policy.Touch(fileName)
	suite.assert.Equal(fileName, suite.policy.head.name)
	suite.assert.Equal(fileName2, suite.policy.tail.name)
}

// 2. enqueueItem
func (suite *lruPolicyTestSuite) TestEnqueue() {
	defer suite.cleanupTest()
	//put one file in
	name := "file1"
	fileName := filepath.Join(cache_path, name)
	suite.policy.Enqueue(fileName)
	suite.assert.Equal(fileName, suite.policy.head.name)
	suite.assert.Equal(fileName, suite.policy.tail.name)

	//put another file in
	name2 := "file2"
	fileName2 := filepath.Join(cache_path, name2)
	suite.policy.Enqueue(fileName2)
	suite.assert.Equal(fileName2, suite.policy.head.name)
	suite.assert.Equal(fileName, suite.policy.tail.name)

	//put another file in
	name3 := "file3"
	fileName3 := filepath.Join(cache_path, name3)
	suite.policy.Enqueue(fileName3)
	suite.assert.Equal(fileName3, suite.policy.head.name)
	suite.assert.Equal(fileName, suite.policy.tail.name)
}

// 3. Dequeue
func (suite *lruPolicyTestSuite) TestDequeue() {
	defer suite.cleanupTest()
	//put one file in
	name := "file1"
	fileName := filepath.Join(cache_path, name)
	suite.policy.Enqueue(fileName)
	suite.assert.Equal(fileName, suite.policy.head.name)
	suite.assert.Equal(fileName, suite.policy.tail.name)

	//put another file in
	name2 := "file2"
	fileName2 := filepath.Join(cache_path, name2)
	suite.policy.Enqueue(fileName2)
	suite.assert.Equal(fileName2, suite.policy.head.name)
	suite.assert.Equal(fileName, suite.policy.tail.name)

	//remove
	suite.policy.Dequeue(fileName)
	suite.assert.Equal(fileName2, suite.policy.head.name)
	suite.assert.Equal(fileName2, suite.policy.tail.name)
}

// 5. Capacity checker, two cases
func (suite *lruPolicyTestSuite) TestCapacityCheckerEviction() {
	defer suite.cleanupTest()

	var mu sync.Mutex

	//1. Define an arbitrary upload function to test the functionality of the channel
	var uploaded []string
	suite.policy.uploadandCleanFn = func(name string) error {
		mu.Lock()
		uploaded = append(uploaded, name)
		os.Remove(filepath.Join(cache_path, name))
		mu.Unlock()
		return nil
	}

	//2. Create files that exceed the 80% threshold, max set at 1MB
	data := make([]byte, 250*1024)
	err := os.WriteFile(filepath.Join(cache_path, "file1"), data, 0644)
	suite.assert.NoError(err)
	suite.policy.Enqueue("file1")

	data = make([]byte, 250*1024)
	err = os.WriteFile(filepath.Join(cache_path, "file2"), data, 0644)
	suite.assert.NoError(err)
	suite.policy.Enqueue("file2")

	data = make([]byte, 250*1024)
	err = os.WriteFile(filepath.Join(cache_path, "file3"), data, 0644)
	suite.assert.NoError(err)
	suite.policy.Enqueue("file3")

	data = make([]byte, 250*1024)
	err = os.WriteFile(filepath.Join(cache_path, "file4"), data, 0644)
	suite.assert.NoError(err)
	suite.policy.Enqueue("file4")

	//file4 should be in upload channel
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	snapshot := make([]string, len(uploaded))
	copy(snapshot, uploaded)
	mu.Unlock()

	// file1 and file2 are the LRU tail so they should be evicted to reach targetRatio (60%)
	suite.assert.Contains(snapshot, "file1")
	suite.assert.Contains(snapshot, "file2")
	// file3 and file4 are the most recently used, so they should NOT be evicted
	suite.assert.NotContains(snapshot, "file3")
	suite.assert.NotContains(snapshot, "file4")
}

// 6. Eviction, file with open handle, file with no open handle,
func (suite *lruPolicyTestSuite) TestCapacityCheckerEvictionOpenHandle() {
	defer suite.cleanupTest()
	var mu sync.Mutex

	//1. Define an arbitrary upload function to test the functionality of the channel
	var uploaded []string
	suite.policy.uploadandCleanFn = func(name string) error {
		mu.Lock()
		uploaded = append(uploaded, name)
		os.Remove(filepath.Join(cache_path, name))
		mu.Unlock()
		return nil
	}

	//2. Create files that exceed the 80% threshold, max set at 1MB
	data := make([]byte, 250*1024)
	err := os.WriteFile(filepath.Join(cache_path, "file1"), data, 0644)
	suite.assert.NoError(err)
	suite.policy.Enqueue("file1")

	//open a file handle for file1 so it should get skipped and touched to the top, file1, file4, file3, file2
	flock := suite.policy.fileLocks.Get("file1")
	flock.Lock()
	flock.Inc()
	flock.Unlock()

	data = make([]byte, 250*1024)
	err = os.WriteFile(filepath.Join(cache_path, "file2"), data, 0644)
	suite.assert.NoError(err)
	suite.policy.Enqueue("file2")

	data = make([]byte, 250*1024)
	err = os.WriteFile(filepath.Join(cache_path, "file3"), data, 0644)
	suite.assert.NoError(err)
	suite.policy.Enqueue("file3")

	data = make([]byte, 250*1024)
	err = os.WriteFile(filepath.Join(cache_path, "file4"), data, 0644)
	suite.assert.NoError(err)
	suite.policy.Enqueue("file4")

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	snapshot := make([]string, len(uploaded))
	copy(snapshot, uploaded)
	mu.Unlock()

	fmt.Print(snapshot)
	fmt.Print(suite.policy.head.name)
	fmt.Print(suite.policy.tail.name)

	//file1 should be head, file4 should be tail
	suite.assert.Equal("file1", suite.policy.head.name)
	suite.assert.Equal("file4", suite.policy.tail.name)

	// file1 and file2 are the LRU tail so they should be evicted to reach targetRatio (60%)
	suite.assert.Contains(snapshot, "file2")
	suite.assert.Contains(snapshot, "file3")
	// file3 and file4 are the most recently used, so they should NOT be evicted
	suite.assert.NotContains(snapshot, "file1")
	suite.assert.NotContains(snapshot, "file4")

}

// 7. Test done channel function
func (suite *lruPolicyTestSuite) TestStopPolicyMidUpload() {
	//fill up upload chan
	//call stop policy
	//make sure all files in upload were indeed uploaded

	var mu sync.Mutex

	//1. Define an arbitrary upload function to test the functionality of the channel
	var uploaded []string
	suite.policy.uploadandCleanFn = func(name string) error {
		mu.Lock()
		//make upload super slow
		time.Sleep(200 * time.Millisecond)
		uploaded = append(uploaded, name)
		os.Remove(filepath.Join(cache_path, name))
		mu.Unlock()
		return nil
	}

	data := make([]byte, 250*1024)
	err := os.WriteFile(filepath.Join(cache_path, "file1"), data, 0644)
	suite.assert.NoError(err)
	suite.policy.Enqueue("file1")

	data = make([]byte, 250*1024)
	err = os.WriteFile(filepath.Join(cache_path, "file2"), data, 0644)
	suite.assert.NoError(err)
	suite.policy.Enqueue("file2")

	data = make([]byte, 250*1024)
	err = os.WriteFile(filepath.Join(cache_path, "file3"), data, 0644)
	suite.assert.NoError(err)
	suite.policy.Enqueue("file3")

	data = make([]byte, 250*1024)
	err = os.WriteFile(filepath.Join(cache_path, "file4"), data, 0644)
	suite.assert.NoError(err)
	suite.policy.Enqueue("file4")

	time.Sleep(5 * time.Millisecond)

	//Stop policy
	err = suite.policy.StopPolicy()
	suite.assert.NoError(err)

	mu.Lock()
	snapshot := make([]string, len(uploaded))
	copy(snapshot, uploaded)
	mu.Unlock()

	fmt.Print(snapshot)

	suite.assert.Contains(snapshot, "file1")
	suite.assert.Contains(snapshot, "file2")
	suite.assert.NotContains(snapshot, "file3")
	suite.assert.NotContains(snapshot, "file4")

	err = os.RemoveAll(cache_path)
	suite.assert.NoError(err)
}

func TestLRUPolicyTestSuite(t *testing.T) {
	suite.Run(t, new(lruPolicyTestSuite))
}
