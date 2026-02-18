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
	suite.assert.Equal(Normal, suite.sizeTracker.evictionMode)
}

// Test behavior when bucket usage is below overuseThreshold (92%)
func (suite *sizeTrackerMockTestSuite) TestStateMachineNormalMode() {
	defer suite.cleanupTest()

	// Bucket usage at 80% (below overuseThreshold of 92%)
	bucketUsage := uint64(8 * 1024 * 1024)
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{
		Blocks:  bucketUsage / 4096,
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
	suite.assert.NotNil(stat)

	// Should remain in Normal mode
	suite.assert.EqualValues(0, stat.Blocks)
}

// TestStateMachineTransitionToOveruse tests transition from Normal to Overuse mode
func (suite *sizeTrackerMockTestSuite) TestStateMachineTransitionToOveruse() {
	defer suite.cleanupTest()

	// Use bucket usage at 93% (above overuseThreshold of 92%)
	// With 10MB bucket capacity, 93% = 9.3MB
	bucketUsage := uint64(9752371) // 9.3 MB
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{
		Blocks:  bucketUsage / 4096,
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
	suite.assert.NotNil(stat)

	// Should transition to Overuse mode
	suite.assert.Equal(Overuse, suite.sizeTracker.evictionMode)

	// In Overuse mode: offset = nxEvictionThreshold - normalizationTarget * intendedCapacity
	// where:
	//   nxEvictionThreshold = targetUtilization * displayCapacity = 0.9 * 1MB
	//   normalizationTarget = bucketNormalizedThreshold - hysteresisMargin = 0.88 - 0.02 = 0.86
	//   intendedCapacity = bucketCapacity / serverCount = 10MB / 10 = 1MB
	// offset = 0.9 * 1MB - 0.86 * 1MB = 0.04MB = 40960 bytes
	// Since mountSize is 0, blocks should be offset / 4096
	expectedBlocks := uint64(40960 / 4096)
	suite.assert.Equal(expectedBlocks, stat.Blocks)
}

// TestStateMachineOveruseMode tests behavior in Overuse mode
func (suite *sizeTrackerMockTestSuite) TestStateMachineOveruseMode() {
	suite.cleanupTest()

	// Setup with 1MB mount size to simulate existing usage
	cfg := fmt.Sprintf(
		"libfuse:\n  display-capacity-mb: 1\nsize_tracker:\n  journal-name: %s\n  bucket-capacity-fallback: 10",
		journal_test_name,
	)
	suite.setupTestHelper(cfg)
	defer suite.cleanupTest()

	// Add mount size to simulate usage
	suite.sizeTracker.mountSize.Add(1 * 1024 * 1024)

	// First call with 93% usage to trigger transition to Overuse
	transitionUsage := uint64(9752371) // 9.3 MB
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{
		Blocks:  transitionUsage / 4096,
		Bavail:  0,
		Bfree:   0,
		Bsize:   4096,
		Ffree:   1e9,
		Files:   1e9,
		Frsize:  4096,
		Namemax: 255,
	}, true, nil)
	_, _, _ = suite.sizeTracker.StatFs()

	// Now test behavior in Overuse mode with 94% usage
	bucketUsage := uint64(9856614) // 9.4 MB
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{
		Blocks:  bucketUsage / 4096,
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
	suite.assert.NotNil(stat)

	// Should remain in Overuse mode
	suite.assert.Equal(Overuse, suite.sizeTracker.evictionMode)

	// In Overuse mode, target is bucketNormalizedThreshold - hysteresisMargin = 88% - 2% = 86%
	// This provides more aggressive offset to drive bucket usage down
}

// TestStateMachineTransitionToEmergency tests transition from Overuse to Emergency mode
func (suite *sizeTrackerMockTestSuite) TestStateMachineTransitionToEmergency() {
	suite.cleanupTest()

	cfg := fmt.Sprintf(
		"libfuse:\n  display-capacity-mb: 1\nsize_tracker:\n  journal-name: %s\n  bucket-capacity-fallback: 10",
		journal_test_name,
	)
	suite.setupTestHelper(cfg)
	defer suite.cleanupTest()

	// Add mount size to simulate usage
	suite.sizeTracker.mountSize.Add(1 * 1024 * 1024)

	// First transition to Overuse mode with 93% usage
	overUseUsage := uint64(9752371) // 9.3 MB
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{
		Blocks:  overUseUsage / 4096,
		Bavail:  0,
		Bfree:   0,
		Bsize:   4096,
		Ffree:   1e9,
		Files:   1e9,
		Frsize:  4096,
		Namemax: 255,
	}, true, nil)
	_, _, _ = suite.sizeTracker.StatFs()

	initialServerCount := suite.sizeTracker.serverCount

	// Increase bucket usage to 98% (above emergencyThreshold of 97%)
	bucketUsage := uint64(10275430) // 9.8 MB
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{
		Blocks:  bucketUsage / 4096,
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
	suite.assert.NotNil(stat)

	// Should transition to Emergency mode
	suite.assert.Equal(Emergency, suite.sizeTracker.evictionMode)

	// Server count should be incremented in Emergency mode
	suite.assert.Equal(initialServerCount+1, suite.sizeTracker.serverCount)
}

// TestStateMachineEmergencyMode tests behavior in Emergency mode
func (suite *sizeTrackerMockTestSuite) TestStateMachineEmergencyMode() {
	suite.cleanupTest()

	cfg := fmt.Sprintf(
		"libfuse:\n  display-capacity-mb: 1\nsize_tracker:\n  journal-name: %s\n  bucket-capacity-fallback: 10",
		journal_test_name,
	)
	suite.setupTestHelper(cfg)
	defer suite.cleanupTest()

	// Add mount size to simulate usage
	suite.sizeTracker.mountSize.Add(1 * 1024 * 1024)

	// First transition to Overuse mode with 93% usage
	overUseUsage := uint64(9752371) // 9.3 MB
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{
		Blocks:  overUseUsage / 4096,
		Bavail:  0,
		Bfree:   0,
		Bsize:   4096,
		Ffree:   1e9,
		Files:   1e9,
		Frsize:  4096,
		Namemax: 255,
	}, true, nil)
	_, _, _ = suite.sizeTracker.StatFs()

	// Then transition to Emergency mode with 98% usage
	emergencyUsage := uint64(10275430) // 9.8 MB
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{
		Blocks:  emergencyUsage / 4096,
		Bavail:  0,
		Bfree:   0,
		Bsize:   4096,
		Ffree:   1e9,
		Files:   1e9,
		Frsize:  4096,
		Namemax: 255,
	}, true, nil)
	_, _, _ = suite.sizeTracker.StatFs()

	// Bucket usage at 98% (Emergency level)
	bucketUsage := uint64(10275430) // 9.8 MB
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{
		Blocks:  bucketUsage / 4096,
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
	suite.assert.NotNil(stat)

	// Should remain in Emergency mode
	suite.assert.Equal(Emergency, suite.sizeTracker.evictionMode)

	// In Emergency mode, offset = bucketUsage - serverUsage
	// This makes the system report the whole bucket usage
	expectedOffset := bucketUsage - suite.sizeTracker.mountSize.GetSize()
	expectedBlocks := (suite.sizeTracker.mountSize.GetSize() + expectedOffset) / 4096
	suite.assert.Equal(expectedBlocks, stat.Blocks)
}

// TestStateMachineTransitionBackToNormalFromOveruse tests Overuse -> Normal transition
func (suite *sizeTrackerMockTestSuite) TestStateMachineTransitionBackToNormalFromOveruse() {
	suite.cleanupTest()

	cfg := fmt.Sprintf(
		"libfuse:\n  display-capacity-mb: 1\nsize_tracker:\n  journal-name: %s\n  bucket-capacity-fallback: 10",
		journal_test_name,
	)
	suite.setupTestHelper(cfg)
	defer suite.cleanupTest()

	// Add mount size to simulate usage
	suite.sizeTracker.mountSize.Add(1 * 1024 * 1024)

	// First transition to Overuse mode with 93% usage
	overUseUsage := uint64(9752371) // 9.3 MB
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{
		Blocks:  overUseUsage / 4096,
		Bavail:  0,
		Bfree:   0,
		Bsize:   4096,
		Ffree:   1e9,
		Files:   1e9,
		Frsize:  4096,
		Namemax: 255,
	}, true, nil)
	_, _, _ = suite.sizeTracker.StatFs()

	// Decrease bucket usage to 87% (below bucketNormalizedThreshold of 88%)
	bucketUsage := uint64(9122611) // 8.7 MB
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{
		Blocks:  bucketUsage / 4096,
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
	suite.assert.NotNil(stat)

	// Should transition back to Normal mode
	suite.assert.Equal(Normal, suite.sizeTracker.evictionMode)
}

// TestStateMachineTransitionBackToNormalFromEmergency tests Emergency -> Normal transition
func (suite *sizeTrackerMockTestSuite) TestStateMachineTransitionBackToNormalFromEmergency() {
	suite.cleanupTest()

	cfg := fmt.Sprintf(
		"libfuse:\n  display-capacity-mb: 1\nsize_tracker:\n  journal-name: %s\n  bucket-capacity-fallback: 10",
		journal_test_name,
	)
	suite.setupTestHelper(cfg)
	defer suite.cleanupTest()

	// Add mount size to simulate usage
	suite.sizeTracker.mountSize.Add(1 * 1024 * 1024)

	// First transition to Overuse mode with 93% usage
	overUseUsage := uint64(9752371) // 9.3 MB
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{
		Blocks:  overUseUsage / 4096,
		Bavail:  0,
		Bfree:   0,
		Bsize:   4096,
		Ffree:   1e9,
		Files:   1e9,
		Frsize:  4096,
		Namemax: 255,
	}, true, nil)
	_, _, _ = suite.sizeTracker.StatFs()

	// Then transition to Emergency mode with 98% usage
	emergencyUsage := uint64(10275430) // 9.8 MB
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{
		Blocks:  emergencyUsage / 4096,
		Bavail:  0,
		Bfree:   0,
		Bsize:   4096,
		Ffree:   1e9,
		Files:   1e9,
		Frsize:  4096,
		Namemax: 255,
	}, true, nil)
	_, _, _ = suite.sizeTracker.StatFs()

	// Decrease bucket usage to 85% (below bucketNormalizedThreshold of 88%)
	bucketUsage := uint64(8912896) // 8.5 MB
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{
		Blocks:  bucketUsage / 4096,
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
	suite.assert.NotNil(stat)

	// Should transition back to Normal mode (Emergency can skip Overuse on way down)
	suite.assert.Equal(Normal, suite.sizeTracker.evictionMode)
}

