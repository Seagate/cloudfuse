/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
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
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type updateTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *updateTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())

	options = mountOptions{}
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}

}

func (suite *updateTestSuite) cleanupTest() {
	resetCLIFlags(*updateCmd)
	resetCLIFlags(*rootCmd)
}

func (suite *updateTestSuite) TestGetRelease() {
	defer suite.cleanupTest()
	ctx := context.Background()

	validVersion := "1.8.0"
	resultVer, err := getRelease(ctx, validVersion)
	suite.assert.NoError(err)
	suite.assert.Equal(validVersion, resultVer.Version)

	// When no version is passed, should get the latest version
	resultVer, err = getRelease(ctx, "")
	suite.assert.NoError(err)

	invalidVersion := "1.1.10"
	resultVer, err = getRelease(ctx, invalidVersion)
	suite.assert.Error(err)
}

func (suite *updateTestSuite) TestUpdateAdminRightsPromptLinux() {
	if runtime.GOOS != "linux" {
		return
	}
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "update", "--package=deb")
	suite.assert.Error(err)
	suite.assert.Equal(err.Error(), ".deb and .rpm requires elevated privileges")
}

func (suite *updateTestSuite) TestUpdateWithOutputDebLinux() {
	if runtime.GOOS != "linux" {
		return
	}
	defer suite.cleanupTest()

	outputFile, err := os.CreateTemp("", "update-file*")
	suite.assert.NoError(err)

	_, err = executeCommandC(rootCmd, "update", "--package=deb", fmt.Sprintf("--output=%s", outputFile.Name()))
	suite.assert.NoError(err)

	os.Remove(outputFile.Name())
}

func (suite *updateTestSuite) TestUpdateWithOutputRpmLinux() {
	if runtime.GOOS != "linux" {
		return
	}
	defer suite.cleanupTest()

	outputFile, err := os.CreateTemp("", "update-file*")
	suite.assert.NoError(err)

	_, err = executeCommandC(rootCmd, "update", "--package=rpm", fmt.Sprintf("--output=%s", outputFile.Name()))
	suite.assert.NoError(err)

	os.Remove(outputFile.Name())
}

func (suite *updateTestSuite) TestUpdateWithOutputTarLinux() {
	if runtime.GOOS != "linux" {
		return
	}
	defer suite.cleanupTest()

	outputFile, err := os.CreateTemp("", "update-file*")
	suite.assert.NoError(err)

	_, err = executeCommandC(rootCmd, "update", "--package=tar", fmt.Sprintf("--output=%s", outputFile.Name()))
	suite.assert.NoError(err)

	os.Remove(outputFile.Name())
}

func TestUpdateCommand(t *testing.T) {
	suite.Run(t, new(updateTestSuite))
}
