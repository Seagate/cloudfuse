/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.

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

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
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
	flags.Set(internal.PropFlagMetadataRetrieved)
	objAttr.Flags = flags
	return objAttr
}

func getPathAttr(path string, size int64, mode os.FileMode, metadata bool) *internal.ObjAttr {
	flags := internal.NewFileBitMap()
	if metadata {
		flags.Set(internal.PropFlagMetadataRetrieved)
	}
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

func assertNotInCache(assert *assert.Assertions, attrCache *AttrCache, path string) {
	_, err := attrCache.cacheMap.get(path)
	assert.NotNil(err)
}

func addPathToCache(assert *assert.Assertions, attrCache *AttrCache, path string, metadata bool) {
	isDir := path[len(path)-1] == '/'
	path = internal.TruncateDirName(path)
	pathAttr := getPathAttr(path, defaultSize, fs.FileMode(defaultMode), metadata)
	if isDir {
		pathAttr = getDirPathAttr(path)
	}
	attrCache.cacheMap.insert(pathAttr, true, time.Now())
}

func assertDeleted(suite *attrCacheTestSuite, path string) {
	cacheItem, err := suite.attrCache.cacheMap.get(path)
	suite.assert.Nil(err)
	suite.assert.EqualValues(&internal.ObjAttr{}, cacheItem.attr)
	suite.assert.True(cacheItem.valid())
	suite.assert.False(cacheItem.exists())
}

func assertInvalid(suite *attrCacheTestSuite, path string) {
	cacheItem, err := suite.attrCache.cacheMap.get(path)
	suite.assert.Nil(err)
	suite.assert.EqualValues(&internal.ObjAttr{}, cacheItem.attr)
	suite.assert.False(cacheItem.valid())
}

func assertUntouched(suite *attrCacheTestSuite, path string) {
	cacheItem, err := suite.attrCache.cacheMap.get(path)
	suite.assert.Nil(err)
	suite.assert.NotEqualValues(cacheItem.attr, &internal.ObjAttr{})
	suite.assert.EqualValues(defaultSize, cacheItem.attr.Size)
	suite.assert.EqualValues(defaultMode, cacheItem.attr.Mode)
	suite.assert.True(cacheItem.valid())
	suite.assert.True(cacheItem.exists())
}

func assertExists(suite *attrCacheTestSuite, path string) {
	checkItem, err := suite.attrCache.cacheMap.get(path)
	suite.assert.Nil(err)
	suite.assert.NotEqualValues(checkItem.attr, &internal.ObjAttr{})
	suite.assert.True(checkItem.valid())
	suite.assert.True(checkItem.exists())
}

func assertInCloud(suite *attrCacheTestSuite, path string) {
	checkItem, err := suite.attrCache.cacheMap.get(path)
	suite.assert.Nil(err)
	suite.assert.NotEqualValues(checkItem.attr, &internal.ObjAttr{})
	suite.assert.True(checkItem.valid())
	suite.assert.True(checkItem.exists())
	suite.assert.True(checkItem.isInCloud())
}

func assertNotInCloud(suite *attrCacheTestSuite, path string) {
	checkItem, err := suite.attrCache.cacheMap.get(path)
	suite.assert.Nil(err)
	suite.assert.NotEqualValues(checkItem.attr, &internal.ObjAttr{})
	suite.assert.True(checkItem.valid())
	suite.assert.True(checkItem.exists())
	suite.assert.False(checkItem.isInCloud())
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
	i := 0
	for p := a.Front(); p != nil; p = p.Next() {
		pString := p.Value.(string)
		isDir := pString[len(pString)-1] == '/'
		pString = internal.TruncateDirName(pString)
		newPathAttr := getPathAttr(pString, size, mode, true)
		if isDir {
			newPathAttr = getDirPathAttr(pString)
		}
		pathAttrs = append(pathAttrs, newPathAttr)
		i++
	}
	return pathAttrs
}

func addDirectoryToCache(assert *assert.Assertions, attrCache *AttrCache, path string, metadata bool) (*list.List, *list.List, *list.List) {
	// TODO: flag directories as such, or else recursion based on IsDir() won't work...
	aPaths, abPaths, acPaths := generateDirectory(path)

	for p := aPaths.Front(); p != nil; p = p.Next() {
		addPathToCache(assert, attrCache, p.Value.(string), metadata)
	}
	for p := abPaths.Front(); p != nil; p = p.Next() {
		addPathToCache(assert, attrCache, p.Value.(string), metadata)
	}
	for p := acPaths.Front(); p != nil; p = p.Next() {
		addPathToCache(assert, attrCache, p.Value.(string), metadata)
	}

	return aPaths, abPaths, acPaths
}

func (suite *attrCacheTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{})
	if err != nil {
		panic("Unable to set silent logger as default.")
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
	suite.assert.Equal(suite.attrCache.Name(), "attr_cache")
	suite.assert.EqualValues(120, suite.attrCache.cacheTimeout)
	suite.assert.Equal(suite.attrCache.cacheOnList, true)
	suite.assert.Equal(suite.attrCache.noSymlinks, false)
	suite.assert.Equal(suite.attrCache.cacheDirs, true)
}

// Tests configuration
func (suite *attrCacheTestSuite) TestConfig() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	config := "attr_cache:\n  timeout-sec: 60\n  no-cache-on-list: true\n  no-symlinks: true\n  no-cache-dirs: true"
	suite.setupTestHelper(config) // setup a new attr cache with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal(suite.attrCache.Name(), "attr_cache")
	suite.assert.EqualValues(60, suite.attrCache.cacheTimeout)
	suite.assert.Equal(suite.attrCache.cacheOnList, false)
	suite.assert.Equal(suite.attrCache.noSymlinks, true)
	suite.assert.Equal(suite.attrCache.cacheDirs, false)
}

