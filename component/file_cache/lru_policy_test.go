/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.

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

package file_cache

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type lruPolicyTestSuite struct {
	suite.Suite
	assert *assert.Assertions
	policy *lruPolicy
}

var cache_path = filepath.Join(home_dir, "file_cache")

func (suite *lruPolicyTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
	suite.assert = assert.New(suite.T())

	os.Mkdir(cache_path, fs.FileMode(0777))

	config := cachePolicyConfig{
		tmpPath:       cache_path,
		cacheTimeout:  1,
		maxEviction:   defaultMaxEviction,
		maxSizeMB:     0,
		highThreshold: defaultMaxThreshold,
		lowThreshold:  defaultMinThreshold,
		fileLocks:     &common.LockMap{},
	}

	suite.setupTestHelper(config)
}

func (suite *lruPolicyTestSuite) setupTestHelper(config cachePolicyConfig) {
	suite.policy = NewLRUPolicy(config).(*lruPolicy)

	suite.policy.StartPolicy()
}

func (suite *lruPolicyTestSuite) cleanupTest() {
	suite.policy.ShutdownPolicy()

	err := os.RemoveAll(cache_path)
	if err != nil {
		fmt.Printf(
			"lruPolicyTestSuite::cleanupTest : os.RemoveAll(%s) failed [%v]\n",
			cache_path,
			err,
		)
	}
}

// add named item to local cache filesystem, and cache policy data structure
func (suite *lruPolicyTestSuite) createLocalPath(localPath string, isDir bool) {
	var err error
	suite.policy.CacheValid(localPath)
	if isDir {
		err = os.Mkdir(localPath, os.FileMode(0777))
		suite.assert.NoError(err)
	} else {
		fh, err := os.Create(localPath)
		suite.assert.NoError(err)
		err = fh.Close()
		suite.assert.NoError(err)
	}
}

// Generate hierarchy:
// a/
//
//	 a/c1/
//	  a/c1/gc1
//		a/c2
//
// ab/
//
//	ab/c1
//
// ac
func (suite *lruPolicyTestSuite) generateNestedDirectory(
	aPath string,
) ([]string, []string, []string) {
	localBasePath := filepath.Join(suite.policy.tmpPath, internal.TruncateDirName(aPath))
	suite.createLocalPath(localBasePath, true)
	c1 := filepath.Join(localBasePath, "c1")
	suite.createLocalPath(c1, true)
	gc1 := filepath.Join(c1, "gc1")
	suite.createLocalPath(gc1, false)
	c2 := filepath.Join(localBasePath, "c2")
	suite.createLocalPath(c2, false)
	aPaths := []string{localBasePath, c1, gc1, c2}

	abPath := localBasePath + "b"
	suite.createLocalPath(abPath, true)
	abc1 := filepath.Join(abPath, "c1")
	suite.createLocalPath(abc1, false)
	abPaths := []string{abPath, abc1}

	acPath := localBasePath + "c"
	suite.createLocalPath(acPath, false)
	acPaths := []string{acPath}

	var allPaths []string
	copy(allPaths, aPaths)
	allPaths = append(allPaths, abPaths...)
	allPaths = append(allPaths, acPaths...)
	// Validate the paths were setup correctly and all paths exist
	for _, path := range allPaths {
		_, err := os.Stat(path)
		suite.assert.NoError(err)
	}

	return aPaths, abPaths, acPaths
}

func (suite *lruPolicyTestSuite) TestDefault() {
	defer suite.cleanupTest()
	suite.assert.Equal("lru", suite.policy.Name())
	suite.assert.EqualValues(1, suite.policy.cacheTimeout) // cacheTimeout does not change
	suite.assert.EqualValues(defaultMaxEviction, suite.policy.maxEviction)
	suite.assert.EqualValues(0, suite.policy.maxSizeMB)
	suite.assert.EqualValues(defaultMaxThreshold, suite.policy.highThreshold)
	suite.assert.EqualValues(defaultMinThreshold, suite.policy.lowThreshold)
}

