//go:build linux

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
	"crypto/rand"
	mrand "math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"golang.org/x/sys/unix"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type blockCacheLinuxTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *blockCacheLinuxTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	suite.assert.NoError(err)
}

func (suite *blockCacheLinuxTestSuite) TestStrongConsistency() {
	tobj, err := setupPipeline("")
	defer func() { _ = tobj.cleanupPipeline() }()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	tobj.blockCache.consistency = true

	path := getTestFileName(suite.T().Name())
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	storagePath := filepath.Join(tobj.fake_storage_path, path)
	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(int64(0), fs.Size())
	//Generate random size of file in bytes less than 2MB
	size := mrand.Int64N(2097152)
	data := make([]byte, size)

	n, err := tobj.blockCache.WriteFile(
		internal.WriteFileOptions{Handle: h, Offset: 0, Data: data},
	) // Write data to file
	suite.assert.NoError(err)
	suite.assert.EqualValues(n, size)
	suite.assert.Equal(h.Size, int64(size))

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)

	localPath := filepath.Join(tobj.disk_cache_path, path+"_0")

	xattrMd5sumOrg := make([]byte, 32)
	_, err = unix.Getxattr(localPath, "user.md5sum", xattrMd5sumOrg)
	suite.assert.NoError(err)

	h, err = tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR})
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	_, _ = tobj.blockCache.ReadInBuffer(
		internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: data},
	)
	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)

	xattrMd5sumRead := make([]byte, 32)
	_, err = unix.Getxattr(localPath, "user.md5sum", xattrMd5sumRead)
	suite.assert.NoError(err)
	suite.assert.Equal(xattrMd5sumOrg, xattrMd5sumRead)

	err = unix.Setxattr(localPath, "user.md5sum", []byte("000"), 0)
	suite.assert.NoError(err)

	xattrMd5sum1 := make([]byte, 32)
	_, err = unix.Getxattr(localPath, "user.md5sum", xattrMd5sum1)
	suite.assert.NoError(err)

	h, err = tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR})
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	_, _ = tobj.blockCache.ReadInBuffer(
		internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: data},
	)
	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)

	xattrMd5sum2 := make([]byte, 32)
	_, err = unix.Getxattr(localPath, "user.md5sum", xattrMd5sum2)
	suite.assert.NoError(err)

	suite.assert.NotEqual(xattrMd5sum1, xattrMd5sum2)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestBlockCacheLinuxTestSuite(t *testing.T) {
	dataBuff = make([]byte, 5*_1MB)
	_, _ = rand.Read(dataBuff)

	suite.Run(t, new(blockCacheLinuxTestSuite))
}
