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
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
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

const lruTestFileSize = 250 * 1024

func (suite *lruPolicyTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
	suite.assert = assert.New(suite.T())

	err = os.Mkdir(cache_path, 0777)
	suite.assert.NoError(err)

	suite.setupTestHelper(cache_path, 1, 0.8, 0.6, 8)
}

// setupTestHelper creates and starts an lruQueue for testing.
func (suite *lruPolicyTestSuite) setupTestHelper(
	cachePath string, maxCacheMB float64, threshold float64, targetRatio float64, numWorkers int,
) {
	suite.policy = &lruQueue{
		cachePath:    cachePath,
		maxCacheSize: maxCacheMB * common.MbToBytes,
		threshold:    threshold,
		targetRatio:  targetRatio,
		numWorkers:   numWorkers,
		pollInterval: time.Millisecond,
		fileLocks:    common.NewLockMap(),
		size:         newCacheSizeTracker(cachePath, 0),

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

func (suite *lruPolicyTestSuite) createQueuedFiles(names ...string) {
	data := make([]byte, lruTestFileSize)
	for _, name := range names {
		suite.Require().NoError(os.WriteFile(filepath.Join(cache_path, name), data, 0644))
		suite.policy.Enqueue(name)
	}
}

func (suite *lruPolicyTestSuite) TestEnqueue() {
	defer suite.cleanupTest()

	suite.policy.Enqueue("file1")
	suite.policy.Enqueue("file2")
	suite.policy.Enqueue("file3")
	suite.assert.Equal("file3", suite.policy.head.name)
	suite.assert.Equal("file1", suite.policy.tail.name)

	suite.policy.Enqueue("file1")
	suite.assert.Equal("file1", suite.policy.head.name)
	suite.assert.Equal("file2", suite.policy.tail.name)
}

func (suite *lruPolicyTestSuite) TestDequeue() {
	defer suite.cleanupTest()

	suite.policy.Enqueue("file1")
	suite.policy.Enqueue("file2")
	suite.policy.Dequeue("file1")
	suite.assert.Equal("file2", suite.policy.head.name)
	suite.assert.Equal("file2", suite.policy.tail.name)
}

func (suite *lruPolicyTestSuite) TestEvictionRunsOnePass() {
	defer suite.cleanupTest()

	cachePath := suite.T().TempDir()
	size := newCacheSizeTracker(cachePath, time.Hour)
	var uploaded atomic.Int32
	policy := &lruQueue{
		cachePath:    cachePath,
		maxCacheSize: common.MbToBytes,
		threshold:    0.8,
		targetRatio:  0.6,
		numWorkers:   1,
		maxEviction:  1,
		pollInterval: time.Hour,
		fileLocks:    common.NewLockMap(),
		size:         size,
	}
	policy.uploadandCleanFn = func(name string) error {
		info, err := os.Stat(filepath.Join(cachePath, name))
		if err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(cachePath, name)); err != nil {
			return err
		}
		size.Add(-info.Size())
		uploaded.Add(1)
		return nil
	}
	suite.Require().NoError(policy.StartPolicy())
	defer policy.StopPolicy()

	for i := 1; i <= 4; i++ {
		name := fmt.Sprintf("file%d", i)
		err := os.WriteFile(filepath.Join(cachePath, name), make([]byte, lruTestFileSize), 0644)
		suite.Require().NoError(err)
		policy.Enqueue(name)
	}
	size.Refresh()

	policy.evictDownTo(int64(policy.maxCacheSize * policy.targetRatio))

	suite.assert.EqualValues(1, uploaded.Load())
	suite.assert.LessOrEqual(
		float64(size.Used()),
		policy.maxCacheSize*policy.threshold,
	)
	suite.assert.Greater(
		float64(size.Used()),
		policy.maxCacheSize*policy.targetRatio,
	)
}

func (suite *lruPolicyTestSuite) TestCapacityCheckerEviction() {
	defer suite.cleanupTest()

	var mu sync.Mutex

	var uploaded []string
	suite.policy.uploadandCleanFn = func(name string) error {
		mu.Lock()
		defer mu.Unlock()
		uploaded = append(uploaded, name)
		return os.Remove(filepath.Join(cache_path, name))
	}

	suite.createQueuedFiles("file1", "file2", "file3", "file4")

	_, ex1 := suite.policy.nodeMap.Load("file1")
	_, ex2 := suite.policy.nodeMap.Load("file2")
	_, ex3 := suite.policy.nodeMap.Load("file3")
	_, ex4 := suite.policy.nodeMap.Load("file4")

	suite.assert.True(ex1)
	suite.assert.True(ex2)
	suite.assert.True(ex3)
	suite.assert.True(ex4)

	suite.assert.Eventually(func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(uploaded) >= 2
	}, time.Second, time.Millisecond)
	mu.Lock()
	snapshot := append([]string(nil), uploaded...)
	mu.Unlock()

	suite.assert.Contains(snapshot, "file1")
	suite.assert.Contains(snapshot, "file2")
	suite.assert.NotContains(snapshot, "file3")
	suite.assert.NotContains(snapshot, "file4")

	_, ex1 = suite.policy.nodeMap.Load("file1")
	_, ex2 = suite.policy.nodeMap.Load("file2")
	_, ex3 = suite.policy.nodeMap.Load("file3")
	_, ex4 = suite.policy.nodeMap.Load("file4")

	suite.assert.False(ex1)
	suite.assert.False(ex2)
	suite.assert.True(ex3)
	suite.assert.True(ex4)
}