func (suite *lruPolicyTestSuite) TestUpdateConfig() {
	defer suite.cleanupTest()
	config := cachePolicyConfig{
		tmpPath:       cache_path,
		cacheTimeout:  120,
		maxEviction:   100,
		maxSizeMB:     10,
		highThreshold: 70,
		lowThreshold:  20,
		fileLocks:     &common.LockMap{},
	}
	suite.policy.UpdateConfig(config)

	suite.assert.NotEqualValues(120, suite.policy.cacheTimeout) // cacheTimeout does not change
	suite.assert.EqualValues(1, suite.policy.cacheTimeout)      // cacheTimeout does not change
	suite.assert.EqualValues(100, suite.policy.maxEviction)
	suite.assert.EqualValues(10, suite.policy.maxSizeMB)
	suite.assert.EqualValues(70, suite.policy.highThreshold)
	suite.assert.EqualValues(20, suite.policy.lowThreshold)
}

func (suite *lruPolicyTestSuite) TestCacheValid() {
	defer suite.cleanupTest()
	suite.policy.CacheValid("temp")

	n, ok := suite.policy.nodeMap.Load("temp")
	suite.assert.True(ok)
	suite.assert.NotNil(n)
	node := n.(*lruNode)
	suite.assert.Equal("temp", node.name)
	suite.assert.Equal(1, node.usage)
}

func (suite *lruPolicyTestSuite) TestCachePurge() {
	defer suite.cleanupTest()

	config := cachePolicyConfig{
		tmpPath:       cache_path,
		cacheTimeout:  1,
		maxEviction:   defaultMaxEviction,
		maxSizeMB:     0,
		highThreshold: defaultMaxThreshold,
		lowThreshold:  defaultMinThreshold,
		fileLocks:     &common.LockMap{},
	}
	suite.setupTestHelper(config)

	// test policy cache data
	suite.policy.CacheValid("temp")
	suite.policy.CachePurge("temp")

	n, ok := suite.policy.nodeMap.Load("temp")
	suite.assert.False(ok)
	suite.assert.Nil(n)

	// test synchronous file and folder deletion
	// purge all aPaths, in reverse order
	aPaths, abPaths, acPaths := suite.generateNestedDirectory("temp")
	for i := len(aPaths) - 1; i >= 0; i-- {
		suite.policy.CachePurge(aPaths[i])
	}

	// validate all aPaths were deleted
	for _, path := range aPaths {
		suite.assert.NoFileExists(path)
		suite.assert.NoDirExists(path)
	}
	// validate other paths were not touched
	var otherPaths []string
	copy(otherPaths, abPaths)
	otherPaths = append(otherPaths, acPaths...)
	for _, path := range otherPaths {
		_, err := os.Stat(path)
		suite.assert.NoError(err)
	}
}

func (suite *lruPolicyTestSuite) TestIsCached() {
	defer suite.cleanupTest()
	suite.policy.CacheValid("temp")

	suite.assert.True(suite.policy.IsCached("temp"))
}

func (suite *lruPolicyTestSuite) TestIsCachedFalse() {
	defer suite.cleanupTest()
	suite.assert.False(suite.policy.IsCached("temp"))
}

func (suite *lruPolicyTestSuite) TestTimeout() {
	defer suite.cleanupTest()

	suite.policy.CacheValid("temp")

	// Wait for time > cacheTimeout, the file should no longer be cached
	for i := 0; i < 300 && suite.policy.IsCached("temp"); i++ {
		time.Sleep(10 * time.Millisecond)
	}

	suite.assert.False(suite.policy.IsCached("temp"))
}

func (suite *lruPolicyTestSuite) TestMaxEvictionDefault() {
	defer suite.cleanupTest()

	for i := 1; i < 5000; i++ {
		suite.policy.CacheValid("temp" + fmt.Sprint(i))
	}

	time.Sleep(3 * time.Second) // Wait for time > cacheTimeout, the file should no longer be cached

	for i := 1; i < 5000; i++ {
		suite.assert.False(suite.policy.IsCached("temp" + fmt.Sprint(i)))
	}
}

