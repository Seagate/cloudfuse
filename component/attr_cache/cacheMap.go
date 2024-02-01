/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2024 Seagate Technology LLC and/or its Affiliates
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
	"os"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/internal"
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
	attr      *internal.ObjAttr
	cachedAt  time.Time
	listedAt  time.Time
	listToken string
	attrFlag  common.BitMap16
	children  map[string]*attrCacheItem
}

// all cache entries are organized into this structure
type cacheTreeMap struct {
	cacheMap  map[string]*attrCacheItem
	cacheTree *attrCacheItem
}

// initialize the cache data structure
func newCacheTreeMap() *cacheTreeMap {
	// initialize map
	cacheMap := make(map[string]*attrCacheItem)
	// create tree root node
	rootAttr := internal.CreateObjAttrDir("")
	rootNode := newAttrCacheItem(rootAttr, true, time.Now())
	// add to cacheMap
	cacheMap[""] = rootNode
	// build struct
	return &cacheTreeMap{
		cacheMap:  cacheMap,
		cacheTree: rootNode,
	}
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
	return item
}

// return the attrCacheItem matching the given path
func (ctm *cacheTreeMap) get(path string) (item *attrCacheItem, found bool) {
	path = internal.TruncateDirName(path)
	// get the entry from the map
	item, found = ctm.cacheMap[path]
	return item, found
}

// insert a new attrCacheItem and return a handle to it
func (ctm *cacheTreeMap) insert(attr *internal.ObjAttr, exists bool, cachedAt time.Time) *attrCacheItem {
	if attr == nil {
		return nil
	}
	// create the new record
	newItem := newAttrCacheItem(attr, exists, cachedAt)
	// insert it (recursively)
	ctm.insertItem(newItem)
	// return a handle to it
	return newItem
}

// use efficient recursion to add an item to the cache
// newChild must be a record for an entry that is in the parent directory (not in a subdirectory)
func (ctm *cacheTreeMap) insertItem(newItem *attrCacheItem) {
	// find the parent
	path := internal.TruncateDirName(newItem.attr.Path)
	parentPath := getParentDir(path)
	parentItem, parentFound := ctm.get(parentPath)
	// if there is no parent, create one and add it
	if !parentFound {
		newParentAttr := internal.CreateObjAttrDir(parentPath)
		parentItem = newAttrCacheItem(newParentAttr, newItem.exists(), newItem.cachedAt)
		// recurse
		ctm.insertItem(parentItem)
	}
	// add the new item to the tree and the map
	if parentItem.children == nil {
		parentItem.children = make(map[string]*attrCacheItem)
	}
	parentItem.children[newItem.attr.Name] = newItem
	ctm.cacheMap[path] = newItem
}

func (value *attrCacheItem) valid() bool {
	return value.attrFlag.IsSet(AttrFlagValid)
}

func (value *attrCacheItem) exists() bool {
	return value.valid() && value.attrFlag.IsSet(AttrFlagExists)
}

// TODO: don't return true for deleted files.
func (value *attrCacheItem) isInCloud() bool {
	isObject := !value.attr.IsDir()
	isDirInCloud := value.attr.IsDir() && !value.attrFlag.IsSet(AttrFlagNotInCloud)
	return isObject || isDirInCloud
}

func (value *attrCacheItem) markDeleted(deletedTime time.Time) {
	if value.exists() {
		value.attrFlag.Clear(AttrFlagExists)
		value.attrFlag.Set(AttrFlagValid)
		value.cachedAt = deletedTime
		value.attr = &internal.ObjAttr{}
		for _, val := range value.children {
			val.markDeleted(deletedTime)
		}
	}
}

func (value *attrCacheItem) invalidate() {
	if value.valid() {
		value.attrFlag.Clear(AttrFlagValid)
		value.attr = &internal.ObjAttr{}
		for _, val := range value.children {
			val.invalidate()
		}
	}
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

func (value *attrCacheItem) setSize(size int64, changedAt time.Time) {
	value.attr.Mtime = changedAt
	value.attr.Size = size
	value.cachedAt = changedAt
}

func (value *attrCacheItem) setMode(mode os.FileMode) {
	value.attr.Mode = mode
	value.attr.Flags.Clear(internal.PropFlagModeDefault)
	value.attr.Ctime = time.Now()
	value.cachedAt = time.Now()
}
