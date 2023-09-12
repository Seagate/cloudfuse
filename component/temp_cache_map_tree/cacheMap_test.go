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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// look at s3wrappers tests for reference. look at common folder tests as well for reference. half way between s3wrappers and utils test

type cacheMapTestSite struct {
	suite.Suite
	assert      *assert.Assertions
	nestedDir   *list.List
	nestedFiles *list.List
}

func (suite *cacheMapTestSite) SetupTest() {
	suite.assert = assert.New(suite.T())

	//set up nested Dir tree
	suite.nestedDir, suite.nestedFiles = GenerateNestedDirectory("david")
	attrCacheItemInstance := AttrCacheItem{}

	//set up the cacheMap Tree
	for dir := suite.nestedDir.Front(); dir != nil; dir = dir.Next() {
		attrCacheItemInstance.attr = internal.CreateObjAttrDir(dir.Value.(string))
		attrCacheItemInstance.insert(attrCacheItemInstance.attr, attrCacheItemInstance.exists(), attrCacheItemInstance.cachedAt)
	}

	for file := suite.nestedFiles.Front(); file != nil; file = file.Next() {
		attrCacheItemInstance.attr = internal.CreateObjAttr(file.Value.(string), 1024, time.Now())
		attrCacheItemInstance.insert(attrCacheItemInstance.attr, attrCacheItemInstance.exists(), attrCacheItemInstance.cachedAt)
	}

}

func (suite *cacheMapTestSite) TestInsertCacheMap() {

	attrCacheItemInstance := AttrCacheItem{}
	// .generate a path directory

	// .populate the tree
	for a := alist.Front(); a != nil; a = a.Next() {
		valueStr := a.Value.(string)
		if valueStr[len(valueStr)-1:] == "/" {
			attrCacheItemInstance.attr = internal.CreateObjAttrDir(valueStr)
			attrCacheItemInstance.insert(attrCacheItemInstance.attr, true, time.Now())
		} else {
			attrCacheItemInstance.attr = internal.CreateObjAttr(valueStr, 1024, time.Now())
			attrCacheItemInstance.insert(attrCacheItemInstance.attr, attrCacheItemInstance.exists(), attrCacheItemInstance.cachedAt)
		}
	}

	for b := blist.Front(); b != nil; b = b.Next() {
		valueStr := b.Value.(string)
		if valueStr[len(valueStr)-1:] == "/" {
			attrCacheItemInstance.attr = internal.CreateObjAttrDir(valueStr)
			attrCacheItemInstance.insert(attrCacheItemInstance.attr, true, time.Now())
		} else {
			attrCacheItemInstance.attr = internal.CreateObjAttr(valueStr, 1024, time.Now())
			attrCacheItemInstance.insert(attrCacheItemInstance.attr, attrCacheItemInstance.exists(), attrCacheItemInstance.cachedAt)
		}
	}

	for c := clist.Front(); c != nil; c = c.Next() {
		valueStr := c.Value.(string)
		if valueStr[len(valueStr)-1:] == "/" {
			attrCacheItemInstance.attr = internal.CreateObjAttrDir(valueStr)
			attrCacheItemInstance.insert(attrCacheItemInstance.attr, true, time.Now())
		} else {
			attrCacheItemInstance.attr = internal.CreateObjAttr(valueStr, 1024, time.Now())
			attrCacheItemInstance.insert(attrCacheItemInstance.attr, attrCacheItemInstance.exists(), attrCacheItemInstance.cachedAt)
		}
	}

	// validate tree is properly populated
	for a := alist.Front(); a != nil; a = a.Next() {
		cachedItem, err := attrCacheItemInstance.get(a.Value.(string))
		suite.assert.NotNil(err)
		suite.assert.EqualValues(cachedItem)
	}

}

func TestCacheMapTestSuite(t *testing.T) {
	suite.Run(t, new(cacheMapTestSite))
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

func GetPathAttr(path string, size int64, mode os.FileMode, metadata bool) *internal.ObjAttr {
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
