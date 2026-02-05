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

package attr_cache

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/handlemap"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

type attrCacheTestSuite struct {
	suite.Suite
	assert    *assert.Assertions
	attrCache *AttrCache
	mockCtrl  *gomock.Controller
	mock      *internal.MockComponent
}

var emptyConfig = ""
var defaultSize = int64(0)
var defaultMode = 0777

const MB = 1024 * 1024

func newTestAttrCache(next internal.Component, configuration string) *AttrCache {
	_ = config.ReadConfigFromReader(strings.NewReader(configuration))
	attrCache := NewAttrCacheComponent()
	attrCache.SetNextComponent(next)
	_ = attrCache.Configure(true)

	return attrCache.(*AttrCache)
}

func getDirPathAttr(path string) *internal.ObjAttr {
	objAttr := getPathAttr(path, defaultSize, fs.FileMode(defaultMode), true)
	flags := internal.NewDirBitMap()

	objAttr.Flags = flags
	return objAttr
}

func getPathAttr(path string, size int64, mode os.FileMode, metadata bool) *internal.ObjAttr {
	flags := internal.NewFileBitMap()
	return &internal.ObjAttr{
		Path:     path,
		Name:     filepath.Base(path),
		Size:     size,
		Mode:     mode,
		Mtime:    time.Now(),
		Atime:    time.Now(),
		Ctime:    time.Now(),
		Crtime:   time.Now(),
		Flags:    flags,
		Metadata: nil,
	}
}

func (suite *attrCacheTestSuite) assertCacheEmpty() bool {
	return len(suite.attrCache.cache.cacheMap[""].children) == 0
}

func (suite *attrCacheTestSuite) assertNotInCache(path string) {
	suite.T().Helper()

	_, found := suite.attrCache.cache.get(path)
	suite.assert.False(found)
}

func (suite *attrCacheTestSuite) addPathToCache(path string, metadata bool) {
	isDir := path[len(path)-1] == '/'
	path = internal.TruncateDirName(path)
	pathAttr := getPathAttr(path, defaultSize, fs.FileMode(defaultMode), metadata)
	if isDir {
		pathAttr = getDirPathAttr(path)
	}
	suite.attrCache.cache.insert(insertOptions{
		attr:     pathAttr,
		exists:   true,
		cachedAt: time.Now(),
	})
}

func (suite *attrCacheTestSuite) assertDeleted(path string) {
	suite.T().Helper()

	cacheItem, found := suite.attrCache.cache.get(path)
	suite.assert.True(found)
	suite.assert.True(cacheItem.valid())
	suite.assert.False(cacheItem.exists())
}

func (suite *attrCacheTestSuite) assertInvalid(path string) {
	suite.T().Helper()

	cacheItem, found := suite.attrCache.cache.get(path)
	suite.assert.True(found)
	suite.assert.False(cacheItem.valid())
}

func (suite *attrCacheTestSuite) assertUntouched(path string) {
	suite.T().Helper()

	cacheItem, found := suite.attrCache.cache.get(path)
	suite.assert.True(found)
	suite.assert.Equal(defaultSize, cacheItem.attr.Size)
	suite.assert.EqualValues(defaultMode, cacheItem.attr.Mode)
	suite.assert.True(cacheItem.valid())
	suite.assert.True(cacheItem.exists())
}

func (suite *attrCacheTestSuite) assertExists(path string) {
	suite.T().Helper()

	checkItem, found := suite.attrCache.cache.get(path)
	suite.assert.True(found)
	suite.assert.True(checkItem.valid())
	suite.assert.True(checkItem.exists())
}

func (suite *attrCacheTestSuite) assertInCloud(path string) {
	suite.T().Helper()

	checkItem, found := suite.attrCache.cache.get(path)
	suite.assert.True(found)
	suite.assert.True(checkItem.valid())
	suite.assert.True(checkItem.exists())
	suite.assert.True(checkItem.isInCloud())
}

func (suite *attrCacheTestSuite) assertNotInCloud(path string) {
	suite.T().Helper()

	checkItem, found := suite.attrCache.cache.get(path)
	suite.assert.True(found)
	suite.assert.True(checkItem.valid())
	suite.assert.True(checkItem.exists())
	suite.assert.False(checkItem.isInCloud())
}

// This method is used when we transfer the attributes from the src to dst, and mark src as invalid
func assertAttributesTransferred(
	suite *attrCacheTestSuite,
	srcAttr *internal.ObjAttr,
	dstAttr *internal.ObjAttr,
) {
	suite.assert.Equal(srcAttr.Size, dstAttr.Size)
	suite.assert.Equal(srcAttr.Mode, dstAttr.Mode)
	checkItem, found := suite.attrCache.cache.get(dstAttr.Path)
	suite.assert.True(found)
	suite.assert.True(checkItem.exists())
	suite.assert.True(checkItem.valid())
}

// If next component changes the times of the attribute.
func assertSrcAttributeTimeChanged(
	suite *attrCacheTestSuite,
	srcAttr *internal.ObjAttr,
	srcAttrCopy internal.ObjAttr,
) {
	suite.assert.NotEqualValues(suite, srcAttr.Atime, srcAttrCopy.Atime)
	suite.assert.NotEqualValues(suite, srcAttr.Mtime, srcAttrCopy.Mtime)
	suite.assert.NotEqualValues(suite, srcAttr.Ctime, srcAttrCopy.Ctime)
}

// Directory structure
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
func generateDirectory(path string) (*list.List, *list.List, *list.List) {
	path = internal.TruncateDirName(path)

	aPaths := list.New()
	aPaths.PushBack(path + "/")

	aPaths.PushBack(path + "/c1" + "/")
	aPaths.PushBack(path + "/c2")
	aPaths.PushBack(path + "/c1" + "/gc1")

	abPaths := list.New()
	abPaths.PushBack(path + "b" + "/")
	abPaths.PushBack(path + "b" + "/c1")

	acPaths := list.New()
	acPaths.PushBack(path + "c")

	return aPaths, abPaths, acPaths
}

func generateNestedPathAttr(path string, size int64, mode os.FileMode) []*internal.ObjAttr {
	a, _, _ := generateDirectory(path)
	pathAttrs := make([]*internal.ObjAttr, 0)
	for p := a.Front(); p != nil; p = p.Next() {
		pString := p.Value.(string)
		isDir := pString[len(pString)-1] == '/'
		pString = internal.TruncateDirName(pString)
		newPathAttr := getPathAttr(pString, size, mode, true)
		if isDir {
			newPathAttr = getDirPathAttr(pString)
		}
		pathAttrs = append(pathAttrs, newPathAttr)
	}
	return pathAttrs
}

func generateListPathAttr(path string, numEntries int) []*internal.ObjAttr {
	path = internal.TruncateDirName(path)
	pathAttrs := make([]*internal.ObjAttr, 0)
	for i := range numEntries {
		filename := fmt.Sprintf("%s/file%d", path, i)
		newPathAttr := getPathAttr(filename, defaultSize, fs.FileMode(defaultMode), true)
		pathAttrs = append(pathAttrs, newPathAttr)
	}
	return pathAttrs
}

func (suite *attrCacheTestSuite) addDirectoryToCache(
	path string,
	metadata bool,
) (*list.List, *list.List, *list.List) {
	// TODO: flag directories as such, or else recursion based on IsDir() won't work...
	aPaths, abPaths, acPaths := generateDirectory(path)

	for p := aPaths.Front(); p != nil; p = p.Next() {
		suite.addPathToCache(p.Value.(string), metadata)
	}
	for p := abPaths.Front(); p != nil; p = p.Next() {
		suite.addPathToCache(p.Value.(string), metadata)
	}
	for p := acPaths.Front(); p != nil; p = p.Next() {
		suite.addPathToCache(p.Value.(string), metadata)
	}

	return aPaths, abPaths, acPaths
}

func (suite *attrCacheTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
	suite.setupTestHelper(emptyConfig)
}

func (suite *attrCacheTestSuite) setupTestHelper(config string) {
	suite.assert = assert.New(suite.T())

	suite.mockCtrl = gomock.NewController(suite.T())
	suite.mock = internal.NewMockComponent(suite.mockCtrl)
	suite.attrCache = newTestAttrCache(suite.mock, config)
	_ = suite.attrCache.Start(context.Background())
}

