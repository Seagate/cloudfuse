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
	"fmt"
	"os"
	"strings"
	"time"

	"cloudfuse/common"
	"cloudfuse/internal"
)

// Flags represented in BitMap for various flags in the attr cache item
const (
	AttrFlagUnknown uint16 = iota
	AttrFlagExists
	AttrFlagValid
	// when using S3, directories with no objects are not represented in cloud storage
	AttrFlagNotInCloud
)

// attrCacheItem : Structure of each item in attr cache
type attrCacheItem struct {
	attr     *internal.ObjAttr
	cachedAt time.Time
	attrFlag common.BitMap16
	children map[string]*attrCacheItem
}

func newAttrCacheItem(attr *internal.ObjAttr, exists bool, cachedAt time.Time) *attrCacheItem {
	item := &attrCacheItem{
		attr:     attr,
		attrFlag: 0,
		cachedAt: cachedAt,
	}

	item.attrFlag.Set(AttrFlagValid)
	if exists {
		item.attrFlag.Set(AttrFlagExists)
	}

	//item.Insert(attr, exists, cachedAt)

	return item
}

func (value *attrCacheItem) insert(attr *internal.ObjAttr, exists bool, cachedAt time.Time) *attrCacheItem {
	path := attr.Path // home/user/folder/file
	path = internal.TruncateDirName(path)
	var itemPath string

	//start recursion
	value = value.insertHelper(attr, exists, cachedAt, path, itemPath)

	return value

}

// TODO: write unit tests for this
func (value *attrCacheItem) insertHelper(attr *internal.ObjAttr, exists bool, cachedAt time.Time, path string, itemPath string) *attrCacheItem {
	paths := strings.SplitN(path, "/", 2) // paths[0] is home paths[1] is user/folder/file

	if value.children == nil {
		value.children = make(map[string]*attrCacheItem)
	}

	if len(paths) < 2 {

		// this is a leaf
		// we end up with string key being a single folder name instead of a full path. This also will take care of using the folder attribute data.
		value.children[paths[0]] = newAttrCacheItem(attr, exists, cachedAt)

	} else {

		itemPath += paths[0] + "/"
		_, ok := value.children[paths[0]]
		if !ok {
			value.children[paths[0]] = newAttrCacheItem(internal.CreateObjAttrDir(itemPath), exists, cachedAt)
		}
		value.children[paths[0]].insertHelper(attr, exists, cachedAt, paths[1], itemPath)
	}
	return value
}

// input: full path to item or file as string
// output: the attrCacheItem value for the key found in path
// description: a lookup of any attrCacheItem based on any given full path.
// TODO: write tests
func (value *attrCacheItem) get(path string) (*attrCacheItem, error) {
	path = internal.TruncateDirName(path)
	paths := strings.Split(path, "/")
	var currentItem *attrCacheItem
	var ok bool
	currentItem = value
	for _, pathElement := range paths {
		currentItem, ok = currentItem.children[pathElement]
		if !ok {
			return nil, fmt.Errorf("The path element : %s does not exist", pathElement)
		}
		//TODO: side note: cacheLocks. channel, sync, semiphore.
	}

	return currentItem, nil

}

func (value *attrCacheItem) valid() bool {
	return value.attrFlag.IsSet(AttrFlagValid)
}

func (value *attrCacheItem) exists() bool {
	return value.attrFlag.IsSet(AttrFlagExists)
}

func (value *attrCacheItem) isInCloud() bool {
	isObject := !value.attr.IsDir()
	isDirInCloud := value.attr.IsDir() && !value.attrFlag.IsSet(AttrFlagNotInCloud)
	return isObject || isDirInCloud
}

func (value *attrCacheItem) markDeleted(deletedTime time.Time) {

	value.attrFlag.Clear(AttrFlagExists)
	value.attrFlag.Set(AttrFlagValid)
	value.cachedAt = deletedTime
	value.attr = &internal.ObjAttr{}
	value.children = make(map[string]*attrCacheItem)
}

func (value *attrCacheItem) invalidate() {

	value.attrFlag.Clear(AttrFlagValid)
	value.attr = &internal.ObjAttr{}
	value.children = make(map[string]*attrCacheItem)
}

func (value *attrCacheItem) markInCloud(inCloud bool) {
	if value.attr.IsDir() {
		if inCloud {
			value.attrFlag.Clear(AttrFlagNotInCloud)
		} else {
			value.attrFlag.Set(AttrFlagNotInCloud)
		}
	}
}

func (value *attrCacheItem) getAttr() *internal.ObjAttr {
	return value.attr
}

func (value *attrCacheItem) isDeleted() bool {
	return !value.exists()
}

func (value *attrCacheItem) setSize(size int64) {
	value.attr.Mtime = time.Now()
	value.attr.Size = size
	value.cachedAt = time.Now()
}

func (value *attrCacheItem) setMode(mode os.FileMode) {
	value.attr.Mode = mode
	value.attr.Ctime = time.Now()
	value.cachedAt = time.Now()
}
