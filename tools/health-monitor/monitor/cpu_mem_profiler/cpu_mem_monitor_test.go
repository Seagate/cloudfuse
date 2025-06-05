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

package cpu_mem_profiler

import (
	"fmt"
	"os"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	hmcommon "github.com/Seagate/cloudfuse/tools/health-monitor/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type cpuMemMonitorTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *cpuMemMonitorTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
}

func (suite *cpuMemMonitorTestSuite) TestGetCpuMemoryUsage() {
	cm := &CpuMemProfiler{
		name:         hmcommon.CpuMemoryProfiler,
		pid:          fmt.Sprintf("%v", os.Getpid()),
		pollInterval: 5,
	}

	c, err := cm.getCpuMemoryUsage()
	suite.assert.NotNil(c)
	suite.assert.NoError(err)
}

func (suite *cpuMemMonitorTestSuite) TestGetCpuMemoryUsageFailure() {
	cm := &CpuMemProfiler{
		name:         hmcommon.CpuMemoryProfiler,
		pid:          "abcd",
		pollInterval: 5,
	}

	c, err := cm.getCpuMemoryUsage()
	suite.assert.Nil(c)
	suite.assert.Error(err)
}

func TestCpuMemMonitor(t *testing.T) {
	suite.Run(t, new(cpuMemMonitorTestSuite))
}
