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
	"path"
	"testing"
	"time"

	"github.com/Seagate/cloudfuse/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type cacheMapTestSuite struct {
	suite.Suite
	assert *assert.Assertions
	cache  cacheTreeMap
}

// what is every test going to need to test with?
func (suite *cacheMapTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	suite.cache = *newCacheTreeMap(5000)
	// set up nested Dir tree
	nestedDir, nestedFiles := generateFSTree("a")
	// directories
	for dir := nestedDir.Front(); dir != nil; dir = dir.Next() {
		attr := internal.CreateObjAttrDir(dir.Value.(string), time.Now())
		suite.cache.insert(insertOptions{
			attr:     attr,
			exists:   true,
			cachedAt: time.Now(),
		})
	}
	// files
	for file := nestedFiles.Front(); file != nil; file = file.Next() {
		attr := internal.CreateObjAttr(file.Value.(string), 1024, time.Now())
		suite.cache.insert(insertOptions{
			attr:     attr,
			exists:   true,
			cachedAt: time.Now(),
		})
	}
}

func (suite *cacheMapTestSuite) TestInsert() {
	workingPath := "a/c1"
	// file
	fileName := "testFile.txt"
	filePath := path.Join(workingPath, fileName)
	fileSize := int64(1024)
	insertTime := time.Now()
	fileAttr := internal.CreateObjAttr(filePath, fileSize, insertTime)
	// insert
	insertedItem := suite.cache.insert(insertOptions{
		attr:     fileAttr,
		exists:   true,
		cachedAt: insertTime,
	})
	// verify item contents
	cachedItem, found := suite.cache.get(filePath)
	suite.assert.True(found)
	suite.assert.NotNil(cachedItem)
	suite.assert.Equal(filePath, cachedItem.attr.Path)
	suite.assert.Equal(fileSize, cachedItem.attr.Size)
	suite.assert.False(cachedItem.attr.IsDir())
	suite.assert.Same(insertedItem, cachedItem)

	// replace existing cache item
	newTime := time.Now()
	newSize := int64(555)
	fileAttr = internal.CreateObjAttr(filePath, newSize, newTime)
	//
	insertedItem = suite.cache.insert(insertOptions{
		attr:     fileAttr,
		exists:   true,
		cachedAt: insertTime,
	})
	// verify new contents
	cachedItem, found = suite.cache.get(filePath)
	suite.assert.True(found)
	suite.assert.NotNil(cachedItem)
	suite.assert.Equal(filePath, cachedItem.attr.Path)
	suite.assert.Equal(newSize, cachedItem.attr.Size)
	suite.assert.Equal(newTime, cachedItem.attr.Mtime)
	suite.assert.False(cachedItem.attr.IsDir())
	suite.assert.Same(insertedItem, cachedItem)

	// directory
	dirName := "testFolder"
	dirPath := path.Join(workingPath, dirName)
	insertTime = time.Now()
	dirAttr := internal.CreateObjAttrDir(dirPath, insertTime)
	// insert
	insertedItem = suite.cache.insert(insertOptions{
		attr:     dirAttr,
		exists:   true,
		cachedAt: insertTime,
	})
	// verify item contents
	cachedItem, found = suite.cache.get(dirPath)
	suite.assert.True(found)
	suite.assert.NotNil(cachedItem)
	suite.assert.Equal(dirPath, cachedItem.attr.Path)
	suite.assert.Equal(int64(4096), cachedItem.attr.Size)
	suite.assert.True(cachedItem.attr.IsDir())
	suite.assert.Same(insertedItem, cachedItem)

	// auto-create parent directories (insert "outer/inner/nestedFile.txt")
	nestedDir1Name := "outer"
	nestedDir2Name := "inner"
	nestedFileName := "nestedFile.txt"
	nestedFilePath := path.Join(workingPath, nestedDir1Name, nestedDir2Name, nestedFileName)
	insertTime = time.Now()
	nestedFileAttr := internal.CreateObjAttr(nestedFilePath, fileSize, insertTime)
	insertedItem = suite.cache.insert(insertOptions{
		attr:     nestedFileAttr,
		exists:   true,
		cachedAt: insertTime,
	})
	// verify item
	cachedItem, found = suite.cache.get(nestedFilePath)
	suite.assert.True(found)
	suite.assert.NotNil(cachedItem)
	suite.assert.Equal(nestedFilePath, cachedItem.attr.Path)
	suite.assert.Equal(fileSize, cachedItem.attr.Size)
	suite.assert.False(cachedItem.attr.IsDir())
	suite.assert.Same(insertedItem, cachedItem)

	// verify parent directories and tree structure
	workingDir, found := suite.cache.get(workingPath)
	suite.assert.True(found)
	suite.assert.NotNil(workingDir.children)
	// file
	treeItem, found := workingDir.children[fileName]
	suite.assert.True(found)
	mapItem, found := suite.cache.get(filePath)
	suite.assert.True(found)
	suite.assert.Same(treeItem, mapItem)
	// dir
	treeItem, found = workingDir.children[dirName]
	suite.assert.True(found)
	mapItem, found = suite.cache.get(dirPath)
	suite.assert.True(found)
	suite.assert.Same(treeItem, mapItem)
	// nested
	// dir1
	treeItem, found = workingDir.children[nestedDir1Name]
	suite.assert.True(found)
	suite.assert.True(treeItem.attr.IsDir())
	mapItem, found = suite.cache.get(path.Join(workingPath, nestedDir1Name))
	suite.assert.True(found)
	suite.assert.Same(treeItem, mapItem)
	// dir1/dir2
	treeItem, found = treeItem.children[nestedDir2Name]
	suite.assert.True(found)
	suite.assert.True(treeItem.attr.IsDir())
	mapItem, found = suite.cache.get(path.Join(workingPath, nestedDir1Name, nestedDir2Name))
	suite.assert.True(found)
	suite.assert.Same(treeItem, mapItem)
	// dir1/dir2/file
	treeItem, found = treeItem.children[nestedFileName]
	suite.assert.True(found)
	mapItem, found = suite.cache.get(nestedFilePath)
	suite.assert.True(found)
	suite.assert.Same(treeItem, mapItem)
}