func (suite *lruPolicyTestSuite) TestCapacityCheckerEvictionOpenHandle() {
	defer suite.cleanupTest()
	var mu sync.Mutex

	var uploaded []string
	suite.policy.uploadandCleanFn = func(name string) error {
		mu.Lock()
		defer mu.Unlock()
		uploaded = append(uploaded, name)
		return os.Remove(filepath.Join(cache_path, name))
	}

	suite.createQueuedFiles("file1")

	flock := suite.policy.fileLocks.Get("file1")
	flock.Lock()
	flock.Inc()
	flock.Unlock()

	suite.createQueuedFiles("file2", "file3", "file4")

	suite.assert.Eventually(func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(uploaded) >= 2
	}, time.Second, time.Millisecond)
	mu.Lock()
	snapshot := append([]string(nil), uploaded...)
	mu.Unlock()

	suite.policy.mu.Lock()
	suite.assert.Equal("file1", suite.policy.head.name)
	suite.assert.Equal("file4", suite.policy.tail.name)
	suite.policy.mu.Unlock()

	suite.assert.Contains(snapshot, "file2")
	suite.assert.Contains(snapshot, "file3")
	suite.assert.NotContains(snapshot, "file1")
	suite.assert.NotContains(snapshot, "file4")
}

func (suite *lruPolicyTestSuite) TestStopPolicyMidUpload() {
	var mu sync.Mutex
	var uploaded []string
	started := make(chan struct{}, 1)
	suite.policy.uploadandCleanFn = func(name string) error {
		select {
		case started <- struct{}{}:
		default:
		}
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		uploaded = append(uploaded, name)
		return os.Remove(filepath.Join(cache_path, name))
	}

	suite.createQueuedFiles("file1", "file2", "file3", "file4")
	<-started

	err := suite.policy.StopPolicy()
	suite.assert.NoError(err)

	mu.Lock()
	snapshot := append([]string(nil), uploaded...)
	mu.Unlock()

	suite.assert.Contains(snapshot, "file1")

	err = os.RemoveAll(cache_path)
	suite.assert.NoError(err)
}

func TestLRUPolicyTestSuite(t *testing.T) {
	suite.Run(t, new(lruPolicyTestSuite))
}
