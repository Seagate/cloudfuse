//go:build linux

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
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var confFileUnMntTest string
var configUnMountLoopback string = `
logging:
  type: syslog
  level: log_debug
  #file-path: cloudfuse.log
default-working-dir: ./
components:
  - libfuse
  - loopbackfs
libfuse:
  attribute-expiration-sec: 120
  entry-expiration-sec: 60
loopbackfs:
`

var currentDir string

type unmountTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *unmountTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())

	options = mountOptions{}
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}

}

func (suite *unmountTestSuite) cleanupTest() {
	resetCLIFlags(*unmountCmd)
	resetCLIFlags(*mountCmd)
	resetCLIFlags(*rootCmd)
}

// mount failure test where the mount directory does not exist
func (suite *unmountTestSuite) TestUnmountCmd() {
	defer suite.cleanupTest()

	mountDirectory1, _ := os.MkdirTemp("", "TestUnMountTemp")
	os.MkdirAll(mountDirectory1, 0777)
	defer os.RemoveAll(mountDirectory1)

	cmd := exec.Command(
		"../cloudfuse",
		"mount",
		mountDirectory1,
		fmt.Sprintf("--config-file=%s", confFileUnMntTest),
	)
	_, err := cmd.Output()
	mountOutput, _ := cmd.CombinedOutput()
	suite.assert.NoError(err)
	if err != nil {
		fmt.Printf("Mount failed with output: %s\n", mountOutput)
	}

	unmountOutput, err := executeCommandC(rootCmd, "unmount", mountDirectory1)
	suite.assert.NoError(err)
	if err != nil {
		fmt.Printf(
			"Unmount failed. Mount output:\n%s\nUnmount output:\n%s\n",
			mountOutput,
			unmountOutput,
		)
	}
}

func (suite *unmountTestSuite) TestUnmountCmdLazy() {
	defer suite.cleanupTest()

	lazyFlags := []string{"--lazy", "-z"}
	flagBeforePath := false
	flagAfterPath := !flagBeforePath
	possibleFlagPositions := []bool{flagBeforePath, flagAfterPath}
	baseCommand := "unmount"

	for _, lazyFlag := range lazyFlags {
		for _, flagPosition := range possibleFlagPositions {
			mountDirectory6, _ := os.MkdirTemp("", "TestUnMountTemp")
			os.MkdirAll(mountDirectory6, 0777)
			defer os.RemoveAll(mountDirectory6)

			cmd := exec.Command(
				"../cloudfuse",
				"mount",
				mountDirectory6,
				fmt.Sprintf("--config-file=%s", confFileUnMntTest),
			)
			_, err := cmd.Output()
			suite.assert.NoError(err)

			// move into the mount directory to cause busy error on regular unmount
			err = os.Chdir(mountDirectory6)
			suite.assert.NoError(err)

			// normal unmount should fail
			_, err = executeCommandC(rootCmd, "unmount", mountDirectory6)
			suite.assert.Error(err)
			if err != nil {
				suite.assert.Contains(err.Error(), "failed to unmount")
			}

			// test lazy unmount
			args := []string{baseCommand}
			if flagPosition == flagBeforePath {
				args = append(args, lazyFlag, mountDirectory6)
			} else {
				args = append(args, mountDirectory6, lazyFlag)
			}
			_, err = executeCommandC(rootCmd, args...)
			suite.assert.NoError(err)

			// leave the mount directory to allow lazy unmount to complete
			err = os.Chdir(currentDir)
			suite.assert.NoError(err)

			// clean up lazy flag
			suite.cleanupTest()
		}
	}
}

func (suite *unmountTestSuite) TestUnmountCmdFail() {
	defer suite.cleanupTest()

	mountDirectory2, _ := os.MkdirTemp("", "TestUnMountTemp")
	os.MkdirAll(mountDirectory2, 0777)
	defer os.RemoveAll(mountDirectory2)

	cmd := exec.Command(
		"../cloudfuse",
		"mount",
		mountDirectory2,
		fmt.Sprintf("--config-file=%s", confFileUnMntTest),
	)
	_, err := cmd.Output()
	suite.assert.NoError(err)

	err = os.Chdir(mountDirectory2)
	suite.assert.NoError(err)

	_, err = executeCommandC(rootCmd, "unmount", mountDirectory2)
	suite.assert.Error(err)

	err = os.Chdir(currentDir)
	suite.assert.NoError(err)
	_, err = executeCommandC(rootCmd, "unmount", mountDirectory2)
	suite.assert.NoError(err)
}