func (suite *cacheMapTestSuite) TestMarkDeletedFile() {
	// insert an item
	path := "a/c1/TempFile.txt"
	insertTime := time.Now()
	attr := internal.CreateObjAttr(path, 1024, insertTime)
	suite.cache.insert(insertOptions{
		attr:     attr,
		exists:   true,
		cachedAt: insertTime,
	})
	// validate it exists
	cachedItem, found := suite.cache.get(path)
	suite.assert.True(found)
	suite.assert.NotNil(cachedItem)
	suite.assert.Equal(path, cachedItem.attr.Path)
	suite.assert.True(cachedItem.exists())

	// mark it deleted
	cachedItem.markDeleted(time.Now())

	// verify it is marked deleted
	suite.confirmMarkedDeleted(cachedItem)
}

func (suite *cacheMapTestSuite) TestInvalidate() {
	// insert an item
	path := "a/c1/TempFile.txt"
	insertTime := time.Now()
	attr := internal.CreateObjAttr(path, 1024, insertTime)
	suite.cache.insert(insertOptions{
		attr:     attr,
		exists:   true,
		cachedAt: insertTime,
	})
	// validate it is there
	cachedItem, found := suite.cache.get(path)
	suite.assert.True(found)
	suite.assert.NotNil(cachedItem)
	suite.assert.Equal(path, cachedItem.attr.Path)
	suite.assert.True(cachedItem.valid())

	// invalidate
	cachedItem.invalidate()

	// verify it is invalid
	suite.confirmInvalid(cachedItem)
}