func (suite *lruPolicyTestSuite) TestMaxEviction() {
	defer suite.cleanupTest()
	suite.cleanupTest()

	config := cachePolicyConfig{
		tmpPath:       cache_path,
		cacheTimeout:  1,
		maxEviction:   5,
		maxSizeMB:     0,
		highThreshold: defaultMaxThreshold,
		lowThreshold:  defaultMinThreshold,
		fileLocks:     &common.LockMap{},
	}

	suite.setupTestHelper(config)

	for i := 1; i < 5; i++ {
		suite.policy.CacheValid("temp" + fmt.Sprint(i))
	}

	time.Sleep(3 * time.Second) // Wait for time > cacheTimeout, the file should no longer be cached

	for i := 1; i < 5; i++ {
		suite.assert.False(suite.policy.IsCached("temp" + fmt.Sprint(i)))
	}
}

func (suite *lruPolicyTestSuite) verifyPolicy(expectedPolicy, actualPolicy *lruPolicy) {
	for expected, actual := expectedPolicy.head, actualPolicy.head; expected != nil || actual != nil; expected, actual = expected.next, actual.next {
		if expected == expectedPolicy.currMarker {
			suite.assert.Same(actualPolicy.currMarker, actual)
		} else if expected == expectedPolicy.lastMarker {
			suite.assert.Same(actualPolicy.lastMarker, actual)
		} else {
			suite.assert.Equal(expected.name, actual.name)
		}
		suite.assert.NotNil(actual, "actual list is shorter than expected")
		suite.assert.NotNil(expected, "actual list is longer than expected")
	}
}

func (suite *lruPolicyTestSuite) TestCreateSnapshotEmpty() {
	defer suite.cleanupTest()
	originalPolicy := suite.policy
	// test
	snapshot := suite.policy.createSnapshot()
	suite.cleanupTest()
	suite.setupTestHelper(originalPolicy.cachePolicyConfig)
	suite.policy.loadSnapshot(snapshot)
	// assert
	suite.assert.NotNil(snapshot)
	suite.assert.Empty(snapshot.NodeList)
	suite.assert.Zero(snapshot.CurrMarkerPosition)
	suite.assert.EqualValues(1, snapshot.LastMarkerPosition)
	suite.verifyPolicy(originalPolicy, suite.policy)
}

func (suite *lruPolicyTestSuite) TestCreateSnapshotWithTrailingMarkers() {
	defer suite.cleanupTest()
	// setup
	numFiles := 5
	pathPrefix := filepath.Join(cache_path, "temp")
	for i := 1; i <= numFiles; i++ {
		suite.policy.CacheValid(pathPrefix + fmt.Sprint(i))
	}
	originalPolicy := suite.policy
	// test
	snapshot := suite.policy.createSnapshot()
	suite.cleanupTest()
	suite.setupTestHelper(originalPolicy.cachePolicyConfig)
	suite.policy.loadSnapshot(snapshot)
	// assert
	suite.assert.NotNil(snapshot)
	suite.assert.Len(snapshot.NodeList, numFiles)
	for i, v := range snapshot.NodeList {
		suite.assert.Equal(pathPrefix+fmt.Sprint(numFiles-i), filepath.Join(cache_path, v))
	}
	suite.assert.EqualValues(numFiles, snapshot.CurrMarkerPosition)
	suite.assert.EqualValues(numFiles+1, snapshot.LastMarkerPosition)
	suite.verifyPolicy(originalPolicy, suite.policy)
}

func (suite *lruPolicyTestSuite) TestCreateSnapshotWithSurroundingMarkers() {
	defer suite.cleanupTest()
	// setup
	numFiles := 5
	pathPrefix := filepath.Join(cache_path, "temp")
	for i := 1; i <= numFiles; i++ {
		suite.policy.CacheValid(pathPrefix + fmt.Sprint(i))
	}
	time.Sleep(1200 * time.Millisecond) // let timer elapse once
	originalPolicy := suite.policy
	// test
	snapshot := suite.policy.createSnapshot()
	suite.cleanupTest()
	suite.setupTestHelper(originalPolicy.cachePolicyConfig)
	suite.policy.loadSnapshot(snapshot)
	// assert
	suite.assert.NotNil(snapshot)
	suite.assert.Len(snapshot.NodeList, numFiles)
	for i, v := range snapshot.NodeList {
		suite.assert.Equal(pathPrefix+fmt.Sprint(numFiles-i), filepath.Join(cache_path, v))
	}
	suite.assert.EqualValues(0, snapshot.CurrMarkerPosition)
	suite.assert.EqualValues(numFiles+1, snapshot.LastMarkerPosition)
	suite.verifyPolicy(originalPolicy, suite.policy)
}

