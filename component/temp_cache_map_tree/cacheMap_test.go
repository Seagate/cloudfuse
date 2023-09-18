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
	nestedDir, nestedFiles := GenerateNestedDirectory("david")

	for dir := nestedDir.Front(); dir != nil; dir = dir.Next() {
		suite.rootAttrCacheItem.attr = internal.CreateObjAttrDir(dir.Value.(string))
		suite.rootAttrCacheItem.insert(suite.rootAttrCacheItem.attr, suite.rootAttrCacheItem.exists(), suite.rootAttrCacheItem.cachedAt)
	}

	for file := nestedFiles.Front(); file != nil; file = file.Next() {
		suite.rootAttrCacheItem.attr = internal.CreateObjAttr(file.Value.(string), 1024, time.Now())
		suite.rootAttrCacheItem.insert(suite.rootAttrCacheItem.attr, suite.rootAttrCacheItem.exists(), suite.rootAttrCacheItem.cachedAt)
	}

}

func (suite *cacheMapTestSuite) TestInsertCacheMap() {

	//create path string in form of david/dir/file

	//insert path into suite.rootAttrCacheItem

	//verify correct values are in cacheMapTree

}

func (suite *cacheMapTestSuite) TestDeleteCacheMap() {

	//create path string in form of david/dir/file
	path := "david/c1/davidTestFile.txt"
	suite.rootAttrCacheItem.attr = internal.CreateObjAttr(path, 1024, time.Now())

	//insert path into suite.rootAttrCacheItem

	suite.rootAttrCacheItem.insert(suite.rootAttrCacheItem.attr, suite.rootAttrCacheItem.exists(), suite.rootAttrCacheItem.cachedAt)

	//verify correct values are in cacheMapTree
	cachedItem, err := suite.rootAttrCacheItem.get(path)
	suite.assert.Nil(err)
	suite.assert.NotNil(cachedItem)
	suite.assert.equal

}

func (suite *cacheMapTestSuite) TestGetCacheMapItem() {

	suite.SetupTest()
	path := "david/c1/gc1"
	item, err := suite.rootAttrCacheItem.get(path)
	suite.assert.Nil(err)
	suite.assert.NotNil(item)
	attrStr := item.attr.Path
	suite.assert.EqualValues(path, attrStr)
	println(attrStr)
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
