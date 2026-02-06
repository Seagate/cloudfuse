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
	"os"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type genConfig struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *genConfig) SetupTest() {
	suite.assert = assert.New(suite.T())
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	suite.assert.NoError(err)
}

func (suite *genConfig) cleanupTest() {
	os.Remove(suite.getDefaultLogLocation())
	optsGenCfg = genConfigParams{}
	resetCLIFlags(*generatedConfig)
}

func (suite *genConfig) getDefaultLogLocation() string {
	return "./cloudfuse.yaml"
}

func (suite *genConfig) TestHelp() {
	defer suite.cleanupTest()
	_, err := executeCommandSecure(rootCmd, "gen-config", "-h")
	suite.assert.NoError(err)
}

var testGenConfigTemplate string = `
foreground: false
read-only: true
allow-other: true

logging:
  type: base
  level: log_debug
  file-path: /home/cloudfuse.log
  max-file-size: 100
  file-count: 300
  track-time: true

components:
  - libfuse
  - file_cache
  - attr_cache
  - azstorage

libfuse:
  attribute-expiration-sec: 1
  entry-expiration-sec: 1

file_cache:
  path: { 0 }
  timeout-sec: 180
  allow-non-empty-temp: true
  cleanup-on-start: false`

func (suite *genConfig) TestGenConfigPassphrase() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile := "config_encrypted.aes"
	passphrase := "12312312312312312312312312312312"

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile)

	_, err := confFile.WriteString(testGenConfigTemplate)
	suite.assert.NoError(err)

	confFile.Close()

	_, err = executeCommandC(
		rootCmd,
		"gen-config",
		fmt.Sprintf("--config-file=%s", confFile.Name()),
		fmt.Sprintf("--passphrase=%s", passphrase),
		fmt.Sprintf("--output-file=%s", outFile),
		"--temp-path=/tmp",
	)
	suite.assert.NoError(err)

	// Out file should exist
	suite.assert.FileExists(outFile)
}

func (suite *genConfig) TestGenConfigGet() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile := "config_encrypted.aes"
	passphrase := "12312312312312312312312312312312"

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile)

	_, err := confFile.WriteString(testGenConfigTemplate)
	suite.assert.NoError(err)

	confFile.Close()

	_, err = executeCommandC(
		rootCmd,
		"gen-config",
		fmt.Sprintf("--config-file=%s", confFile.Name()),
		fmt.Sprintf("--passphrase=%s", passphrase),
		fmt.Sprintf("--output-file=%s", outFile),
		"--temp-path=/tmp",
	)
	suite.assert.NoError(err)

	// Out file should exist
	suite.assert.FileExists(outFile)

	// Gen-config should correctly set the temp path for the file_cache
	path, err := executeCommandC(
		rootCmd,
		"secure",
		"get",
		fmt.Sprintf("--config-file=%s", outFile),
		fmt.Sprintf("--passphrase=%s", passphrase),
		"--key=file_cache.path",
	)
	suite.assert.NoError(err)
	suite.assert.Equal("Fetching scalar configuration\nfile_cache.path = /tmp\n", path)
}

func (suite *genConfig) TestNoPath() {
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "gen-config", "--o", "./blobfuse2.yaml")
	suite.assert.Error(err)
}

// TestGenConfigHelp tests the help output
func (suite *genConfig) TestGenConfigHelp() {
	defer suite.cleanupTest()

	output, err := executeCommandC(rootCmd, "gen-config", "--help")
	suite.assert.NoError(err)
	suite.assert.Contains(output, "gen-config")
	suite.assert.Contains(output, "temp-path")
	suite.assert.Contains(output, "config-file")
}

// TestValidateGenConfigOptionsInvalidConfigFile tests validation with invalid config file
func (suite *genConfig) TestValidateGenConfigOptionsInvalidConfigFile() {
	defer suite.cleanupTest()

	_, err := executeCommandC(
		rootCmd,
		"gen-config",
		"--config-file=/nonexistent/path/config.yaml",
		"--temp-path=/tmp",
	)
	suite.assert.Error(err)
}

func TestGenConfig(t *testing.T) {
	suite.Run(t, new(genConfig))
}