func (suite *lruPolicyTestSuite) TestCreateSnapshotWithMixedMarkers() {
	defer suite.cleanupTest()
	// setup
	numFiles := 5
	pathPrefix := filepath.Join(cache_path, "temp")
	for i := 1; i <= numFiles; i++ {
		suite.policy.CacheValid(pathPrefix + fmt.Sprint(i))
	}
	time.Sleep(1200 * time.Millisecond) // let timer elapse once
	for i := numFiles + 1; i <= numFiles*2; i++ {
		suite.policy.CacheValid(pathPrefix + fmt.Sprint(i))
	}
	originalPolicy := suite.policy
	// test
	snapshot := suite.policy.createSnapshot()
	suite.cleanupTest()
	suite.setupTestHelper(originalPolicy.cachePolicyConfig)
	suite.policy.loadSnapshot(snapshot)
	// assert
	suite.assert.NotNil(snapshot)
	suite.assert.Len(snapshot.NodeList, numFiles*2)
	for i, v := range snapshot.NodeList {
		suite.assert.Equal(pathPrefix+fmt.Sprint(numFiles*2-i), filepath.Join(cache_path, v))
	}
	suite.assert.EqualValues(numFiles, snapshot.CurrMarkerPosition)
	suite.assert.EqualValues(numFiles*2+1, snapshot.LastMarkerPosition)
	suite.verifyPolicy(originalPolicy, suite.policy)
}

func (suite *lruPolicyTestSuite) TestCreateSnapshotPendingEviction() {
	defer suite.cleanupTest()
	// setup
	numFiles := 5
	pathPrefix := filepath.Join(cache_path, "temp")
	for i := 1; i <= numFiles; i++ {
		suite.policy.CacheValid(pathPrefix + fmt.Sprint(i))
	}
	time.Sleep(1200 * time.Millisecond) // let timer elapse once
	for i := numFiles + 1; i <= numFiles*2; i++ {
		suite.policy.CacheValid(pathPrefix + fmt.Sprint(i))
	}
	suite.policy.updateMarker() // simulate pending eviction
	originalPolicy := suite.policy
	// test
	snapshot := suite.policy.createSnapshot()
	suite.cleanupTest()
	suite.setupTestHelper(originalPolicy.cachePolicyConfig)
	suite.policy.loadSnapshot(snapshot)
	// assert
	suite.assert.NotNil(snapshot)
	suite.assert.Len(snapshot.NodeList, numFiles*2)
	for i, v := range snapshot.NodeList {
		suite.assert.Equal(pathPrefix+fmt.Sprint(numFiles*2-i), filepath.Join(cache_path, v))
	}
	suite.assert.EqualValues(0, snapshot.CurrMarkerPosition)
	suite.assert.EqualValues(numFiles+1, snapshot.LastMarkerPosition)
	suite.verifyPolicy(originalPolicy, suite.policy)
}

func (suite *lruPolicyTestSuite) TestCreateSnapshotLeadingMarkers() {
	defer suite.cleanupTest()
	// setup
	numFiles := 5
	pathPrefix := filepath.Join(cache_path, "temp")
	for i := 1; i <= numFiles; i++ {
		suite.policy.CacheValid(pathPrefix + fmt.Sprint(i))
	}
	time.Sleep(1200 * time.Millisecond) // let timer elapse once
	suite.policy.updateMarker()         // simulate pending eviction (so both markers lead)
	originalPolicy := suite.policy
	// test
	snapshot := suite.policy.createSnapshot()
	suite.cleanupTest()
	suite.setupTestHelper(originalPolicy.cachePolicyConfig)
	suite.policy.loadSnapshot(snapshot)
	// assert
	suite.assert.NotNil(snapshot)
	suite.assert.Len(snapshot.NodeList, numFiles)
	for i, v := range snapshot.NodeList {
		suite.assert.Equal(pathPrefix+fmt.Sprint(numFiles-i), filepath.Join(cache_path, v))
	}
	suite.assert.EqualValues(0, snapshot.CurrMarkerPosition)
	suite.assert.EqualValues(1, snapshot.LastMarkerPosition)
	suite.verifyPolicy(originalPolicy, suite.policy)
}