// Tests max files config
func (suite *attrCacheTestSuite) TestConfigMaxFiles() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	cacheTimeout := 1
	maxFiles := 10
	config := fmt.Sprintf("attr_cache:\n  timeout-sec: %d\n  max-files: %d", cacheTimeout, maxFiles)
	suite.setupTestHelper(config) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
	suite.assert.EqualValues(suite.attrCache.maxFiles, maxFiles)
}

func (suite *attrCacheTestSuite) TestConfigZero() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	config := "attr_cache:\n  timeout-sec: 0\n  no-cache-on-list: true\n  no-symlinks: true\n  no-cache-dirs: true"
	suite.setupTestHelper(config) // setup a new attr cache with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal(suite.attrCache.Name(), "attr_cache")
	suite.assert.EqualValues(0, suite.attrCache.cacheTimeout)
	suite.assert.Equal(suite.attrCache.cacheOnList, false)
	suite.assert.Equal(suite.attrCache.noSymlinks, true)
	suite.assert.Equal(suite.attrCache.cacheDirs, false)
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
			suite.assert.NotNil(err)

			assertNotInCache(suite.assert, suite.attrCache, truncatedPath)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().CreateDir(options).Return(nil)

			err = suite.attrCache.CreateDir(options)
			suite.assert.Nil(err)

			_, err = suite.attrCache.cacheMap.get(truncatedPath)
			suite.assert.Nil(err)

			// Entry Already Exists
			suite.mock.EXPECT().CreateDir(options).Return(nil)

			err = suite.attrCache.CreateDir(options)
			suite.assert.Equal(os.ErrExist, err)

			_, err = suite.attrCache.cacheMap.get(truncatedPath)
			suite.assert.Nil(err)
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
		suite.setupTestHelper(config) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
		suite.assert.EqualValues(!noCacheDirs, suite.attrCache.cacheDirs)
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			extendedPath := internal.ExtendDirName(path)
			options := internal.CreateDirOptions{Name: path}

			// Error
			suite.mock.EXPECT().CreateDir(options).Return(errors.New("Failed"))

			err := suite.attrCache.CreateDir(options)
			suite.assert.NotNil(err)

			assertNotInCache(suite.assert, suite.attrCache, truncatedPath)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().CreateDir(options).Return(nil)

			err = suite.attrCache.CreateDir(options)
			suite.assert.Nil(err)
			assertNotInCache(suite.assert, suite.attrCache, truncatedPath)

			// Entry Already Exists
			addPathToCache(suite.assert, suite.attrCache, extendedPath, false)
			suite.mock.EXPECT().CreateDir(options).Return(nil)

			err = suite.attrCache.CreateDir(options)
			suite.assert.Nil(err)
			assertInvalid(suite, truncatedPath)
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
			suite.assert.NotNil(err)
			assertNotInCache(suite.assert, suite.attrCache, truncatedPath)

			// Entry Does Not Exist
			suite.mock.EXPECT().DeleteDir(options).Return(nil)

			err = suite.attrCache.DeleteDir(options)
			suite.assert.True(os.IsNotExist(err))
			assertNotInCache(suite.assert, suite.attrCache, truncatedPath)

			// Entry Exists
			a, ab, ac := addDirectoryToCache(suite.assert, suite.attrCache, path, false)

			suite.mock.EXPECT().DeleteDir(options).Return(nil)

			err = suite.attrCache.DeleteDir(options)
			suite.assert.Nil(err)
			// a paths should be deleted
			for p := a.Front(); p != nil; p = p.Next() {
				truncatedPath = internal.TruncateDirName(p.Value.(string))
				assertDeleted(suite, truncatedPath)
			}
			ab.PushBackList(ac) // ab and ac paths should be untouched
			for p := ab.Front(); p != nil; p = p.Next() {
				truncatedPath = internal.TruncateDirName(p.Value.(string))
				assertUntouched(suite, truncatedPath)
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
		suite.setupTestHelper(config) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
		suite.assert.EqualValues(!noCacheDirs, suite.attrCache.cacheDirs)
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			options := internal.DeleteDirOptions{Name: path}

			// Error
			suite.mock.EXPECT().DeleteDir(options).Return(errors.New("Failed"))

			err := suite.attrCache.DeleteDir(options)
			suite.assert.NotNil(err)
			assertNotInCache(suite.assert, suite.attrCache, truncatedPath)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().DeleteDir(options).Return(nil)

			err = suite.attrCache.DeleteDir(options)
			suite.assert.Nil(err)
			assertNotInCache(suite.assert, suite.attrCache, truncatedPath)

			// Entry Already Exists
			a, ab, ac := addDirectoryToCache(suite.assert, suite.attrCache, path, false)

			suite.mock.EXPECT().DeleteDir(options).Return(nil)

			err = suite.attrCache.DeleteDir(options)
			suite.assert.Nil(err)
			// a paths should be deleted
			for p := a.Front(); p != nil; p = p.Next() {
				truncatedPath = internal.TruncateDirName(p.Value.(string))
				assertDeleted(suite, truncatedPath)
			}
			ab.PushBackList(ac) // ab and ac paths should be untouched
			for p := ab.Front(); p != nil; p = p.Next() {
				truncatedPath = internal.TruncateDirName(p.Value.(string))
				assertUntouched(suite, truncatedPath)
			}
		})
	}
}