func (suite *attrCacheTestSuite) cleanupTest() {
	_ = suite.attrCache.Stop()
	suite.mockCtrl.Finish()
}

// Tests the default configuration of attribute cache
func (suite *attrCacheTestSuite) TestDefault() {
	defer suite.cleanupTest()
	suite.assert.Equal("attr_cache", suite.attrCache.Name())
	suite.assert.EqualValues(120, suite.attrCache.cacheTimeout)
	suite.assert.True(suite.attrCache.cacheOnList)
	suite.assert.False(suite.attrCache.enableSymlinks)
	suite.assert.True(suite.attrCache.cacheDirs)
}

// Tests configuration
func (suite *attrCacheTestSuite) TestConfig() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	config := "attr_cache:\n  timeout-sec: 60\n  no-cache-on-list: true\n  enable-symlinks: true\n  no-cache-dirs: true"
	suite.setupTestHelper(
		config,
	) // setup a new attr cache with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal("attr_cache", suite.attrCache.Name())
	suite.assert.EqualValues(60, suite.attrCache.cacheTimeout)
	suite.assert.False(suite.attrCache.cacheOnList)
	suite.assert.True(suite.attrCache.enableSymlinks)
	suite.assert.False(suite.attrCache.cacheDirs)
}

// Tests backward compatibility
func (suite *attrCacheTestSuite) TestOldConfig() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	config := "attr_cache:\n    no-symlinks: false"
	suite.setupTestHelper(
		config,
	) // setup a new attr cache with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal("attr_cache", suite.attrCache.Name())
	suite.assert.True(suite.attrCache.enableSymlinks)
}

// Tests max files config
func (suite *attrCacheTestSuite) TestConfigMaxFiles() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	cacheTimeout := 1
	maxFiles := 10
	config := fmt.Sprintf("attr_cache:\n  timeout-sec: %d\n  max-files: %d", cacheTimeout, maxFiles)
	suite.setupTestHelper(
		config,
	) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
	suite.assert.Equal(suite.attrCache.maxFiles, maxFiles)
}

func (suite *attrCacheTestSuite) TestConfigZero() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	config := "attr_cache:\n  timeout-sec: 0\n  no-cache-on-list: true\n  enable-symlinks: true\n  no-cache-dirs: true"
	suite.setupTestHelper(
		config,
	) // setup a new attr cache with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal("attr_cache", suite.attrCache.Name())
	suite.assert.EqualValues(0, suite.attrCache.cacheTimeout)
	suite.assert.False(suite.attrCache.cacheOnList)
	suite.assert.True(suite.attrCache.enableSymlinks)
	suite.assert.False(suite.attrCache.cacheDirs)
}

// Tests Create Directory
func (suite *attrCacheTestSuite) TestCreateDir() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/"}

	for _, path := range paths {
		log.Debug(path)
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			options := internal.CreateDirOptions{Name: path}

			// Error
			suite.mock.EXPECT().CreateDir(options).Return(errors.New("Failed"))

			err := suite.attrCache.CreateDir(options)
			suite.assert.Error(err)
			suite.assertNotInCache(truncatedPath)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().CreateDir(options).Return(nil)

			err = suite.attrCache.CreateDir(options)
			suite.assert.NoError(err)

			_, found := suite.attrCache.cache.get(truncatedPath)
			suite.assert.True(found)

			// Entry Already Exists
			suite.mock.EXPECT().CreateDir(options).Return(nil)

			err = suite.attrCache.CreateDir(options)
			suite.assert.Equal(os.ErrExist, err)

			_, found = suite.attrCache.cache.get(truncatedPath)
			suite.assert.True(found)
		})
	}
}

// Tests Create Directory Without Caching Empty Directories
func (suite *attrCacheTestSuite) TestCreateDirNoCacheDirs() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/"}

	noCacheDirs := true
	config := fmt.Sprintf("attr_cache:\n  no-cache-dirs: %t", noCacheDirs)

	for _, path := range paths {
		log.Debug(path)
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.setupTestHelper(
			config,
		) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
		suite.assert.Equal(!noCacheDirs, suite.attrCache.cacheDirs)
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			extendedPath := internal.ExtendDirName(path)
			options := internal.CreateDirOptions{Name: path}

			// Error
			suite.mock.EXPECT().CreateDir(options).Return(errors.New("Failed"))

			err := suite.attrCache.CreateDir(options)
			suite.assert.Error(err)
			suite.assertNotInCache(truncatedPath)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().CreateDir(options).Return(nil)

			err = suite.attrCache.CreateDir(options)
			suite.assert.NoError(err)
			suite.assertExists(truncatedPath)

			// Entry Already Exists
			suite.addPathToCache(extendedPath, false)
			suite.mock.EXPECT().CreateDir(options).Return(nil)

			err = suite.attrCache.CreateDir(options)
			suite.assert.NoError(err)
			suite.assertExists(truncatedPath)
		})
	}
}

// Tests Delete Directory
func (suite *attrCacheTestSuite) TestDeleteDir() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			options := internal.DeleteDirOptions{Name: path}

			// Error
			suite.mock.EXPECT().DeleteDir(options).Return(errors.New("Failed"))

			err := suite.attrCache.DeleteDir(options)
			suite.assert.Error(err)
			suite.assertNotInCache(truncatedPath)

			// Entry Does Not Exist
			suite.mock.EXPECT().DeleteDir(options).Return(nil)

			err = suite.attrCache.DeleteDir(options)
			suite.assert.True(os.IsNotExist(err))
			suite.assertNotInCache(truncatedPath)

			// Entry Exists
			a, ab, ac := suite.addDirectoryToCache(path, false)

			suite.mock.EXPECT().DeleteDir(options).Return(nil)

			err = suite.attrCache.DeleteDir(options)
			suite.assert.NoError(err)
			// a paths should be deleted
			for p := a.Front(); p != nil; p = p.Next() {
				truncatedPath = internal.TruncateDirName(p.Value.(string))
				suite.assertDeleted(truncatedPath)
			}
			ab.PushBackList(ac) // ab and ac paths should be untouched
			for p := ab.Front(); p != nil; p = p.Next() {
				truncatedPath = internal.TruncateDirName(p.Value.(string))
				suite.assertUntouched(truncatedPath)
			}
		})
	}
}

// Tests Delete Directory Without Caching Empty Directories
func (suite *attrCacheTestSuite) TestDeleteDirNoCacheDirs() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/"}

	noCacheDirs := true
	config := fmt.Sprintf("attr_cache:\n  no-cache-dirs: %t", noCacheDirs)

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.setupTestHelper(
			config,
		) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
		suite.assert.Equal(!noCacheDirs, suite.attrCache.cacheDirs)
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			options := internal.DeleteDirOptions{Name: path}

			// Error
			suite.mock.EXPECT().DeleteDir(options).Return(errors.New("Failed"))

			err := suite.attrCache.DeleteDir(options)
			suite.assert.Error(err)
			suite.assertNotInCache(truncatedPath)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().DeleteDir(options).Return(nil)

			err = suite.attrCache.DeleteDir(options)
			suite.assert.NoError(err)
			suite.assertDeleted(truncatedPath)

			// Entry Already Exists
			a, ab, ac := suite.addDirectoryToCache(path, false)

			suite.mock.EXPECT().DeleteDir(options).Return(nil)

			err = suite.attrCache.DeleteDir(options)
			suite.assert.NoError(err)
			// a paths should be deleted
			for p := a.Front(); p != nil; p = p.Next() {
				truncatedPath = internal.TruncateDirName(p.Value.(string))
				suite.assertDeleted(truncatedPath)
			}
			ab.PushBackList(ac) // ab and ac paths should be untouched
			for p := ab.Front(); p != nil; p = p.Next() {
				truncatedPath = internal.TruncateDirName(p.Value.(string))
				suite.assertUntouched(truncatedPath)
			}
		})
	}
}

