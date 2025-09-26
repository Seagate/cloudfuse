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

package cmd

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type secureConfigTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *secureConfigTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
}

func (suite *secureConfigTestSuite) cleanupTest() {
	resetSecureCLIFlags()
}

func executeCommandSecure(root *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err = root.Execute()

	return buf.String(), err
}

func resetSecureCLIFlags() {
	secureCmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		f.Value.Set(f.DefValue)
	})
}

func TestSecureConfig(t *testing.T) {
	suite.Run(t, new(secureConfigTestSuite))
}

func (suite *secureConfigTestSuite) TestHelp() {
	defer suite.cleanupTest()
	_, err := executeCommandSecure(rootCmd, "secure", "-h")
	suite.assert.NoError(err)
}

var testPlainTextConfig string = `
config-version: 1.0.0
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
  entry-expiration-sec: 1`

func (suite *secureConfigTestSuite) TestSecureConfigEncrypt() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "conf*.yaml")
	passphrase := "12312312312312312312312312312312"

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	confFile.Close()

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"encrypt",
		fmt.Sprintf("--config-file=%s", confFile.Name()),
		fmt.Sprintf("--passphrase=%s", passphrase),
		fmt.Sprintf("--output-file=%s", outFile.Name()),
	)
	suite.assert.NoError(err)

	// Config file should be deleted
	suite.assert.NoFileExists(confFile.Name())
}

func (suite *secureConfigTestSuite) TestSecureConfigEncrypt2() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "conf*.yaml")
	passphrase := "hvHlJUKlmZql3gLAcP6Ho41Js5rm8zUAKnwGb1lIffg="

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	confFile.Close()

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"encrypt",
		fmt.Sprintf("--config-file=%s", confFile.Name()),
		fmt.Sprintf("--passphrase=%s", passphrase),
		fmt.Sprintf("--output-file=%s", outFile.Name()),
	)
	suite.assert.NoError(err)

	// Config file should be deleted
	suite.assert.NoFileExists(confFile.Name())
}

func (suite *secureConfigTestSuite) TestSecureConfigEncryptNoOutfile() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile := confFile.Name() + SecureConfigExtension
	passphrase := "12312312312312312312312312312312"

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile)

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	confFile.Close()

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"encrypt",
		fmt.Sprintf("--config-file=%s", confFile.Name()),
		fmt.Sprintf("--passphrase=%s", passphrase),
	)
	suite.assert.NoError(err)

	// Config file should be deleted
	suite.assert.NoFileExists(confFile.Name())

	// Outfile should exist with proper extension
	suite.assert.FileExists(outFile)
}

func (suite *secureConfigTestSuite) TestSecureConfigEncryptNotExistent() {
	defer suite.cleanupTest()
	confFile := "abcd.yaml"
	passphrase := "12312312312312312312312312312312"
	_, err := executeCommandSecure(
		rootCmd,
		"secure",
		"encrypt",
		fmt.Sprintf("--config-file=%s", confFile),
		fmt.Sprintf("--passphrase=%s", passphrase),
	)
	suite.assert.Error(err)
}

func (suite *secureConfigTestSuite) TestSecureConfigEncryptNoConfig() {
	defer suite.cleanupTest()

	_, err := executeCommandSecure(rootCmd, "secure", "encrypt")
	suite.assert.Error(err)
}

func (suite *secureConfigTestSuite) TestSecureConfigEncryptNoKey() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")

	defer os.Remove(confFile.Name())

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	confFile.Close()

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"encrypt",
		fmt.Sprintf("--config-file=%s", confFile.Name()),
	)
	suite.assert.Error(err)
}

func (suite *secureConfigTestSuite) TestSecureConfigEncryptInvalidKey() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "conf*.yaml")
	passphrase := ""

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	confFile.Close()
	outFile.Close()

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"encrypt",
		fmt.Sprintf("--config-file=%s", confFile.Name()),
		fmt.Sprintf("--passphrase=%s", passphrase),
		fmt.Sprintf("--output-file=%s", outFile.Name()),
	)
	suite.assert.Error(err)
}

func (suite *secureConfigTestSuite) TestSecureConfigDecrypt() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "conf*.yaml")
	passphrase := "12312312312312312312312312312312"
	fmt.Println(passphrase)

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	confFile.Close()
	outFile.Close()

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"encrypt",
		fmt.Sprintf("--config-file=%s", confFile.Name()),
		fmt.Sprintf("--passphrase=%s", passphrase),
		fmt.Sprintf("--output-file=%s", outFile.Name()),
	)
	suite.assert.NoError(err)

	// Config file should be deleted
	suite.assert.NoFileExists(confFile.Name())

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"decrypt",
		fmt.Sprintf("--config-file=%s", outFile.Name()),
		fmt.Sprintf("--passphrase=%s", passphrase),
		"--output-file=./tmp.yaml",
	)
	suite.assert.NoError(err)

	data, err := os.ReadFile("./tmp.yaml")
	suite.assert.NoError(err)

	suite.assert.Equal(testPlainTextConfig, string(data))

	os.Remove("./tmp.yaml")
	os.Remove(confFile.Name() + "." + SecureConfigExtension)
}

