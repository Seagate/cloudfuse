/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2026 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.

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
	"runtime"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type mountListTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *mountListTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
}

func (suite *mountListTestSuite) cleanupTest() {
	resetCLIFlags(*mountListCmd)
	resetCLIFlags(*mountCmd)
	resetCLIFlags(*rootCmd)
}

func (suite *mountListTestSuite) TestMountListNoMounts() {
	if runtime.GOOS == "windows" {
		suite.T().Skip("Skipping mount list test on Windows")
	}
	defer suite.cleanupTest()

	// When no mounts exist, should print message
	output, err := executeCommandC(rootCmd, "mount", "list")
	suite.assert.NoError(err)
	// Either no mounts or lists some mounts - both are valid
	suite.assert.True(
		len(output) > 0,
		"Expected output from mount list command",
	)
}

func (suite *mountListTestSuite) TestMountListHelp() {
	defer suite.cleanupTest()

	output, err := executeCommandC(rootCmd, "mount", "list", "--help")
	suite.assert.NoError(err)
	suite.assert.Contains(output, "List all cloudfuse mountpoints")
}

func TestMountListCommand(t *testing.T) {
	suite.Run(t, new(mountListTestSuite))
}