// Tests Stream Directory
func (suite *attrCacheTestSuite) TestStreamDirDoesNotExist() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/"}
	size := int64(1024)
	mode := os.FileMode(0)

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			aAttr := generateNestedPathAttr(path, size, mode)

			options := internal.StreamDirOptions{Name: path}

			// Success
			// Entries Do Not Already Exist
			suite.mock.EXPECT().StreamDir(options).Return(aAttr, "", nil).Times(1)

			suite.assertCacheEmpty() // cacheMap should be empty before call
			returnedAttr, token, err := suite.attrCache.StreamDir(options)
			suite.assert.NoError(err)
			suite.assert.Equal(aAttr, returnedAttr)
			suite.assert.Empty(token)

			// Entries should now be in the cache
			for _, p := range aAttr {
				checkItem, found := suite.attrCache.cache.get(p.Path)
				suite.assert.True(found)
				if !p.IsDir() {
					suite.assert.Equal(size, checkItem.attr.Size) // new size should be set
					suite.assert.Equal(mode, checkItem.attr.Mode) // new mode should be set
				}
				suite.assert.True(checkItem.valid())
				suite.assert.True(checkItem.exists())
			}

			// test same result from subsequent call without using cloud storage
			returnedAttr, token, err = suite.attrCache.StreamDir(options)
			suite.assert.NoError(err)
			suite.assert.Empty(token)
			suite.assert.Equal(aAttr, returnedAttr)
		})
	}
}

func (suite *attrCacheTestSuite) TestStreamDirPaginated() {
	defer suite.cleanupTest()
	path := "a"
	manyAttr := generateListPathAttr(path, 6)
	mockTokens := []string{"firstPair", "secondPair"}

	// return first two results
	options0 := internal.StreamDirOptions{Name: path, Token: "", Count: 2}
	suite.mock.EXPECT().StreamDir(options0).Return(manyAttr[0:2], mockTokens[0], nil).Times(1)

	suite.assertCacheEmpty() // cacheMap should be empty before call
	returnedAttr, token, err := suite.attrCache.StreamDir(options0)
	suite.assert.NoError(err)
	suite.assert.Equal(mockTokens[0], token)
	suite.assert.Equal(manyAttr[0:2], returnedAttr)

	returnedAttr, token, err = suite.attrCache.StreamDir(options0)
	suite.assert.NoError(err)
	suite.assert.Equal(mockTokens[0], token)
	suite.assert.Equal(manyAttr[0:2], returnedAttr)

	// return second pair of results
	options1 := internal.StreamDirOptions{Name: path, Token: mockTokens[0], Count: 2}
	suite.mock.EXPECT().StreamDir(options1).Return(manyAttr[2:4], mockTokens[1], nil).Times(1)

	returnedAttr, token, err = suite.attrCache.StreamDir(options1)
	suite.assert.NoError(err)
	suite.assert.Equal(mockTokens[1], token)
	suite.assert.Equal(manyAttr[2:4], returnedAttr)

	returnedAttr, token, err = suite.attrCache.StreamDir(options0)
	suite.assert.NoError(err)
	suite.assert.Equal(mockTokens[0], token)
	suite.assert.Equal(manyAttr[0:2], returnedAttr)

	returnedAttr, token, err = suite.attrCache.StreamDir(options1)
	suite.assert.NoError(err)
	suite.assert.Equal(mockTokens[1], token)
	suite.assert.Equal(manyAttr[2:4], returnedAttr)

	// return last pair of results
	options2 := internal.StreamDirOptions{Name: path, Token: mockTokens[1], Count: 2}
	suite.mock.EXPECT().StreamDir(options2).Return(manyAttr[4:6], "", nil).Times(1)

	returnedAttr, token, err = suite.attrCache.StreamDir(options2)
	suite.assert.NoError(err)
	suite.assert.Empty(token)
	suite.assert.Equal(manyAttr[4:6], returnedAttr)

	returnedAttr, token, err = suite.attrCache.StreamDir(options0)
	suite.assert.NoError(err)
	suite.assert.Equal(mockTokens[0], token)
	suite.assert.Equal(manyAttr[0:2], returnedAttr)

	returnedAttr, token, err = suite.attrCache.StreamDir(options1)
	suite.assert.NoError(err)
	suite.assert.Equal(mockTokens[1], token)
	suite.assert.Equal(manyAttr[2:4], returnedAttr)

	returnedAttr, token, err = suite.attrCache.StreamDir(options2)
	suite.assert.NoError(err)
	suite.assert.Empty(token)
	suite.assert.Equal(manyAttr[4:6], returnedAttr)
}

func (suite *attrCacheTestSuite) TestStreamDirExists() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/"}
	size := int64(1024)
	mode := os.FileMode(0)

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			aAttr := generateNestedPathAttr(path, size, mode)

			options := internal.StreamDirOptions{Name: path}

			// Success
			// Entries Already Exist
			a, ab, ac := suite.addDirectoryToCache(path, false)

			// cache entries should be untouched before read dir call
			for _, p := range aAttr {
				suite.assertUntouched(p.Path)
			}
			suite.mock.EXPECT().StreamDir(options).Return(aAttr, "", nil).Times(1)
			returnedAttr, token, err := suite.attrCache.StreamDir(options)
			suite.assert.NoError(err)
			suite.assert.Empty(token)
			suite.assert.Equal(aAttr, returnedAttr)

			// a paths should now be updated in the cache
			for p := a.Front(); p != nil; p = p.Next() {
				pString := p.Value.(string)
				cachePath := internal.TruncateDirName(pString)
				checkItem, found := suite.attrCache.cache.get(cachePath)
				suite.assert.True(found)
				if !checkItem.attr.IsDir() {
					suite.assert.Equal(size, checkItem.attr.Size) // new size should be set
					suite.assert.Equal(mode, checkItem.attr.Mode) // new mode should be set
				}
				suite.assert.True(checkItem.valid())
				suite.assert.True(checkItem.exists())
			}

			// ab and ac paths should be untouched
			ab.PushBackList(ac)
			for p := ab.Front(); p != nil; p = p.Next() {
				pString := p.Value.(string)
				cachePath := internal.TruncateDirName(pString)
				suite.assertUntouched(cachePath)
			}

			// test same result from subsequent call without using cloud storage
			returnedAttr, token, err = suite.attrCache.StreamDir(options)
			suite.assert.NoError(err)
			suite.assert.Empty(token)
			suite.assert.Equal(aAttr, returnedAttr)
		})
	}
}

func (suite *attrCacheTestSuite) TestStreamDirNoCacheOnList() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	cacheOnList := false
	config := fmt.Sprintf("attr_cache:\n  no-cache-on-list: %t", !cacheOnList)
	suite.setupTestHelper(
		config,
	) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
	suite.assert.Equal(cacheOnList, suite.attrCache.cacheOnList)
	path := "a"
	size := int64(1024)
	mode := os.FileMode(0)
	aAttr := generateNestedPathAttr(path, size, mode)

	options := internal.StreamDirOptions{Name: path}
	suite.mock.EXPECT().StreamDir(options).Return(aAttr, "", nil).Times(1)

	suite.assertCacheEmpty() // cacheMap should be empty before call
	returnedAttr, token, err := suite.attrCache.StreamDir(options)
	suite.assert.NoError(err)
	suite.assert.Empty(token)
	suite.assert.Equal(aAttr, returnedAttr)

	// cacheMap should only have the listed after the call
	suite.assertExists(path)
}

func (suite *attrCacheTestSuite) TestStreamDirNoCacheOnListNoCacheDirs() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	cacheOnList := false
	cacheDirs := false
	config := fmt.Sprintf(
		"attr_cache:\n  no-cache-on-list: %t\n  no-cache-dirs: %t",
		!cacheOnList,
		!cacheDirs,
	)
	suite.setupTestHelper(
		config,
	) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
	suite.assert.Equal(cacheOnList, suite.attrCache.cacheOnList)
	suite.assert.Equal(cacheDirs, suite.attrCache.cacheDirs)
	path := "a"
	size := int64(1024)
	mode := os.FileMode(0)
	aAttr := generateNestedPathAttr(path, size, mode)

	options := internal.StreamDirOptions{Name: path}
	suite.mock.EXPECT().StreamDir(options).Return(aAttr, "", nil)

	suite.assertCacheEmpty() // cacheMap should be empty before call
	returnedAttr, _, err := suite.attrCache.StreamDir(options)
	suite.assert.NoError(err)
	suite.assert.Equal(aAttr, returnedAttr)

	suite.assertCacheEmpty() // cacheMap should be empty after call
}