// Tests Read Directory
func (suite *attrCacheTestSuite) TestReadDirDoesNotExist() {
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

			options := internal.ReadDirOptions{Name: path}

			// Success
			// Entries Do Not Already Exist
			suite.mock.EXPECT().ReadDir(options).Return(aAttr, nil)

			suite.assert.Empty(suite.attrCache.cacheMap.children) // cacheMap should be empty before call
			returnedAttr, err := suite.attrCache.ReadDir(options)
			suite.assert.Nil(err)
			suite.assert.Equal(aAttr, returnedAttr)

			// Entries should now be in the cache
			for _, p := range aAttr {
				checkItem, err := suite.attrCache.cacheMap.get(p.Path)
				suite.assert.Nil(err)
				suite.assert.NotEqualValues(checkItem.attr, &internal.ObjAttr{})
				if !p.IsDir() {
					suite.assert.EqualValues(size, checkItem.attr.Size) // new size should be set
					suite.assert.EqualValues(mode, checkItem.attr.Mode) // new mode should be set
				}
				suite.assert.True(checkItem.valid())
				suite.assert.True(checkItem.exists())
			}
		})
	}
}

func (suite *attrCacheTestSuite) TestReadDirExists() {
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

			options := internal.ReadDirOptions{Name: path}

			// Success
			// Entries Already Exist
			a, ab, ac := addDirectoryToCache(suite.assert, suite.attrCache, path, false)

			suite.assert.NotEmpty(suite.attrCache.cacheMap) // cacheMap should NOT be empty before read dir call and values should be untouched
			for _, p := range aAttr {
				assertUntouched(suite, p.Path)
			}
			suite.mock.EXPECT().ReadDir(options).Return(aAttr, nil)
			returnedAttr, err := suite.attrCache.ReadDir(options)
			suite.assert.Nil(err)
			suite.assert.Equal(aAttr, returnedAttr)

			// a paths should now be updated in the cache
			for p := a.Front(); p != nil; p = p.Next() {
				pString := p.Value.(string)
				cachePath := internal.TruncateDirName(pString)
				checkItem, err := suite.attrCache.cacheMap.get(cachePath)
				suite.assert.Nil(err)
				suite.assert.NotEqualValues(checkItem.attr, &internal.ObjAttr{})
				if !checkItem.attr.IsDir() {
					suite.assert.EqualValues(size, checkItem.attr.Size) // new size should be set
					suite.assert.EqualValues(mode, checkItem.attr.Mode) // new mode should be set
				}
				suite.assert.True(checkItem.valid())
				suite.assert.True(checkItem.exists())
			}

			// ab and ac paths should be untouched
			ab.PushBackList(ac)
			for p := ab.Front(); p != nil; p = p.Next() {
				pString := p.Value.(string)
				cachePath := internal.TruncateDirName(pString)
				assertUntouched(suite, cachePath)
			}
		})
	}
}

// Test whether the attribute cache correctly tracks which directories are in cloud storage
func (suite *attrCacheTestSuite) TestDirInCloud() {
	defer suite.cleanupTest()
	// build up the attribute cache
	addDirectoryToCache(suite.assert, suite.attrCache, "a", true)
	deepPath := "a/b/c/d"
	addPathToCache(suite.assert, suite.attrCache, deepPath, true)

	// delete file a/b/c/d and make sure a/b/ and a/b/c/ are marked not in cloud storage
	delOptions := internal.DeleteFileOptions{Name: deepPath}
	suite.mock.EXPECT().DeleteFile(delOptions).Return(nil)

	err := suite.attrCache.DeleteFile(delOptions)
	suite.assert.Nil(err)
	assertDeleted(suite, deepPath)
	assertNotInCloud(suite, "a/b/c")
	assertNotInCloud(suite, "a/b")
	assertInCloud(suite, "a")

	// add file a/b/c/d back in and make sure all its ancestors are marked in cloud storage
	createOptions := internal.CreateFileOptions{Name: deepPath}
	suite.mock.EXPECT().CreateFile(createOptions).Return(&handlemap.Handle{}, nil)

	_, err = suite.attrCache.CreateFile(createOptions)
	suite.assert.Nil(err)
	assertInvalid(suite, deepPath)
	assertInCloud(suite, "a/b/c")
	assertInCloud(suite, "a/b")
	assertInCloud(suite, "a")
}

