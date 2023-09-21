/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

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

package temp_cache_map_tree

import (
	"cloudfuse/internal"
	"container/list"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// look at s3wrappers tests for reference. look at common folder tests as well for reference. half way between s3wrappers and utils test

type cacheMapTestSuite struct {
	suite.Suite
	assert            *assert.Assertions
	rootAttrCacheItem attrCacheItem
}

// what is every test going to need to test with?
func (suite *cacheMapTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	suite.rootAttrCacheItem = attrCacheItem{}

	//set up nested Dir tree
	nestedDir, nestedFiles := GenerateNestedDirectory("test")

	for dir := nestedDir.Front(); dir != nil; dir = dir.Next() {
		attr := internal.CreateObjAttrDir(dir.Value.(string))
		suite.rootAttrCacheItem.insert(attr, true, time.Now())
	}

	for file := nestedFiles.Front(); file != nil; file = file.Next() {
		attr := internal.CreateObjAttr(file.Value.(string), 1024, time.Now())
		suite.rootAttrCacheItem.insert(attr, true, time.Now())
	}

}

func (suite *cacheMapTestSuite) TestInsertFileCacheMap() {

	//create path string in form of test/dir/file
	path := "/a/c1/TestFile.txt"
	startTime := time.Now()
	attr := internal.CreateObjAttr(path, 1024, startTime)

	//insert path into suite.rootAttrCacheItem
	suite.rootAttrCacheItem.insert(attr, true, startTime)

	//verify correct values are in cacheMapTree
	cachedItem, err := suite.rootAttrCacheItem.get(path)
	suite.assert.Nil(err)
	suite.assert.NotNil(cachedItem)
	suite.assert.EqualValues(path, cachedItem.attr.Path)
	suite.assert.EqualValues(1024, cachedItem.attr.Size)
	suite.assert.EqualValues(startTime, cachedItem.attr.Mtime)
	suite.assert.EqualValues(false, cachedItem.attr.IsDir())

}

func (suite *cacheMapTestSuite) TestInsertFolderCacheMap() {

	//create path string in form of test/dir/file
	path := "a/c1/TestFolder"
	startTime := time.Now()
	attr := internal.CreateObjAttrDir(path)

	//insert path into suite.rootAttrCacheItem

	suite.rootAttrCacheItem.insert(attr, true, startTime)

	//verify correct values are in cacheMapTree
	cachedItem, err := suite.rootAttrCacheItem.get(path)
	suite.assert.Nil(err)
	suite.assert.NotNil(cachedItem)
	suite.assert.EqualValues(path, cachedItem.attr.Path)
	suite.assert.EqualValues(4096, cachedItem.attr.Size)
	suite.assert.EqualValues(startTime, cachedItem.attr.Mtime)
	suite.assert.EqualValues(true, cachedItem.attr.IsDir())

}

func (suite *cacheMapTestSuite) TestDeleteAttrItem() {

	deleteTime := time.Now()

	//insert an item
	path := "/a/c1/TempFile.txt"
	startTime := time.Now()
	attr := internal.CreateObjAttr(path, 1024, startTime)

	//insert path into suite.rootAttrCacheItem
	suite.rootAttrCacheItem.insert(attr, true, startTime)

	//validate it is there
	cachedItem, err := suite.rootAttrCacheItem.get(path)
	suite.assert.Nil(err)
	suite.assert.NotNil(cachedItem)
	suite.assert.EqualValues(path, cachedItem.attr.Path)
	suite.assert.EqualValues(1024, cachedItem.attr.Size)
	suite.assert.EqualValues(startTime, cachedItem.attr.Mtime)
	suite.assert.EqualValues(false, cachedItem.attr.IsDir())

	//delete it
	cachedItem.markDeleted(deleteTime)
	cachedItem.invalidate()

	//verify it is gone
	cachedItem, err = suite.rootAttrCacheItem.get(path)
	suite.assert.Nil(err)
	suite.assert.NotNil(cachedItem)
	suite.assert.EqualValues(true, cachedItem.isDeleted())
	suite.assert.EqualValues(false, cachedItem.exists())

}

func (suite *cacheMapTestSuite) TestGetCacheMapItem() {

	path := "a/c1/gc1"
	item, err := suite.rootAttrCacheItem.get(path)
	suite.assert.Nil(err)
	suite.assert.NotNil(item)
	attrStr := item.attr.Path
	suite.assert.EqualValues(path, attrStr)
}

func TestCacheMapTestSuite(t *testing.T) {
	suite.Run(t, new(cacheMapTestSuite))
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
func GenerateNestedDirectory(path string) (*list.List, *list.List) {

	path = internal.TruncateDirName(path)

	dirPaths := list.New()
	dirPaths.PushBack(path + "/")
	dirPaths.PushBack(path + "/c1" + "/")
	dirPaths.PushBack(path + "b" + "/")

	filePaths := list.New()
	filePaths.PushBack(path + "/c2")
	filePaths.PushBack(path + "/c1" + "/gc1")
	filePaths.PushBack(path + "b" + "/c1")
	filePaths.PushBack(path + "c")

	return dirPaths, filePaths
}
