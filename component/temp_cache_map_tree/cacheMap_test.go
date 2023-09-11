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
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// look at s3wrappers tests for reference. look at common folder tests as well for reference. half way between s3wrappers and utils test

type cacheMapTestSite struct {
	suite.Suite
	assert *assert.Assertions
	//attrCache *AttrCache
}

func (suite *cacheMapTestSite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

func (suite *cacheMapTestSite) TestInsertCacheMap() {

	// .generate a path directory
	alist, _, _ := GenerateNestedDirectory("David")

	// .populate the tree
	for a := alist.Front(); a != nil; a = a.Next() {
		value := a.Value

		cachedAttrItem := attrItem.Insert(attr, true, time.Now())

		println(cachedAttrItem)

	}

	// for b := blist.Front(); b != nil; b = b.Next() {
	// 	value := b.Value

	// 	attr := attr_cache.GetPathAttr(value.(string), 1024, os.FileMode(0), false)
	// 	cacheItem := attrItem.Insert(attr, true, time.Now())
	// 	for
	// }

	// for c := clist.Front(); c != nil; c = c.Next() {
	// 	value := c.Value

	// 	attr := attr_cache.GetPathAttr(value.(string), 1024, os.FileMode(0), false)
	// 	attrItem.Insert(attr, true, time.Now())
	// }

	//atters := generateNestedPathAttr("david", int64(1024), os.FileMode(0))

	// .populate the tree
	// var cacheItem *attrCacheItem
	// for _, attr := range atters {
	// 	cacheItem = suite.attrCache.cacheMap.insert(attr, true, time.Now())
	// }

	// for item := range cacheItem.children {
	// 	println(item)
	// }
	// validate tree is properly populated

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
func GenerateNestedDirectory(path string) (*list.List, *list.List, *list.List) {
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