func (suite *attrCacheTestSuite) TestReadDirNoCacheOnList() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	cacheOnList := false
	config := fmt.Sprintf("attr_cache:\n  no-cache-on-list: %t", !cacheOnList)
	suite.setupTestHelper(config) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
	suite.assert.EqualValues(cacheOnList, suite.attrCache.cacheOnList)
	path := "a"
	size := int64(1024)
	mode := os.FileMode(0)
	aAttr := generateNestedPathAttr(path, size, mode)

	options := internal.ReadDirOptions{Name: path}
	suite.mock.EXPECT().ReadDir(options).Return(aAttr, nil)

	suite.assert.Empty(suite.attrCache.cacheMap.children) // cacheMap should be empty before call
	returnedAttr, err := suite.attrCache.ReadDir(options)
	suite.assert.Nil(err)
	suite.assert.Equal(aAttr, returnedAttr)

	// cacheMap should only have the listed after the call
	assertExists(suite, path)
}

func (suite *attrCacheTestSuite) TestReadDirNoCacheOnListNoCacheDirs() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	cacheOnList := false
	cacheDirs := false
	config := fmt.Sprintf("attr_cache:\n  no-cache-on-list: %t\n  no-cache-dirs: %t", !cacheOnList, !cacheDirs)
	suite.setupTestHelper(config) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
	suite.assert.EqualValues(cacheOnList, suite.attrCache.cacheOnList)
	suite.assert.EqualValues(cacheDirs, suite.attrCache.cacheDirs)
	path := "a"
	size := int64(1024)
	mode := os.FileMode(0)
	aAttr := generateNestedPathAttr(path, size, mode)

	options := internal.ReadDirOptions{Name: path}
	suite.mock.EXPECT().ReadDir(options).Return(aAttr, nil)

	suite.assert.Empty(suite.attrCache.cacheMap.children) // cacheMap should be empty before call
	returnedAttr, err := suite.attrCache.ReadDir(options)
	suite.assert.Nil(err)
	suite.assert.Equal(aAttr, returnedAttr)

	suite.assert.Empty(suite.attrCache.cacheMap.children) // cacheMap should be empty after call
}

func (suite *attrCacheTestSuite) TestReadDirError() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/", "ab", "ab/"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			options := internal.ReadDirOptions{Name: path}

			// Error
			suite.mock.EXPECT().ReadDir(options).Return(make([]*internal.ObjAttr, 0), errors.New("Failed to read a directory"))

			_, err := suite.attrCache.ReadDir(options)
			suite.assert.NotNil(err)
			assertNotInCache(suite.assert, suite.attrCache, truncatedPath)
		})
	}
}

