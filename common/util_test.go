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
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

type utilTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (s *utilTestSuite) SetupTest() {
	s.assert = assert.New(s.T())
}

func TestUtil(t *testing.T) {
	suite.Run(t, new(utilTestSuite))
}

func (s *utilTestSuite) TestIsMountActiveNoMount() {
	// Only test on Linux
	if runtime.GOOS == "windows" {
		return
	}
	var out bytes.Buffer
	cmd := exec.Command("../cloudfuse", "unmount", "all")
	cmd.Stdout = &out
	err := cmd.Run()
	s.assert.NoError(err)
	cmd = exec.Command("pidof", "cloudfuse")
	cmd.Stdout = &out
	err = cmd.Run()
	s.assert.Equal("exit status 1", err.Error())
	res, err := IsMountActive("/mnt/cloudfuse")
	s.assert.NoError(err)
	s.assert.False(res)
}

// TODO: Fix broken test
// func (suite *utilTestSuite) TestIsMountActiveTwoMounts() {
// 	var out bytes.Buffer

// 	// Define the file name and the content you want to write
// 	fileName := "config.yaml"

// 	lbpath := filepath.Join(home_dir, "lbpath")
// 	_ = os.MkdirAll(lbpath, 0777)
// 	defer os.RemoveAll(lbpath)

// 	content := "components:\n" +
// 		"  - libfuse\n" +
// 		"  - loopbackfs\n\n" +
// 		"loopbackfs:\n" +
// 		"  path: " + lbpath + "\n\n"

// 	mntdir := filepath.Join(home_dir, "mountdir")
// 	_ = os.MkdirAll(mntdir, 0777)
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
	_ = os.MkdirAll(dir, 0777)
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
	_, _ = rand.Read(key)

	encryptedPassphrase := memguard.NewEnclave(key)

	data := make([]byte, 1024)
	_, _ = rand.Read(data)

	_, err := DecryptData(data, encryptedPassphrase)
	suite.assert.Error(err)
}

// func (suite *typesTestSuite) TestEncryptBadKeyTooLong() {
// 	// Generate a random key
// 	key := make([]byte, 36)
// 	encodedKey := make([]byte, 48)
// 	_, _ = rand.Read(key)
// 	base64.StdEncoding.Encode(encodedKey, key)

// 	encryptedPassphrase := memguard.NewEnclave(encodedKey)

// 	data := make([]byte, 1024)
// 	_, _ = rand.Read(data)

// 	_, err := EncryptData(data, encryptedPassphrase)
// 	suite.assert.Error(err)
// }

func (suite *typesTestSuite) TestDecryptBadKeyTooLong() {
	// Generate a random key
	key := make([]byte, 36)
	encodedKey := make([]byte, 48)
	_, _ = rand.Read(key)
	base64.StdEncoding.Encode(encodedKey, key)

	encryptedPassphrase := memguard.NewEnclave(encodedKey)

	data := make([]byte, 1024)
	_, _ = rand.Read(data)

	_, err := DecryptData(data, encryptedPassphrase)
	suite.assert.Error(err)
}

// TODO: Fix flaky tests
// func (suite *typesTestSuite) TestEncryptDecrypt1() {
// 	// Generate a random key
// 	key := make([]byte, 16)
// 	encodedKey := make([]byte, 24)
// 	_, _ = rand.Read(key)
// 	base64.StdEncoding.Encode(encodedKey, key)

// 	encryptedPassphrase := memguard.NewEnclave(encodedKey)

// 	data := make([]byte, 1024)
// 	_, _ = rand.Read(data)

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
// 	_, _ = rand.Read(key)
// 	base64.StdEncoding.Encode(encodedKey, key)

// 	encryptedPassphrase := memguard.NewEnclave(encodedKey)

// 	data := make([]byte, 1024)
// 	_, _ = rand.Read(data)

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
// 	_, _ = rand.Read(key)
// 	base64.StdEncoding.Encode(encodedKey, key)

// 	encryptedPassphrase := memguard.NewEnclave(encodedKey)

// 	data := make([]byte, 1024)
// 	_, _ = rand.Read(data)

// 	cipher, err := EncryptData(data, encryptedPassphrase)
// 	suite.assert.NoError(err)

// 	d, err := DecryptData(cipher, encryptedPassphrase)
// 	suite.assert.NoError(err)
// 	suite.assert.EqualValues(data, d)
// }

func (suite *typesTestSuite) TestEncryptDecrypt4() {
	// Generate a random key
	key := make([]byte, 32)
	_, _ = rand.Read(key)

	encryptedPassphrase := memguard.NewEnclave(key)

	data := make([]byte, 1024)
	_, _ = rand.Read(data)

	cipher, err := EncryptData(data, encryptedPassphrase)
	suite.assert.NoError(err)

	d, err := DecryptData(cipher, encryptedPassphrase)
	suite.assert.NoError(err)
	suite.assert.Equal(data, d)
}

