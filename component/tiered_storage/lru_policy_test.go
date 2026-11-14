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
var local_path = filepath.Join(cache_path, localDirName)

const lruTestFileSize = 250 * 1024

func (suite *lruPolicyTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
	suite.assert = assert.New(suite.T())

	suite.Require().NoError(makeTieredRoot(cache_path))

	suite.setupTestHelper(lruQueueConfig{
		localRoot:    local_path,
		maxSize:      1 * common.MbToBytes,
		threshold:    0.8,
		targetRatio:  0.6,
		numWorkers:   8,
		pollInterval: time.Millisecond,
		fileLocks:    common.NewLockMap(),
		size:         newDiskUsageTracker(cache_path, 0),

		uploadandCleanFn: func(name string) error {
			return nil
		},
	})
}

// makeTieredRoot creates the local/ and cache/ trees the size tracker measures.
func makeTieredRoot(root string) error {
	for _, tree := range []string{localDirName, cacheDirName} {
		if err := os.MkdirAll(filepath.Join(root, tree), 0777); err != nil {
			return err
		}
	}
	return nil
}

// setupTestHelper creates and starts an lruQueue for testing.
func (suite *lruPolicyTestSuite) setupTestHelper(cfg lruQueueConfig) {
	suite.policy = newLRUQueue(cfg)

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
		suite.Require().NoError(os.WriteFile(filepath.Join(local_path, name), data, 0644))
		suite.policy.Enqueue(name)
	}
}

func queued(q *lruQueue, name string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	_, found := q.nodes[name]
	return found
}

// waitEvictOnClose waits until evict-on-close requests for names are handled.
func waitEvictOnClose(t *testing.T, q *lruQueue, names ...string) {
	for _, name := range names {
		assert.Eventually(t, func() bool {
			q.mu.Lock()
			defer q.mu.Unlock()
			_, pending := q.closed[name]
			return !pending
		}, time.Second, time.Millisecond)
		// The request is removed under the file lock, which is held until the
		// upload finishes, so taking the lock waits for that to happen.
		flock := q.fileLocks.Get(name)
		flock.Lock()
		flock.Unlock() //nolint:staticcheck // SA2001: the empty section is the wait
	}
}