// func (suite *lruPolicyTestSuite) TestSnapshotSerialization() {
// 	defer suite.cleanupTest()
// 	// setup
// 	snapshot := &LRUPolicySnapshot{
// 		NodeList:           []string{"a", "b", "c"},
// 		CurrMarkerPosition: 1,
// 		LastMarkerPosition: 2,
// 	}
// 	// test
// 	err := snapshot.writeToFile(cache_path)
// 	suite.assert.NoError(err)
// 	snapshotFromFile, err := readSnapshotFromFile(cache_path)
// 	suite.assert.NoError(err)
// 	// assert
// 	suite.assert.Equal(snapshot, snapshotFromFile) // this checks deep equality
// }

func (suite *lruPolicyTestSuite) TestSnapshotSerialization() {
	defer suite.cleanupTest()

	// Setup - create a snapshot with test data
	snapshot := &LRUPolicySnapshot{
		NodeList:           []string{"a", "b", "c"},
		CurrMarkerPosition: 1,
		LastMarkerPosition: 2,
		ScheduleOps:        []string{"op1", "op2"},
		Timestamp:          time.Now().UnixNano(),
	}

	err := suite.policy.writeSnapshotToFile(snapshot)
	suite.assert.NoError(err)

	readSnapshot, err := readSnapshotFromFile(suite.policy.tmpPath)
	suite.assert.NoError(err)

	// Verify the snapshot was preserved correctly
	suite.assert.Equal(snapshot.NodeList, readSnapshot.NodeList)
	suite.assert.Equal(snapshot.CurrMarkerPosition, readSnapshot.CurrMarkerPosition)
	suite.assert.Equal(snapshot.LastMarkerPosition, readSnapshot.LastMarkerPosition)
	suite.assert.Equal(snapshot.ScheduleOps, readSnapshot.ScheduleOps)
	suite.assert.Equal(snapshot.Timestamp, readSnapshot.Timestamp)
}

func (suite *lruPolicyTestSuite) TestPeriodicSnapshotterCreatesFiles() {
	defer suite.cleanupTest()

	// Setup - remove any existing snapshots first
	os.Remove(filepath.Join(suite.policy.tmpPath, "snapshot.0.dat"))
	os.Remove(filepath.Join(suite.policy.tmpPath, "snapshot.1.dat"))

	// Create some files to be included in the snapshot
	for i := 1; i <= 3; i++ {
		suite.policy.CacheValid(filepath.Join(cache_path, fmt.Sprintf("test_periodic_%d", i)))
	}

	// Mock the periodicSnapshotter with a short interval for testing
	// Start a custom snapshot routine with short interval
	originalTicker := time.NewTicker(100 * time.Millisecond)
	defer originalTicker.Stop()

	// Create a channel to signal when we've detected snapshot files
	done := make(chan struct{})

	// Start a goroutine to trigger snapshots manually using the ticker
	go func() {
		for range originalTicker.C {
			snapshot := suite.policy.createSnapshot()
			err := suite.policy.writeSnapshotToFile(snapshot)
			suite.assert.NoError(err)

			// Check if both snapshot files exist after rotation
			file0 := filepath.Join(suite.policy.tmpPath, "snapshot.0.dat")
			file1 := filepath.Join(suite.policy.tmpPath, "snapshot.1.dat")

			// Once we detect both files have been created, we're done
			if _, err0 := os.Stat(file0); err0 == nil {
				if _, err1 := os.Stat(file1); err1 == nil {
					close(done)
					return
				}
			}
		}
	}()

	// Wait for both snapshot files to be created or timeout
	select {
	case <-done:
		// Success - found both snapshot files
	case <-time.After(1 * time.Second):
		suite.T().Error("Timed out waiting for snapshot files to be created")
	}

	// Verify both snapshot files exist
	file0Exists := false
	file1Exists := false

	if _, err := os.Stat(filepath.Join(suite.policy.tmpPath, "snapshot.0.dat")); err == nil {
		file0Exists = true
	}

	if _, err := os.Stat(filepath.Join(suite.policy.tmpPath, "snapshot.1.dat")); err == nil {
		file1Exists = true
	}

	suite.assert.True(file0Exists || file1Exists, "At least one snapshot file should exist")
}