// TestStateMachineHysteresis tests that hysteresis prevents rapid state changes
func (suite *sizeTrackerMockTestSuite) TestStateMachineHysteresis() {
	suite.cleanupTest()

	cfg := fmt.Sprintf(
		"libfuse:\n  display-capacity-mb: 1\nsize_tracker:\n  journal-name: %s\n  bucket-capacity-fallback: 10",
		journal_test_name,
	)
	suite.setupTestHelper(cfg)
	defer suite.cleanupTest()

	// Add mount size to simulate usage
	suite.sizeTracker.mountSize.Add(1 * 1024 * 1024)

	// Start in Normal mode at 91% (below 92% overuseThreshold)
	suite.assert.Equal(Normal, suite.sizeTracker.evictionMode)
	bucketUsage := uint64(9542042) // 9.1 MB
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{
		Blocks:  bucketUsage / 4096,
		Bsize:   4096,
		Bavail:  0,
		Bfree:   0,
		Ffree:   1e9,
		Files:   1e9,
		Frsize:  4096,
		Namemax: 255,
	}, true, nil)
	_, _, err := suite.sizeTracker.StatFs()
	suite.assert.NoError(err)
	suite.assert.Equal(Normal, suite.sizeTracker.evictionMode)

	// Transition to Overuse at 93%
	bucketUsage = uint64(9752371) // 9.3 MB
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{
		Blocks:  bucketUsage / 4096,
		Bsize:   4096,
		Bavail:  0,
		Bfree:   0,
		Ffree:   1e9,
		Files:   1e9,
		Frsize:  4096,
		Namemax: 255,
	}, true, nil)
	_, _, err = suite.sizeTracker.StatFs()
	suite.assert.NoError(err)
	suite.assert.Equal(Overuse, suite.sizeTracker.evictionMode)

	// Stay at 91% - should remain in Overuse due to hysteresis
	// (needs to go below 88% bucketNormalizedThreshold to return to Normal)
	bucketUsage = uint64(9542042) // 9.1 MB
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{
		Blocks:  bucketUsage / 4096,
		Bsize:   4096,
		Bavail:  0,
		Bfree:   0,
		Ffree:   1e9,
		Files:   1e9,
		Frsize:  4096,
		Namemax: 255,
	}, true, nil)
	_, _, err = suite.sizeTracker.StatFs()
	suite.assert.NoError(err)
	suite.assert.Equal(
		Overuse,
		suite.sizeTracker.evictionMode,
		"Hysteresis should prevent immediate transition back",
	)
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

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSizeTrackerMockTestSuite(t *testing.T) {
	suite.Run(t, new(sizeTrackerMockTestSuite))
}
