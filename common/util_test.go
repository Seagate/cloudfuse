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

package common

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/awnumar/memguard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var home_dir, _ = os.UserHomeDir()

func randomString(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

type utilTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *utilTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

func TestUtil(t *testing.T) {
	suite.Run(t, new(utilTestSuite))
}

func (suite *typesTestSuite) TestDirectoryExists() {
	rand := randomString(8)
	dir := filepath.Join(home_dir, "dir"+rand)
	os.MkdirAll(dir, 0777)
	defer os.RemoveAll(dir)

	exists := DirectoryExists(dir)
	suite.assert.True(exists)
}

func (suite *typesTestSuite) TestDirectoryDoesNotExist() {
	rand := randomString(8)
	dir := filepath.Join(home_dir, "dir"+rand)

	exists := DirectoryExists(dir)
	suite.assert.False(exists)
}

func (suite *typesTestSuite) TestEncryptBadKeyTooSmall() {
	// Generate a random key
	key := make([]byte, 20)
	encodedKey := make([]byte, 28)
	rand.Read(key)
	base64.StdEncoding.Encode(encodedKey, key)

	encryptedPassphrase := memguard.NewEnclave(encodedKey)

	data := make([]byte, 1024)
	rand.Read(data)

	_, err := EncryptData(data, encryptedPassphrase)
	suite.assert.Error(err)
}

func (suite *typesTestSuite) TestDecryptBadKeyTooSmall() {
	// Generate a random key
	key := make([]byte, 20)
	encodedKey := make([]byte, 28)
	rand.Read(key)
	base64.StdEncoding.Encode(encodedKey, key)

	encryptedPassphrase := memguard.NewEnclave(encodedKey)

	data := make([]byte, 1024)
	rand.Read(data)

	_, err := DecryptData(data, encryptedPassphrase)
	suite.assert.Error(err)
}

func (suite *typesTestSuite) TestEncryptBadKeyTooLong() {
	// Generate a random key
	key := make([]byte, 36)
	encodedKey := make([]byte, 48)
	rand.Read(key)
	base64.StdEncoding.Encode(encodedKey, key)

	encryptedPassphrase := memguard.NewEnclave(encodedKey)

	data := make([]byte, 1024)
	rand.Read(data)

	_, err := EncryptData(data, encryptedPassphrase)
	suite.assert.Error(err)
}

func (suite *typesTestSuite) TestDecryptBadKeyTooLong() {
	// Generate a random key
	key := make([]byte, 36)
	encodedKey := make([]byte, 48)
	rand.Read(key)
	base64.StdEncoding.Encode(encodedKey, key)

	encryptedPassphrase := memguard.NewEnclave(encodedKey)

	data := make([]byte, 1024)
	rand.Read(data)

	_, err := DecryptData(data, encryptedPassphrase)
	suite.assert.Error(err)
}

func (suite *typesTestSuite) TestEncryptDecrypt16() {
	// Generate a random key
	key := make([]byte, 16)
	encodedKey := make([]byte, 24)
	rand.Read(key)
	base64.StdEncoding.Encode(encodedKey, key)

	encryptedPassphrase := memguard.NewEnclave(encodedKey)

	data := make([]byte, 1024)
	rand.Read(data)

	cipher, err := EncryptData(data, encryptedPassphrase)
	suite.assert.NoError(err)

	d, err := DecryptData(cipher, encryptedPassphrase)
	suite.assert.NoError(err)
	suite.assert.EqualValues(data, d)
}

func (suite *typesTestSuite) TestEncryptDecrypt24() {
	// Generate a random key
	key := make([]byte, 24)
	encodedKey := make([]byte, 32)
	rand.Read(key)
	base64.StdEncoding.Encode(encodedKey, key)

	encryptedPassphrase := memguard.NewEnclave(encodedKey)

	data := make([]byte, 1024)
	rand.Read(data)

	cipher, err := EncryptData(data, encryptedPassphrase)
	suite.assert.NoError(err)

	d, err := DecryptData(cipher, encryptedPassphrase)
	suite.assert.NoError(err)
	suite.assert.EqualValues(data, d)
}

func (suite *typesTestSuite) TestEncryptDecrypt32() {
	// Generate a random key
	key := make([]byte, 32)
	encodedKey := make([]byte, 44)
	rand.Read(key)
	base64.StdEncoding.Encode(encodedKey, key)

	encryptedPassphrase := memguard.NewEnclave(encodedKey)

	data := make([]byte, 1024)
	rand.Read(data)

	cipher, err := EncryptData(data, encryptedPassphrase)
	suite.assert.NoError(err)

	d, err := DecryptData(cipher, encryptedPassphrase)
	suite.assert.NoError(err)
	suite.assert.EqualValues(data, d)
}

func (suite *utilTestSuite) TestMonitorCfs() {
	monitor := MonitorCfs()
	suite.assert.False(monitor)
}

func (suite *utilTestSuite) TestExpandPath() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	homeDir = JoinUnixFilepath(homeDir)

	pwd, err := os.Getwd()
	if err != nil {
		return
	}
	pwd = JoinUnixFilepath(pwd)

	path := "~/a/b/c/d"
	expandedPath := ExpandPath(path)
	suite.assert.NotEqual(expandedPath, path)
	suite.assert.Contains(expandedPath, path[2:])
	suite.assert.Contains(expandedPath, homeDir)

	path = "$HOME/a/b/c/d"
	expandedPath = ExpandPath(path)
	suite.assert.NotEqual(expandedPath, path)
	suite.assert.Contains(expandedPath, path[5:])
	suite.assert.Contains(expandedPath, homeDir)

	path = "/a/b/c/d"
	expandedPath = ExpandPath(path)
	if runtime.GOOS != "windows" {
		suite.assert.Equal(expandedPath, path)
	}

	path = "./a"
	expandedPath = ExpandPath(path)
	suite.assert.NotEqual(expandedPath, path)
	suite.assert.Contains(expandedPath, pwd)

	path = "./a/../a/b/c/d/../../../a/b/c/d/.././a"
	expandedPath = ExpandPath(path)
	suite.assert.NotEqual(expandedPath, path)
	suite.assert.Contains(expandedPath, pwd)

	path = "~/a/../$HOME/a/b/c/d/../../../a/b/c/d/.././a"
	expandedPath = ExpandPath(path)
	suite.assert.NotEqual(expandedPath, path)
	suite.assert.Contains(expandedPath, homeDir)

	path = "$HOME/a/b/c/d/../../../a/b/c/d/.././a"
	expandedPath = ExpandPath(path)
	suite.assert.NotEqual(expandedPath, path)
	suite.assert.Contains(expandedPath, homeDir)

	path = "/$HOME/a/b/c/d/../../../a/b/c/d/.././a"
	expandedPath = ExpandPath(path)
	suite.assert.NotEqual(expandedPath, path)
	suite.assert.Contains(expandedPath, homeDir)

	path = ""
	expandedPath = ExpandPath(path)
	suite.assert.Equal(expandedPath, path)
}

