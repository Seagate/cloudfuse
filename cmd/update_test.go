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
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
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

	// setup
	ctx := context.Background()
	const validVersion = "1.8.0"
	// GitHub has a rate limit of 60 requests per hour for unauthenticated requests.
	// So we'll use a mock server to simulate the GitHub API response.
	releasePath := strings.TrimPrefix(common.CloudfuseReleaseURL, "https://api.github.com")
	validVersionPath := releasePath + "/tags/v" + validVersion
	latestVersionPath := releasePath + "/latest"
	const serverVersion = "1.12.0"
	const assetBaseName = serverVersion + "_" + runtime.GOOS + "_" + runtime.GOARCH
	const assetsJson = `"assets": [{"name": "checksums_sha256.txt"},{"name": "cloudfuse_` + assetBaseName + `.exe"},{"name": "cloudfuse_` + assetBaseName + `.tar.gz"}]`
	const respJson = `{"tag_name": "v1.12.0","name": "v1.12.0",` + assetsJson + "}"
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case latestVersionPath:
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, respJson)
		case validVersionPath:
			w.Header().Set("Content-Type", "application/json")
			responseJson := strings.ReplaceAll(respJson, serverVersion, validVersion)
			fmt.Fprintln(w, responseJson)
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer testServer.Close()
	testReleaseUrl := testServer.URL + releasePath

	// test
	resultVer, err := getRelease(ctx, validVersion, testReleaseUrl)
	suite.assert.NoError(err)
	suite.assert.Equal(validVersion, resultVer.Version)

	// When no version is passed, should get the latest version
	resultVer, err = getRelease(ctx, "", testReleaseUrl)
	suite.assert.NoError(err)

	invalidVersion := "1.1.10"
	resultVer, err = getRelease(ctx, invalidVersion, testReleaseUrl)
	suite.assert.Error(err)
}

func (suite *updateTestSuite) TestUpdateAdminRightsPromptLinuxDefault() {
	if runtime.GOOS != "linux" {
		return
	}
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "update")
	suite.assert.Error(err)
	suite.assert.Equal(".deb and .rpm requires elevated privileges", err.Error())
}

func (suite *updateTestSuite) TestUpdateAdminRightsPromptLinux() {
	if runtime.GOOS != "linux" {
		return
	}
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "update", "--package=deb")
	suite.assert.Error(err)
	suite.assert.Equal(".deb and .rpm requires elevated privileges", err.Error())
}

func (suite *updateTestSuite) TestUpdateWithOutputDebLinux() {
	if runtime.GOOS != "linux" {
		return
	}
	defer suite.cleanupTest()

	outputFile, err := os.CreateTemp("", "update-file*")
	suite.assert.NoError(err)

	_, err = executeCommandC(
		rootCmd,
		"update",
		"--package=deb",
		fmt.Sprintf("--output=%s", outputFile.Name()),
	)
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

	_, err = executeCommandC(
		rootCmd,
		"update",
		"--package=rpm",
		fmt.Sprintf("--output=%s", outputFile.Name()),
	)
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

	_, err = executeCommandC(
		rootCmd,
		"update",
		"--package=tar",
		fmt.Sprintf("--output=%s", outputFile.Name()),
	)
	suite.assert.NoError(err)

	os.Remove(outputFile.Name())
}

func (suite *updateTestSuite) TestInvalidOptionsLinux() {
	if runtime.GOOS != "Linux" {
		return
	}
	defer suite.cleanupTest()

	outputFile, err := os.CreateTemp("", "update-file*")
	suite.assert.NoError(err)

	_, err = executeCommandC(
		rootCmd,
		"update",
		"--package=ede",
		fmt.Sprintf("--output=%s", outputFile.Name()),
	)
	suite.assert.Error(err)

	_, err = executeCommandC(
		rootCmd,
		"update",
		"--package=zip",
		fmt.Sprintf("--output=%s", outputFile.Name()),
	)
	suite.assert.Error(err)

	os.Remove(outputFile.Name())
}

func (suite *updateTestSuite) TestUpdateWithOutputZipWindows() {
	if runtime.GOOS != "windows" {
		return
	}
	defer suite.cleanupTest()

	outputFile, err := os.CreateTemp("", "update-file*")
	suite.assert.NoError(err)

	_, err = executeCommandC(
		rootCmd,
		"update",
		"--package=zip",
		fmt.Sprintf("--output=%s", outputFile.Name()),
	)
	suite.assert.NoError(err)

	os.Remove(outputFile.Name())
}

func (suite *updateTestSuite) TestUpdateWithOutputExeWindows() {
	if runtime.GOOS != "windows" {
		return
	}
	defer suite.cleanupTest()

	outputFile, err := os.CreateTemp("", "update-file*")
	suite.assert.NoError(err)

	_, err = executeCommandC(
		rootCmd,
		"update",
		"--package=exe",
		fmt.Sprintf("--output=%s", outputFile.Name()),
	)
	suite.assert.NoError(err)

	os.Remove(outputFile.Name())
}

func (suite *updateTestSuite) TestInvalidOptionsWindows() {
	if runtime.GOOS != "windows" {
		return
	}
	defer suite.cleanupTest()

	outputFile, err := os.CreateTemp("", "update-file*")
	suite.assert.NoError(err)

	_, err = executeCommandC(
		rootCmd,
		"update",
		"--package=tar",
		fmt.Sprintf("--output=%s", outputFile.Name()),
	)
	suite.assert.Error(err)

	_, err = executeCommandC(
		rootCmd,
		"update",
		"--package=deb",
		fmt.Sprintf("--output=%s", outputFile.Name()),
	)
	suite.assert.Error(err)

	_, err = executeCommandC(
		rootCmd,
		"update",
		"--package=rpm",
		fmt.Sprintf("--output=%s", outputFile.Name()),
	)
	suite.assert.Error(err)

	os.Remove(outputFile.Name())
}

func TestUpdateCommand(t *testing.T) {
	suite.Run(t, new(updateTestSuite))
}