func (suite *attrCacheTestSuite) TestStreamDirError() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/", "ab", "ab/"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			options := internal.StreamDirOptions{Name: path}

			// Error
			suite.mock.EXPECT().
				StreamDir(options).
				Return(make([]*internal.ObjAttr, 0), "", errors.New("Failed to read a directory"))

			_, _, err := suite.attrCache.StreamDir(options)
			suite.assert.Error(err)
			suite.assertNotInCache(truncatedPath)
		})
	}
}

// Test whether the attribute cache correctly tracks which directories are in cloud storage
func (suite *attrCacheTestSuite) TestDirInCloud() {
	defer suite.cleanupTest()
	// build up the attribute cache
	suite.addDirectoryToCache("a", true)
	deepPath := "a/b/c/d"
	suite.addPathToCache(deepPath, true)

	// delete file a/b/c/d and make sure a/b/ and a/b/c/ are marked not in cloud storage
	delOptions := internal.DeleteFileOptions{Name: deepPath}
	suite.mock.EXPECT().DeleteFile(delOptions).Return(nil)

	err := suite.attrCache.DeleteFile(delOptions)
	suite.assert.NoError(err)
	suite.assertDeleted(deepPath)
	suite.assertNotInCloud("a/b/c")
	suite.assertNotInCloud("a/b")
	suite.assertInCloud("a")

	// add file a/b/c/d back in and make sure all its ancestors are marked in cloud storage
	createOptions := internal.CreateFileOptions{Name: deepPath}
	suite.mock.EXPECT().CreateFile(createOptions).Return(&handlemap.Handle{}, nil)

	_, err = suite.attrCache.CreateFile(createOptions)
	suite.assert.NoError(err)
	suite.assertExists(deepPath)
	suite.assertInCloud("a/b/c")
	suite.assertInCloud("a/b")
	suite.assertInCloud("a")
}

func (suite *attrCacheTestSuite) TestIsDirEmpty() {
	defer suite.cleanupTest()
	// Setup
	path := "dir/"
	options := internal.IsDirEmptyOptions{
		Name: path,
	}
	suite.addPathToCache(path, false)
	suite.mock.EXPECT().IsDirEmpty(options).Return(true)

	empty := suite.attrCache.IsDirEmpty(options)
	suite.assert.True(empty)
}

func (suite *attrCacheTestSuite) TestIsDirEmptyFalse() {
	defer suite.cleanupTest()
	// Setup
	path := "dir/"
	options := internal.IsDirEmptyOptions{
		Name: path,
	}
	suite.mock.EXPECT().IsDirEmpty(options).Return(false)

	empty := suite.attrCache.IsDirEmpty(options)
	suite.assert.False(empty)
}

func (suite *attrCacheTestSuite) TestIsDirEmptyFalseInCache() {
	defer suite.cleanupTest()
	// Setup
	path := "dir/"
	options := internal.IsDirEmptyOptions{
		Name: path,
	}
	suite.addDirectoryToCache(path, false)
	// make sure the attribute cache handles the request itself
	suite.mock.EXPECT().IsDirEmpty(options).MaxTimes(0)

	empty := suite.attrCache.IsDirEmpty(options)
	suite.assert.False(empty)
}

// Tests Rename Directory
func (suite *attrCacheTestSuite) TestRenameDir() {
	defer suite.cleanupTest()
	var inputs = []struct {
		src string
		dst string
	}{
		{src: "a", dst: "ab"},
		{src: "a/", dst: "ab"},
		{src: "a", dst: "ab/"},
		{src: "a/", dst: "ab/"},
	}

	for _, input := range inputs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(input.src+"->"+input.dst, func() {
			truncatedSrc := internal.TruncateDirName(input.src)
			truncatedDst := internal.TruncateDirName(input.dst)
			options := internal.RenameDirOptions{Src: input.src, Dst: input.dst}

			// Error
			suite.mock.EXPECT().
				RenameDir(options).
				Return(errors.New("Failed to rename a directory"))

			err := suite.attrCache.RenameDir(options)
			suite.assert.Error(err)
			suite.assertNotInCache(truncatedSrc)
			suite.assertNotInCache(truncatedDst)

			// Error
			// Source Entry Does Not Exist
			suite.mock.EXPECT().RenameDir(options).Return(nil)

			err = suite.attrCache.RenameDir(options)
			suite.assert.Error(err)
			suite.assertNotInCache(truncatedSrc)
			suite.assertNotInCache(truncatedDst)

			// Error
			// Destination Entry (ab) Already Exists
			a, ab, ac := suite.addDirectoryToCache(input.src, false)

			suite.mock.EXPECT().RenameDir(options).Return(nil)

			err = suite.attrCache.RenameDir(options)
			suite.assert.True(os.IsExist(err))

			// Success
			// Source Entry Exists and Destination Entry Does Not Already Exist
			deleteDirOptions := internal.DeleteDirOptions{Name: input.dst}
			suite.mock.EXPECT().DeleteDir(deleteDirOptions).Return(nil)

			err = suite.attrCache.DeleteDir(deleteDirOptions)
			suite.assert.NoError(err)

			suite.mock.EXPECT().RenameDir(options).Return(nil)

			err = suite.attrCache.RenameDir(options)
			suite.assert.NoError(err)

			// a paths should be deleted
			for p := a.Front(); p != nil; p = p.Next() {
				truncatedPath := internal.TruncateDirName(p.Value.(string))
				suite.assertDeleted(truncatedPath)
			}
			// ab paths happen to both be dir paths now, so they should exist
			for p := ab.Front(); p != nil; p = p.Next() {
				pString := p.Value.(string)
				truncatedPath := internal.TruncateDirName(pString)
				suite.assertExists(truncatedPath)
			}
			// ac paths should be untouched
			for p := ac.Front(); p != nil; p = p.Next() {
				truncatedPath := internal.TruncateDirName(p.Value.(string))
				suite.assertUntouched(truncatedPath)
			}
		})
	}
}

// Tests Rename Directory Without Caching Empty Directories
func (suite *attrCacheTestSuite) TestRenameDirNoCacheDirs() {
	defer suite.cleanupTest()
	var inputs = []struct {
		src string
		dst string
	}{
		{src: "a", dst: "ab"},
		{src: "a/", dst: "ab"},
		{src: "a", dst: "ab/"},
		{src: "a/", dst: "ab/"},
	}

	noCacheDirs := true
	config := fmt.Sprintf("attr_cache:\n  no-cache-dirs: %t", noCacheDirs)

	for _, input := range inputs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.setupTestHelper(
			config,
		) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
		suite.assert.Equal(!noCacheDirs, suite.attrCache.cacheDirs)
		suite.Run(input.src+"->"+input.dst, func() {
			truncatedSrc := internal.TruncateDirName(input.src)
			truncatedDst := internal.TruncateDirName(input.dst)
			options := internal.RenameDirOptions{Src: input.src, Dst: input.dst}

			// Error
			suite.mock.EXPECT().
				RenameDir(options).
				Return(errors.New("Failed to rename a directory"))

			err := suite.attrCache.RenameDir(options)
			suite.assert.Error(err)
			suite.assertNotInCache(truncatedSrc)
			suite.assertNotInCache(truncatedDst)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().RenameDir(options).Return(nil)

			err = suite.attrCache.RenameDir(options)
			suite.assert.NoError(err)
			suite.assertNotInCache(truncatedSrc)
			suite.assertNotInCache(truncatedDst)

			// Entry Already Exists
			a, ab, ac := suite.addDirectoryToCache(input.src, false)

			suite.mock.EXPECT().RenameDir(options).Return(nil)

			err = suite.attrCache.RenameDir(options)
			suite.assert.NoError(err)
			// a paths should be deleted
			for p := a.Front(); p != nil; p = p.Next() {
				truncatedPath := internal.TruncateDirName(p.Value.(string))
				suite.assertDeleted(truncatedPath)
			}
			// ab paths should be invalidated
			for p := ab.Front(); p != nil; p = p.Next() {
				truncatedPath := internal.TruncateDirName(p.Value.(string))
				suite.assertExists(truncatedPath)
			}
			// ac paths should be untouched
			for p := ac.Front(); p != nil; p = p.Next() {
				truncatedPath := internal.TruncateDirName(p.Value.(string))
				suite.assertUntouched(truncatedPath)
			}
		})
	}
}