// markStale creates local files and marks them cloudStale.
func (suite *lruPolicyTestSuite) markStale(names ...string) {
	for _, name := range names {
		suite.Require().NoError(os.WriteFile(filepath.Join(local_path, name), nil, 0644))
		suite.policy.fileLocks.Get(name).CloudStale = true
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

func (suite *lruPolicyTestSuite) TestRenameKeepsPosition() {
	defer suite.cleanupTest()

	suite.policy.Enqueue("file1")
	suite.policy.Enqueue("file2")
	suite.policy.Enqueue("file3")
	suite.policy.Rename("file2", "file3")
	suite.policy.Rename("file1", "renamed")

	suite.assert.Equal("file3", suite.policy.head.name)
	suite.assert.Equal("renamed", suite.policy.tail.name)
	suite.assert.Equal(suite.policy.tail, suite.policy.head.next)
	suite.assert.Nil(suite.policy.tail.next)
	found := queued(suite.policy, "file1")
	suite.assert.False(found)
	found = queued(suite.policy, "file2")
	suite.assert.False(found)
}

func (suite *lruPolicyTestSuite) TestEvictionKeepsNodeMidRename() {
	defer suite.cleanupTest()

	suite.Require().NoError(os.WriteFile(filepath.Join(local_path, "src"), nil, 0644))
	suite.policy.Enqueue("src")

	// RenameFile holds the file lock from the local rename until policy.Rename.
	flock := suite.policy.fileLocks.Get("src")
	flock.Lock()
	suite.Require().NoError(
		os.Rename(filepath.Join(local_path, "src"), filepath.Join(local_path, "dst")),
	)
	names := suite.policy.nominate(1, 10)
	suite.assert.Equal([]string{"src"}, names)
	suite.assert.True(queued(suite.policy, "src"))
	suite.policy.Rename("src", "dst")
	flock.Unlock()

	suite.policy.evictAll(names, false)
	suite.assert.True(queued(suite.policy, "dst"))
}

func (suite *lruPolicyTestSuite) TestEvictionRunsOnePass() {
	defer suite.cleanupTest()
	suite.cleanupTest() // teardown the default policy generated in SetupTest

	root := suite.T().TempDir()
	suite.Require().NoError(makeTieredRoot(root))
	cachePath := filepath.Join(root, localDirName)
	size := newDiskUsageTracker(root, time.Hour)
	var uploaded atomic.Int32
	suite.setupTestHelper(lruQueueConfig{
		localRoot:    cachePath,
		maxSize:      common.MbToBytes,
		threshold:    0.8,
		targetRatio:  0.6,
		numWorkers:   1,
		maxEviction:  1,
		pollInterval: time.Hour,
		fileLocks:    common.NewLockMap(),
		size:         size,

		uploadandCleanFn: func(name string) error {
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
		},
	})

	for i := 1; i <= 4; i++ {
		name := fmt.Sprintf("file%d", i)
		err := os.WriteFile(filepath.Join(cachePath, name), make([]byte, lruTestFileSize), 0644)
		suite.Require().NoError(err)
		suite.policy.Enqueue(name)
	}
	suite.Require().NoError(size.Reconcile())

	suite.policy.evictDownTo(int64(suite.policy.maxSize * suite.policy.targetRatio))

	suite.assert.EqualValues(1, uploaded.Load())
	suite.assert.LessOrEqual(
		float64(size.Used()),
		suite.policy.maxSize*suite.policy.threshold,
	)
	suite.assert.Greater(
		float64(size.Used()),
		suite.policy.maxSize*suite.policy.targetRatio,
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
		return os.Remove(filepath.Join(local_path, name))
	}

	suite.createQueuedFiles("file1", "file2", "file3", "file4")

	ex1 := queued(suite.policy, "file1")
	ex2 := queued(suite.policy, "file2")
	ex3 := queued(suite.policy, "file3")
	ex4 := queued(suite.policy, "file4")

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

	ex1 = queued(suite.policy, "file1")
	ex2 = queued(suite.policy, "file2")
	ex3 = queued(suite.policy, "file3")
	ex4 = queued(suite.policy, "file4")

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
		return os.Remove(filepath.Join(local_path, name))
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
		return os.Remove(filepath.Join(local_path, name))
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

func (suite *lruPolicyTestSuite) TestStopPolicyWaitsForEvictOnClose() {
	started, release := make(chan struct{}), make(chan struct{})
	var uploaded []string
	var mu sync.Mutex
	suite.policy.uploadandCleanFn = func(name string) error {
		if name == "first" {
			close(started)
			<-release
		}
		mu.Lock()
		defer mu.Unlock()
		uploaded = append(uploaded, name)
		return nil
	}

	suite.markStale("first", "second", "third")
	suite.policy.EvictOnClose("first")
	<-started
	suite.policy.EvictOnClose("second")

	stopped := make(chan error, 1)
	go func() { stopped <- suite.policy.StopPolicy() }()
	suite.Never(func() bool { return len(stopped) > 0 }, 50*time.Millisecond, 5*time.Millisecond)
	close(release)
	suite.Require().NoError(<-stopped)

	// The in-flight upload finished; the one that had not started was skipped.
	suite.assert.Equal([]string{"first"}, uploaded)
	// Nothing new starts after stopping.
	suite.policy.EvictOnClose("third")
	time.Sleep(10 * time.Millisecond)
	suite.assert.Equal([]string{"first"}, uploaded)

	err := os.RemoveAll(cache_path)
	suite.assert.NoError(err)
}

func (suite *lruPolicyTestSuite) TestEvictOnCloseSkipsOpenOrFreshFiles() {
	defer suite.cleanupTest()
	var mu sync.Mutex
	var uploaded []string
	suite.policy.uploadandCleanFn = func(name string) error {
		mu.Lock()
		defer mu.Unlock()
		uploaded = append(uploaded, name)
		return nil
	}

	suite.markStale("open", "stale")
	suite.Require().NoError(os.WriteFile(filepath.Join(local_path, "fresh"), nil, 0644))
	suite.policy.fileLocks.Get("open").Inc()
	for _, name := range []string{"open", "fresh", "stale", "missing"} {
		suite.policy.EvictOnClose(name)
	}
	waitEvictOnClose(suite.T(), suite.policy, "open", "fresh", "stale", "missing")

	mu.Lock()
	defer mu.Unlock()
	suite.assert.Equal([]string{"stale"}, uploaded)
}

func TestLRUPolicyTestSuite(t *testing.T) {
	suite.Run(t, new(lruPolicyTestSuite))
}