func (suite *attrCacheTestSuite) TestIsDirEmpty() {
	defer suite.cleanupTest()
	// Setup
	path := "dir/"
	options := internal.IsDirEmptyOptions{
		Name: path,
	}
	addPathToCache(suite.assert, suite.attrCache, path, false)
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
	addDirectoryToCache(suite.assert, suite.attrCache, path, false)
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
			suite.mock.EXPECT().RenameDir(options).Return(errors.New("Failed to rename a directory"))

			err := suite.attrCache.RenameDir(options)
			suite.assert.NotNil(err)
			assertNotInCache(suite.assert, suite.attrCache, truncatedSrc)
			assertNotInCache(suite.assert, suite.attrCache, truncatedDst)

			// Error
			// Source Entry Does Not Exist
			suite.mock.EXPECT().RenameDir(options).Return(nil)

			err = suite.attrCache.RenameDir(options)
			suite.assert.NotNil(err)

			assertNotInCache(suite.assert, suite.attrCache, truncatedSrc)
			assertNotInCache(suite.assert, suite.attrCache, truncatedDst)

			// Error
			// Destination Entry (ab) Already Exists
			a, ab, ac := addDirectoryToCache(suite.assert, suite.attrCache, input.src, false)

			suite.mock.EXPECT().RenameDir(options).Return(nil)

			err = suite.attrCache.RenameDir(options)
			suite.assert.True(os.IsExist(err))

			// Success
			// Source Entry Exists and Destination Entry Does Not Already Exist
			deleteDirOptions := internal.DeleteDirOptions{Name: input.dst}
			suite.mock.EXPECT().DeleteDir(deleteDirOptions).Return(nil)

			err = suite.attrCache.DeleteDir(deleteDirOptions)
			suite.assert.Nil(err)

			suite.mock.EXPECT().RenameDir(options).Return(nil)

			err = suite.attrCache.RenameDir(options)
			suite.assert.Nil(err)

			// a paths should be deleted
			for p := a.Front(); p != nil; p = p.Next() {
				truncatedPath := internal.TruncateDirName(p.Value.(string))
				assertDeleted(suite, truncatedPath)
			}
			// ab paths happen to both be dir paths now, so they should exist
			for p := ab.Front(); p != nil; p = p.Next() {
				pString := p.Value.(string)
				truncatedPath := internal.TruncateDirName(pString)
				assertExists(suite, truncatedPath)
			}
			// ac paths should be untouched
			for p := ac.Front(); p != nil; p = p.Next() {
				truncatedPath := internal.TruncateDirName(p.Value.(string))
				assertUntouched(suite, truncatedPath)
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
		suite.setupTestHelper(config) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
		suite.assert.EqualValues(!noCacheDirs, suite.attrCache.cacheDirs)
		suite.Run(input.src+"->"+input.dst, func() {
			truncatedSrc := internal.TruncateDirName(input.src)
			truncatedDst := internal.TruncateDirName(input.dst)
			options := internal.RenameDirOptions{Src: input.src, Dst: input.dst}

			// Error
			suite.mock.EXPECT().RenameDir(options).Return(errors.New("Failed to rename a directory"))

			err := suite.attrCache.RenameDir(options)
			suite.assert.NotNil(err)

			assertNotInCache(suite.assert, suite.attrCache, truncatedSrc)
			assertNotInCache(suite.assert, suite.attrCache, truncatedDst)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().RenameDir(options).Return(nil)

			err = suite.attrCache.RenameDir(options)
			suite.assert.Nil(err)
			assertNotInCache(suite.assert, suite.attrCache, truncatedSrc)
			assertNotInCache(suite.assert, suite.attrCache, truncatedDst)

			// Entry Already Exists
			a, ab, ac := addDirectoryToCache(suite.assert, suite.attrCache, input.src, false)

			suite.mock.EXPECT().RenameDir(options).Return(nil)

			err = suite.attrCache.RenameDir(options)
			suite.assert.Nil(err)
			// a paths should be deleted
			for p := a.Front(); p != nil; p = p.Next() {
				truncatedPath := internal.TruncateDirName(p.Value.(string))
				assertDeleted(suite, truncatedPath)
			}
			// ab paths should be invalidated
			for p := ab.Front(); p != nil; p = p.Next() {
				truncatedPath := internal.TruncateDirName(p.Value.(string))
				assertInvalid(suite, truncatedPath)
			}
			// ac paths should be untouched
			for p := ac.Front(); p != nil; p = p.Next() {
				truncatedPath := internal.TruncateDirName(p.Value.(string))
				assertUntouched(suite, truncatedPath)
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
	suite.assert.NotNil(err)
	assertNotInCache(suite.assert, suite.attrCache, path)

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().CreateFile(options).Return(&handlemap.Handle{}, nil)

	_, err = suite.attrCache.CreateFile(options)
	suite.assert.Nil(err)

	assertNotInCache(suite.assert, suite.attrCache, path)

	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, path, false)
	suite.mock.EXPECT().CreateFile(options).Return(&handlemap.Handle{}, nil)

	_, err = suite.attrCache.CreateFile(options)
	suite.assert.Nil(err)
	assertInvalid(suite, path)
}

// Tests Delete File
func (suite *attrCacheTestSuite) TestDeleteFile() {
	defer suite.cleanupTest()
	path := "a"

	options := internal.DeleteFileOptions{Name: path}

	// Error
	suite.mock.EXPECT().DeleteFile(options).Return(errors.New("Failed to delete a file"))

	err := suite.attrCache.DeleteFile(options)
	suite.assert.NotNil(err)
	assertNotInCache(suite.assert, suite.attrCache, path)

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().DeleteFile(options).Return(nil)

	err = suite.attrCache.DeleteFile(options)
	suite.assert.Nil(err)
	assertNotInCache(suite.assert, suite.attrCache, path)

	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, path, false)
	suite.mock.EXPECT().DeleteFile(options).Return(nil)

	err = suite.attrCache.DeleteFile(options)
	suite.assert.Nil(err)
	assertDeleted(suite, path)
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
	suite.assert.NotNil(err)
	assertNotInCache(suite.assert, suite.attrCache, path)

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().SyncFile(options).Return(nil)

	err = suite.attrCache.SyncFile(options)
	suite.assert.Nil(err)
	assertNotInCache(suite.assert, suite.attrCache, path)

	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, path, false)
	suite.mock.EXPECT().SyncFile(options).Return(nil)

	err = suite.attrCache.SyncFile(options)
	suite.assert.Nil(err)
	assertInvalid(suite, path)
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
			suite.assert.NotNil(err)
			assertNotInCache(suite.assert, suite.attrCache, truncatedPath)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().SyncDir(options).Return(nil)

			err = suite.attrCache.SyncDir(options)
			suite.assert.Nil(err)
			assertNotInCache(suite.assert, suite.attrCache, truncatedPath)

			// Entry Already Exists
			a, ab, ac := addDirectoryToCache(suite.assert, suite.attrCache, path, false)

			suite.mock.EXPECT().SyncDir(options).Return(nil)

			err = suite.attrCache.SyncDir(options)
			suite.assert.Nil(err)
			// directory cache is enabled, so a paths should NOT be deleted
			for p := a.Front(); p != nil; p = p.Next() {
				path := p.Value.(string)
				isDir := path[len(path)-1] == '/'
				truncatedPath = internal.TruncateDirName(path)
				if isDir {
					assertUntouched(suite, truncatedPath)
				} else {
					assertInvalid(suite, truncatedPath)
				}
			}
			ab.PushBackList(ac) // ab and ac paths should be untouched
			for p := ab.Front(); p != nil; p = p.Next() {
				truncatedPath = internal.TruncateDirName(p.Value.(string))
				assertUntouched(suite, truncatedPath)
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
		suite.setupTestHelper(config) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
		suite.assert.EqualValues(!noCacheDirs, suite.attrCache.cacheDirs)
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			options := internal.SyncDirOptions{Name: path}

			// Error
			suite.mock.EXPECT().SyncDir(options).Return(errors.New("Failed"))

			err := suite.attrCache.SyncDir(options)
			suite.assert.NotNil(err)
			assertNotInCache(suite.assert, suite.attrCache, truncatedPath)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().SyncDir(options).Return(nil)

			err = suite.attrCache.SyncDir(options)
			suite.assert.Nil(err)
			assertNotInCache(suite.assert, suite.attrCache, truncatedPath)

			// Entry Already Exists
			a, ab, ac := addDirectoryToCache(suite.assert, suite.attrCache, path, false)

			suite.mock.EXPECT().SyncDir(options).Return(nil)

			err = suite.attrCache.SyncDir(options)
			suite.assert.Nil(err)
			// a paths should be deleted
			for p := a.Front(); p != nil; p = p.Next() {
				truncatedPath = internal.TruncateDirName(p.Value.(string))
				assertInvalid(suite, truncatedPath)
			}
			ab.PushBackList(ac) // ab and ac paths should be untouched
			for p := ab.Front(); p != nil; p = p.Next() {
				truncatedPath = internal.TruncateDirName(p.Value.(string))
				assertUntouched(suite, truncatedPath)
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
	suite.assert.NotNil(err)
	assertNotInCache(suite.assert, suite.attrCache, src)
	assertNotInCache(suite.assert, suite.attrCache, dst)

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().RenameFile(options).Return(nil)

	err = suite.attrCache.RenameFile(options)
	suite.assert.Nil(err)
	assertNotInCache(suite.assert, suite.attrCache, src)
	assertNotInCache(suite.assert, suite.attrCache, dst)

	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, src, false)
	addPathToCache(suite.assert, suite.attrCache, dst, false)
	suite.mock.EXPECT().RenameFile(options).Return(nil)

	err = suite.attrCache.RenameFile(options)
	suite.assert.Nil(err)
	assertDeleted(suite, src)
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
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: path, RetrieveMetadata: true}).Return(&internal.ObjAttr{Path: path}, nil)
	suite.mock.EXPECT().WriteFile(options).Return(0, errors.New("Failed to write a file"))

	_, err := suite.attrCache.WriteFile(options)
	suite.assert.NotNil(err)
	_, err = suite.attrCache.cacheMap.get(path)
	suite.assert.Nil(err)
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
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: path, RetrieveMetadata: true}).Return(&internal.ObjAttr{Path: path}, nil)
	suite.mock.EXPECT().WriteFile(options).Return(0, nil)

	_, err := suite.attrCache.WriteFile(options)
	suite.assert.Nil(err)
	suite.attrCache.cacheMap.get(path)
	suite.assert.Nil(err)
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
	addPathToCache(suite.assert, suite.attrCache, path, true)
	suite.mock.EXPECT().WriteFile(options).Return(0, nil)

	_, err := suite.attrCache.WriteFile(options)
	suite.assert.Nil(err)
	assertInvalid(suite, path)
}

