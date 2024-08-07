//go:build linux

/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2024 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.

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
	"context"
	"crypto/rand"
	"fmt"
	mrand "math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/component/loopback"
	"github.com/Seagate/cloudfuse/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var home_dir, _ = os.UserHomeDir()

type blockCacheTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *blockCacheTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

type testObj struct {
	fake_storage_path string
	disk_cache_path   string
	loopback          internal.Component
	blockCache        *BlockCache
}

func randomString(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func getFakeStoragePath(base string) string {
	tmp_path := filepath.Join(home_dir, base+randomString(8))
	_ = os.Mkdir(tmp_path, 0777)
	return tmp_path
}

func setupPipeline(cfg string) (*testObj, error) {
	tobj := &testObj{
		fake_storage_path: getFakeStoragePath("block_cache"),
		disk_cache_path:   getFakeStoragePath("fake_storage"),
	}

	if cfg == "" {
		cfg = fmt.Sprintf("read-only: true\n\nloopbackfs:\n  path: %s\n\nblock_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: 12\n  parallelism: 10\n  path: %s\n  disk-size-mb: 50\n  disk-timeout-sec: 20", tobj.fake_storage_path, tobj.disk_cache_path)
	} else {
		cfg = fmt.Sprintf("%s\n\nloopbackfs:\n  path: %s\n", cfg, tobj.fake_storage_path)
	}

	config.ReadConfigFromReader(strings.NewReader(cfg))

	tobj.loopback = loopback.NewLoopbackFSComponent()
	err := tobj.loopback.Configure(true)
	if err != nil {
		return nil, fmt.Errorf("Unable to configure loopback [%s]", err.Error())
	}

	tobj.blockCache = NewBlockCacheComponent().(*BlockCache)
	tobj.blockCache.SetNextComponent(tobj.loopback)
	err = tobj.blockCache.Configure(true)
	if err != nil {
		return nil, fmt.Errorf("Unable to configure blockcache [%s]", err.Error())
	}

	err = tobj.loopback.Start(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Unable to start loopback [%s]", err.Error())
	}

	err = tobj.blockCache.Start(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Unable to start blockcache [%s]", err.Error())
	}

	return tobj, nil
}

func (tobj *testObj) cleanupPipeline() error {
	if tobj == nil {
		return nil
	}

	if tobj.loopback != nil {
		err := tobj.loopback.Stop()
		if err != nil {
			return fmt.Errorf("Unable to stop loopback [%s]", err.Error())
		}
	}

	if tobj.blockCache != nil {
		err := tobj.blockCache.Stop()
		if err != nil {
			return fmt.Errorf("Unable to stop block cache [%s]", err.Error())
		}
	}

	os.RemoveAll(tobj.fake_storage_path)
	os.RemoveAll(tobj.disk_cache_path)

	return nil
}

// Tests the default configuration of block cache
func (suite *blockCacheTestSuite) TestEmpty() {
	emptyConfig := "read-only: true"
	tobj, err := setupPipeline(emptyConfig)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.Equal("block_cache", tobj.blockCache.Name())
	suite.assert.EqualValues(16*_1MB, tobj.blockCache.blockSize)
	suite.assert.EqualValues(4192*_1MB, tobj.blockCache.memSize)
	suite.assert.EqualValues(4192, tobj.blockCache.diskSize)
	suite.assert.EqualValues(defaultTimeout, tobj.blockCache.diskTimeout)
	suite.assert.EqualValues(128, tobj.blockCache.workers)
	suite.assert.EqualValues(MIN_PREFETCH, tobj.blockCache.prefetch)
	suite.assert.False(tobj.blockCache.noPrefetch)
	suite.assert.NotNil(tobj.blockCache.blockPool)
	suite.assert.NotNil(tobj.blockCache.threadPool)
}

func (suite *blockCacheTestSuite) TestInvalidPrefetchCount() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 16\n  mem-size-mb: 500\n  prefetch: 8\n  parallelism: 10\n  path: abcd\n  disk-size-mb: 100\n  disk-timeout-sec: 5"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "invalid prefetch count")
}

