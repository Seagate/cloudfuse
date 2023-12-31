/*
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

package attr_cache

import (
	"container/list"
	"testing"
	"time"

	"github.com/Seagate/cloudfuse/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type cacheMapTestSuite struct {
	suite.Suite
	assert            *assert.Assertions
	rootAttrCacheItem attrCacheItem
}

// what is every test going to need to test with?
func (suite *cacheMapTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())

	rootAttr := internal.CreateObjAttrDir("")
	suite.rootAttrCacheItem = *newAttrCacheItem(rootAttr, true, time.Now())

	//set up nested Dir tree
	nestedDir, nestedFiles := generateFSTree("a")

	for dir := nestedDir.Front(); dir != nil; dir = dir.Next() {
		attr := internal.CreateObjAttrDir(dir.Value.(string))
		suite.rootAttrCacheItem.insert(attr, true, time.Now())
	}

	for file := nestedFiles.Front(); file != nil; file = file.Next() {
		attr := internal.CreateObjAttr(file.Value.(string), 1024, time.Now())
		suite.rootAttrCacheItem.insert(attr, true, time.Now())
	}

}

func (suite *cacheMapTestSuite) TestInsertFile() {
	//create path string in form of test/dir/file
	path := "a/c1/TestFile.txt"
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
	suite.assert.EqualValues(false, cachedItem.attr.IsDir())
}

func (suite *cacheMapTestSuite) TestInsertFolder() {
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
	suite.assert.EqualValues(true, cachedItem.attr.IsDir())
}

func (suite *cacheMapTestSuite) TestInsertDirsAndFiles() {
	path := "a/c1/c12/c3p0/newfile.txt"
	timestamp := time.Now()
	attr := internal.CreateObjAttr(path, 1024, timestamp)

	cachedItem := suite.rootAttrCacheItem.insert(attr, true, timestamp)

	suite.assert.NotNil(cachedItem)
	suite.assert.Equal(path, cachedItem.attr.Path)
	suite.assert.Equal(int64(1024), cachedItem.attr.Size)
	suite.assert.False(cachedItem.attr.IsDir())
}

func (suite *cacheMapTestSuite) TestMarkDeleted() {
	deleteTime := time.Now()

	//insert an item
	path := "a/c1/TempFile.txt"
	startTime := time.Now()
	attr := internal.CreateObjAttr(path, 1024, startTime)

	//insert path into suite.rootAttrCacheItem
	cachedItem := suite.rootAttrCacheItem.insert(attr, true, startTime)

	//validate it is there
	suite.assert.NotNil(cachedItem)
	suite.assert.EqualValues(path, cachedItem.attr.Path)
	suite.assert.EqualValues(1024, cachedItem.attr.Size)
	suite.assert.EqualValues(startTime, cachedItem.attr.Mtime)
	suite.assert.EqualValues(false, cachedItem.attr.IsDir())
	suite.assert.EqualValues("TempFile.txt", cachedItem.attr.Name)
	suite.assert.EqualValues(true, cachedItem.attrFlag.IsSet(AttrFlagValid))

	// mark it deleted
	cachedItem.markDeleted(deleteTime)

	//verify it is marked deleted
	suite.confirmMarkedDeleted(cachedItem)
}

func (suite *cacheMapTestSuite) TestInvalidate() {
	//insert an item
	path := "a/c1/TempFile.txt"
	startTime := time.Now()
	attr := internal.CreateObjAttr(path, 1024, startTime)

	//insert path into suite.rootAttrCacheItem
	cachedItem := suite.rootAttrCacheItem.insert(attr, true, startTime)

	//validate it is there
	suite.assert.NotNil(cachedItem)
	suite.assert.EqualValues(path, cachedItem.attr.Path)
	suite.assert.EqualValues(1024, cachedItem.attr.Size)
	suite.assert.EqualValues("TempFile.txt", cachedItem.attr.Name)
	suite.assert.EqualValues(startTime, cachedItem.attr.Mtime)
	suite.assert.EqualValues(false, cachedItem.attr.IsDir())
	suite.assert.EqualValues(true, cachedItem.attrFlag.IsSet(AttrFlagValid))
	suite.assert.EqualValues(true, cachedItem.attrFlag.IsSet(AttrFlagExists))

	//invalidate
	cachedItem.invalidate()

	//verify it is invalid
	suite.confirmInvalidated(cachedItem)
}

func (suite *cacheMapTestSuite) TestDeleteFolder() {
	deleteTime := time.Now()

	//insert an item
	path := "a/c1/f/TempFile.txt"
	parentPath := "a/c1"
	startTime := time.Now()
	attr := internal.CreateObjAttr(path, 1024, startTime)

	//insert path into suite.rootAttrCacheItem
	cachedItem := suite.rootAttrCacheItem.insert(attr, true, startTime)

	//validate file is there
	suite.assert.NotNil(cachedItem)
	suite.assert.EqualValues(path, cachedItem.attr.Path)
	suite.assert.EqualValues(1024, cachedItem.attr.Size)
	suite.assert.EqualValues(startTime, cachedItem.attr.Mtime)
	suite.assert.EqualValues(false, cachedItem.attr.IsDir())
	suite.assert.EqualValues("TempFile.txt", cachedItem.attr.Name)
	suite.assert.EqualValues(path, cachedItem.attr.Path)
	suite.assert.EqualValues(true, cachedItem.attrFlag.IsSet(AttrFlagValid))

	//validate folder "c1"
	cachedItem, err := suite.rootAttrCacheItem.get(parentPath)
	suite.assert.Nil(err)
	suite.assert.NotNil(cachedItem)
	suite.assert.EqualValues(parentPath, cachedItem.attr.Path)
	suite.assert.EqualValues(4096, cachedItem.attr.Size)
	suite.assert.EqualValues(true, cachedItem.attr.IsDir())
	suite.assert.EqualValues("c1", cachedItem.attr.Name)
	suite.assert.EqualValues(parentPath, cachedItem.attr.Path)
	suite.assert.EqualValues(true, cachedItem.attrFlag.IsSet(AttrFlagValid))

	//mark "c1" folder deleted
	cachedItem.markDeleted(deleteTime)

	//verify "c1" folder is marked deleted
	suite.confirmMarkedDeleted(cachedItem)
}

func (suite *cacheMapTestSuite) TestInvalidateFolder() {
	//insert an item
	path := "a/c1/f/TempFile.txt"
	parentPath := "a/c1"
	startTime := time.Now()
	attr := internal.CreateObjAttr(path, 1024, startTime)

	//insert path into suite.rootAttrCacheItem
	suite.rootAttrCacheItem.insert(attr, true, startTime)

	//validate file is there
	cachedItem, err := suite.rootAttrCacheItem.get(path)
	suite.assert.Nil(err)
	suite.assert.NotNil(cachedItem)
	suite.assert.EqualValues(path, cachedItem.attr.Path)
	suite.assert.EqualValues(1024, cachedItem.attr.Size)
	suite.assert.EqualValues(startTime, cachedItem.attr.Mtime)
	suite.assert.EqualValues(false, cachedItem.attr.IsDir())
	suite.assert.EqualValues("TempFile.txt", cachedItem.attr.Name)
	suite.assert.EqualValues(path, cachedItem.attr.Path)
	suite.assert.EqualValues(true, cachedItem.attrFlag.IsSet(AttrFlagValid))

	//validate folder "g"
	cachedItem, err = suite.rootAttrCacheItem.get(parentPath)
	suite.assert.Nil(err)
	suite.assert.NotNil(cachedItem)
	suite.assert.EqualValues(parentPath, cachedItem.attr.Path)
	suite.assert.EqualValues(4096, cachedItem.attr.Size)
	suite.assert.EqualValues(true, cachedItem.attr.IsDir())
	suite.assert.EqualValues("c1", cachedItem.attr.Name)
	suite.assert.EqualValues(parentPath, cachedItem.attr.Path)
	suite.assert.EqualValues(true, cachedItem.attrFlag.IsSet(AttrFlagValid))

	//mark "c1" folder as invalid
	cachedItem.invalidate()

	//verify "c1" folder is invalid
	cachedItem, err = suite.rootAttrCacheItem.get(parentPath)
	suite.assert.Nil(err)
	suite.assert.NotNil(cachedItem)
	suite.assert.EqualValues(false, cachedItem.attrFlag.IsSet(AttrFlagValid))
	suite.assert.EqualValues(cachedItem.attr, &internal.ObjAttr{})

	// verify subtree is invalid
	suite.assert.NotNil(cachedItem.children)
	suite.confirmInvalidated(cachedItem)
}

func (suite *cacheMapTestSuite) TestGetRoot() {
	path := ""
	item, err := suite.rootAttrCacheItem.get(path)
	suite.assert.Nil(err)
	suite.assert.NotNil(item)
	attrStr := item.attr.Path
	suite.assert.EqualValues(path, attrStr)
}

func (suite *cacheMapTestSuite) TestGet() {
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
func generateFSTree(path string) (*list.List, *list.List) {
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

func (suite *cacheMapTestSuite) confirmMarkedDeleted(item *attrCacheItem) {

	suite.assert.NotNil(item)
	suite.assert.EqualValues(true, item.isDeleted())
	suite.assert.EqualValues(false, item.exists())
	suite.assert.EqualValues(item.attr, &internal.ObjAttr{})

	if item.children != nil {
		for _, val := range item.children {
			suite.confirmMarkedDeleted(val)
		}
	}
}

func (suite *cacheMapTestSuite) confirmInvalidated(item *attrCacheItem) {

	suite.assert.NotNil(item)
	suite.assert.EqualValues(false, item.attrFlag.IsSet(AttrFlagValid))
	suite.assert.EqualValues(item.attr, &internal.ObjAttr{})

	if item.children != nil {
		for _, val := range item.children {
			suite.confirmInvalidated(val)
		}
	}
}
