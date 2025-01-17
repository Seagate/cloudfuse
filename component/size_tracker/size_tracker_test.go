/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
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

package size_tracker

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/component/loopback"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/handlemap"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type sizeTrackerTestSuite struct {
	suite.Suite
	assert      *assert.Assertions
	sizeTracker *SizeTracker
	loopback    internal.Component
	mockCtrl    *gomock.Controller
	mock        *internal.MockComponent
}

const journal_test_name = "size_tracker_test.dat"

var emptyConfig = "size_tracker:\n  journal-name: " + journal_test_name

const MB = 1024 * 1024

func newLoopbackFS() internal.Component {
	loopback := loopback.NewLoopbackFSComponent()
	loopback.Configure(true)

	return loopback
}

func newTestSizeTracker(next internal.Component, configuration string) *SizeTracker {
	_ = config.ReadConfigFromReader(strings.NewReader(configuration))
	sizeTracker := NewSizeTrackerComponent()
	sizeTracker.SetNextComponent(next)
	_ = sizeTracker.Configure(true)

	return sizeTracker.(*SizeTracker)
}

func (suite *sizeTrackerTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
	suite.setupTestHelper(emptyConfig)
}

func (suite *sizeTrackerTestSuite) setupTestHelper(config string) {
	suite.assert = assert.New(suite.T())

	suite.mockCtrl = gomock.NewController(suite.T())
	suite.mock = internal.NewMockComponent(suite.mockCtrl)
	suite.loopback = newLoopbackFS()
	suite.sizeTracker = newTestSizeTracker(suite.loopback, config)
	_ = suite.sizeTracker.Start(context.Background())
}

func (suite *sizeTrackerTestSuite) cleanupTest() {
	_ = suite.loopback.Stop()
	err := suite.sizeTracker.Stop()
	if err != nil {
		panic(fmt.Sprintf("Unable to stop size tracker [%s]", err.Error()))
	}
	journal_file := common.JoinUnixFilepath(common.DefaultWorkDir, journal_test_name)
	os.Remove(journal_file)
	suite.mockCtrl.Finish()
}

// Tests the default configuration of attribute cache
func (suite *sizeTrackerTestSuite) TestDefault() {
	defer suite.cleanupTest()
	suite.assert.Equal("size_tracker", suite.sizeTracker.Name())
	print(suite.sizeTracker.mountSize.GetSize())
	suite.assert.EqualValues(uint64(0), suite.sizeTracker.mountSize.GetSize())
}

func (suite *sizeTrackerTestSuite) TestDeleteDir() {
	defer suite.cleanupTest()
	// Setup

	dir := "dir"
	path := dir + "/file"
	err := suite.sizeTracker.CreateDir(internal.CreateDirOptions{Name: dir, Mode: 0777})
	suite.assert.NoError(err)
	handle, err := suite.sizeTracker.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.assert.NoError(err)

	testData := "test data"
	data := []byte(testData)
	_, err = suite.sizeTracker.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.assert.NoError(err)
	suite.assert.EqualValues(len(testData), suite.sizeTracker.mountSize.GetSize())

	err = suite.sizeTracker.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)
	err = suite.sizeTracker.DeleteFile(internal.DeleteFileOptions{Name: path})
	suite.assert.NoError(err)

	// Delete the directory
	err = suite.sizeTracker.DeleteDir(internal.DeleteDirOptions{Name: dir})
	suite.assert.NoError(err)

	// Final size should be 0
	suite.assert.EqualValues(0, suite.sizeTracker.mountSize.GetSize())
}

func (suite *sizeTrackerTestSuite) TestRenameDir() {
	defer suite.cleanupTest()

	// Setup
	src := "src"
	dst := "dst"
	testData := "test data"
	data := []byte(testData)
	err := suite.sizeTracker.CreateDir(internal.CreateDirOptions{Name: src, Mode: 0777})
	suite.assert.NoError(err)
	path := src + "/file"
	for i := 0; i < 5; i++ {
		handle, err := suite.sizeTracker.CreateFile(internal.CreateFileOptions{Name: path + strconv.Itoa(i), Mode: 0777})
		suite.assert.NoError(err)
		_, err = suite.sizeTracker.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
		suite.assert.NoError(err)
		err = suite.sizeTracker.CloseFile(internal.CloseFileOptions{Handle: handle})
		suite.assert.NoError(err)
	}
	suite.assert.EqualValues(5*len(testData), suite.sizeTracker.mountSize.GetSize())

	// Rename the directory
	err = suite.sizeTracker.RenameDir(internal.RenameDirOptions{Src: src, Dst: dst})
	suite.assert.NoError(err)

	suite.assert.EqualValues(5*len(testData), suite.sizeTracker.mountSize.GetSize())
}

func (suite *sizeTrackerTestSuite) TestCreateFile() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	path := "file1"
	options := internal.CreateFileOptions{Name: path}
	_, err := suite.sizeTracker.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.EqualValues(0, suite.sizeTracker.mountSize.GetSize())
}

func (suite *sizeTrackerTestSuite) TestDeleteFile() {
	defer suite.cleanupTest()
	path := "file4"

	handle, err := suite.sizeTracker.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.assert.NoError(err)
	testData := "test data"
	data := []byte(testData)
	_, err = suite.sizeTracker.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.assert.NoError(err)
	suite.assert.EqualValues(len(testData), suite.sizeTracker.mountSize.GetSize())
	err = suite.sizeTracker.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	err = suite.sizeTracker.DeleteFile(internal.DeleteFileOptions{Name: path})
	suite.assert.NoError(err)

	suite.assert.EqualValues(0, suite.sizeTracker.mountSize.GetSize())
}