func (suite *utilTestSuite) TestExpandPathDriveLetter() {
	path := "D:"
	expandedPath := ExpandPath(path)
	suite.assert.Equal(path, expandedPath)

	path = "x:"
	expandedPath = ExpandPath(path)
	suite.assert.Equal(path, expandedPath)
}

func (suite *utilTestSuite) TestIsDriveLetter() {
	path := "D:"
	match := IsDriveLetter(path)
	suite.assert.True(match)

	path = "x:"
	match = IsDriveLetter(path)
	suite.assert.True(match)

	path = "D"
	match = IsDriveLetter(path)
	suite.assert.False(match)

	path = "C/folder"
	match = IsDriveLetter(path)
	suite.assert.False(match)

	path = "C:\\Users"
	match = IsDriveLetter(path)
	suite.assert.False(match)
}

func (suite *utilTestSuite) TestGetUSage() {
	pwd, err := os.Getwd()
	if err != nil {
		return
	}

	dirName := filepath.Join(pwd, "util_test")
	err = os.Mkdir(dirName, 0777)
	suite.assert.NoError(err)

	data := make([]byte, 1024*1024)
	err = os.WriteFile(dirName+"/1.txt", data, 0777)
	suite.assert.NoError(err)

	err = os.WriteFile(dirName+"/2.txt", data, 0777)
	suite.assert.NoError(err)

	usage, err := GetUsage(dirName)
	suite.assert.NoError(err)
	suite.assert.GreaterOrEqual(int(usage), 2)
	suite.assert.LessOrEqual(int(usage), 4)

	_ = os.RemoveAll(dirName)
}

func (suite *utilTestSuite) TestGetDiskUsage() {
	pwd, err := os.Getwd()
	if err != nil {
		return
	}

	dirName := filepath.Join(pwd, "util_test", "a", "b", "c")
	err = os.MkdirAll(dirName, 0777)
	suite.assert.NoError(err)

	usage, usagePercent, err := GetDiskUsageFromStatfs(dirName)
	suite.assert.NoError(err)
	suite.assert.NotEqual(0, usage)
	suite.assert.NotEqual(0, usagePercent)
	suite.assert.NotEqual(100, usagePercent)
	_ = os.RemoveAll(filepath.Join(pwd, "util_test"))
}

func (suite *utilTestSuite) TestDirectoryCleanup() {
	dirName := "./TestDirectoryCleanup"

	// Directory does not exists
	exists := DirectoryExists(dirName)
	suite.assert.False(exists)

	err := TempCacheCleanup(dirName)
	suite.assert.NoError(err)

	// Directory exists but is empty
	_ = os.MkdirAll(dirName, 0777)
	exists = DirectoryExists(dirName)
	suite.assert.True(exists)

	empty := IsDirectoryEmpty(dirName)
	suite.assert.True(empty)

	err = TempCacheCleanup(dirName)
	suite.assert.NoError(err)

	// Directory exists and is not empty
	_ = os.MkdirAll(dirName+"/A", 0777)
	exists = DirectoryExists(dirName)
	suite.assert.True(exists)

	empty = IsDirectoryEmpty(dirName)
	suite.assert.False(empty)

	err = TempCacheCleanup(dirName)
	suite.assert.NoError(err)

	os.Remove(dirName)
}
