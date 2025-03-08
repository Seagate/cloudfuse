//go:build !authtest
// +build !authtest

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates

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

package s3storage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type s3wrapperTestSuite struct {
	suite.Suite
	assert *assert.Assertions
	client *Client
	config string
}

func (s *s3wrapperTestSuite) SetupTest() {
	// Logging config
	cfg := common.LogConfig{
		FilePath:    "./logfile.txt",
		MaxFileSize: 10,
		FileCount:   10,
		Level:       common.ELogLevel.LOG_DEBUG(),
	}
	_ = log.SetDefaultLogger("base", cfg)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Unable to get home directory")
		os.Exit(1)
	}
	cfgFile, err := os.Open(homeDir + "/s3test.json")
	if err != nil {
		fmt.Println("Unable to open config file")
		os.Exit(1)
	}

	cfgData, _ := io.ReadAll(cfgFile)
	err = json.Unmarshal(cfgData, &storageTestConfigurationParameters)
	if err != nil {
		fmt.Println("Failed to parse the config file")
		os.Exit(1)
	}

	cfgFile.Close()
	s.setupTestHelper("")
}

func (s *s3wrapperTestSuite) setupTestHelper(configuration string) {
	if configuration == "" {
		configuration = fmt.Sprintf(
			"s3storage:\n  bucket-name: %s\n  key-id: %s\n  secret-key: %s\n  region: %s\n  profile: %s\n  endpoint: %s",
			storageTestConfigurationParameters.BucketName, storageTestConfigurationParameters.KeyID,
			storageTestConfigurationParameters.SecretKey, storageTestConfigurationParameters.Region,
			storageTestConfigurationParameters.Profile, storageTestConfigurationParameters.Endpoint)
	}
	s.config = configuration

	s.assert = assert.New(s.T())
	s.client, _ = newTestClient(configuration)
}

func (s *s3wrapperTestSuite) TestGetFileSymlink() {
	fileName := "test" + symlinkStr
	expectedName := "test"
	newName, isSymLink := s.client.getFile(fileName)
	s.assert.True(isSymLink)
	s.assert.Equal(expectedName, newName)
}

func (s *s3wrapperTestSuite) TestGetFileSymlinkDisabled() {
	s.client.Config.disableSymlink = true
	fileName := "test" + symlinkStr
	newName, isSymLink := s.client.getFile(fileName)
	s.assert.False(isSymLink)
	s.assert.Equal(fileName, newName)
}

func (s *s3wrapperTestSuite) TestGetKeySymlink() {
	fileName := "test"
	expectedName := "test" + symlinkStr
	isSymLink := true
	newName := s.client.getKey(fileName, isSymLink, false)
	s.assert.Equal(expectedName, newName)
}

func (s *s3wrapperTestSuite) TestGetKeySymlinkDisabled() {
	s.client.Config.disableSymlink = true
	fileName := "test"
	expectedName := "test" + symlinkStr
	isSymLink := true
	newName := s.client.getKey(fileName, isSymLink, false)
	s.assert.Equal(expectedName, newName)
}

func (s *s3wrapperTestSuite) TestGetFileWindowsNameConvert() {
	// Skip test if not on Windows
	if runtime.GOOS != "windows" {
		return
	}
	s.client.Config.restrictedCharsWin = true
	fileName := "test\"*:<>?|"
	expectedName := "test＂＊：＜＞？｜"
	newName, isSymLink := s.client.getFile(fileName)
	s.assert.False(isSymLink)
	s.assert.Equal(expectedName, newName)
}

func (s *s3wrapperTestSuite) TestGetKeyWindowsNameConvert() {
	// Skip test if not on Windows
	if runtime.GOOS != "windows" {
		return
	}
	s.client.Config.restrictedCharsWin = true
	fileName := "test＂＊：＜＞？｜"
	expectedName := "test\"*:<>?|"
	isSymLink := false
	newName := s.client.getKey(fileName, isSymLink, false)
	s.assert.Equal(expectedName, newName)
}

func TestS3WrapperTestSuite(t *testing.T) {
	suite.Run(t, new(s3wrapperTestSuite))
}