func (suite *blockCacheTestSuite) TestInvalidMemoryLimitPrefetchCount() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 16\n  mem-size-mb: 320\n  prefetch: 50\n  parallelism: 10\n  path: abcd\n  disk-size-mb: 100\n  disk-timeout-sec: 5"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "[memory limit too low for configured prefetch")
}

func (suite *blockCacheTestSuite) TestNoPrefetchConfig() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 1\n  mem-size-mb: 500\n  prefetch: 0\n  parallelism: 10\n  path: abcd\n  disk-size-mb: 100\n  disk-timeout-sec: 5"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)
	suite.assert.True(tobj.blockCache.noPrefetch)
}

func (suite *blockCacheTestSuite) TestInvalidDiskPath() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 16\n  mem-size-mb: 500\n  prefetch: 12\n  parallelism: 10\n  path: /abcd\n  disk-size-mb: 100\n  disk-timeout-sec: 5"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "permission denied")
}

func (suite *blockCacheTestSuite) TestSomeInvalidConfigs() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 8\n  mem-size-mb: 800\n  prefetch: 12\n  parallelism: 0\n"
	_, err := setupPipeline(cfg)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "fail to init thread pool")

	cfg = "read-only: true\n\nblock_cache:\n  block-size-mb: 1024000\n  mem-size-mb: 20240000\n  prefetch: 12\n  parallelism: 1\n"
	_, err = setupPipeline(cfg)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "fail to init block pool")

	cfg = "read-only: true\n\nblock_cache:\n  block-size-mb: 8\n  mem-size-mb: 800\n  prefetch: 12\n  parallelism: 5\n  path: ./\n  disk-size-mb: 100\n  disk-timeout-sec: 0"
	_, err = setupPipeline(cfg)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "timeout can not be zero")
}

func (suite *blockCacheTestSuite) TestManualConfig() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 16\n  mem-size-mb: 500\n  prefetch: 12\n  parallelism: 10\n  path: abcd\n  disk-size-mb: 100\n  disk-timeout-sec: 5"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.Equal("block_cache", tobj.blockCache.Name())
	suite.assert.EqualValues(16*_1MB, tobj.blockCache.blockSize)
	suite.assert.EqualValues(500*_1MB, tobj.blockCache.memSize)
	suite.assert.EqualValues(10, tobj.blockCache.workers)
	suite.assert.EqualValues(100, tobj.blockCache.diskSize)
	suite.assert.EqualValues(5, tobj.blockCache.diskTimeout)
	suite.assert.EqualValues(12, tobj.blockCache.prefetch)
	suite.assert.EqualValues(10, tobj.blockCache.workers)

	suite.assert.NotNil(tobj.blockCache.blockPool)
}

func (suite *blockCacheTestSuite) TestOpenFileFail() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := "a"
	options := internal.OpenFileOptions{Name: path}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.Error(err)
	suite.assert.Nil(h)
	suite.assert.Contains(err.Error(), "no such file or directory")
}

func (suite *blockCacheTestSuite) TestFileOpneClose() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	fileName := "bc.tst"
	stroagePath := filepath.Join(tobj.fake_storage_path, fileName)
	data := make([]byte, 5*_1MB)
	_, _ = rand.Read(data)
	os.WriteFile(stroagePath, data, 0777)

	options := internal.OpenFileOptions{Name: fileName}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(5*_1MB))
	suite.assert.NotNil(h.Buffers.Cooked)
	suite.assert.NotNil(h.Buffers.Cooking)

	tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)
}

func (suite *blockCacheTestSuite) TestFileRead() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	fileName := "bc.tst"
	stroagePath := filepath.Join(tobj.fake_storage_path, fileName)
	data := make([]byte, 50*_1MB)
	_, _ = rand.Read(data)
	os.WriteFile(stroagePath, data, 0777)

	options := internal.OpenFileOptions{Name: fileName}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(50*_1MB))
	suite.assert.NotNil(h.Buffers.Cooked)
	suite.assert.NotNil(h.Buffers.Cooking)

	data = make([]byte, 1000)

	// Read beyond end of file
	n, err := tobj.blockCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: int64((50 * _1MB) + 1), Data: data})
	suite.assert.Error(err)
	suite.assert.Equal(0, n)
	suite.assert.Contains(err.Error(), "EOF")

	// Read exactly at last offset
	n, err = tobj.blockCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: int64(50 * _1MB), Data: data})
	suite.assert.Error(err)
	suite.assert.Equal(0, n)
	suite.assert.Contains(err.Error(), "EOF")

	n, err = tobj.blockCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: data})
	suite.assert.NoError(err)
	suite.assert.Equal(1000, n)

	cnt := h.Buffers.Cooked.Len() + h.Buffers.Cooking.Len()
	suite.assert.Equal(MIN_PREFETCH*2, cnt)

	tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)
}

