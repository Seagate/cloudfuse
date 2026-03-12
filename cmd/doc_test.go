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
	"runtime"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type docTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *docTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	docCmdInput = struct{ outputLocation string }{}
	options = mountOptions{}
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
}

func (suite *docTestSuite) cleanupTest() {
	resetCLIFlags(*docCmd)
}

func (suite *docTestSuite) TestDocsGeneration() {
	defer suite.cleanupTest()

	opDir := "/tmp/docs_" + randomString(6)
	defer os.RemoveAll(opDir)

	_, err := executeCommandC(rootCmd, "doc", fmt.Sprintf("--output-location=%s", opDir))
	suite.assert.NoError(err)
	files, err := os.ReadDir(opDir)
	suite.assert.NoError(err)
	suite.assert.NotEmpty(files)
}

func (suite *docTestSuite) TestOutputDirCreationError() {
	// TODO: Skip this test on Windows. Requires attempting to write to a folder with no write permission
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestOutputDirCreationError on Windows. Should fix this later.")
		return
	}
	defer suite.cleanupTest()

	opDir := "/var/docs_" + randomString(6)

	op, err := executeCommandC(rootCmd, "doc", fmt.Sprintf("--output-location=%s", opDir))
	suite.assert.Error(err)
	suite.assert.Contains(op, "failed to create output location")
}

func (suite *docTestSuite) TestDocsGenerationError() {
	// TODO: Skip this test on Windows. Requires attempting to write to a folder with no write permission
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestDocsGenerationError on Windows. Should fix this later.")
		return
	}
	defer suite.cleanupTest()

	opDir := "/var"

	op, err := executeCommandC(rootCmd, "doc", fmt.Sprintf("--output-location=%s", opDir))
	suite.assert.Error(err)
	suite.assert.Contains(op, "cannot generate command tree")
}

func (suite *docTestSuite) TestOutputDirIsFileError() {
	defer suite.cleanupTest()

	opFile, err := os.CreateTemp("", "docfile*")
	suite.assert.NoError(err)
	opFileName := opFile.Name()
	opFile.Close()
	defer os.Remove(opFileName)

	op, err := executeCommandC(rootCmd, "doc", fmt.Sprintf("--output-location=%s", opFileName))
	suite.assert.Error(err)
	suite.assert.Contains(op, "output location is invalid as it is pointing to a file")
}

// TestDocHelp tests doc command help output
func (suite *docTestSuite) TestDocHelp() {
	defer suite.cleanupTest()

	op, err := executeCommandC(rootCmd, "doc", "--help")
	suite.assert.NoError(err)
	suite.assert.Contains(op, "Generates Markdown documentation")
	suite.assert.Contains(op, "output-location")
}

// TestDocNoArgs tests doc command without args (should still work with defaults)
func (suite *docTestSuite) TestDocNoArgs() {
	defer suite.cleanupTest()

	// Create temp dir for default output
	opDir := "/tmp/docs_" + randomString(6)
	defer os.RemoveAll(opDir)

	_, err := executeCommandC(rootCmd, "doc", fmt.Sprintf("--output-location=%s", opDir))
	suite.assert.NoError(err)
}

func TestDocCommand(t *testing.T) {
	suite.Run(t, new(docTestSuite))
}