// Tests Create File
func (suite *attrCacheTestSuite) TestCreateFile() {
	defer suite.cleanupTest()
	path := "a"

	options := internal.CreateFileOptions{Name: path}

	// Error
	suite.mock.EXPECT().CreateFile(options).Return(nil, errors.New("Failed to create a file"))

	_, err := suite.attrCache.CreateFile(options)
	suite.assert.Error(err)
	suite.assertNotInCache(path)

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().CreateFile(options).Return(&handlemap.Handle{}, nil)

	_, err = suite.attrCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assertExists(options.Name)
	checkItem, found := suite.attrCache.cache.get(path)
	suite.assert.True(found)
	suite.assert.EqualValues(0, checkItem.attr.Size)

	// Entry Already Exists
	suite.addPathToCache(path, false)
	suite.mock.EXPECT().CreateFile(options).Return(&handlemap.Handle{}, nil)

	_, err = suite.attrCache.CreateFile(options)
	suite.assert.NoError(err)
	checkItem, found = suite.attrCache.cache.get(path)
	suite.assert.True(found)
	suite.assert.True(checkItem.exists())
	suite.assert.EqualValues(0, checkItem.attr.Size)
}

// Tests Open File
func (suite *attrCacheTestSuite) TestOpenFile() {
	defer suite.cleanupTest()
	path := "a"
	options := internal.OpenFileOptions{Name: path}
	handle := &handlemap.Handle{Size: int64(100 * MB), Path: path}

	// If the file is opened successfully, don't change (or create) its attribute entry
	// If the file does not exist, create or update its attribute entry to be marked as deleted

	// Attribute cache entry does not exist

	// OpenFile succeeds
	suite.mock.EXPECT().OpenFile(options).Return(handle, nil)

	returnedHandle, err := suite.attrCache.OpenFile(options)
	// entry should not be cached
	suite.assert.NoError(err)
	suite.assert.Equal(handle, returnedHandle)
	suite.assertNotInCache(path)

	// OpenFile fails
	suite.mock.EXPECT().OpenFile(options).Return(nil, syscall.ENOENT)

	returnedHandle, err = suite.attrCache.OpenFile(options)
	// entry should not be cached
	suite.assert.Error(err)
	suite.assert.Nil(returnedHandle)
	suite.assertNotInCache(path)

	// Attribute cache entry does exist
	suite.addPathToCache(path, true)
	// OpenFile fails
	suite.mock.EXPECT().OpenFile(options).Return(nil, syscall.ENOENT)

	returnedHandle, err = suite.attrCache.OpenFile(options)
	// entry should be marked deleted
	suite.assert.Error(err)
	suite.assert.Nil(returnedHandle)
	suite.assertDeleted(path)
}

// Tests Delete File
func (suite *attrCacheTestSuite) TestDeleteFile() {
	defer suite.cleanupTest()
	path := "a"

	options := internal.DeleteFileOptions{Name: path}

	// Error
	suite.mock.EXPECT().DeleteFile(options).Return(errors.New("Failed to delete a file"))

	err := suite.attrCache.DeleteFile(options)
	suite.assert.Error(err)
	suite.assertNotInCache(path)

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().DeleteFile(options).Return(nil)

	err = suite.attrCache.DeleteFile(options)
	suite.assert.NoError(err)
	suite.assertDeleted(path)

	// Entry Already Exists
	suite.addPathToCache(path, false)
	suite.mock.EXPECT().DeleteFile(options).Return(nil)

	err = suite.attrCache.DeleteFile(options)
	suite.assert.NoError(err)
	suite.assertDeleted(path)
}

// Tests Sync File
func (suite *attrCacheTestSuite) TestSyncFile() {
	defer suite.cleanupTest()
	path := "a"
	handle := handlemap.Handle{
		Path: path,
	}

	options := internal.SyncFileOptions{Handle: &handle}

	// Error
	suite.mock.EXPECT().SyncFile(options).Return(errors.New("Failed to sync a file"))

	err := suite.attrCache.SyncFile(options)
	suite.assert.Error(err)
	suite.assertNotInCache(path)

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().SyncFile(options).Return(nil)

	err = suite.attrCache.SyncFile(options)
	suite.assert.NoError(err)
	suite.assertNotInCache(path)

	// Entry Already Exists
	suite.addPathToCache(path, false)
	suite.mock.EXPECT().SyncFile(options).Return(nil)

	err = suite.attrCache.SyncFile(options)
	suite.assert.NoError(err)
	suite.assertInvalid(path)
}

// Tests Sync Directory
func (suite *attrCacheTestSuite) TestSyncDir() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/"}

	for _, path := range paths {
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			options := internal.SyncDirOptions{Name: path}

			// Error
			suite.mock.EXPECT().SyncDir(options).Return(errors.New("Failed"))

			err := suite.attrCache.SyncDir(options)
			suite.assert.Error(err)
			suite.assertNotInCache(truncatedPath)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().SyncDir(options).Return(nil)

			err = suite.attrCache.SyncDir(options)
			suite.assert.NoError(err)
			suite.assertNotInCache(truncatedPath)

			// Entry Already Exists
			a, ab, ac := suite.addDirectoryToCache(path, false)

			suite.mock.EXPECT().SyncDir(options).Return(nil)

			err = suite.attrCache.SyncDir(options)
			suite.assert.NoError(err)
			// directory cache is enabled, so a dir paths should NOT be invalid
			for p := a.Front(); p != nil; p = p.Next() {
				path := p.Value.(string)
				isDir := path[len(path)-1] == '/'
				truncatedPath = internal.TruncateDirName(path)
				if isDir {
					suite.assertUntouched(truncatedPath)
				} else {
					suite.assertInvalid(truncatedPath)
				}
			}
			ab.PushBackList(ac) // ab and ac paths should be untouched
			for p := ab.Front(); p != nil; p = p.Next() {
				truncatedPath = internal.TruncateDirName(p.Value.(string))
				suite.assertUntouched(truncatedPath)
			}
		})
	}
}

// Tests Sync Directory
func (suite *attrCacheTestSuite) TestSyncDirNoCacheDirs() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/"}

	noCacheDirs := true
	config := fmt.Sprintf("attr_cache:\n  no-cache-dirs: %t", noCacheDirs)

	for _, path := range paths {
		suite.cleanupTest()
		suite.setupTestHelper(
			config,
		) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
		suite.assert.Equal(!noCacheDirs, suite.attrCache.cacheDirs)
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			options := internal.SyncDirOptions{Name: path}

			// Error
			suite.mock.EXPECT().SyncDir(options).Return(errors.New("Failed"))

			err := suite.attrCache.SyncDir(options)
			suite.assert.Error(err)
			suite.assertNotInCache(truncatedPath)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().SyncDir(options).Return(nil)

			err = suite.attrCache.SyncDir(options)
			suite.assert.NoError(err)
			suite.assertNotInCache(truncatedPath)

			// Entry Already Exists
			a, ab, ac := suite.addDirectoryToCache(path, false)

			suite.mock.EXPECT().SyncDir(options).Return(nil)

			err = suite.attrCache.SyncDir(options)
			suite.assert.NoError(err)
			// a paths should be deleted
			for p := a.Front(); p != nil; p = p.Next() {
				truncatedPath = internal.TruncateDirName(p.Value.(string))
				suite.assertInvalid(truncatedPath)
			}
			ab.PushBackList(ac) // ab and ac paths should be untouched
			for p := ab.Front(); p != nil; p = p.Next() {
				truncatedPath = internal.TruncateDirName(p.Value.(string))
				suite.assertUntouched(truncatedPath)
			}
		})
	}
}

