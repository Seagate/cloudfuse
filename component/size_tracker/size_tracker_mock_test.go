/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2026 Seagate Technology LLC and/or its Affiliates

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
	"strings"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
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
	cfg := fmt.Sprintf(
		"libfuse:\n  display-capacity-mb: 1\nsize_tracker:\n  journal-name: %s\n  bucket-capacity-fallback: 10",
		journal_test_name,
	)
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

// Tests the default configuration of size tracker
func (suite *sizeTrackerMockTestSuite) TestDefault() {
	defer suite.cleanupTest()
	suite.assert.Equal("size_tracker", suite.sizeTracker.Name())
	suite.assert.Equal(uint64(0), suite.sizeTracker.mountSize.GetSize())
}

// StatFs reports tracked size only; gomock fails if the next component's StatFs is called.
func (suite *sizeTrackerMockTestSuite) TestStatFsWithBucketCapacity() {
	defer suite.cleanupTest()

	suite.sizeTracker.mountSize.Add(5 * 1024 * 1024)

	stat, ret, err := suite.sizeTracker.StatFs()
	suite.assert.True(ret)
	suite.assert.NoError(err)
	suite.assert.NotNil(stat)
	suite.assert.EqualValues((5*1024*1024)/4096, stat.Blocks)
	suite.assert.Zero(stat.Bavail)
	suite.assert.Zero(stat.Bfree)
}

// TestStatFsWithoutBucketCapacity tests that StatFs works when bucket capacity is not configured
func (suite *sizeTrackerMockTestSuite) TestStatFsWithoutBucketCapacity() {
	suite.cleanupTest()

	// Setup without bucket-capacity-fallback
	cfg := fmt.Sprintf(
		"libfuse:\n  display-capacity-mb: 1\nsize_tracker:\n  journal-name: %s",
		journal_test_name,
	)
	suite.setupTestHelper(cfg)
	defer suite.cleanupTest()

	// Add mount size
	suite.sizeTracker.mountSize.Add(5 * 1024 * 1024)

	stat, ret, err := suite.sizeTracker.StatFs()
	suite.assert.True(ret)
	suite.assert.NoError(err)
	suite.assert.NotNil(stat)

	// Should just report mount size without any offset
	expectedBlocks := (5 * 1024 * 1024) / 4096
	suite.assert.Equal(uint64(expectedBlocks), stat.Blocks)
	suite.assert.Equal(uint64(0), stat.Bavail)
	suite.assert.Equal(uint64(0), stat.Bfree)
}

func (suite *sizeTrackerMockTestSuite) TestConfigureDisplayCapacityExceedsBucketCapacity() {
	defer suite.cleanupTest()

	cfg := fmt.Sprintf(
		"libfuse:\n  display-capacity-mb: 11\nsize_tracker:\n  journal-name: %s\n  bucket-capacity-fallback: 10",
		journal_test_name,
	)
	_ = config.ReadConfigFromReader(strings.NewReader(cfg))
	st := NewSizeTrackerComponent()
	suite.assert.ErrorContains(
		st.Configure(true),
		"cannot be less than libfuse.display-capacity-mb",
	)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSizeTrackerMockTestSuite(t *testing.T) {
	suite.Run(t, new(sizeTrackerMockTestSuite))
}