func (suite *blockCacheTestSuite) TestFileReadSerial() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	fileName := "bc.tst"
	stroagePath := filepath.Join(tobj.fake_storage_path, fileName)
	data := make([]byte, 50*_1MB)
	_, _ = rand.Read(data)
	os.WriteFile(stroagePath, data, 0777)

	options := internal.OpenFileOptions{Name: fileName}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(50*_1MB))
	suite.assert.NotNil(h.Buffers.Cooked)
	suite.assert.NotNil(h.Buffers.Cooking)

	data = make([]byte, 1000)

	totaldata := uint64(0)
	for {
		n, err := tobj.blockCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: int64(totaldata), Data: data})
		totaldata += uint64(n)
		if err != nil {
			break
		}
		suite.assert.LessOrEqual(n, 1000)
	}

	suite.assert.Equal(totaldata, uint64(50*_1MB))
	cnt := h.Buffers.Cooked.Len() + h.Buffers.Cooking.Len()
	suite.assert.Equal(12, cnt)

	tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)
}

func (suite *blockCacheTestSuite) TestFileReadRandom() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	fileName := "bc.tst"
	stroagePath := filepath.Join(tobj.fake_storage_path, fileName)
	data := make([]byte, 100*_1MB)
	_, _ = rand.Read(data)
	os.WriteFile(stroagePath, data, 0777)

	options := internal.OpenFileOptions{Name: fileName}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(100*_1MB))
	suite.assert.NotNil(h.Buffers.Cooked)
	suite.assert.NotNil(h.Buffers.Cooking)

	data = make([]byte, 100)
	max := int64(100 * _1MB)
	for i := 0; i < 50; i++ {
		offset := mrand.Int64N(max)
		n, _ := tobj.blockCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: offset, Data: data})
		suite.assert.LessOrEqual(n, 100)
	}

	cnt := h.Buffers.Cooked.Len() + h.Buffers.Cooking.Len()
	suite.assert.LessOrEqual(cnt, 8)

	tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)
}

func (suite *blockCacheTestSuite) TestFileReadRandomNoPrefetch() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	// Set the no prefetch mode here
	tobj.blockCache.noPrefetch = true
	tobj.blockCache.prefetch = 0

	fileName := "bc.tst"
	stroagePath := filepath.Join(tobj.fake_storage_path, fileName)
	data := make([]byte, 100*_1MB)
	_, _ = rand.Read(data)
	os.WriteFile(stroagePath, data, 0777)

	options := internal.OpenFileOptions{Name: fileName}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(100*_1MB))
	suite.assert.NotNil(h.Buffers.Cooked)
	suite.assert.NotNil(h.Buffers.Cooking)

	data = make([]byte, 100)
	max := int64(100 * _1MB)
	for i := 0; i < 50; i++ {
		offset := mrand.Int64N(max)
		n, _ := tobj.blockCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: offset, Data: data})
		suite.assert.Equal(1, h.Buffers.Cooked.Len())
		suite.assert.Equal(0, h.Buffers.Cooking.Len())
		suite.assert.LessOrEqual(n, 100)
	}

	cnt := h.Buffers.Cooked.Len() + h.Buffers.Cooking.Len()
	suite.assert.Equal(1, cnt)

	tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)
}

