/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

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

package cmd

import (
	"fmt"
	"os"
	"runtime"
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

	_, err = executeCommandC(rootCmd, "gen-config", fmt.Sprintf("--config-file=%s", confFile.Name()), fmt.Sprintf("--passphrase=%s", passphrase), fmt.Sprintf("--output-file=%s", outFile), "--temp-path=/tmp")
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

	_, err = executeCommandC(rootCmd, "gen-config", fmt.Sprintf("--config-file=%s", confFile.Name()), fmt.Sprintf("--passphrase=%s", passphrase), fmt.Sprintf("--output-file=%s", outFile), "--temp-path=/tmp")
	suite.assert.NoError(err)

	// Out file should exist
	suite.assert.FileExists(outFile)

	// Gen-config should correctly set the temp path for the file_cache
	path, err := executeCommandC(rootCmd, "secure", "get", fmt.Sprintf("--config-file=%s", outFile), fmt.Sprintf("--passphrase=%s", passphrase), "--key=file_cache.path")
	suite.assert.NoError(err)
	suite.assert.Equal("Fetching scalar configuration\nfile_cache.path = /tmp\n", path)
}

func (suite *genConfig) TestNoTempPath() {
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "gen-config")
	suite.assert.Error(err)
}

func (suite *genConfig) TestFileCacheConfigGen() {
	defer suite.cleanupTest()

	tempDir, _ := os.MkdirTemp("", "TestTempDir")
	os.MkdirAll(tempDir, 0777)
	defer os.RemoveAll(tempDir)

	_, err := executeCommandC(rootCmd, "gen-config", fmt.Sprintf("--tmp-path=%s", tempDir))
	suite.assert.Nil(err)

	logFilePath := suite.getDefaultLogLocation()
	defer os.Remove(logFilePath)

	//Check if a file is generated named generatedConfig.yaml
	suite.assert.FileExists(logFilePath)

	//check if the generated file is not empty
	file, err := os.ReadFile(logFilePath)
	suite.assert.Nil(err)
	suite.assert.NotEmpty(file)

	//check if the generated file has the correct component
	suite.assert.Contains(string(file), "file_cache")

	//check if the generated file has the correct temp path
	suite.assert.Contains(string(file), common.JoinUnixFilepath(tempDir))
}

func (suite *genConfig) TestBlockCacheConfigGen() {
	// TODO: Skip this test on Windows until block cache supported
	if runtime.GOOS == "windows" {
		return
	}
	defer suite.cleanupTest()

	tempDir, _ := os.MkdirTemp("", "TestTempDir")
	os.MkdirAll(tempDir, 0777)
	defer os.RemoveAll(tempDir)

	_, err := executeCommandC(rootCmd, "gen-config", "--block-cache", fmt.Sprintf("--tmp-path=%s", tempDir))
	suite.assert.Nil(err)

	logFilePath := suite.getDefaultLogLocation()
	defer os.Remove(logFilePath)

	//Check if a file is generated named generatedConfig.yaml
	suite.assert.FileExists(logFilePath)

	//check if the generated file is not empty
	file, err := os.ReadFile(logFilePath)
	suite.assert.Nil(err)
	suite.assert.NotEmpty(file)

	//check if the generated file has the correct component
	suite.assert.Contains(string(file), "block_cache")
	suite.assert.NotContains(string(file), "file_cache")

	//check if the generated file has the correct temp path
	suite.assert.Contains(string(file), tempDir)
}

func (suite *genConfig) TestBlockCacheConfigGen1() {
	// TODO: Skip this test on Windows until block cache supported
	if runtime.GOOS == "windows" {
		return
	}
	defer suite.cleanupTest()

	tempDir, _ := os.MkdirTemp("", "TestTempDir")
	os.MkdirAll(tempDir, 0777)
	defer os.RemoveAll(tempDir)

	_, err := executeCommandC(rootCmd, "gen-config", "--block-cache")
	suite.assert.Nil(err)

	logFilePath := suite.getDefaultLogLocation()
	defer os.Remove(logFilePath)

	//Check if a file is generated named generatedConfig.yaml
	suite.assert.FileExists(logFilePath)

	//check if the generated file is not empty
	file, err := os.ReadFile(logFilePath)
	suite.assert.Nil(err)
	suite.assert.NotEmpty(file)

	//check if the generated file has the correct component
	suite.assert.Contains(string(file), "block_cache")
	suite.assert.NotContains(string(file), "file_cache")

	//check if the generated file has the correct temp path
	suite.assert.NotContains(string(file), tempDir)
}

// test direct io flag
func (suite *genConfig) TestDirectIOConfigGen() {
	// TODO: Skip this test on Windows until block cache supported
	if runtime.GOOS == "windows" {
		return
	}
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "gen-config", "--block-cache", "--direct-io")
	suite.assert.Nil(err)

	logFilePath := suite.getDefaultLogLocation()
	defer os.Remove(logFilePath)

	suite.assert.FileExists(logFilePath)

	//check if the generated file is not empty
	file, err := os.ReadFile(logFilePath)
	suite.assert.Nil(err)
	suite.assert.NotEmpty(file)

	//check if the generated file has the correct direct io flag
	suite.assert.Contains(string(file), "direct-io: true")
	suite.assert.NotContains(string(file), " path: ")
}

func (suite *genConfig) TestOutputFile() {
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "gen-config", "--direct-io", "--o", "1.yaml", "--tmp-path=/tmp")
	suite.assert.Nil(err)

	//check if the generated file is not empty
	file, err := os.ReadFile("1.yaml")
	suite.assert.Nil(err)
	suite.assert.NotEmpty(file)

	//check if the generated file has the correct direct io flag
	suite.assert.Contains(string(file), "direct-io: true")
	suite.assert.Contains(string(file), " path: ")
	_ = os.Remove("1.yaml")
}

func (suite *genConfig) TestConsoleOutput() {
	defer suite.cleanupTest()

	op, err := executeCommandC(rootCmd, "gen-config", "--direct-io", "--o", "console", "--tmp-path=/tmp")
	suite.assert.Nil(err)

	//check if the generated file has the correct direct io flag
	suite.assert.Empty(op)
}

func TestGenConfig(t *testing.T) {
	suite.Run(t, new(genConfig))
}
