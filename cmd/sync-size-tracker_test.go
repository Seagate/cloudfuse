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

package cmd

import (
	"fmt"
	"os"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type syncCmdSuite struct {
	suite.Suite
	assert *assert.Assertions
	wd     string
}

var configNoSubdirectory string = `
s3storage:
components:
  - libfuse
  - file_cache
  - attr_cache
  - s3storage
`

var configWithSubdirectory string = `
s3storage:
    subdirectory: "/my/prefix"
components:
  - libfuse
  - file_cache
  - attr_cache
  - s3storage
`

func (suite *syncCmdSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	// Silence logs
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
	wd, _ := os.Getwd()
	suite.wd = wd
}

func (suite *syncCmdSuite) cleanupTest() {
	defer resetCLIFlags(*syncCmd)
	_ = os.Chdir(suite.wd)
}

func (suite *syncCmdSuite) TestSyncMissingConfigFile() {
	defer suite.cleanupTest()

	td := os.TempDir() + string(os.PathSeparator) + "cloudfuse_sync_" + randomString(6)
	_ = os.MkdirAll(td, 0755)
	defer os.RemoveAll(td)
	_ = os.Chdir(td)

	out, err := executeCommandC(rootCmd, "sync-size-tracker")
	suite.assert.Error(err)
	suite.assert.Contains(out, "config file not provided")
}

func (suite *syncCmdSuite) TestSyncMissingSubdirectory() {
	defer suite.cleanupTest()

	// Create a minimal config without s3storage.subdirectory
	cfgFile, err := os.CreateTemp("", "cloudfuse_sync_config_*.yaml")
	suite.assert.NoError(err)
	cfgPath := cfgFile.Name()
	_, _ = cfgFile.WriteString(configNoSubdirectory)
	_ = cfgFile.Close()
	defer os.Remove(cfgPath)

	out, err := executeCommandC(
		rootCmd,
		"sync-size-tracker",
		fmt.Sprintf("--config-file=%s", cfgPath),
	)
	suite.assert.Error(err)
	suite.assert.Contains(out, "s3storage.subdirectory must be set")
}

func (suite *syncCmdSuite) TestSyncNoS3Credentials() {
	defer suite.cleanupTest()

	// File without s3storage credentials should fail when trying to start s3storage
	cfgFile, err := os.CreateTemp("", "cloudfuse_sync_config_*.yaml")
	suite.assert.NoError(err)
	cfgPath := cfgFile.Name()
	_, _ = cfgFile.WriteString(configWithSubdirectory)
	_ = cfgFile.Close()
	defer os.Remove(cfgPath)

	out, err := executeCommandC(
		rootCmd,
		"sync-size-tracker",
		fmt.Sprintf("--config-file=%s", cfgPath),
	)
	suite.assert.Error(err)
	suite.assert.Contains(out, "s3storage configure failed")
}

func TestSyncCmd(t *testing.T) {
	suite.Run(t, new(syncCmdSuite))
}