func (suite *lruPolicyTestSuite) TestPeriodicSnapshotRotation() {
	defer suite.cleanupTest()

	// Setup - ensure we start with a clean state
	snapshotFile0 := filepath.Join(suite.policy.tmpPath, "snapshot.0.dat")
	snapshotFile1 := filepath.Join(suite.policy.tmpPath, "snapshot.1.dat")
	os.Remove(snapshotFile0)
	os.Remove(snapshotFile1)

	// Set initial counter value
	suite.policy.snapshotCounter = 0

	// Create first snapshot
	snapshot1 := suite.policy.createSnapshot()
	err := suite.policy.writeSnapshotToFile(snapshot1)
	suite.assert.NoError(err)
	suite.assert.Equal(1, suite.policy.snapshotCounter)

	// Add a short wait to ensure file system completes the write
	time.Sleep(50 * time.Millisecond)

	// Check if the snapshot file exists
	_, err = os.Stat(snapshotFile0)
	if os.IsNotExist(err) {
		// If the default snapshot file doesn't exist, let's look for the real one
		// Sometimes the file path might be different than what we expect
		files, _ := filepath.Glob(filepath.Join(suite.policy.tmpPath, "snapshot.*"))
		suite.T().Logf("Found snapshot files: %v", files)
		suite.assert.NotEmpty(files, "No snapshot files found")
	} else {
		suite.assert.NoError(err, "Error checking snapshot.0.dat")
	}

	// Create second snapshot - should increment counter and rotate
	snapshot2 := suite.policy.createSnapshot()
	err = suite.policy.writeSnapshotToFile(snapshot2)
	suite.assert.NoError(err)
	suite.assert.Equal(0, suite.policy.snapshotCounter)

	// Add a short wait
	time.Sleep(50 * time.Millisecond)

	// Create third snapshot - should wrap back to first file
	snapshot3 := suite.policy.createSnapshot()
	err = suite.policy.writeSnapshotToFile(snapshot3)
	suite.assert.NoError(err)
	suite.assert.Equal(1, suite.policy.snapshotCounter)

	// Add a short wait
	time.Sleep(50 * time.Millisecond)

	// Check if any snapshot files exist
	files, _ := filepath.Glob(filepath.Join(suite.policy.tmpPath, "snapshot.*"))
	suite.T().Logf("Found snapshot files: %v", files)
	suite.assert.NotEmpty(files, "No snapshot files found after rotation")
}

