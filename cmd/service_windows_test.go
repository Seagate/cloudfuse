//go:build windows

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023 Seagate Technology LLC and/or its Affiliates

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
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type serviceTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *serviceTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	servOpts = serviceOptions{}
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
}

func (suite *serviceTestSuite) cleanupTest() {
	resetCLIFlags(*serviceCmd)
	viper.Reset()
}

func (suite *serviceTestSuite) TestHelp() {
	defer suite.cleanupTest()
	_, err := executeCommandC(rootCmd, "service", "-h")
	suite.assert.Nil(err)
}

// Mount Tests

func (suite *serviceTestSuite) TestMountMissingArgs() {
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "service", "mount")
	suite.assert.NotNil(err)
}

func (suite *serviceTestSuite) TestMountPathEmpty() {
	defer suite.cleanupTest()

	mntPath := ""
	cfgFile := "cfgNotFound.yaml"

	op, err := executeCommandC(rootCmd, "service", "mount", mntPath, fmt.Sprintf("--config-file=%s", cfgFile))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount path not provided]")
}

func (suite *serviceTestSuite) TestConfigFileEmpty() {
	defer suite.cleanupTest()

	mntPath := "mntdir" + randomString(8)
	cfgFile := ""

	op, err := executeCommandC(rootCmd, "service", "mount", mntPath, fmt.Sprintf("--config-file=%s", cfgFile))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "config file not provided")
}

func (suite *serviceTestSuite) TestMountDirExist() {
	defer suite.cleanupTest()

	// Create Mount Directory
	mntPath, err := os.MkdirTemp("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntPath)

	// Create config file
	confFile, err := os.CreateTemp("", "conf*.yaml")
	suite.assert.Nil(err)
	cfgFile := confFile.Name()
	defer os.Remove(cfgFile)

	op, err := executeCommandC(rootCmd, "service", "mount", mntPath, fmt.Sprintf("--config-file=%s", cfgFile))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount path exists")
}

func (suite *serviceTestSuite) TestConfigFileNotExist() {
	defer suite.cleanupTest()

	mntPath := "mntdir" + randomString(8)
	cfgFile := "cfgNotFound.yaml"

	op, err := executeCommandC(rootCmd, "service", "mount", mntPath, fmt.Sprintf("--config-file=%s", cfgFile))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "config file does not exist")
}

// Unmount Tests

func (suite *serviceTestSuite) TestUnmountMountPathEmpty() {
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "service", "unmount")
	suite.assert.NotNil(err)
}

func TestServiceCommand(t *testing.T) {
	suite.Run(t, new(serviceTestSuite))
}