func (suite *cacheMapTestSuite) TestMarkDeletedFolder() {
	// insert an item
	parentPath := "a/c1"
	filePath := "a/c1/f/TempFile.txt"
	insertTime := time.Now()
	attr := internal.CreateObjAttr(filePath, 1024, insertTime)
	suite.cache.insert(insertOptions{
		attr:     attr,
		exists:   true,
		cachedAt: insertTime,
	})
	// validate file item
	cachedItem, found := suite.cache.get(filePath)
	suite.assert.True(found)
	suite.assert.NotNil(cachedItem)
	suite.assert.Equal(filePath, cachedItem.attr.Path)
	suite.assert.True(cachedItem.exists())
	// validate parent item
	cachedItem, found = suite.cache.get(parentPath)
	suite.assert.True(found)
	suite.assert.NotNil(cachedItem)
	suite.assert.Equal(parentPath, cachedItem.attr.Path)
	suite.assert.True(cachedItem.exists())
	suite.assert.True(cachedItem.attr.IsDir())

	// mark "c1" folder deleted
	cachedItem.markDeleted(time.Now())

	// verify deletion
	deletedFolderItem, found := suite.cache.get(parentPath)
	suite.assert.True(found)
	suite.confirmMarkedDeleted(deletedFolderItem)
	suite.assert.Same(cachedItem, deletedFolderItem)
	fileItem, found := suite.cache.get(filePath)
	suite.assert.True(found)
	suite.confirmMarkedDeleted(fileItem)
}

func (suite *cacheMapTestSuite) TestInvalidateFolder() {
	// insert an item
	parentPath := "a/c1"
	filePath := "a/c1/f/TempFile.txt"
	insertTime := time.Now()
	attr := internal.CreateObjAttr(filePath, 1024, insertTime)
	suite.cache.insert(insertOptions{
		attr:     attr,
		exists:   true,
		cachedAt: insertTime,
	})
	// validate file item
	cachedItem, found := suite.cache.get(filePath)
	suite.assert.True(found)
	suite.assert.NotNil(cachedItem)
	suite.assert.Equal(filePath, cachedItem.attr.Path)
	suite.assert.True(cachedItem.valid())
	// validate parent item
	cachedItem, found = suite.cache.get(parentPath)
	suite.assert.True(found)
	suite.assert.NotNil(cachedItem)
	suite.assert.Equal(parentPath, cachedItem.attr.Path)
	suite.assert.True(cachedItem.valid())
	suite.assert.True(cachedItem.attr.IsDir())

	// mark "c1" folder deleted
	cachedItem.invalidate()

	// verify invalid
	invalidFolderItem, found := suite.cache.get(parentPath)
	suite.assert.True(found)
	suite.confirmInvalid(invalidFolderItem)
	suite.assert.Same(cachedItem, invalidFolderItem)
	fileItem, found := suite.cache.get(filePath)
	suite.assert.True(found)
	suite.confirmInvalid(fileItem)
}

func (suite *cacheMapTestSuite) TestGetRoot() {
	path := ""
	item, found := suite.cache.get(path)
	suite.assert.True(found)
	suite.assert.NotNil(item)
	attrStr := item.attr.Path
	suite.assert.Equal(path, attrStr)
}

func (suite *cacheMapTestSuite) TestGet() {
	path := "a/c1/gc1"
	item, found := suite.cache.get(path)
	suite.assert.True(found)
	suite.assert.NotNil(item)
	suite.assert.Equal(path, item.attr.Path)
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
	// check the item
	suite.T().Helper()

	suite.assert.NotNil(item)
	suite.assert.False(item.exists())
	// recurse over its children
	if item.children != nil {
		for _, val := range item.children {
			suite.confirmMarkedDeleted(val)
		}
	}
}

func (suite *cacheMapTestSuite) confirmInvalid(item *attrCacheItem) {
	// check item
	suite.T().Helper()

	suite.assert.NotNil(item)
	suite.assert.False(item.attrFlag.IsSet(AttrFlagValid))
	// recurse over its children
	if item.children != nil {
		for _, val := range item.children {
			suite.confirmInvalid(val)
		}
	}
}