// Tests Rename File
func (suite *attrCacheTestSuite) TestRenameFile() {
	defer suite.cleanupTest()
	src := "a"
	dst := "b"

	options := internal.RenameFileOptions{Src: src, Dst: dst}

	// Error
	suite.mock.EXPECT().RenameFile(options).Return(errors.New("Failed to rename a file"))

	err := suite.attrCache.RenameFile(options)
	suite.assert.Error(err)
	suite.assertNotInCache(src)
	suite.assertNotInCache(dst)

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().RenameFile(options).Return(nil)

	err = suite.attrCache.RenameFile(options)
	suite.assert.NoError(err)
	suite.assertNotInCache(src)
	suite.assertNotInCache(dst)

	// Entry Already Exists
	suite.addPathToCache(src, false)
	suite.addPathToCache(dst, false)

	attr, found := suite.attrCache.cache.get(src)
	suite.assert.True(found)
	options.SrcAttr = attr.attr
	options.SrcAttr.Size = 1
	options.SrcAttr.Mode = 2
	attr, found = suite.attrCache.cache.get(dst)
	suite.assert.True(found)
	options.DstAttr = attr.attr
	options.DstAttr.Size = 3
	options.DstAttr.Mode = 4
	srcAttrCopy := *options.SrcAttr

	suite.mock.EXPECT().RenameFile(options).Return(nil)
	err = suite.attrCache.RenameFile(options)
	suite.assert.NoError(err)
	suite.assertDeleted(src)
	attr, found = suite.attrCache.cache.get(dst)
	suite.assert.True(found)
	modifiedDstAttr := attr.attr
	assertSrcAttributeTimeChanged(suite, options.SrcAttr, srcAttrCopy)
	// Check the attributes of the dst are same as the src.
	assertAttributesTransferred(suite, options.SrcAttr, modifiedDstAttr)

	// Src Entry Exist and Dst Entry Don't Exist
	suite.addPathToCache(src, false)
	// Add negative entry to cache for Dst
	suite.attrCache.cache.insert(
		insertOptions{attr: internal.CreateObjAttrDir(dst), exists: false, cachedAt: time.Now()},
	)
	attr, found = suite.attrCache.cache.get(src)
	suite.assert.True(found)
	options.SrcAttr = attr.attr
	attr, found = suite.attrCache.cache.get(dst)
	suite.assert.True(found)
	options.DstAttr = attr.attr
	options.SrcAttr.Size = 1
	options.SrcAttr.Mode = 2
	suite.mock.EXPECT().RenameFile(options).Return(nil)
	err = suite.attrCache.RenameFile(options)
	suite.assert.NoError(err)
	suite.assertDeleted(src)
	attr, found = suite.attrCache.cache.get(dst)
	suite.assert.True(found)
	modifiedDstAttr = attr.attr
	assertSrcAttributeTimeChanged(suite, options.SrcAttr, srcAttrCopy)
	assertAttributesTransferred(suite, options.SrcAttr, modifiedDstAttr)
}

// Tests Write File
func (suite *attrCacheTestSuite) TestWriteFileError() {
	defer suite.cleanupTest()
	path := "a"
	handle := handlemap.Handle{
		Path: path,
	}

	options := internal.WriteFileOptions{Handle: &handle, Metadata: nil}

	// Error
	suite.mock.EXPECT().
		GetAttr(internal.GetAttrOptions{Name: path, RetrieveMetadata: true}).
		Return(&internal.ObjAttr{Path: path}, nil)
	suite.mock.EXPECT().WriteFile(&options).Return(0, errors.New("Failed to write a file"))

	_, err := suite.attrCache.WriteFile(&options)
	suite.assert.Error(err)
	_, found := suite.attrCache.cache.get(path)
	suite.assert.True(found)
	// GetAttr call will add this to the cache
}

func (suite *attrCacheTestSuite) TestWriteFileDoesNotExist() {
	defer suite.cleanupTest()
	path := "a"
	handle := handlemap.Handle{
		Path: path,
	}

	options := internal.WriteFileOptions{Handle: &handle, Metadata: nil}
	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().
		GetAttr(internal.GetAttrOptions{Name: path, RetrieveMetadata: true}).
		Return(&internal.ObjAttr{Path: path}, nil)
	suite.mock.EXPECT().WriteFile(&options).Return(0, nil)

	_, err := suite.attrCache.WriteFile(&options)
	suite.assert.NoError(err)
	_, found := suite.attrCache.cache.get(path)
	suite.assert.True(found)
	// GetAttr call will add this to the cache

}

func (suite *attrCacheTestSuite) TestWriteFileExists() {
	defer suite.cleanupTest()
	path := "a"
	handle := handlemap.Handle{
		Path: path,
	}

	options := internal.WriteFileOptions{Handle: &handle, Metadata: nil}
	// Entry Already Exists
	suite.addPathToCache(path, true)
	suite.mock.EXPECT().WriteFile(&options).Return(0, nil)

	_, err := suite.attrCache.WriteFile(&options)
	suite.assert.NoError(err)
	suite.assertExists(path)
}

// Tests Truncate File
func (suite *attrCacheTestSuite) TestTruncateFile() {
	defer suite.cleanupTest()
	path := "a"
	size := 1024

	options := internal.TruncateFileOptions{Name: path, NewSize: int64(size)}

	// Error
	suite.mock.EXPECT().TruncateFile(options).Return(errors.New("Failed to truncate a file"))

	err := suite.attrCache.TruncateFile(options)
	suite.assert.Error(err)
	suite.assertNotInCache(path)

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().TruncateFile(options).Return(nil)

	err = suite.attrCache.TruncateFile(options)
	suite.assert.NoError(err)
	_, found := suite.attrCache.cache.get(path)
	suite.assert.True(found)

	// Entry Already Exists
	suite.addPathToCache(path, false)
	suite.mock.EXPECT().TruncateFile(options).Return(nil)

	err = suite.attrCache.TruncateFile(options)
	suite.assert.NoError(err)

	checkItem, found := suite.attrCache.cache.get(path)
	suite.assert.True(found)
	suite.assert.EqualValues(size, checkItem.attr.Size) // new size should be set
	suite.assert.EqualValues(defaultMode, checkItem.attr.Mode)
	suite.assert.True(checkItem.valid())
	suite.assert.True(checkItem.exists())
}

// Tests CopyFromFile
func (suite *attrCacheTestSuite) TestCopyFromFileError() {
	defer suite.cleanupTest()
	path := "a"

	options := internal.CopyFromFileOptions{Name: path, File: nil, Metadata: nil}
	suite.mock.EXPECT().
		GetAttr(internal.GetAttrOptions{Name: path, RetrieveMetadata: true}).
		Return(&internal.ObjAttr{Path: path}, nil)
	// Error
	suite.mock.EXPECT().CopyFromFile(options).Return(errors.New("Failed to copy from file"))

	err := suite.attrCache.CopyFromFile(options)
	suite.assert.Error(err)
	_, found := suite.attrCache.cache.get(path)
	suite.assert.True(found)
	// GetAttr call will add this to the cache
}

func (suite *attrCacheTestSuite) TestCopyFromFileDoesNotExist() {
	defer suite.cleanupTest()
	path := "a"

	options := internal.CopyFromFileOptions{Name: path, File: nil, Metadata: nil}
	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().
		GetAttr(internal.GetAttrOptions{Name: path, RetrieveMetadata: true}).
		Return(&internal.ObjAttr{Path: path}, nil)
	suite.mock.EXPECT().CopyFromFile(options).Return(nil)

	err := suite.attrCache.CopyFromFile(options)
	suite.assert.NoError(err)
	_, found := suite.attrCache.cache.get(path)
	suite.assert.True(found)
	// GetAttr call will add this to the cache
}

func (suite *attrCacheTestSuite) TestCopyFromFileExists() {
	defer suite.cleanupTest()

	path := "a"
	options := internal.CopyFromFileOptions{Name: path, File: nil, Metadata: nil}

	// Entry Already Exists
	suite.addPathToCache(path, true)
	suite.mock.EXPECT().CopyFromFile(options).Return(nil)

	_, found := suite.attrCache.cache.get(options.Name)
	suite.assert.True(found)

	err := suite.attrCache.CopyFromFile(options)
	suite.assert.NoError(err)
}

