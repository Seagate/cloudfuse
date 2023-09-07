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

package attr_cache

import (
	"testing"

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

	alist, blist, clist := generateNestedDirectory("david")

	println(alist)
	println(blist)
	println(clist)
	// .generate a path
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