func (suite *typesTestSuite) TestEncryptDecrypt5() {
	// Generate a random key
	key := make([]byte, 64)
	_, _ = rand.Read(key)

	encryptedPassphrase := memguard.NewEnclave(key)

	data := make([]byte, 1024)
	_, _ = rand.Read(data)

	cipher, err := EncryptData(data, encryptedPassphrase)
	suite.assert.NoError(err)

	d, err := DecryptData(cipher, encryptedPassphrase)
	suite.assert.NoError(err)
	suite.assert.Equal(data, d)
}

func (s *utilTestSuite) TestMonitorCfs() {
	monitor := MonitorCfs()
	s.assert.False(monitor)
}

func (s *utilTestSuite) TestExpandPath() {
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
	s.assert.NotEqual(expandedPath, path)
	s.assert.Contains(expandedPath, path[2:])
	s.assert.Contains(expandedPath, homeDir)

	path = "$HOME/a/b/c/d"
	expandedPath = ExpandPath(path)
	s.assert.NotEqual(expandedPath, path)
	s.assert.Contains(expandedPath, path[5:])
	s.assert.Contains(expandedPath, homeDir)

	path = "/a/b/c/d"
	expandedPath = ExpandPath(path)
	if runtime.GOOS != "windows" {
		s.assert.Equal(expandedPath, path)
	}

	path = "./a"
	expandedPath = ExpandPath(path)
	s.assert.NotEqual(expandedPath, path)
	s.assert.Contains(expandedPath, pwd)

	path = "./a/../a/b/c/d/../../../a/b/c/d/.././a"
	expandedPath = ExpandPath(path)
	s.assert.NotEqual(expandedPath, path)
	s.assert.Contains(expandedPath, pwd)

	path = "~/a/../$HOME/a/b/c/d/../../../a/b/c/d/.././a"
	expandedPath = ExpandPath(path)
	s.assert.NotEqual(expandedPath, path)
	s.assert.Contains(expandedPath, homeDir)

	path = "$HOME/a/b/c/d/../../../a/b/c/d/.././a"
	expandedPath = ExpandPath(path)
	s.assert.NotEqual(expandedPath, path)
	s.assert.Contains(expandedPath, homeDir)

	path = "/$HOME/a/b/c/d/../../../a/b/c/d/.././a"
	expandedPath = ExpandPath(path)
	s.assert.NotEqual(expandedPath, path)
	s.assert.Contains(expandedPath, homeDir)

	path = ""
	expandedPath = ExpandPath(path)
	s.assert.Equal(expandedPath, path)

	path = "$HOME/.cloudfuse/config_$web.yaml"
	expandedPath = ExpandPath(path)
	s.assert.NotEqual(expandedPath, path)
	s.assert.Contains(path, "$web")

	path = "$HOME/.cloudfuse/$web"
	expandedPath = ExpandPath(path)
	s.assert.NotEqual(expandedPath, path)
	s.assert.Contains(path, "$web")
}

func (s *utilTestSuite) TestExpandPathDriveLetter() {
	path := "D:"
	expandedPath := ExpandPath(path)
	s.assert.Equal(path, expandedPath)

	path = "x:"
	expandedPath = ExpandPath(path)
	s.assert.Equal(path, expandedPath)
}

func (s *utilTestSuite) TestIsDriveLetter() {
	path := "D:"
	match := IsDriveLetter(path)
	s.assert.True(match)

	path = "x:"
	match = IsDriveLetter(path)
	s.assert.True(match)

	path = "D"
	match = IsDriveLetter(path)
	s.assert.False(match)

	path = "C/folder"
	match = IsDriveLetter(path)
	s.assert.False(match)

	path = "C:\\Users"
	match = IsDriveLetter(path)
	s.assert.False(match)
}

func (s *utilTestSuite) TestGetUSage() {
	pwd, err := os.Getwd()
	if err != nil {
		return
	}

	dirName := filepath.Join(pwd, "util_test")
	err = os.Mkdir(dirName, 0777)
	s.assert.NoError(err)

	data := make([]byte, 1024*1024)
	err = os.WriteFile(dirName+"/1.txt", data, 0777)
	s.assert.NoError(err)

	err = os.WriteFile(dirName+"/2.txt", data, 0777)
	s.assert.NoError(err)

	usage, err := GetUsage(dirName)
	s.assert.NoError(err)
	s.assert.GreaterOrEqual(int(usage), 2)
	s.assert.LessOrEqual(int(usage), 4)

	_ = os.RemoveAll(dirName)
}

func (s *utilTestSuite) TestGetDiskUsage() {
	pwd, err := os.Getwd()
	if err != nil {
		return
	}

	dirName := filepath.Join(pwd, "util_test", "a", "b", "c")
	err = os.MkdirAll(dirName, 0777)
	s.assert.NoError(err)

	usage, usagePercent, err := GetDiskUsageFromStatfs(dirName)
	s.assert.NoError(err)
	s.assert.NotEqual(0, usage)
	s.assert.NotEqual(0, usagePercent)
	s.assert.NotEqual(100, usagePercent)
	_ = os.RemoveAll(filepath.Join(pwd, "util_test"))
}