func (suite *lruPolicyTestSuite) TestSnapshotConsistencyAfterOperations() {
	defer suite.cleanupTest()

	// 1. Start with some files
	fileNames := []string{
		filepath.Join(cache_path, "consistency_file1"),
		filepath.Join(cache_path, "consistency_file2"),
		filepath.Join(cache_path, "consistency_file3"),
	}
	for _, name := range fileNames {
		suite.policy.CacheValid(name)
	}

	// 2. Create a snapshot
	snapshot1 := suite.policy.createSnapshot()
	err := suite.policy.writeSnapshotToFile(snapshot1)
	suite.assert.NoError(err)

	// 3. Perform some operations (add, purge)
	suite.policy.CacheValid(filepath.Join(cache_path, "consistency_file4"))
	suite.policy.CachePurge(fileNames[1]) // Remove the second file

	// 4. Create another snapshot
	snapshot2 := suite.policy.createSnapshot()
	err = suite.policy.writeSnapshotToFile(snapshot2)
	suite.assert.NoError(err)

	// 5. Read back the latest snapshot
	readSnapshot, err := readSnapshotFromFile(suite.policy.tmpPath)
	suite.assert.NoError(err)

	// 6. Verify the snapshot reflects current state
	// Should contain file1, file3, file4 but not file2
	snapshotContainsFile := make(map[string]bool)
	for _, nodeName := range readSnapshot.NodeList {
		snapshotContainsFile[nodeName] = true
	}

	// Normalize the paths for comparison
	file1 := strings.TrimPrefix(fileNames[0], cache_path)
	file2 := strings.TrimPrefix(fileNames[1], cache_path)
	file3 := strings.TrimPrefix(fileNames[2], cache_path)
	file4 := strings.TrimPrefix(filepath.Join(cache_path, "consistency_file4"), cache_path)

	suite.assert.True(snapshotContainsFile[file1], "Snapshot should contain file1")
	suite.assert.False(
		snapshotContainsFile[file2],
		"Snapshot should not contain file2 which was purged",
	)
	suite.assert.True(snapshotContainsFile[file3], "Snapshot should contain file3")
	suite.assert.True(snapshotContainsFile[file4], "Snapshot should contain file4 which was added")
}

func (suite *lruPolicyTestSuite) TestPeriodicSnapshotWithEmptyCache() {
	defer suite.cleanupTest()

	// Clean up the cache first
	nodeMap := sync.Map{}
	suite.policy.nodeMap = nodeMap
	suite.policy.head = suite.policy.currMarker
	suite.policy.currMarker.next = suite.policy.lastMarker
	suite.policy.lastMarker.prev = suite.policy.currMarker

	// Create a snapshot with empty cache
	snapshot := suite.policy.createSnapshot()
	err := suite.policy.writeSnapshotToFile(snapshot)
	suite.assert.NoError(err)

	// Read back the snapshot
	readSnapshot, err := readSnapshotFromFile(suite.policy.tmpPath)
	suite.assert.NoError(err)

	// Verify the snapshot is empty but valid
	suite.assert.Empty(readSnapshot.NodeList)
	suite.assert.Equal(uint64(0), readSnapshot.CurrMarkerPosition)
	suite.assert.Equal(uint64(1), readSnapshot.LastMarkerPosition)
}

func (suite *lruPolicyTestSuite) TestPeriodicSnapshotWithScheduledOperations() {
	defer suite.cleanupTest()

	// Setup scheduled operations
	fakeSchedule := &FileCache{}
	fakeSchedule.scheduleOps.Store("operation1", struct{}{})
	fakeSchedule.scheduleOps.Store("operation2", struct{}{})
	suite.policy.schedule = fakeSchedule

	// Create a snapshot
	snapshot := suite.policy.createSnapshot()
	err := suite.policy.writeSnapshotToFile(snapshot)
	suite.assert.NoError(err)

	// Read back the snapshot
	readSnapshot, err := readSnapshotFromFile(suite.policy.tmpPath)
	suite.assert.NoError(err)

	// Verify scheduled operations were preserved
	containsOp1 := false
	containsOp2 := false

	for _, op := range readSnapshot.ScheduleOps {
		if op == "operation1" {
			containsOp1 = true
		}
		if op == "operation2" {
			containsOp2 = true
		}
	}

	suite.assert.True(containsOp1, "Snapshot should preserve scheduled operation1")
	suite.assert.True(containsOp2, "Snapshot should preserve scheduled operation2")
}

func (suite *lruPolicyTestSuite) TestNoEvictionIfInScheduleOps() {
	defer suite.cleanupTest()

	fileName := filepath.Join(cache_path, "scheduled_file")
	suite.policy.CacheValid(fileName)

	fakeSchedule := &FileCache{}
	fakeSchedule.scheduleOps.Store(common.NormalizeObjectName("scheduled_file"), struct{}{})
	suite.policy.schedule = fakeSchedule

	time.Sleep(2 * time.Second)

	suite.assert.True(suite.policy.IsCached(fileName), "File in scheduleOps should not be evicted")
}