func (suite *blockCacheTestSuite) TestDiskUsageCheck() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	usage, err := common.GetUsage(tobj.disk_cache_path)
	suite.assert.NoError(err)
	suite.assert.Less(usage, float64(1.0))
	suite.assert.False(tobj.blockCache.checkDiskUsage())

	// Default disk size is 50MB
	data := make([]byte, 5*_1MB)
	_, _ = rand.Read(data)

	type diskusagedata struct {
		name     string
		diskflag bool
	}

	localfiles := make([]diskusagedata, 0)
	for i := 0; i < 13; i++ {
		fname := randomString(5)
		diskFile := filepath.Join(tobj.disk_cache_path, fname)
		localfiles = append(localfiles, diskusagedata{name: diskFile, diskflag: i >= 7})
	}

	for i := 0; i < 13; i++ {
		os.WriteFile(localfiles[i].name, data, 0777)
		usage, err := common.GetUsage(tobj.disk_cache_path)
		suite.assert.NoError(err)
		fmt.Printf("%d : %v (%v : %v) Usage %v\n", i, localfiles[i].name, localfiles[i].diskflag, tobj.blockCache.checkDiskUsage(), usage)
		suite.assert.Equal(tobj.blockCache.checkDiskUsage(), localfiles[i].diskflag)
	}

	for i := 0; i < 13; i++ {
		localfiles[i].diskflag = i < 8
	}

	for i := 0; i < 13; i++ {
		os.Remove(localfiles[i].name)
		usage, err := common.GetUsage(tobj.disk_cache_path)
		suite.assert.NoError(err)
		fmt.Printf("%d : %v (%v : %v) Usage %v\n", i, localfiles[i].name, localfiles[i].diskflag, tobj.blockCache.checkDiskUsage(), usage)
		suite.assert.Equal(tobj.blockCache.checkDiskUsage(), localfiles[i].diskflag)
	}
}

// Block-cache Writer related test cases
func (suite *blockCacheTestSuite) TestCreateFile() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := "testCreate"
	options := internal.CreateFileOptions{Name: path}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	stroagePath := filepath.Join(tobj.fake_storage_path, path)
	fs, err := os.Stat(stroagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(int64(0), fs.Size())

	path = "FailThis"
	options = internal.CreateFileOptions{Name: path}
	h, err = tobj.blockCache.CreateFile(options)
	suite.assert.Error(err)
	suite.assert.Nil(h)
	suite.assert.Contains(err.Error(), "Failed to create file")
}

func (suite *blockCacheTestSuite) TestOpenWithTruncate() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	fileName := "testTruncate.tst"
	stroagePath := filepath.Join(tobj.fake_storage_path, fileName)
	data := make([]byte, 5*_1MB)
	_, _ = rand.Read(data)
	os.WriteFile(stroagePath, data, 0777)

	options := internal.OpenFileOptions{Name: fileName}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(5*_1MB))

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)

	options = internal.OpenFileOptions{Name: fileName, Flags: os.O_TRUNC}
	h, err = tobj.blockCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.True(h.Dirty())

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)
}

func (suite *blockCacheTestSuite) TestWriteFileSimple() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := "testWriteSimple"
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	stroagePath := filepath.Join(tobj.fake_storage_path, path)
	fs, err := os.Stat(stroagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(int64(0), fs.Size())

	n, err := tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: []byte("Hello")}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Equal(5, n)
	suite.assert.Equal(int64(5), h.Size)
	suite.assert.True(h.Dirty())
	suite.assert.Equal(0, h.Buffers.Cooked.Len())
	suite.assert.Equal(1, h.Buffers.Cooking.Len())

	node, found := h.GetValue("0")
	suite.assert.True(found)
	block := node.(*Block)
	suite.assert.NotNil(block)
	suite.assert.Equal(int64(0), block.id)
	suite.assert.Equal(uint64(0), block.offset)

	err = tobj.blockCache.FlushFile(internal.FlushFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.False(h.Dirty())

	stroagePath = filepath.Join(tobj.fake_storage_path, path)
	fs, err = os.Stat(stroagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(int64(5), fs.Size())

	n, err = tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 5, Data: []byte("Gello")}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Equal(5, n)
	suite.assert.Equal(int64(10), h.Size)
	suite.assert.True(h.Dirty())
	suite.assert.Equal(0, h.Buffers.Cooked.Len())
	suite.assert.Equal(1, h.Buffers.Cooking.Len())

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)

	stroagePath = filepath.Join(tobj.fake_storage_path, path)
	fs, err = os.Stat(stroagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(int64(10), fs.Size())

	suite.assert.NoError(err)
}

