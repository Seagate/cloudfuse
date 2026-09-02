/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2026 Seagate Technology LLC and/or its Affiliates
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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

	mockServer                *httptest.Server
	originalReleaseAPIBaseURL string
}

func (suite *updateTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())

	options = mountOptions{}
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}

	suite.originalReleaseAPIBaseURL = releaseAPIBaseURL
	releaseVersion := "9.9.9"

	linuxDebAsset := fmt.Sprintf(
		"cloudfuse_%s_%s_%s_%s.deb",
		releaseVersion,
		runtime.GOOS,
		runtime.GOARCH,
		common.FuseVersion,
	)
	linuxRpmAsset := fmt.Sprintf(
		"cloudfuse_%s_%s_%s_%s.rpm",
		releaseVersion,
		runtime.GOOS,
		runtime.GOARCH,
		common.FuseVersion,
	)
	linuxTarAsset := fmt.Sprintf(
		"cloudfuse_%s_%s_%s_%s.tar.gz",
		releaseVersion,
		runtime.GOOS,
		runtime.GOARCH,
		common.FuseVersion,
	)
	windowsZipAsset := fmt.Sprintf(
		"cloudfuse_%s_%s_%s.zip",
		releaseVersion,
		runtime.GOOS,
		runtime.GOARCH,
	)
	windowsExeAsset := fmt.Sprintf(
		"cloudfuse_%s_%s_%s.exe",
		releaseVersion,
		runtime.GOOS,
		runtime.GOARCH,
	)

	assetBodies := map[string][]byte{
		linuxDebAsset:   []byte("mock deb content"),
		linuxRpmAsset:   []byte("mock rpm content"),
		linuxTarAsset:   []byte("mock tar content"),
		windowsZipAsset: []byte("mock zip content"),
		windowsExeAsset: []byte("mock exe content"),
	}

	checksumByAsset := make(map[string]string, len(assetBodies))
	for name, body := range assetBodies {
		h := sha256.Sum256(body)
		checksumByAsset[name] = hex.EncodeToString(h[:])
	}

	suite.mockServer = httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			assets := []asset{
				{
					Name:               linuxDebAsset,
					BrowserDownloadURL: suite.mockServer.URL + "/assets/" + linuxDebAsset,
				},
				{
					Name:               linuxRpmAsset,
					BrowserDownloadURL: suite.mockServer.URL + "/assets/" + linuxRpmAsset,
				},
				{
					Name:               linuxTarAsset,
					BrowserDownloadURL: suite.mockServer.URL + "/assets/" + linuxTarAsset,
				},
				{
					Name:               windowsZipAsset,
					BrowserDownloadURL: suite.mockServer.URL + "/assets/" + windowsZipAsset,
				},
				{
					Name:               windowsExeAsset,
					BrowserDownloadURL: suite.mockServer.URL + "/assets/" + windowsExeAsset,
				},
				{
					Name:               "cloudfuse_checksums_sha256.txt",
					BrowserDownloadURL: suite.mockServer.URL + "/checksums/cloudfuse_checksums_sha256.txt",
				},
			}

			switch {
			case r.URL.Path == "/latest":
				_ = json.NewEncoder(w).Encode(GithubApiReleaseData{
					TagName: "v" + releaseVersion,
					Name:    "Cloudfuse v" + releaseVersion,
					Assets:  assets,
				})
			case r.URL.Path == "/tags/v1.8.0":
				_ = json.NewEncoder(w).Encode(GithubApiReleaseData{
					TagName: "v1.8.0",
					Name:    "Cloudfuse v1.8.0",
					Assets:  assets,
				})
			case len(r.URL.Path) > len("/assets/") && r.URL.Path[:len("/assets/")] == "/assets/":
				assetName := r.URL.Path[len("/assets/"):]
				body, found := assetBodies[assetName]
				if !found {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte("asset not found"))
					return
				}
				w.Header().Set("Content-Type", "application/octet-stream")
				_, _ = w.Write(body)
			case r.URL.Path == "/checksums/cloudfuse_checksums_sha256.txt":
				w.Header().Set("Content-Type", "text/plain")
				for name, sum := range checksumByAsset {
					_, _ = fmt.Fprintf(w, "%s  %s\n", sum, name)
				}
			default:
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"message":"Not Found"}`))
			}
		}),
	)

	releaseAPIBaseURL = suite.mockServer.URL

}

func (suite *updateTestSuite) cleanupTest() {
	if suite.mockServer != nil {
		suite.mockServer.Close()
		suite.mockServer = nil
	}
	releaseAPIBaseURL = suite.originalReleaseAPIBaseURL

	resetCLIFlags(*updateCmd)
	resetCLIFlags(*rootCmd)
}

func (suite *updateTestSuite) TestUpdateAdminRightsPromptLinuxDefault() {
	if runtime.GOOS != "linux" {
		return
	}
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "update", "--version=1.8.0")
	suite.assert.Error(err)
	suite.assert.Equal("update failed: .deb and .rpm requires elevated privileges", err.Error())
}

func (suite *updateTestSuite) TestUpdateAdminRightsPromptLinux() {
	if runtime.GOOS != "linux" {
		return
	}
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "update", "--package=deb", "--version=1.8.0")
	suite.assert.Error(err)
	suite.assert.Equal("update failed: .deb and .rpm requires elevated privileges", err.Error())
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
	if runtime.GOOS != "linux" {
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
	// Skip until we have Windows ARM builds
	if runtime.GOOS == "windows" && runtime.GOARCH == "arm64" {
		suite.T().Skip("Skipping test on Windows ARM")
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
	// Skip until we have Windows ARM builds
	if runtime.GOOS == "windows" && runtime.GOARCH == "arm64" {
		suite.T().Skip("Skipping test on Windows ARM")
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
