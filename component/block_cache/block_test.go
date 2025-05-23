//go:build !authtest

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

package block_cache

import (
	"runtime"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type blockTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *blockTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	suite.assert.NoError(err)
}

func (suite *blockTestSuite) cleanupTest() {
}

func (suite *blockTestSuite) TestAllocate() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(0)
	suite.assert.Nil(b)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "invalid size")

	b, err = AllocateBlock(10)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	suite.assert.NotNil(b.data)

	_ = b.Delete()
}

func (suite *blockTestSuite) TestAllocateBig() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(100 * 1024 * 1024)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	suite.assert.NotNil(b.data)
	suite.assert.Equal(100*1024*1024, cap(b.data))

	b.Delete()
}

func (suite *blockTestSuite) TestAllocateHuge() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(50 * 1024 * 1024 * 1024)
	suite.assert.Nil(b)
	suite.assert.Error(err)
	if runtime.GOOS == "windows" {
		suite.assert.Contains(err.Error(), "insufficient memory available:")
	} else {
		suite.assert.Contains(err.Error(), "mmap error")
	}
}

func (suite *blockTestSuite) TestFreeNilData() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(1)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	b.data = nil

	err = b.Delete()
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "invalid buffer")
}

func (suite *blockTestSuite) TestFreeInvalidData() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(1)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	b.data = make([]byte, 1)

	err = b.Delete()
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "invalid")
}

func (suite *blockTestSuite) TestResuse() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(1)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	suite.assert.Nil(b.state)

	b.ReUse()
	suite.assert.NotNil(b.state)
	suite.assert.Nil(b.node)

	_ = b.Delete()
}

func (suite *blockTestSuite) TestReady() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(1)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	suite.assert.Nil(b.state)

	b.ReUse()
	suite.assert.NotNil(b.state)

	b.Ready(BlockStatusDownloaded)
	suite.assert.Len(b.state, 1)

	<-b.state
	suite.assert.Empty(b.state)

	b.ReUse()
	suite.assert.NotNil(b.state)

	_ = b.Delete()
}

func (suite *blockTestSuite) TestUnBlock() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(1)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	suite.assert.Nil(b.state)

	b.ReUse()
	suite.assert.NotNil(b.state)
	suite.assert.Nil(b.node)

	b.Ready(BlockStatusDownloaded)
	suite.assert.Len(b.state, 1)

	<-b.state
	suite.assert.Empty(b.state)

	b.Unblock()
	suite.assert.NotNil(b.state)
	suite.assert.Empty(b.state)

	<-b.state
	suite.assert.Empty(b.state)

	_ = b.Delete()
}

func (suite *blockTestSuite) TestWriter() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(1)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	suite.assert.Nil(b.state)
	suite.assert.Nil(b.node)
	suite.assert.False(b.IsDirty())

	b.ReUse()
	suite.assert.NotNil(b.state)
	suite.assert.Nil(b.node)
	suite.assert.Zero(b.offset)
	suite.assert.Equal(b.id, int64(-1))
	suite.assert.False(b.IsDirty())

	b.Ready(BlockStatusDownloaded)
	suite.assert.Len(b.state, 1)

	<-b.state
	suite.assert.Empty(b.state)

	b.Unblock()
	suite.assert.NotNil(b.state)
	suite.assert.Empty(b.state)

	b.Uploading()
	suite.assert.NotNil(b.state)

	b.Dirty()
	suite.assert.True(b.IsDirty())

	b.Failed()
	suite.assert.True(b.IsDirty())

	b.NoMoreDirty()
	suite.assert.False(b.IsDirty())

	b.Ready(BlockStatusUploaded)
	suite.assert.Len(b.state, 1)

	<-b.state
	suite.assert.Empty(b.state)

	b.Unblock()
	suite.assert.NotNil(b.state)
	suite.assert.Empty(b.state)

	<-b.state
	suite.assert.Empty(b.state)

	_ = b.Delete()
}

func TestBlockSuite(t *testing.T) {
	suite.Run(t, new(blockTestSuite))
}