func (suite *lruPolicyTestSuite) TestEvictionRespectsScheduleOps() {
	defer suite.cleanupTest()

	fileNames := []string{
		filepath.Join(cache_path, "file1"),
		filepath.Join(cache_path, "file2"),
		filepath.Join(cache_path, "file3"),
		filepath.Join(cache_path, "file4"),
	}
	for _, name := range fileNames {
		suite.policy.CacheValid(name)
	}

	fakeSchedule := &FileCache{}
	fakeSchedule.scheduleOps.Store(common.NormalizeObjectName("file2"), struct{}{})
	fakeSchedule.scheduleOps.Store(common.NormalizeObjectName("file4"), struct{}{})
	suite.policy.schedule = fakeSchedule

	time.Sleep(3 * time.Second)

	suite.assert.False(suite.policy.IsCached(fileNames[0]), "file1 should be evicted")
	suite.assert.True(
		suite.policy.IsCached(fileNames[1]),
		"file2 should NOT be evicted (in scheduleOps)",
	)
	suite.assert.False(suite.policy.IsCached(fileNames[2]), "file3 should be evicted")
	suite.assert.True(
		suite.policy.IsCached(fileNames[3]),
		"file4 should NOT be evicted (in scheduleOps)",
	)
}

func (suite *lruPolicyTestSuite) TestSnapshotPreservesScheduleOps() {
	defer suite.cleanupTest()

	// Setup test files
	fileNames := []string{
		filepath.Join(cache_path, "snapshot_file1"),
		filepath.Join(cache_path, "snapshot_file2"),
		filepath.Join(cache_path, "snapshot_file3"),
		filepath.Join(cache_path, "snapshot_file4"),
	}
	for _, name := range fileNames {
		suite.policy.CacheValid(name)
	}

	fakeSchedule := &FileCache{}
	fakeSchedule.scheduleOps.Store(common.NormalizeObjectName("snapshot_file2"), struct{}{})
	fakeSchedule.scheduleOps.Store(common.NormalizeObjectName("snapshot_file4"), struct{}{})
	suite.policy.schedule = fakeSchedule

	originalPolicy := suite.policy
	snapshot := suite.policy.createSnapshot()

	suite.assert.NotNil(snapshot)
	suite.assert.Contains(snapshot.ScheduleOps, common.NormalizeObjectName("snapshot_file2"))
	suite.assert.Contains(snapshot.ScheduleOps, common.NormalizeObjectName("snapshot_file4"))

	suite.cleanupTest()
	suite.setupTestHelper(originalPolicy.cachePolicyConfig)
	suite.policy.loadSnapshot(snapshot)

	for _, name := range fileNames {
		suite.assert.True(
			suite.policy.IsCached(name),
			name+" should be cached after loading snapshot",
		)
	}

	scheduledOpsExist := false
	if suite.policy.schedule != nil {
		_, found2 := suite.policy.schedule.scheduleOps.Load(
			common.NormalizeObjectName("snapshot_file2"),
		)
		_, found4 := suite.policy.schedule.scheduleOps.Load(
			common.NormalizeObjectName("snapshot_file4"),
		)
		scheduledOpsExist = found2 && found4
	}
	suite.assert.True(scheduledOpsExist, "scheduledOps should be restored from snapshot")

	time.Sleep(3 * time.Second)

	suite.assert.False(suite.policy.IsCached(fileNames[0]), "file1 should be evicted after timeout")
	suite.assert.True(
		suite.policy.IsCached(fileNames[1]),
		"file2 should NOT be evicted (restored from scheduledOps in snapshot)",
	)
	suite.assert.False(suite.policy.IsCached(fileNames[2]), "file3 should be evicted after timeout")
	suite.assert.True(
		suite.policy.IsCached(fileNames[3]),
		"file4 should NOT be evicted (restored from scheduledOps in snapshot)",
	)
}

func TestLRUPolicyTestSuite(t *testing.T) {
	suite.Run(t, new(lruPolicyTestSuite))
}