// GetAttr
func (suite *attrCacheTestSuite) TestGetAttrExistsDeleted() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/", "a/c1", "a/c1/", "a/c2", "a/c1/gc1", "ac"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {

			suite.addDirectoryToCache("a", false)
			// delete directory a and file ac
			suite.mock.EXPECT().DeleteDir(gomock.Any()).Return(nil)
			suite.mock.EXPECT().DeleteFile(gomock.Any()).Return(nil)
			_ = suite.attrCache.DeleteDir(internal.DeleteDirOptions{Name: "a"})
			_ = suite.attrCache.DeleteFile(internal.DeleteFileOptions{Name: "ac"})

			options := internal.GetAttrOptions{Name: path}

			result, err := suite.attrCache.GetAttr(options)
			suite.assert.Equal(syscall.ENOENT, err)
			suite.assert.Nil(result)
		})
	}
}

func (suite *attrCacheTestSuite) TestGetAttrExistsWithMetadata() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/", "a/c1", "a/c1/", "a/c2", "a/c1/gc1", "ab", "ab/", "ab/c1", "ac"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			suite.addDirectoryToCache(
				"a",
				true,
			) // add the paths to the cache with IsMetadataRetrieved=true

			options := internal.GetAttrOptions{Name: path}
			// no call to mock component since attributes are accessible

			_, err := suite.attrCache.GetAttr(options)
			suite.assert.NoError(err)
			suite.assertUntouched(truncatedPath)
		})
	}
}

func (suite *attrCacheTestSuite) TestGetAttrExistsWithoutMetadataNoSymlinks() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/", "a/c1", "a/c1/", "a/c2", "a/c1/gc1", "ab", "ab/", "ab/c1", "ac"}

	noSymlinks := true
	config := fmt.Sprintf("attr_cache:\n  no-symlinks: %t", noSymlinks)

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.setupTestHelper(
			config,
		) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			suite.addDirectoryToCache(
				"a",
				true,
			) // add the paths to the cache with IsMetadataRetrived=true

			options := internal.GetAttrOptions{Name: path}
			// no call to mock component since metadata is not needed in noSymlinks mode

			_, err := suite.attrCache.GetAttr(options)
			suite.assert.NoError(err)
			suite.assertUntouched(truncatedPath)
		})
	}
}

func (suite *attrCacheTestSuite) TestGetAttrExistsWithoutMetadata() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/", "a/c1", "a/c1/", "a/c2", "a/c1/gc1", "ab", "ab/", "ab/c1", "ac"}

	for _, path := range paths {
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			suite.addDirectoryToCache(
				"a",
				true,
			) // add the paths to the cache with IsMetadataRetrieved=true

			options := internal.GetAttrOptions{Name: path}
			// no call to mock component since metadata is not needed when symlinks are disabled

			_, err := suite.attrCache.GetAttr(options)
			suite.assert.NoError(err)
			suite.assertUntouched(truncatedPath)
		})
	}
}

func (suite *attrCacheTestSuite) TestGetAttrExistsWithoutMetadataWithSymlinks() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/", "a/c1", "a/c1/", "a/c2", "a/c1/gc1", "ab", "ab/", "ab/c1", "ac"}

	enableSymlinks := true
	config := fmt.Sprintf("attr_cache:\n  enable-symlinks: %t", enableSymlinks)

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.setupTestHelper(
			config,
		) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
		suite.assert.Equal(enableSymlinks, suite.attrCache.enableSymlinks)
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			suite.addDirectoryToCache(
				"a",
				false,
			) // add the paths to the cache with IsMetadataRetrieved=false

			options := internal.GetAttrOptions{Name: path}
			// attributes should not be accessible so call the mock
			//suite.mock.EXPECT().GetAttr(options).Return(getPathAttr(path, defaultSize, fs.FileMode(defaultMode), false), nil)

			_, err := suite.attrCache.GetAttr(options)
			suite.assert.NoError(err)
			suite.assertUntouched(truncatedPath)
		})
	}
}

func (suite *attrCacheTestSuite) TestGetAttrDoesNotExist() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/", "a/c1", "a/c1/", "a/c2", "a/c1/gc1", "ab", "ab/", "ab/c1", "ac"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)

			options := internal.GetAttrOptions{Name: path}
			// attributes should not be accessible so call the mock
			suite.mock.EXPECT().
				GetAttr(options).
				Return(getPathAttr(path, defaultSize, fs.FileMode(defaultMode), false), nil)

			suite.assertCacheEmpty() // cacheMap should be empty before call
			_, err := suite.attrCache.GetAttr(options)
			suite.assert.NoError(err)
			suite.assertUntouched(truncatedPath) // item added to cache after
		})
	}
}

func (suite *attrCacheTestSuite) TestGetAttrOtherError() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)

			options := internal.GetAttrOptions{Name: path}
			suite.mock.EXPECT().GetAttr(options).Return(nil, os.ErrNotExist)

			result, err := suite.attrCache.GetAttr(options)
			suite.assert.Equal(err, os.ErrNotExist)
			suite.assert.Nil(result)
			suite.assertNotInCache(truncatedPath)
		})
	}
}

func (suite *attrCacheTestSuite) TestGetAttrEnoentError() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)

			options := internal.GetAttrOptions{Name: path}
			suite.mock.EXPECT().GetAttr(options).Return(nil, syscall.ENOENT)

			result, err := suite.attrCache.GetAttr(options)
			suite.assert.Equal(syscall.ENOENT, err)
			suite.assert.Nil(result)
			checkItem, found := suite.attrCache.cache.get(truncatedPath)
			suite.assert.True(found)
			suite.assert.True(checkItem.valid())
			suite.assert.False(checkItem.exists())
			suite.assert.NotNil(checkItem.cachedAt)
		})
	}
}

// Tests Cache Timeout
func (suite *attrCacheTestSuite) TestCacheTimeout() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	cacheTimeout := 1
	config := fmt.Sprintf("attr_cache:\n  timeout-sec: %d", cacheTimeout)
	suite.setupTestHelper(
		config,
	) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
	suite.assert.EqualValues(cacheTimeout, suite.attrCache.cacheTimeout)

	path := "a"
	options := internal.GetAttrOptions{Name: path}
	// attributes should not be accessible so call the mock
	suite.mock.EXPECT().
		GetAttr(options).
		Return(getPathAttr(path, defaultSize, fs.FileMode(defaultMode), true), nil)

	suite.assertCacheEmpty() // cacheMap should be empty before call
	_, err := suite.attrCache.GetAttr(options)
	suite.assert.NoError(err)
	suite.assertUntouched(path) // item added to cache after

	// Before cache timeout elapses, subsequent get attr should work without calling next component
	_, err = suite.attrCache.GetAttr(options)
	suite.assert.NoError(err)

	// Wait for cache timeout
	time.Sleep(time.Second * time.Duration(cacheTimeout))

	// After cache timeout elapses, subsequent get attr should need to call next component
	suite.mock.EXPECT().
		GetAttr(options).
		Return(getPathAttr(path, defaultSize, fs.FileMode(defaultMode), true), nil)
	_, err = suite.attrCache.GetAttr(options)
	suite.assert.NoError(err)
}

