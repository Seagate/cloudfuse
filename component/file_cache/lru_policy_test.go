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
	"testing"
	"time"

	"github.com/Seagate/cloudfuse/common"
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
	// err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	// if err != nil {
	// 	panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	// }
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
	suite.assert.Equal(int64(1), node.usage.Load())
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

func TestLRUPolicyTestSuite(t *testing.T) {
	suite.Run(t, new(lruPolicyTestSuite))
}