// Tests Truncate File
func (suite *attrCacheTestSuite) TestTruncateFile() {
	defer suite.cleanupTest()
	path := "a"
	size := 1024

	options := internal.TruncateFileOptions{Name: path, Size: int64(size)}

	// Error
	suite.mock.EXPECT().TruncateFile(options).Return(errors.New("Failed to truncate a file"))

	err := suite.attrCache.TruncateFile(options)
	suite.assert.NotNil(err)
	assertNotInCache(suite.assert, suite.attrCache, path)

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().TruncateFile(options).Return(nil)

	err = suite.attrCache.TruncateFile(options)
	suite.assert.Nil(err)
	_, err = suite.attrCache.cacheMap.get(path)
	suite.assert.Nil(err)

	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, path, false)
	suite.mock.EXPECT().TruncateFile(options).Return(nil)

	err = suite.attrCache.TruncateFile(options)
	suite.assert.Nil(err)

	checkItem, err := suite.attrCache.cacheMap.get(path)
	suite.assert.Nil(err)
	suite.assert.NotEqualValues(checkItem.attr, &internal.ObjAttr{})
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
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: path, RetrieveMetadata: true}).Return(&internal.ObjAttr{Path: path}, nil)
	// Error
	suite.mock.EXPECT().CopyFromFile(options).Return(errors.New("Failed to copy from file"))

	err := suite.attrCache.CopyFromFile(options)
	suite.assert.NotNil(err)
	_, err = suite.attrCache.cacheMap.get(path)
	suite.assert.Nil(err)
	// GetAttr call will add this to the cache
}