// Tests Cache Cleanup - expired entries are actually removed from cache map
func (suite *attrCacheTestSuite) TestCacheCleanupExpiredEntries() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	cacheTimeout := 2
	config := fmt.Sprintf("attr_cache:\n  timeout-sec: %d", cacheTimeout)
	suite.setupTestHelper(config) // setup a new attr cache with a custom config
	suite.assert.EqualValues(suite.attrCache.cacheTimeout, cacheTimeout)

	path1 := "file1"
	path2 := "file2"
	options1 := internal.GetAttrOptions{Name: path1}
	options2 := internal.GetAttrOptions{Name: path2}

	// Initially, only the root entry should exist
	suite.assert.Len(suite.attrCache.cache.cacheMap, 1)
	_, rootPresent := suite.attrCache.cache.cacheMap[""]
	suite.assert.True(rootPresent)

	// Add two files to cache
	suite.mock.EXPECT().
		GetAttr(options1).
		Return(getPathAttr(path1, defaultSize, fs.FileMode(defaultMode), true), nil)
	suite.mock.EXPECT().
		GetAttr(options2).
		Return(getPathAttr(path2, defaultSize, fs.FileMode(defaultMode), true), nil)

	_, err := suite.attrCache.GetAttr(options1)
	suite.assert.NoError(err)
	_, err = suite.attrCache.GetAttr(options2)
	suite.assert.NoError(err)

	// Add a nested directory with a child (both old/expired)
	parentDir := "parentdir"
	childFile := parentDir + "/childfile"
	parentOptions := internal.GetAttrOptions{Name: parentDir}
	childOptions := internal.GetAttrOptions{Name: childFile}

	// Expect GetAttr calls for the directory and its child
	suite.mock.EXPECT().GetAttr(parentOptions).Return(getDirPathAttr(parentDir), nil)
	suite.mock.EXPECT().
		GetAttr(childOptions).
		Return(getPathAttr(childFile, defaultSize, fs.FileMode(defaultMode), true), nil)

	_, err = suite.attrCache.GetAttr(parentOptions)
	suite.assert.NoError(err)
	_, err = suite.attrCache.GetAttr(childOptions)
	suite.assert.NoError(err)

	// Verify items are in cache (plus root)
	suite.assert.Len(
		suite.attrCache.cache.cacheMap,
		5,
	) // root + file1 + file2 + parentdir + parentdir/childfile
	suite.assertUntouched(path1)
	suite.assertUntouched(path2)
	suite.assertUntouched(childFile)

	// Wait for cache timeout to expire, plus additional time for background cleanup to run
	time.Sleep(time.Second * time.Duration(cacheTimeout+1))

	// Wait a bit more if cleanup is still in progress
	maxWait := 3 * time.Second
	waitInterval := 100 * time.Millisecond
	waited := time.Duration(0)

	for waited < maxWait {
		suite.attrCache.cacheLock.RLock()
		cacheSize := len(suite.attrCache.cache.cacheMap)
		suite.attrCache.cacheLock.RUnlock()

		// only root should remain
		if cacheSize == 1 {
			break
		}

		time.Sleep(waitInterval)
		waited += waitInterval
	}

	// Verify that expired entries have been cleaned up, but root remains
	suite.assert.Len(suite.attrCache.cache.cacheMap, 1)
	suite.assert.NotContains(suite.attrCache.cache.cacheMap, path1)
	suite.assert.NotContains(suite.attrCache.cache.cacheMap, path2)
	suite.assert.NotContains(suite.attrCache.cache.cacheMap, parentDir)
	suite.assert.NotContains(suite.attrCache.cache.cacheMap, childFile)
	suite.assert.Contains(suite.attrCache.cache.cacheMap, "")
}

func (suite *attrCacheTestSuite) TestCacheCleanupDuringBulkCaching() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	cacheTimeout := 3   // Use a longer timeout for this test
	config := fmt.Sprintf("attr_cache:\n  timeout-sec: %d", cacheTimeout)
	suite.setupTestHelper(config) // setup a new attr cache with a custom config
	suite.assert.EqualValues(suite.attrCache.cacheTimeout, cacheTimeout)

	// Add some items to cache manually with old timestamps
	path1 := "oldfile1"
	path2 := "oldfile2"
	oldTime := time.Now().Add(-time.Second * time.Duration(cacheTimeout+1))
	suite.attrCache.cache.cacheMap[path1] = newAttrCacheItem(
		getPathAttr(path1, defaultSize, fs.FileMode(defaultMode), true),
		true,
		oldTime,
	)
	suite.attrCache.cache.cacheMap[path2] = newAttrCacheItem(
		getPathAttr(path2, defaultSize, fs.FileMode(defaultMode), true),
		true,
		oldTime,
	)

	// Verify both old items are in cache plus root
	suite.assert.Len(suite.attrCache.cache.cacheMap, 3)

	// Wait a bit for background cleanup to run and remove expired items
	time.Sleep(time.Second * time.Duration(cacheTimeout+1))

	// Wait for cleanup to complete
	maxWait := 2 * time.Second
	waitInterval := 100 * time.Millisecond
	waited := time.Duration(0)

	for waited < maxWait {
		suite.attrCache.cacheLock.RLock()
		cacheSize := len(suite.attrCache.cache.cacheMap)
		suite.attrCache.cacheLock.RUnlock()

		// only root should remain
		if cacheSize == 1 {
			break
		}

		time.Sleep(waitInterval)
		waited += waitInterval
	}

	// Verify that expired entries have been cleaned up, but root remains
	suite.assert.Len(suite.attrCache.cache.cacheMap, 1)
	suite.assert.NotContains(suite.attrCache.cache.cacheMap, path1)
	suite.assert.NotContains(suite.attrCache.cache.cacheMap, path2)
	suite.assert.Contains(suite.attrCache.cache.cacheMap, "")
}

// Tests CreateLink
func (suite *attrCacheTestSuite) TestCreateLink() {
	defer suite.cleanupTest()
	// enabled symlinks
	suite.cleanupTest() // clean up the default attr cache generated
	config := "attr_cache:\n  enable-symlinks: true"
	suite.setupTestHelper(
		config,
	) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
	suite.assert.True(suite.attrCache.enableSymlinks)
	link := "a.lnk"
	path := "a"

	options := internal.CreateLinkOptions{Name: link, Target: path}

	// Error
	suite.mock.EXPECT().CreateLink(options).Return(errors.New("Failed to create a link to a file"))

	err := suite.attrCache.CreateLink(options)
	suite.assert.Error(err)
	suite.assertNotInCache(link)

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().CreateLink(options).Return(nil)

	err = suite.attrCache.CreateLink(options)
	suite.assert.NoError(err)
	suite.assertExists(link)

	// Entry Already Exists
	suite.addPathToCache(link, false)
	suite.addPathToCache(path, false)
	suite.mock.EXPECT().CreateLink(options).Return(nil)

	err = suite.attrCache.CreateLink(options)
	suite.assert.NoError(err)
	suite.assertExists(link)
	suite.assertUntouched(path)
}

// Tests Chmod
func (suite *attrCacheTestSuite) TestChmod() {
	defer suite.cleanupTest()
	mode := fs.FileMode(0)
	var paths = []string{"a", "a/"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			options := internal.ChmodOptions{Name: path, Mode: mode}

			// Error
			suite.mock.EXPECT().Chmod(options).Return(errors.New("Failed to chmod"))

			err := suite.attrCache.Chmod(options)
			suite.assert.Error(err)
			suite.assertNotInCache(truncatedPath)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().Chmod(options).Return(nil)

			err = suite.attrCache.Chmod(options)
			suite.assert.NoError(err)
			suite.assertNotInCache(truncatedPath)

			// Entry Already Exists
			suite.addPathToCache(path, false)
			suite.mock.EXPECT().Chmod(options).Return(nil)

			err = suite.attrCache.Chmod(options)
			suite.assert.NoError(err)

			checkItem, found := suite.attrCache.cache.get(truncatedPath)
			suite.assert.True(found)

			suite.assert.Equal(defaultSize, checkItem.attr.Size)
			suite.assert.Equal(mode, checkItem.attr.Mode) // new mode should be set
			suite.assert.True(checkItem.valid())
			suite.assert.True(checkItem.exists())
		})
	}
}

// Tests Chown
func (suite *attrCacheTestSuite) TestChown() {
	defer suite.cleanupTest()
	// TODO: Implement when datalake chown is supported.
	owner := 0
	group := 0
	var paths = []string{"a", "a/"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			options := internal.ChownOptions{Name: path, Owner: owner, Group: group}

			// Error
			suite.mock.EXPECT().Chown(options).Return(errors.New("Failed to chown"))

			err := suite.attrCache.Chown(options)
			suite.assert.Error(err)
			suite.assertNotInCache(truncatedPath)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().Chown(options).Return(nil)

			err = suite.attrCache.Chown(options)
			suite.assert.NoError(err)
			suite.assertNotInCache(truncatedPath)

			// Entry Already Exists
			suite.addPathToCache(path, false)
			suite.mock.EXPECT().Chown(options).Return(nil)

			err = suite.attrCache.Chown(options)
			suite.assert.NoError(err)
			suite.assertUntouched(truncatedPath)
		})
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestAttrCacheTestSuite(t *testing.T) {
	suite.Run(t, new(attrCacheTestSuite))
}