func (suite *sizeTrackerTestSuite) TestDeleteFileError() {
	defer suite.cleanupTest()
	path := "file6"
	err := suite.sizeTracker.DeleteFile(internal.DeleteFileOptions{Name: path})
	suite.assert.Error(err)
	suite.assert.EqualValues(syscall.ENOENT, err)
	suite.assert.EqualValues(0, suite.sizeTracker.mountSize.GetSize())
}

func (suite *sizeTrackerTestSuite) TestWriteFile() {
	defer suite.cleanupTest()
	// Setup
	file := "file19"
	handle, _ := suite.sizeTracker.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

	testData := "test data"
	data := []byte(testData)
	_, err := suite.sizeTracker.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.assert.NoError(err)
	suite.assert.EqualValues(len(data), suite.sizeTracker.mountSize.GetSize())
}

func (suite *sizeTrackerTestSuite) TestWriteFileErrorBadFd() {
	defer suite.cleanupTest()
	// Setup
	file := "file20"
	handle := handlemap.NewHandle(file)
	len, err := suite.sizeTracker.WriteFile(internal.WriteFileOptions{Handle: handle})
	suite.assert.Error(err)
	suite.assert.EqualValues(syscall.EBADF, err)
	suite.assert.EqualValues(0, len)
	suite.assert.EqualValues(0, suite.sizeTracker.mountSize.GetSize())
}

func (suite *sizeTrackerTestSuite) TestFlushFileEmpty() {
	defer suite.cleanupTest()
	// Setup
	file := "file21"
	handle, err := suite.sizeTracker.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.assert.Error(err)

	// Flush the Empty File
	err = suite.sizeTracker.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NoError(err)
	suite.assert.False(handle.Dirty())

	// Path should be in fake storage
	suite.assert.EqualValues(0, suite.sizeTracker.mountSize.GetSize())
}

func (suite *sizeTrackerTestSuite) TestFlushFile() {
	defer suite.cleanupTest()
	// Setup
	file := "file22"
	handle, _ := suite.sizeTracker.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	_, err := suite.sizeTracker.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.assert.Error(err)

	// Flush the Empty File
	err = suite.sizeTracker.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NoError(err)

	suite.assert.EqualValues(len(testData), suite.sizeTracker.mountSize.GetSize())
}

func (suite *sizeTrackerTestSuite) TestFlushFileErrorBadFd() {
	defer suite.cleanupTest()
	// Setup
	file := "file23"
	handle := handlemap.NewHandle(file)
	handle.Flags.Set(handlemap.HandleFlagDirty)
	err := suite.sizeTracker.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.Error(err)
	suite.assert.EqualValues(syscall.EBADF, err)
	suite.assert.EqualValues(0, suite.sizeTracker.mountSize.GetSize())
}

func (suite *sizeTrackerTestSuite) TestRenameFile() {
	defer suite.cleanupTest()
	// Setup
	src := "source2"
	dst := "destination2"
	handle, err := suite.sizeTracker.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0666})
	suite.assert.NoError(err)
	err = suite.sizeTracker.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	testData := "test data"
	data := []byte(testData)
	_, err = suite.sizeTracker.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.assert.Error(err)

	// RenameFile
	err = suite.sizeTracker.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	suite.assert.NoError(err)

	suite.assert.EqualValues(len(testData), suite.sizeTracker.mountSize.GetSize())
}

func (suite *sizeTrackerTestSuite) TestRenameOpenFile() {
	defer suite.cleanupTest()

	src := "source6"
	dst := "destination6"

	// create source file
	handle, err := suite.sizeTracker.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0666})
	suite.assert.NoError(err)

	// rename open file
	err = suite.sizeTracker.RenameFile(internal.RenameFileOptions{
		Src: src,
		Dst: dst,
	})
	suite.assert.NoError(err)

	// write to file handle
	data := []byte("newdata")
	n, err := suite.sizeTracker.WriteFile(internal.WriteFileOptions{
		Handle: handle,
		Data:   data,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(len(data), n)

	// Close file handle
	err = suite.sizeTracker.CloseFile(internal.CloseFileOptions{
		Handle: handle,
	})
	suite.assert.NoError(err)

	suite.assert.EqualValues(len(data), suite.sizeTracker.mountSize.GetSize())
}

func (suite *sizeTrackerTestSuite) TestTruncateFile() {
	defer suite.cleanupTest()
	// Setup
	path := "file30"
	handle, _ := suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.loopback.CloseFile(internal.CloseFileOptions{Handle: handle})

	size := 1024
	err := suite.sizeTracker.TruncateFile(internal.TruncateFileOptions{Name: path, Size: int64(size)})
	suite.assert.NoError(err)

	suite.assert.EqualValues(size, suite.sizeTracker.mountSize.GetSize())
}

func (suite *sizeTrackerTestSuite) TestStatFS() {
	defer suite.cleanupTest()

	file := "file41"
	handle, _ := suite.sizeTracker.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	data := make([]byte, 1024*1024)
	_, err := suite.sizeTracker.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.assert.NoError(err)
	err = suite.sizeTracker.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NoError(err)
	stat, ret, err := suite.sizeTracker.StatFs()
	suite.assert.True(ret)
	suite.assert.NoError(err)
	suite.assert.NotEqual(&common.Statfs_t{}, stat)

	suite.assert.Equal(uint64(512), stat.Blocks)
	suite.assert.Equal(int64(4096), stat.Bsize)
	suite.assert.Equal(int64(4096), stat.Frsize)
	suite.assert.Equal(uint64(255), stat.Namemax)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSizeTrackerTestSuite(t *testing.T) {
	suite.Run(t, new(sizeTrackerTestSuite))
}