func (suite *attrCacheTestSuite) TestCopyFromFileDoesNotExist() {
	defer suite.cleanupTest()
	path := "a"

	options := internal.CopyFromFileOptions{Name: path, File: nil, Metadata: nil}
	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: path, RetrieveMetadata: true}).Return(&internal.ObjAttr{Path: path}, nil)
	suite.mock.EXPECT().CopyFromFile(options).Return(nil)

	err := suite.attrCache.CopyFromFile(options)
	suite.assert.Nil(err)
	_, err = suite.attrCache.cacheMap.get(path)
	suite.assert.Nil(err)
	// GetAttr call will add this to the cache
}

func (suite *attrCacheTestSuite) TestCopyFromFileExists() {
	defer suite.cleanupTest()

	path := "a"
	options := internal.CopyFromFileOptions{Name: path, File: nil, Metadata: nil}

	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, path, true)
	suite.mock.EXPECT().CopyFromFile(options).Return(nil)

	_, getErr := suite.attrCache.cacheMap.get(options.Name)
	suite.assert.Nil(getErr)

	err := suite.attrCache.CopyFromFile(options)
	suite.assert.Nil(err)
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

			addDirectoryToCache(suite.assert, suite.attrCache, "a", false)
			// delete directory a and file ac
			suite.mock.EXPECT().DeleteDir(gomock.Any()).Return(nil)
			suite.mock.EXPECT().DeleteFile(gomock.Any()).Return(nil)
			_ = suite.attrCache.DeleteDir(internal.DeleteDirOptions{Name: "a"})
			_ = suite.attrCache.DeleteFile(internal.DeleteFileOptions{Name: "ac"})

			options := internal.GetAttrOptions{Name: path}
			// no call to mock component since attributes are accessible

			result, err := suite.attrCache.GetAttr(options)
			suite.assert.Equal(err, syscall.ENOENT)
			suite.assert.EqualValues(&internal.ObjAttr{}, result)
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
			addDirectoryToCache(suite.assert, suite.attrCache, "a", true) // add the paths to the cache with IsMetadataRetrieved=true

			options := internal.GetAttrOptions{Name: path}
			// no call to mock component since attributes are accessible

			_, err := suite.attrCache.GetAttr(options)
			suite.assert.Nil(err)
			assertUntouched(suite, truncatedPath)
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
		suite.setupTestHelper(config) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
		suite.assert.EqualValues(noSymlinks, suite.attrCache.cacheOnList)
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			addDirectoryToCache(suite.assert, suite.attrCache, "a", true) // add the paths to the cache with IsMetadataRetrieved=true

			options := internal.GetAttrOptions{Name: path}
			// no call to mock component since metadata is not needed in noSymlinks mode

			_, err := suite.attrCache.GetAttr(options)
			suite.assert.Nil(err)
			assertUntouched(suite, truncatedPath)
		})
	}
}