func (suite *secureConfigTestSuite) TestSecureConfigDecrypt2() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "conf*.yaml")
	passphrase := "hvHlJUKlmZql3gLAcP6Ho41Js5rm8zUAKnwGb1lIffg="
	fmt.Println(passphrase)

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	confFile.Close()
	outFile.Close()

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"encrypt",
		fmt.Sprintf("--config-file=%s", confFile.Name()),
		fmt.Sprintf("--passphrase=%s", passphrase),
		fmt.Sprintf("--output-file=%s", outFile.Name()),
	)
	suite.assert.NoError(err)

	// Config file should be deleted
	suite.assert.NoFileExists(confFile.Name())

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"decrypt",
		fmt.Sprintf("--config-file=%s", outFile.Name()),
		fmt.Sprintf("--passphrase=%s", passphrase),
		"--output-file=./tmp.yaml",
	)
	suite.assert.NoError(err)

	data, err := os.ReadFile("./tmp.yaml")
	suite.assert.NoError(err)

	suite.assert.Equal(testPlainTextConfig, string(data))

	os.Remove("./tmp.yaml")
	os.Remove(confFile.Name() + "." + SecureConfigExtension)
}

func (suite *secureConfigTestSuite) TestSecureConfigDecryptNoOutputFile() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile := confFile.Name() + SecureConfigExtension
	passphrase := "12312312312312312312312312312312"

	defer os.Remove(confFile.Name())

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	confFile.Close()

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"encrypt",
		fmt.Sprintf("--config-file=%s", confFile.Name()),
		fmt.Sprintf("--passphrase=%s", passphrase),
	)
	suite.assert.NoError(err)

	// Config file should be deleted
	suite.assert.NoFileExists(confFile.Name())

	// Encrypted file should exist
	suite.assert.FileExists(outFile)

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"decrypt",
		fmt.Sprintf("--config-file=%s", outFile),
		fmt.Sprintf("--passphrase=%s", passphrase),
	)
	suite.assert.NoError(err)

	// Config file should exist
	suite.assert.FileExists(confFile.Name())

	// Encrypted file should be deleted
	suite.assert.NoFileExists(outFile)

	data, err := os.ReadFile(confFile.Name())
	suite.assert.NoError(err)

	suite.assert.Equal(testPlainTextConfig, string(data))
}

func (suite *secureConfigTestSuite) TestSecureConfigDecryptNoConfig() {
	defer suite.cleanupTest()

	_, err := executeCommandSecure(rootCmd, "secure", "decrypt")
	suite.assert.Error(err)
}

func (suite *secureConfigTestSuite) TestSecureConfigDecryptNoKey() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")

	defer os.Remove(confFile.Name())

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"decrypt",
		fmt.Sprintf("--config-file=%s", confFile.Name()),
	)
	suite.assert.Error(err)
}

func (suite *secureConfigTestSuite) TestSecureConfigGet() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "conf*.yaml")
	passphrase := "12312312312312312312312312312312"

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	confFile.Close()
	outFile.Close()

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"encrypt",
		fmt.Sprintf("--config-file=%s", confFile.Name()),
		fmt.Sprintf("--passphrase=%s", passphrase),
		fmt.Sprintf("--output-file=%s", outFile.Name()),
	)
	suite.assert.NoError(err)

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"get",
		fmt.Sprintf("--config-file=%s", outFile.Name()),
		fmt.Sprintf("--passphrase=%s", passphrase),
		"--key=logging.level",
	)
	suite.assert.NoError(err)
}

func (suite *secureConfigTestSuite) TestSecureConfigGetInvalidKey() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "conf*.yaml")
	passphrase := "12312312312312312312312312312312"

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	confFile.Close()
	outFile.Close()

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"encrypt",
		fmt.Sprintf("--config-file=%s", confFile.Name()),
		fmt.Sprintf("--passphrase=%s", passphrase),
		fmt.Sprintf("--output-file=%s", outFile.Name()),
	)
	suite.assert.NoError(err)

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"get",
		fmt.Sprintf("--config-file=%s", outFile.Name()),
		fmt.Sprintf("--passphrase=%s", passphrase),
		"--key=abcd.efg",
	)
	suite.assert.Error(err)
}

func (suite *secureConfigTestSuite) TestSecureConfigSet() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "conf*.yaml")
	passphrase := "12312312312312312312312312312312"

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	confFile.Close()
	outFile.Close()

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"encrypt",
		fmt.Sprintf("--config-file=%s", confFile.Name()),
		fmt.Sprintf("--passphrase=%s", passphrase),
		fmt.Sprintf("--output-file=%s", outFile.Name()),
	)
	suite.assert.NoError(err)

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"get",
		fmt.Sprintf("--config-file=%s", outFile.Name()),
		fmt.Sprintf("--passphrase=%s", passphrase),
		"--key=logging.level",
	)
	suite.assert.NoError(err)

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"set",
		fmt.Sprintf("--config-file=%s", outFile.Name()),
		fmt.Sprintf("--passphrase=%s", passphrase),
		"--key=logging.level",
		"--value=log_err",
	)
	suite.assert.NoError(err)

	_, err = executeCommandSecure(
		rootCmd,
		"secure",
		"get",
		fmt.Sprintf("--config-file=%s", outFile.Name()),
		fmt.Sprintf("--passphrase=%s", passphrase),
		"--key=logging.level",
	)
	suite.assert.NoError(err)
}