func (suite *unmountTestSuite) TestUnmountCmdWildcard() {
	defer suite.cleanupTest()

	mountDirectory3, _ := os.MkdirTemp("", "TestUnMountTemp")
	os.MkdirAll(mountDirectory3, 0777)
	defer os.RemoveAll(mountDirectory3)

	cmd := exec.Command(
		"../cloudfuse",
		"mount",
		mountDirectory3,
		fmt.Sprintf("--config-file=%s", confFileUnMntTest),
	)
	_, err := cmd.Output()
	suite.assert.NoError(err)

	_, err = executeCommandC(rootCmd, "unmount", mountDirectory3+"*")
	suite.assert.NoError(err)
}

func (suite *unmountTestSuite) TestUnmountCmdWildcardFail() {
	defer suite.cleanupTest()

	mountDirectory4, _ := os.MkdirTemp("", "TestUnMountTemp")
	os.MkdirAll(mountDirectory4, 0777)
	defer os.RemoveAll(mountDirectory4)

	cmd := exec.Command(
		"../cloudfuse",
		"mount",
		mountDirectory4,
		fmt.Sprintf("--config-file=%s", confFileUnMntTest),
	)
	_, err := cmd.Output()
	suite.assert.NoError(err)

	err = os.Chdir(mountDirectory4)
	suite.assert.NoError(err)

	_, err = executeCommandC(rootCmd, "unmount", mountDirectory4+"*")
	suite.assert.Error(err)
	if err != nil {
		suite.assert.Contains(err.Error(), "failed to unmount")
	}

	err = os.Chdir(currentDir)
	suite.assert.NoError(err)

	_, err = executeCommandC(rootCmd, "unmount", mountDirectory4+"*")
	suite.assert.NoError(err)
}

func (suite *unmountTestSuite) TestUnmountCmdValidArg() {
	defer suite.cleanupTest()

	mountDirectory5, _ := os.MkdirTemp("", "TestUnMountTemp")
	os.MkdirAll(mountDirectory5, 0777)
	defer os.RemoveAll(mountDirectory5)

	cmd := exec.Command(
		"../cloudfuse",
		"mount",
		mountDirectory5,
		fmt.Sprintf("--config-file=%s", confFileUnMntTest),
	)
	_, err := cmd.Output()
	suite.assert.NoError(err)

	// Give the system time to register the mount
	time.Sleep(100 * time.Millisecond)

	lst, _ := unmountCmd.ValidArgsFunction(nil, nil, "")
	suite.assert.NotEmpty(lst)

	_, err = executeCommandC(rootCmd, "unmount", mountDirectory5+"*")
	suite.assert.NoError(err)

	// After unmount, ValidArgsFunction returns a message when no mounts are found
	// or returns nil if there are already arguments. Both cases mean no valid mount completions.
	lst, _ = unmountCmd.ValidArgsFunction(nil, []string{mountDirectory5}, "abcd")
	suite.assert.Nil(lst)
}

func TestUnMountCommand(t *testing.T) {
	confFile, err := os.CreateTemp("", "conf*.yaml")
	if err != nil {
		t.Error("Failed to create config file")
	}

	currentDir, _ = os.Getwd()
	tempDir, _ := os.MkdirTemp("", "TestUnMountTemp")

	confFileUnMntTest = confFile.Name()
	defer os.Remove(confFileUnMntTest)

	_, err = confFile.WriteString(configUnMountLoopback)
	if err != nil {
		t.Error("Failed to write to config file")
	}

	_, err = confFile.WriteString("  path: " + tempDir + "\n")
	if err != nil {
		t.Error("Failed to write to config file")
	}

	confFile.Close()

	err = os.MkdirAll(tempDir, 0777)
	if err != nil {
		t.Error("Failed to create loopback dir ", err.Error())
	}

	defer os.RemoveAll(tempDir)
	suite.Run(t, new(unmountTestSuite))
}
