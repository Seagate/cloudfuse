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

type manTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *manTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	manCmdInput = struct{ outputLocation string }{}
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
}

func (suite *manTestSuite) cleanupTest() {
	resetCLIFlags(*manCmd)
}

func (suite *manTestSuite) TestManGeneration() {
	defer suite.cleanupTest()

	opDir := "/tmp/man_" + randomString(6)
	defer os.RemoveAll(opDir)

	_, err := executeCommandC(rootCmd, "man", fmt.Sprintf("--output-location=%s", opDir))
	suite.assert.NoError(err)

	files, err := os.ReadDir(opDir)
	suite.assert.NoError(err)
	suite.assert.NotEmpty(files)

	// Verify man page files were created (should have .1 extension)
	hasManPages := false
	for _, f := range files {
		if len(f.Name()) > 2 && f.Name()[len(f.Name())-2:] == ".1" {
			hasManPages = true
			break
		}
	}
	suite.assert.True(hasManPages, "Expected man page files with .1 extension")
}

func (suite *manTestSuite) TestManOutputDirCreation() {
	defer suite.cleanupTest()

	opDir := "/tmp/man_nested_" + randomString(6) + "/subdir"
	defer os.RemoveAll("/tmp/man_nested_" + opDir[17:23])

	_, err := executeCommandC(rootCmd, "man", fmt.Sprintf("--output-location=%s", opDir))
	suite.assert.NoError(err)

	// Verify directory was created
	_, err = os.Stat(opDir)
	suite.assert.NoError(err)
}

func TestManCommand(t *testing.T) {
	suite.Run(t, new(manTestSuite))
}
