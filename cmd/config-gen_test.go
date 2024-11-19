/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2024 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.

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
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type genConfigTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *genConfigTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
}

func (suite *genConfigTestSuite) cleanupTest() {
	resetGenCLIFlags()
}

func executeCommandGen(root *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err = root.Execute()

	out, readErr := io.ReadAll(buf)
	if readErr != nil {
		panic(fmt.Sprintf("Unable to read buffer: %v", readErr))
	}

	return string(out), err
}

func resetGenCLIFlags() {
	secureCmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		f.Value.Set(f.DefValue)
	})
}

func TestGenConfig(t *testing.T) {
	suite.Run(t, new(genConfigTestSuite))
}

func (suite *genConfigTestSuite) TestHelp() {
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

func (suite *genConfigTestSuite) TestGenConfig() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile := "config_encrypted.aes"
	passphrase := base64.StdEncoding.EncodeToString([]byte("12312312312312312312312312312312"))

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile)

	_, err := confFile.WriteString(testGenConfigTemplate)
	suite.assert.NoError(err)

	confFile.Close()

	_, err = executeCommandGen(rootCmd, "gen-config", fmt.Sprintf("--config-file=%s", confFile.Name()), fmt.Sprintf("--passphrase=%s", passphrase), fmt.Sprintf("--output-file=%s", outFile), "--temp-path=/tmp")
	suite.assert.NoError(err)

	// Out file should exist
	suite.assert.FileExists(outFile)
}

func (suite *genConfigTestSuite) TestGenConfigGet() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile := "config_encrypted.aes"
	passphrase := base64.StdEncoding.EncodeToString([]byte("12312312312312312312312312312312"))

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile)

	_, err := confFile.WriteString(testGenConfigTemplate)
	suite.assert.NoError(err)

	confFile.Close()

	_, err = executeCommandGen(rootCmd, "gen-config", fmt.Sprintf("--config-file=%s", confFile.Name()), fmt.Sprintf("--passphrase=%s", passphrase), fmt.Sprintf("--output-file=%s", outFile), "--temp-path=/tmp")
	suite.assert.NoError(err)

	// Out file should exist
	suite.assert.FileExists(outFile)

	// Gen-config should correctly set the temp path for the file_cache
	path, err := executeCommandGen(rootCmd, "secure", "get", fmt.Sprintf("--config-file=%s", outFile), fmt.Sprintf("--passphrase=%s", passphrase), "--key=file_cache.path")
	suite.assert.NoError(err)
	suite.assert.Equal("Fetching scalar configuration\nfile_cache.path = /tmp\n", path)
}