func (suite *attrCacheTestSuite) TestGetAttrExistsWithoutMetadata() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/", "a/c1", "a/c1/", "a/c2", "a/c1/gc1", "ab", "ab/", "ab/c1", "ac"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			addDirectoryToCache(suite.assert, suite.attrCache, "a", false) // add the paths to the cache with IsMetadataRetrieved=false

			options := internal.GetAttrOptions{Name: path}
			// attributes should not be accessible so call the mock
			suite.mock.EXPECT().GetAttr(options).Return(getPathAttr(path, defaultSize, fs.FileMode(defaultMode), false), nil).AnyTimes()

			_, err := suite.attrCache.GetAttr(options)
			suite.assert.Nil(err)
			assertUntouched(suite, truncatedPath)
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
			suite.mock.EXPECT().GetAttr(options).Return(getPathAttr(path, defaultSize, fs.FileMode(defaultMode), false), nil)

			suite.assert.Empty(suite.attrCache.cacheMap.children) // cacheMap should be empty before call
			_, err := suite.attrCache.GetAttr(options)
			suite.assert.Nil(err)
			assertUntouched(suite, truncatedPath) // item added to cache after
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
			suite.mock.EXPECT().GetAttr(options).Return(&internal.ObjAttr{}, os.ErrNotExist)

			result, err := suite.attrCache.GetAttr(options)
			suite.assert.Equal(err, os.ErrNotExist)
			suite.assert.EqualValues(&internal.ObjAttr{}, result)
			assertNotInCache(suite.assert, suite.attrCache, truncatedPath)
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
			suite.mock.EXPECT().GetAttr(options).Return(&internal.ObjAttr{}, syscall.ENOENT)

			result, err := suite.attrCache.GetAttr(options)
			suite.assert.Equal(err, syscall.ENOENT)
			suite.assert.EqualValues(&internal.ObjAttr{}, result)
			checkItem, err := suite.attrCache.cacheMap.get(truncatedPath)
			suite.assert.Nil(err)
			suite.assert.EqualValues(&internal.ObjAttr{Path: "a"}, checkItem.attr)
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
	suite.setupTestHelper(config) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
	suite.assert.EqualValues(cacheTimeout, suite.attrCache.cacheTimeout)

	path := "a"
	options := internal.GetAttrOptions{Name: path}
	// attributes should not be accessible so call the mock
	suite.mock.EXPECT().GetAttr(options).Return(getPathAttr(path, defaultSize, fs.FileMode(defaultMode), true), nil)

	suite.assert.Empty(suite.attrCache.cacheMap.children) // cacheMap should be empty before call
	_, err := suite.attrCache.GetAttr(options)
	suite.assert.Nil(err)
	assertUntouched(suite, path) // item added to cache after

	// Before cache timeout elapses, subsequent get attr should work without calling next component
	_, err = suite.attrCache.GetAttr(options)
	suite.assert.Nil(err)

	// Wait for cache timeout
	time.Sleep(time.Second * time.Duration(cacheTimeout))

	// After cache timeout elapses, subsequent get attr should need to call next component
	suite.mock.EXPECT().GetAttr(options).Return(getPathAttr(path, defaultSize, fs.FileMode(defaultMode), true), nil)
	_, err = suite.attrCache.GetAttr(options)
	suite.assert.Nil(err)
}

// Tests CreateLink
func (suite *attrCacheTestSuite) TestCreateLink() {
	defer suite.cleanupTest()
	link := "a.lnk"
	path := "a"

	options := internal.CreateLinkOptions{Name: link, Target: path}

	// Error
	suite.mock.EXPECT().CreateLink(options).Return(errors.New("Failed to create a link to a file"))

	err := suite.attrCache.CreateLink(options)
	suite.assert.NotNil(err)

	assertNotInCache(suite.assert, suite.attrCache, link)

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().CreateLink(options).Return(nil)

	err = suite.attrCache.CreateLink(options)
	suite.assert.Nil(err)
	assertNotInCache(suite.assert, suite.attrCache, link)

	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, link, false)
	addPathToCache(suite.assert, suite.attrCache, path, false)
	suite.mock.EXPECT().CreateLink(options).Return(nil)

	err = suite.attrCache.CreateLink(options)
	suite.assert.Nil(err)
	assertInvalid(suite, link)
	assertUntouched(suite, path)
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
			suite.assert.NotNil(err)

			assertNotInCache(suite.assert, suite.attrCache, truncatedPath)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().Chmod(options).Return(nil)

			err = suite.attrCache.Chmod(options)
			suite.assert.Nil(err)

			assertNotInCache(suite.assert, suite.attrCache, truncatedPath)

			// Entry Already Exists
			addPathToCache(suite.assert, suite.attrCache, path, false)
			suite.mock.EXPECT().Chmod(options).Return(nil)

			err = suite.attrCache.Chmod(options)
			suite.assert.Nil(err)

			checkItem, err := suite.attrCache.cacheMap.get(truncatedPath)
			suite.assert.Nil(err)

			suite.assert.NotEqualValues(checkItem.attr, &internal.ObjAttr{})
			suite.assert.EqualValues(defaultSize, checkItem.attr.Size)
			suite.assert.EqualValues(mode, checkItem.attr.Mode) // new mode should be set
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
			suite.assert.NotNil(err)

			assertNotInCache(suite.assert, suite.attrCache, truncatedPath)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().Chown(options).Return(nil)

			err = suite.attrCache.Chown(options)
			suite.assert.Nil(err)
			assertNotInCache(suite.assert, suite.attrCache, truncatedPath)

			// Entry Already Exists
			addPathToCache(suite.assert, suite.attrCache, path, false)
			suite.mock.EXPECT().Chown(options).Return(nil)

			err = suite.attrCache.Chown(options)
			suite.assert.Nil(err)
			assertUntouched(suite, truncatedPath)
		})
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestAttrCacheTestSuite(t *testing.T) {
	suite.Run(t, new(attrCacheTestSuite))
}
