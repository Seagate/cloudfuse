/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates

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
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/handlemap"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type sizeTrackerMockTestSuite struct {
	suite.Suite
	assert      *assert.Assertions
	sizeTracker *SizeTracker
	mockCtrl    *gomock.Controller
	mock        *internal.MockComponent
}

func newTestSizeTrackerMock(next internal.Component, configuration string) *SizeTracker {
	_ = config.ReadConfigFromReader(strings.NewReader(configuration))
	sizeTracker := NewSizeTrackerComponent()
	sizeTracker.SetNextComponent(next)
	_ = sizeTracker.Configure(true)

	return sizeTracker.(*SizeTracker)
}

func (suite *sizeTrackerMockTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
	cfg := fmt.Sprintf("size_tracker:\n  journal-name: %s", journal_test_name)
	suite.setupTestHelper(cfg)
}

func (suite *sizeTrackerMockTestSuite) setupTestHelper(config string) {
	suite.assert = assert.New(suite.T())

	suite.mockCtrl = gomock.NewController(suite.T())
	suite.mock = internal.NewMockComponent(suite.mockCtrl)
	suite.sizeTracker = newTestSizeTrackerMock(suite.mock, config)
	_ = suite.sizeTracker.Start(context.Background())
}

func (suite *sizeTrackerMockTestSuite) cleanupTest() {
	err := suite.sizeTracker.Stop()
	if err != nil {
		panic(fmt.Sprintf("Unable to stop size tracker [%s]", err.Error()))
	}
	journal_file := common.JoinUnixFilepath(common.DefaultWorkDir, journal_test_name)
	os.Remove(journal_file)
	suite.mockCtrl.Finish()
}

// Tests the default configuration of attribute cache
func (suite *sizeTrackerMockTestSuite) TestDefault() {
	defer suite.cleanupTest()
	suite.assert.Equal("size_tracker", suite.sizeTracker.Name())
	suite.assert.Equal(uint64(0), suite.sizeTracker.mountSize.GetSize())
}

func (suite *sizeTrackerMockTestSuite) TestStatFSFallBackEnabledUnderThreshold() {
	defer suite.cleanupTest()
	suite.assert.EqualValues(0, suite.sizeTracker.mountSize.GetSize())
	suite.sizeTracker.totalBucketCapacity = 10 * 1024 * 1024

	// Create File
	file := generateFileName()
	suite.mock.EXPECT().
		GetAttr(internal.GetAttrOptions{Name: file}).
		Return(&internal.ObjAttr{Path: file}, nil)
	suite.mock.EXPECT().
		CreateFile(internal.CreateFileOptions{Name: file, Mode: 0644}).
		Return(&handlemap.Handle{}, nil)
	handle, err := suite.sizeTracker.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0644})
	suite.assert.NoError(err)

	// Write File
	data := make([]byte, 1024*1024)
	_, _ = rand.Read(data)
	suite.mock.EXPECT().
		GetAttr(internal.GetAttrOptions{Name: handle.Path}).
		Return(&internal.ObjAttr{Path: file}, nil)
	suite.mock.EXPECT().
		WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data}).
		Return(len(data), nil)
	_, err = suite.sizeTracker.WriteFile(
		internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)

	// Flush File
	suite.mock.EXPECT().
		GetAttr(internal.GetAttrOptions{Name: handle.Path}).
		Return(&internal.ObjAttr{Path: file, Size: int64(len(data))}, nil)
	suite.mock.EXPECT().
		GetAttr(internal.GetAttrOptions{Name: handle.Path}).
		Return(&internal.ObjAttr{Path: file, Size: int64(len(data))}, nil)
	suite.mock.EXPECT().FlushFile(internal.FlushFileOptions{Handle: handle}).Return(nil)
	err = suite.sizeTracker.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NoError(err)
	suite.assert.EqualValues(len(data), suite.sizeTracker.mountSize.GetSize())

	// Call Statfs
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{
		Blocks:  9 * 1024 * 1024 / 4096,
		Bavail:  0,
		Bfree:   0,
		Bsize:   4096,
		Ffree:   1e9,
		Files:   1e9,
		Frsize:  4096,
		Namemax: 255,
	}, true, nil)
	stat, ret, err := suite.sizeTracker.StatFs()
	suite.assert.True(ret)
	suite.assert.NoError(err)
	suite.assert.NotEqual(&common.Statfs_t{}, stat)
	suite.assert.Equal(uint64(1024*1024/4096), stat.Blocks)
	suite.assert.Equal(int64(4096), stat.Bsize)
	suite.assert.Equal(int64(4096), stat.Frsize)
	suite.assert.Equal(uint64(255), stat.Namemax)
}

func (suite *sizeTrackerMockTestSuite) TestStatFSFallBackEnabledOverThreshold() {
	defer suite.cleanupTest()
	suite.assert.EqualValues(0, suite.sizeTracker.mountSize.GetSize())
	suite.sizeTracker.totalBucketCapacity = 10 * 1024 * 1024

	// Create File
	file := generateFileName()
	suite.mock.EXPECT().
		GetAttr(internal.GetAttrOptions{Name: file}).
		Return(&internal.ObjAttr{Path: file}, nil)
	suite.mock.EXPECT().
		CreateFile(internal.CreateFileOptions{Name: file, Mode: 0644}).
		Return(&handlemap.Handle{}, nil)
	handle, err := suite.sizeTracker.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0644})
	suite.assert.NoError(err)

	// Write File
	data := make([]byte, 1024*1024)
	_, _ = rand.Read(data)
	suite.mock.EXPECT().
		GetAttr(internal.GetAttrOptions{Name: handle.Path}).
		Return(&internal.ObjAttr{Path: file}, nil)
	suite.mock.EXPECT().
		WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data}).
		Return(len(data), nil)
	_, err = suite.sizeTracker.WriteFile(
		internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)

	// Flush File
	suite.mock.EXPECT().
		GetAttr(internal.GetAttrOptions{Name: handle.Path}).
		Return(&internal.ObjAttr{Path: file, Size: int64(len(data))}, nil)
	suite.mock.EXPECT().
		GetAttr(internal.GetAttrOptions{Name: handle.Path}).
		Return(&internal.ObjAttr{Path: file, Size: int64(len(data))}, nil)
	suite.mock.EXPECT().FlushFile(internal.FlushFileOptions{Handle: handle}).Return(nil)
	err = suite.sizeTracker.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NoError(err)
	suite.assert.EqualValues(len(data), suite.sizeTracker.mountSize.GetSize())

	// Call Statfs
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{
		Blocks:  10 * 1024 * 1024 / 4096,
		Bavail:  0,
		Bfree:   0,
		Bsize:   4096,
		Ffree:   1e9,
		Files:   1e9,
		Frsize:  4096,
		Namemax: 255,
	}, true, nil)
	stat, ret, err := suite.sizeTracker.StatFs()
	suite.assert.True(ret)
	suite.assert.NoError(err)
	suite.assert.NotEqual(&common.Statfs_t{}, stat)
	suite.assert.Equal(uint64(10*1024*1024/4096), stat.Blocks)
	suite.assert.Equal(int64(4096), stat.Bsize)
	suite.assert.Equal(int64(4096), stat.Frsize)
	suite.assert.Equal(uint64(255), stat.Namemax)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSizeTrackerMockTestSuite(t *testing.T) {
	suite.Run(t, new(sizeTrackerMockTestSuite))
}