func (suite *blockCacheTestSuite) TestWriteFileMultiBlock() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := "testWriteBlock"
	stroagePath := filepath.Join(tobj.fake_storage_path, path)

	data := make([]byte, 5*_1MB)
	_, _ = rand.Read(data)

	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	n, err := tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Len(data, n)
	suite.assert.Equal(h.Size, int64(len(data)))
	suite.assert.True(h.Dirty())
	suite.assert.Equal(2, h.Buffers.Cooked.Len())
	suite.assert.Equal(3, h.Buffers.Cooking.Len())

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)

	stroagePath = filepath.Join(tobj.fake_storage_path, path)
	fs, err := os.Stat(stroagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(len(data)))

	suite.assert.NoError(err)
}

func (suite *blockCacheTestSuite) TestWriteFileMultiBlockWithOverwrite() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := "testWriteBlock"
	stroagePath := filepath.Join(tobj.fake_storage_path, path)

	data := make([]byte, 5*_1MB)
	_, _ = rand.Read(data)

	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	n, err := tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Len(data, n)
	suite.assert.Equal(h.Size, int64(len(data)))
	suite.assert.True(h.Dirty())
	suite.assert.Equal(2, h.Buffers.Cooked.Len())
	suite.assert.Equal(3, h.Buffers.Cooking.Len())

	err = tobj.blockCache.FlushFile(internal.FlushFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.False(h.Dirty())

	n, err = tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data[:100]}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Equal(100, n)

	n, err = tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data[:100]}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Equal(100, n)

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)

	stroagePath = filepath.Join(tobj.fake_storage_path, path)
	fs, err := os.Stat(stroagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(len(data)))

	suite.assert.NoError(err)
}

func (suite *blockCacheTestSuite) TestWritefileWithAppend() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	tobj.blockCache.prefetchOnOpen = true

	path := "testWriteBlockAppend"
	data := make([]byte, 20*_1MB)
	_, _ = rand.Read(data)

	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	n, err := tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Len(data, n)
	suite.assert.Equal(h.Size, int64(len(data)))
	suite.assert.True(h.Dirty())

	err = tobj.blockCache.SyncFile(internal.SyncFileOptions{Handle: h})
	suite.assert.NoError(err)

	err = tobj.blockCache.FlushFile(internal.FlushFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.False(h.Dirty())

	n, err = tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Len(data, n)
	suite.assert.Equal(h.Size, int64(len(data)))
	suite.assert.True(h.Dirty())

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)

	h, err = tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR, Mode: 0777})
	suite.assert.NoError(err)
	dataNew := make([]byte, 10*_1MB)
	_, _ = rand.Read(data)

	n, err = tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: h.Size, Data: dataNew}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Len(dataNew, n)
	suite.assert.Equal(h.Size, int64(len(data)+len(dataNew)))
	suite.assert.True(h.Dirty())

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)

	h, err = tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR, Mode: 0777})
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(len(data)+len(dataNew)))

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)
}

func (suite *blockCacheTestSuite) TestWriteBlockOutOfRange() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	tobj.blockCache.prefetchOnOpen = true
	tobj.blockCache.blockSize = 10

	path := "testInvalidWriteBlock"
	data := make([]byte, 20*_1MB)
	_, _ = rand.Read(data)

	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)

	dataNew := make([]byte, 1*_1MB)
	_, _ = rand.Read(data)

	n, err := tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 10 * 50001, Data: dataNew}) // 5 bytes
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "block index out of range")
	suite.assert.Equal(0, n)

	tobj.blockCache.blockSize = 1048576
	n, err = tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 10 * 50001, Data: dataNew}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Len(dataNew, n)

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)
}

