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
	attr     *internal.ObjAttr
	cachedAt time.Time
	attrFlag common.BitMap16
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
}

func (value *attrCacheItem) invalidate() {
	value.attrFlag.Clear(AttrFlagValid)
	value.attr = &internal.ObjAttr{}
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
