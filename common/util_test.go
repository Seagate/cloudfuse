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

package common

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
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

func (suite *utilTestSuite) TestIsMountActiveNoMount() {
	// Only test on Linux
	if runtime.GOOS == "windows" {
		return
	}
	var out bytes.Buffer
	cmd := exec.Command("../cloudfuse", "unmount", "all")
	cmd.Stdout = &out
	err := cmd.Run()
	suite.assert.Nil(err)
	cmd = exec.Command("pidof", "cloudfuse")
	cmd.Stdout = &out
	err = cmd.Run()
	suite.assert.Equal("exit status 1", err.Error())
	res, err := IsMountActive("/mnt/cloudfuse")
	suite.assert.Nil(err)
	suite.assert.False(res)
}

// TODO: Fix broken test
// func (suite *utilTestSuite) TestIsMountActiveTwoMounts() {
// 	var out bytes.Buffer

// 	// Define the file name and the content you want to write
// 	fileName := "config.yaml"

// 	lbpath := filepath.Join(home_dir, "lbpath")
// 	os.MkdirAll(lbpath, 0777)
// 	defer os.RemoveAll(lbpath)

// 	content := "components:\n" +
// 		"  - libfuse\n" +
// 		"  - loopbackfs\n\n" +
// 		"loopbackfs:\n" +
// 		"  path: " + lbpath + "\n\n"

// 	mntdir := filepath.Join(home_dir, "mountdir")
// 	os.MkdirAll(mntdir, 0777)
// 	defer os.RemoveAll(mntdir)

// 	dir, err := os.Getwd()
// 	suite.assert.Nil(err)
// 	configFile := filepath.Join(dir, "config.yaml")
// 	// Create or open the file. If it doesn't exist, it will be created.
// 	file, err := os.Create(fileName)
// 	suite.assert.Nil(err)
// 	defer file.Close() // Ensure the file is closed after we're done

// 	// Write the content to the file
// 	_, err = file.WriteString(content)
// 	suite.assert.Nil(err)

// 	err = os.Chdir("..")
// 	suite.assert.Nil(err)

// 	dir, err = os.Getwd()
// 	suite.assert.Nil(err)
// 	binary := filepath.Join(dir, "cloudfuse")
// 	cmd := exec.Command(binary, mntdir, "--config-file", configFile)
// 	cmd.Stdout = &out
// 	err = cmd.Run()
// 	suite.assert.Nil(err)

// 	res, err := IsMountActive(mntdir)
// 	suite.assert.Nil(err)
// 	suite.assert.True(res)

// 	res, err = IsMountActive("/mnt/cloudfuse")
// 	suite.assert.Nil(err)
// 	suite.assert.False(res)

// 	cmd = exec.Command(binary, "unmount", mntdir)
// 	cmd.Stdout = &out
// 	err = cmd.Run()
// 	suite.assert.Nil(err)
// }

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

func (suite *typesTestSuite) TestDecryptBadKey() {
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

// func (suite *typesTestSuite) TestEncryptBadKeyTooLong() {
// 	// Generate a random key
// 	key := make([]byte, 36)
// 	encodedKey := make([]byte, 48)
// 	rand.Read(key)
// 	base64.StdEncoding.Encode(encodedKey, key)

// 	encryptedPassphrase := memguard.NewEnclave(encodedKey)

// 	data := make([]byte, 1024)
// 	rand.Read(data)

// 	_, err := EncryptData(data, encryptedPassphrase)
// 	suite.assert.Error(err)
// }

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

// TODO: Fix flaky tests
// func (suite *typesTestSuite) TestEncryptDecrypt1() {
// 	// Generate a random key
// 	key := make([]byte, 16)
// 	encodedKey := make([]byte, 24)
// 	rand.Read(key)
// 	base64.StdEncoding.Encode(encodedKey, key)

// 	encryptedPassphrase := memguard.NewEnclave(encodedKey)

// 	data := make([]byte, 1024)
// 	rand.Read(data)

// 	cipher, err := EncryptData(data, encryptedPassphrase)
// 	suite.assert.NoError(err)

// 	d, err := DecryptData(cipher, encryptedPassphrase)
// 	suite.assert.NoError(err)
// 	suite.assert.EqualValues(data, d)
// }

// func (suite *typesTestSuite) TestEncryptDecrypt2() {
// 	// Generate a random key
// 	key := make([]byte, 24)
// 	encodedKey := make([]byte, 32)
// 	rand.Read(key)
// 	base64.StdEncoding.Encode(encodedKey, key)

// 	encryptedPassphrase := memguard.NewEnclave(encodedKey)

// 	data := make([]byte, 1024)
// 	rand.Read(data)

// 	cipher, err := EncryptData(data, encryptedPassphrase)
// 	suite.assert.NoError(err)

// 	d, err := DecryptData(cipher, encryptedPassphrase)
// 	suite.assert.NoError(err)
// 	suite.assert.EqualValues(data, d)
// }

// func (suite *typesTestSuite) TestEncryptDecrypt3() {
// 	// Generate a random key
// 	key := make([]byte, 32)
// 	encodedKey := make([]byte, 44)
// 	rand.Read(key)
// 	base64.StdEncoding.Encode(encodedKey, key)

// 	encryptedPassphrase := memguard.NewEnclave(encodedKey)

// 	data := make([]byte, 1024)
// 	rand.Read(data)

// 	cipher, err := EncryptData(data, encryptedPassphrase)
// 	suite.assert.NoError(err)

// 	d, err := DecryptData(cipher, encryptedPassphrase)
// 	suite.assert.NoError(err)
// 	suite.assert.EqualValues(data, d)
// }

func (suite *typesTestSuite) TestEncryptDecrypt4() {
	// Generate a random key
	key := make([]byte, 32)
	rand.Read(key)

	encryptedPassphrase := memguard.NewEnclave(key)

	data := make([]byte, 1024)
	rand.Read(data)

	cipher, err := EncryptData(data, encryptedPassphrase)
	suite.assert.NoError(err)

	d, err := DecryptData(cipher, encryptedPassphrase)
	suite.assert.NoError(err)
	suite.assert.EqualValues(data, d)
}

func (suite *typesTestSuite) TestEncryptDecrypt5() {
	// Generate a random key
	key := make([]byte, 64)
	rand.Read(key)

	encryptedPassphrase := memguard.NewEnclave(key)

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

	_ = os.RemoveAll(dirName)
}

func (suite *utilTestSuite) TestWriteToFile() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error getting home directory:", err)
		return
	}
	filePath := fmt.Sprintf("test_%s.txt", randomString(8))
	content := "Hello World"
	filePath = filepath.Join(homeDir, filePath)

	defer os.Remove(filePath)

	err = WriteToFile(filePath, content, WriteToFileOptions{})
	suite.assert.NoError(err)

	// Check if file exists
	suite.assert.FileExists(filePath)

	// Check the content of the file
	data, err := os.ReadFile(filePath)
	suite.assert.NoError(err)
	suite.assert.Equal(content, string(data))
}