func (suite *blockCacheTestSuite) TestDeleteAndRenameDirAndFile() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	err = tobj.blockCache.CreateDir(internal.CreateDirOptions{Name: "testCreateDir", Mode: 0777})
	suite.assert.NoError(err)

	options := internal.CreateFileOptions{Name: "testCreateDir/a.txt", Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	n, err := tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: []byte("Hello")}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Equal(5, n)
	suite.assert.Equal(int64(5), h.Size)
	suite.assert.True(h.Dirty())
	suite.assert.Equal(0, h.Buffers.Cooked.Len())
	suite.assert.Equal(1, h.Buffers.Cooking.Len())

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)

	err = tobj.blockCache.RenameDir(internal.RenameDirOptions{Src: "testCreateDir", Dst: "testCreateDirNew"})
	suite.assert.NoError(err)

	err = tobj.blockCache.DeleteDir(internal.DeleteDirOptions{Name: "testCreateDirNew"})
	suite.assert.Error(err)

	err = os.MkdirAll(filepath.Join(filepath.Join(tobj.blockCache.tmpPath, "testCreateDirNew")), 0777)
	suite.assert.NoError(err)
	err = os.WriteFile(filepath.Join(tobj.blockCache.tmpPath, "testCreateDirNew/a.txt::0"), []byte("Hello"), 0777)
	suite.assert.NoError(err)
	err = os.WriteFile(filepath.Join(tobj.blockCache.tmpPath, "testCreateDirNew/a.txt::1"), []byte("Hello"), 0777)
	suite.assert.NoError(err)
	err = os.WriteFile(filepath.Join(tobj.blockCache.tmpPath, "testCreateDirNew/a.txt::2"), []byte("Hello"), 0777)
	suite.assert.NoError(err)

	err = tobj.blockCache.RenameFile(internal.RenameFileOptions{Src: "testCreateDirNew/a.txt", Dst: "testCreateDirNew/b.txt"})
	suite.assert.NoError(err)

	err = tobj.blockCache.DeleteFile(internal.DeleteFileOptions{Name: "testCreateDirNew/b.txt"})
	suite.assert.NoError(err)

	err = tobj.blockCache.DeleteDir(internal.DeleteDirOptions{Name: "testCreateDirNew"})
	suite.assert.NoError(err)
}

func (suite *blockCacheTestSuite) TestTempCacheCleanup() {
	tobj, _ := setupPipeline("")
	defer tobj.cleanupPipeline()

	items, _ := os.ReadDir(tobj.disk_cache_path)
	suite.assert.Empty(items)
	_ = tobj.blockCache.TempCacheCleanup()

	for i := 0; i < 5; i++ {
		_ = os.Mkdir(filepath.Join(tobj.disk_cache_path, fmt.Sprintf("temp_%d", i)), 0777)
		for j := 0; j < 5; j++ {
			_, _ = os.Create(filepath.Join(tobj.disk_cache_path, fmt.Sprintf("temp_%d", i), fmt.Sprintf("temp_%d", j)))
		}
	}

	items, _ = os.ReadDir(tobj.disk_cache_path)
	suite.assert.Len(items, 5)

	_ = tobj.blockCache.TempCacheCleanup()
	items, _ = os.ReadDir(tobj.disk_cache_path)
	suite.assert.Empty(items)

	tobj.blockCache.tmpPath = ""
	_ = tobj.blockCache.TempCacheCleanup()

	tobj.blockCache.tmpPath = "~/ABCD"
	err := tobj.blockCache.TempCacheCleanup()
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "failed to list directory")
}

func (suite *blockCacheTestSuite) TestZZZZLazyWrite() {
	tobj, _ := setupPipeline("")
	defer tobj.cleanupPipeline()

	tobj.blockCache.lazyWrite = true

	file := "file101"
	handle, _ := tobj.blockCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	data := make([]byte, 10*1024*1024)
	_, _ = tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	_ = tobj.blockCache.FlushFile(internal.FlushFileOptions{Handle: handle})

	// As lazy write is enabled flush shall not upload the file
	suite.assert.True(handle.Dirty())

	_ = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	time.Sleep(5 * time.Second)
	tobj.blockCache.lazyWrite = false

	// As lazy write is enabled flush shall not upload the file
	suite.assert.False(handle.Dirty())
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestBlockCacheTestSuite(t *testing.T) {
	bcsuite := new(blockCacheTestSuite)
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}

	suite.Run(t, bcsuite)
}
