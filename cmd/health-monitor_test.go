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

package cmd

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	"cloudfuse/common"
	"cloudfuse/common/log"
	"cloudfuse/component/file_cache"
	hmcommon "cloudfuse/tools/health-monitor/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var configHmonTest string = `
file_cache:
  path: /tmp/fileCachePath
  max-size-mb: 500
health_monitor:
  enable-monitoring: true
  stats-poll-interval-sec: 10
  process-monitor-interval-sec: 30
  output-path: /tmp/monitor
  monitor-disable-list:
    - cloudfuse_stats
    - file_cache_monitor
    - cpu_profiler
    - memory_profiler
    - network_profiler
`

type hmonTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *hmonTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
}

func (suite *hmonTestSuite) cleanupTest() {
	resetCLIFlags(*healthMonCmd)
	resetCLIFlags(*healthMonStop)
	resetCLIFlags(*healthMonStopAll)
}

func (suite *hmonTestSuite) TestValidateHmonOptions() {
	defer suite.cleanupTest()

	pid = ""
	configFile = ""

	err := validateHMonOptions()
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "pid of cloudfuse process not given")
	suite.assert.Contains(err.Error(), "config file not given")

	pid = "12345"
	configFile = "config.yaml"
	err = validateHMonOptions()
	suite.assert.Nil(err)
}

func (suite *hmonTestSuite) TestBuildHmonCliParams() {
	defer suite.cleanupTest()

	options = mountOptions{}
	options.MonitorOpt = monitorOptions{
		EnableMon:       true,
		DisableList:     []string{hmcommon.CloudfuseStats, hmcommon.CpuProfiler, hmcommon.MemoryProfiler, hmcommon.NetworkProfiler, hmcommon.FileCacheMon, "invalid_monitor"},
		CfsPollInterval: 10,
		ProcMonInterval: 10,
		OutputPath:      "/tmp/health_monitor",
	}
	cacheMonitorOptions = file_cache.FileCacheOptions{
		TmpPath:   "/tmp/file_cache",
		MaxSizeMB: 200,
	}

	cliParams := buildCliParamForMonitor()
	suite.assert.Equal(len(cliParams), 11)
}

func (suite *hmonTestSuite) TestHmonInvalidOptions() {
	defer suite.cleanupTest()

	op, err := executeCommandC(rootCmd, "health-monitor", "--pid=", "--config-file=")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "pid of cloudfuse process not given")
	suite.assert.Contains(op, "config file not given")
}

func (suite *hmonTestSuite) TestHmonInvalidConfigFile() {
	defer suite.cleanupTest()

	op, err := executeCommandC(rootCmd, "health-monitor", "--pid=12345", "--config-file=cfgNotFound.yaml")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "invalid config file")
	// The error message is different on Windows, so need to test with cases
	if runtime.GOOS == "windows" {
		suite.assert.Contains(op, "cannot find the file specified")
	} else {
		suite.assert.Contains(op, "no such file or directory")
	}
}

func (suite *hmonTestSuite) TestHmonWithConfigFailure() {
	defer suite.cleanupTest()

	confFile, err := os.CreateTemp("", "conf*.yaml")
	suite.assert.Nil(err)
	cfgFileHmonTest := confFile.Name()
	defer os.Remove(cfgFileHmonTest)

	_, err = confFile.WriteString(configHmonTest)
	suite.assert.Nil(err)
	confFile.Close()

	op, err := executeCommandC(rootCmd, "health-monitor", "--pid=12345", fmt.Sprintf("--config-file=%s", cfgFileHmonTest))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to start health monitor")
}

func (suite *hmonTestSuite) TestHmonStopAllFailure() {
	op, err := executeCommandC(rootCmd, "health-monitor", "stop", "all")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to stop all health monitor binaries")
}

func (suite *hmonTestSuite) TestHmonStopPidEmpty() {
	op, err := executeCommandC(rootCmd, "health-monitor", "stop", "--pid=")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "pid of cloudfuse process not given")
}

func (suite *hmonTestSuite) TestHmonStopPidInvalid() {
	op, err := executeCommandC(rootCmd, "health-monitor", "stop", "--pid=12345")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to get health monitor pid")
}

func (suite *hmonTestSuite) TestHmonStopPidFailure() {
	err := stop("12345")
	suite.assert.NotNil(err)
}

func TestHealthMonitorCommand(t *testing.T) {
	suite.Run(t, new(hmonTestSuite))
}
