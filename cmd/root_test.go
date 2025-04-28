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
	"crypto/rand"
	"fmt"
	"strings"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type rootCmdSuite struct {
	suite.Suite
	assert *assert.Assertions
}

type osArgs struct {
	input  string
	output string
}

func resetCLIFlags(cmd cobra.Command) {
	// reset all CLI flags before next test
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		f.Value.Set(f.DefValue)
	})
	viper.Reset()
}

func randomString(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

// Taken from cobra library's testing https://github.com/spf13/cobra/blob/master/command_test.go#L34
func executeCommandC(root *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err = root.Execute()

	return buf.String(), err
}

func (suite *rootCmdSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
	// suite.testExecute()
}

func (suite *rootCmdSuite) cleanupTest() {
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	rootCmd.SetArgs(nil)
}

func (suite *rootCmdSuite) TestNoOptions() {
	defer suite.cleanupTest()
	out, err := executeCommandC(rootCmd, "")
	suite.assert.Contains(out, "missing command options")
	suite.assert.Error(err)
}

func (suite *rootCmdSuite) TestNoOptionsNoVersionCheck() {
	defer suite.cleanupTest()
	out, err := executeCommandC(rootCmd, "--disable-version-check")
	suite.assert.Contains(out, "missing command options")
	suite.assert.Error(err)
}

func (suite *rootCmdSuite) TestNoMountPath() {
	defer suite.cleanupTest()
	out, err := executeCommandC(rootCmd, "mount")
	suite.assert.Contains(out, "accepts 1 arg(s), received 0")
	suite.assert.Error(err)
}

func (suite *rootCmdSuite) TestGetRemoteVersionInvalidURL() {
	defer suite.cleanupTest()
	out, err := getRemoteVersion("abcd")
	suite.assert.Empty(out)
	suite.assert.Error(err)
}

func (suite *rootCmdSuite) TestGetRemoteVersionInvalidRelease() {
	defer suite.cleanupTest()
	latestVersionUrl := common.CloudfuseReleaseURL + "/latest1"
	out, err := getRemoteVersion(latestVersionUrl)
	suite.assert.Empty(out)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "unable to get latest version")
}

func getDummyVersion() string {
	return "0.0.0"
}

func (suite *rootCmdSuite) TestGetRemoteVersionValidURL() {
	defer suite.cleanupTest()
	latestVersionUrl := common.CloudfuseReleaseURL + "/latest"
	out, err := getRemoteVersion(latestVersionUrl)
	suite.assert.NotEmpty(out)
	suite.assert.NoError(err)
}

func (suite *rootCmdSuite) TestGetRemoteVersionCurrentOlder() {
	defer suite.cleanupTest()
	common.CloudfuseVersion = getDummyVersion()
	msg := <-beginDetectNewVersion()
	suite.assert.NotEmpty(msg)
	suite.assert.Contains(msg, "A new version of Cloudfuse is available")
}

func (suite *rootCmdSuite) TestGetRemoteVersionCurrentSame() {
	defer suite.cleanupTest()
	common.CloudfuseVersion = common.CloudfuseVersion_()
	msg := <-beginDetectNewVersion()
	suite.assert.Nil(msg)
}

func (suite *rootCmdSuite) testExecute() {
	suite.T().Helper()

	defer suite.cleanupTest()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--version"})

	err := Execute()
	suite.assert.NoError(err)
	suite.assert.Contains(buf.String(), "cloudfuse version")
}

func (suite *rootCmdSuite) TestParseArgs() {
	defer suite.cleanupTest()
	var inputs = []osArgs{
		{input: "mount abc", output: "mount abc"},
		{input: "mount abc --config-file=./config.yaml", output: "mount abc --config-file=./config.yaml"},
		{input: "help", output: "help"},
		{input: "--help", output: "--help"},
		{input: "version", output: "version"},
		{input: "--version", output: "--version"},
		{input: "version --check=true", output: "version --check=true"},
		{input: "mount abc --config-file=./config.yaml -o ro", output: "mount abc --config-file=./config.yaml -o ro"},
		{input: "abc", output: "mount abc"},
		{input: "-o", output: ""},
		{input: "", output: ""},

		{input: "/home/mntdir -o rw,--config-file=config.yaml,dev,suid", output: "mount /home/mntdir -o rw,dev,suid --config-file=config.yaml"},
		{input: "/home/mntdir -o --config-file=config.yaml,rw,dev,suid", output: "mount /home/mntdir -o rw,dev,suid --config-file=config.yaml"},
		{input: "/home/mntdir -o --config-file=config.yaml,rw", output: "mount /home/mntdir -o rw --config-file=config.yaml"},
		{input: "/home/mntdir -o rw,--config-file=config.yaml,dev,suid -o allow_other", output: "mount /home/mntdir -o rw,dev,suid --config-file=config.yaml -o allow_other"},
		{input: "/home/mntdir -o rw,--config-file=config.yaml,dev,suid -o allow_other,--adls=true", output: "mount /home/mntdir -o rw,dev,suid --config-file=config.yaml -o allow_other --adls=true"},
		{input: "/home/mntdir -o --config-file=config.yaml", output: "mount /home/mntdir --config-file=config.yaml"},
		{input: "/home/mntdir -o", output: "mount /home/mntdir"},
		{input: "mount /home/mntdir -o --config-file=config.yaml,rw", output: "mount /home/mntdir -o rw --config-file=config.yaml"},
	}
	for _, i := range inputs {
		o := parseArgs(strings.Split("cloudfuse "+i.input, " "))
		suite.assert.Equal(i.output, strings.Join(o, " "))
	}
}

func TestRootCmd(t *testing.T) {
	suite.Run(t, new(rootCmdSuite))
}