func (s *utilTestSuite) TestDirectoryCleanup() {
	dirName := "./TestDirectoryCleanup"

	// Directory does not exists
	exists := DirectoryExists(dirName)
	s.assert.False(exists)

	err := TempCacheCleanup(dirName)
	s.assert.NoError(err)

	// Directory exists but is empty
	_ = os.MkdirAll(dirName, 0777)
	exists = DirectoryExists(dirName)
	s.assert.True(exists)

	empty := IsDirectoryEmpty(dirName)
	s.assert.True(empty)

	err = TempCacheCleanup(dirName)
	s.assert.NoError(err)

	// Directory exists and is not empty
	_ = os.MkdirAll(dirName+"/A", 0777)
	exists = DirectoryExists(dirName)
	s.assert.True(exists)

	empty = IsDirectoryEmpty(dirName)
	s.assert.False(empty)

	err = TempCacheCleanup(dirName)
	s.assert.NoError(err)

	_ = os.RemoveAll(dirName)
}

func (s *utilTestSuite) TestWriteToFile() {
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
	s.assert.NoError(err)

	// Check if file exists
	s.assert.FileExists(filePath)

	// Check the content of the file
	data, err := os.ReadFile(filePath)
	s.assert.NoError(err)
	s.assert.Equal(content, string(data))
}

func (s *utilTestSuite) TestCRC64() {
	data := []byte("Hello World")
	crc := GetCRC64(data, len(data))

	data = []byte("Hello World!")
	crc1 := GetCRC64(data, len(data))

	s.assert.NotEqual(crc, crc1)
}

func (s *utilTestSuite) TestGetMD5() {
	assert := assert.New(s.T())

	f, err := os.Create("abc.txt")
	assert.NoError(err)

	_, err = f.Write([]byte(randomString(50)))
	assert.NoError(err)

	f.Close()

	f, err = os.Open("abc.txt")
	assert.NoError(err)

	md5Sum, err := GetMD5(f)
	assert.NoError(err)
	assert.NotZero(md5Sum)

	f.Close()
	os.Remove("abc.txt")
}

func (s *utilTestSuite) TestComponentExists() {
	components := []string{
		"component1",
		"component2",
		"component3",
	}

	exists := ComponentInPipeline(components, "component1")
	s.True(exists)

	exists = ComponentInPipeline(components, "component4")
	s.False(exists)

}

func (s *utilTestSuite) TestValidatePipeline() {
	err := ValidatePipeline([]string{"libfuse", "file_cache", "block_cache", "azstorage"})
	s.Error(err)

	err = ValidatePipeline([]string{"libfuse", "file_cache", "xload", "azstorage"})
	s.Error(err)

	err = ValidatePipeline([]string{"libfuse", "block_cache", "xload", "azstorage"})
	s.Error(err)

	err = ValidatePipeline([]string{"libfuse", "file_cache", "block_cache", "xload", "azstorage"})
	s.Error(err)

	err = ValidatePipeline([]string{"libfuse", "file_cache", "azstorage"})
	s.NoError(err)

	err = ValidatePipeline([]string{"libfuse", "block_cache", "azstorage"})
	s.NoError(err)

	err = ValidatePipeline([]string{"libfuse", "xload", "attr_cache", "azstorage"})
	s.NoError(err)
}

func (s *utilTestSuite) TestUpdatePipeline() {
	pipeline := UpdatePipeline([]string{"libfuse", "file_cache", "azstorage"}, "xload")
	s.NotNil(pipeline)
	s.False(ComponentInPipeline(pipeline, "file_cache"))
	s.Equal([]string{"libfuse", "xload", "azstorage"}, pipeline)

	pipeline = UpdatePipeline([]string{"libfuse", "block_cache", "azstorage"}, "xload")
	s.NotNil(pipeline)
	s.False(ComponentInPipeline(pipeline, "block_cache"))
	s.Equal([]string{"libfuse", "xload", "azstorage"}, pipeline)

	pipeline = UpdatePipeline([]string{"libfuse", "file_cache", "azstorage"}, "block_cache")
	s.NotNil(pipeline)
	s.False(ComponentInPipeline(pipeline, "file_cache"))
	s.Equal([]string{"libfuse", "block_cache", "azstorage"}, pipeline)

	pipeline = UpdatePipeline([]string{"libfuse", "xload", "azstorage"}, "block_cache")
	s.NotNil(pipeline)
	s.False(ComponentInPipeline(pipeline, "xload"))
	s.Equal([]string{"libfuse", "block_cache", "azstorage"}, pipeline)

	pipeline = UpdatePipeline([]string{"libfuse", "xload", "azstorage"}, "xload")
	s.NotNil(pipeline)
	s.Equal([]string{"libfuse", "xload", "azstorage"}, pipeline)
}
