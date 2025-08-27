//go:build !unittest
// +build !unittest

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

package e2e_tests

import (
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

var fileTestPathPtr string
var fileTestTempPathPtr string
var fileTestAdlsPtr string
var fileTestGitClonePtr string
var fileTestStreamDirectPtr string
var fileTestDistroName string
var fileTestEnableSymlinkADLS string

type fileTestSuite struct {
	suite.Suite
	testPath      string
	adlsTest      bool
	testCachePath string
	minBuff       []byte
	medBuff       []byte
	hugeBuff      []byte
}

func regFileTestFlag(p *string, name string, value string, usage string) {
	if flag.Lookup(name) == nil {
		flag.StringVar(p, name, value, usage)
	}
}

func getFileTestFlag(name string) string {
	return flag.Lookup(name).Value.(flag.Getter).Get().(string)
}

func initFileFlags() {
	fileTestPathPtr = getFileTestFlag("mnt-path")
	fileTestAdlsPtr = getFileTestFlag("adls")
	fileTestTempPathPtr = getFileTestFlag("tmp-path")
	fileTestGitClonePtr = getFileTestFlag("clone")
	fileTestStreamDirectPtr = getFileTestFlag("stream-direct-test")
	fileTestDistroName = getFileTestFlag("distro-name")
	fileTestEnableSymlinkADLS = getFileTestFlag("enable-symlink-adls")
}

func getFileTestDirName(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:n]
}

func (suite *fileTestSuite) fileTestCleanup(toRemove []string) {
	for _, path := range toRemove {
		// don't assert.Nil(err) here, since it's flaky
		err := os.RemoveAll(path)
		if err != nil {
			fmt.Printf("FileTestSuite::fileTestCleanup : Cleanup failed with error %v\n", err)
		}
	}
}

// waitForCondition polls for a condition to be true, failing the test on timeout.
func (suite *fileTestSuite) waitForCondition(timeout time.Duration, interval time.Duration, condition func() (bool, error), msgAndArgs ...interface{}) {
    startTime := time.Now()
    var lastErr error
    for {
        var met bool
        met, lastErr = condition()
        if met {
            return
        }
        if time.Since(startTime) > timeout {
            errMsg := fmt.Sprintf("Timeout waiting for condition: %s", formatMessage(msgAndArgs...))
            if lastErr != nil {
                errMsg = fmt.Sprintf("%s. Last error: %v", errMsg, lastErr)
            }
            suite.FailNow(errMsg) // Use FailNow to stop the current test immediately
            return
        }
        time.Sleep(interval)
    }
}

// // -------------- File Tests -------------------

// # Create file test
func (suite *fileTestSuite) TestFileCreate() {
	fileName := filepath.Join(suite.testPath, "small_write.txt")
	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()

	suite.fileTestCleanup([]string{fileName})
}

func (suite *fileTestSuite) TestOpenFlag_O_TRUNC() {
	fileName := suite.testPath + "/test_on_open"
	buf := "foo"
	tempbuf := make([]byte, 4096)
	srcFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	suite.NoError(err)
	bytesWritten, err := srcFile.Write([]byte(buf))
	suite.Equal(len(buf), bytesWritten)
	suite.NoError(err)
	err = srcFile.Close()
	suite.NoError(err)

	srcFile, err = os.OpenFile(fileName, os.O_WRONLY, 0666)
	suite.NoError(err)
	err = srcFile.Close()
	suite.NoError(err)

	fileInfo, err := os.Stat(fileName)
	suite.Equal(int64(len(buf)), fileInfo.Size())
	suite.NoError(err)

	srcFile, err = os.OpenFile(fileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	suite.NoError(err)
	read, _ := srcFile.Read(tempbuf)
	suite.Equal(0, read)
	err = srcFile.Close()
	suite.NoError(err)

	fileInfo, err = os.Stat(fileName)
	suite.Equal(int64(0), fileInfo.Size())
	suite.NoError(err)

	srcFile, err = os.OpenFile(fileName, os.O_RDONLY, 0666)
	suite.NoError(err)
	read, _ = srcFile.Read(tempbuf)
	suite.Equal(0, read)
}

func (suite *fileTestSuite) TestFileCreateUtf8Char() {
	fileName := filepath.Join(suite.testPath, "भारत.txt")
	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()

	suite.fileTestCleanup([]string{fileName})
}

func (suite *fileTestSuite) TestFileCreatSpclChar() {
	// special characters not supported on Windows
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestFileCreatSpclChar for Windows")
		return
	}
	fmt.Println("Skipping TestFileCreatSpclChar (flaky)")
	// return
	// speclChar := "abcd%23ABCD%34123-._~!$&'()*+,;=!@ΣΑΠΦΩ$भारत.txt"
	// fileName := filepath.Join(suite.testPath, speclChar)

	// srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	// suite.NoError(err)
	// srcFile.Close()
	// time.Sleep(time.Second * 1)

	// suite.FileExists(fileName)

	// files, err := os.ReadDir(suite.testPath)
	// suite.NoError(err)
	// suite.GreaterOrEqual(len(files), 1)

	// found := false
	// for _, file := range files {
	// 	if file.Name() == speclChar {
	// 		found = true
	// 	}
	// }
	// // TODO: why did this come back false occasionally in CI (flaky)
	// suite.True(found)

	// suite.fileTestCleanup([]string{fileName})
}

func (suite *fileTestSuite) TestFileCreateEncodeChar() {
	speclChar := "%282%29+class_history_by_item.log"
	fileName := filepath.Join(suite.testPath, speclChar)

	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()
	
    var statErr error
    suite.waitForCondition(defaultPollTimeout, defaultPollInterval, func() (bool, error) {
		_, statErr = os.Stat(fileName)
        if statErr != nil {
            return false, statErr
        }
        return true, nil
    }, "file %s stat to update with non-zero ModTime", fileName)
    suite.NoError(statErr)

	suite.FileExists(fileName)

	files, err := os.ReadDir(suite.testPath)
	suite.NoError(err)
	suite.GreaterOrEqual(len(files), 1)

	found := false
	for _, file := range files {
		if file.Name() == speclChar {
			found = true
		}
	}
	suite.True(found)

	suite.fileTestCleanup([]string{fileName})
}

func (suite *fileTestSuite) TestFileCreateMultiSpclCharWithinSpclDir() {
	// Some of these characters are not allowed on Windows.
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestFileCreateMultiSpclCharWithinSpclDir on Windows")
		return
	}
	speclChar := "abcd%23ABCD%34123-._~!$&'()*+,;=!@ΣΑΠΦΩ$भारत.txt"
	speclDirName := filepath.Join(suite.testPath, "abc%23%24%25efg-._~!$&'()*+,;=!@ΣΑΠΦΩ$भारत")
	secFile := filepath.Join(
		speclDirName,
		"abcd123~!@#$%^&*()_+=-{}][\":;'?><,.|abcd123~!@#$%^&*()_+=-{}][\":;'?><,.|.txt",
	)
	fileName := filepath.Join(speclDirName, speclChar)

	err := os.Mkdir(speclDirName, 0777)
	suite.NoError(err)

	srcFile, err := os.OpenFile(secFile, os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()

	srcFile, err = os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()
	var statErr error
    suite.waitForCondition(defaultPollTimeout, defaultPollInterval, func() (bool, error) {
		_, statErr = os.Stat(fileName)
        if statErr != nil {
            return false, statErr
        }
        return true, nil
    }, "file %s stat to update with non-zero ModTime", fileName)
    suite.NoError(statErr)

	suite.FileExists(fileName)

	files, err := os.ReadDir(speclDirName)
	suite.NoError(err)
	suite.GreaterOrEqual(len(files), 1)

	found := false
	for _, file := range files {
		if file.Name() == speclChar {
			found = true
		}
	}
	suite.True(found)

	suite.fileTestCleanup([]string{speclDirName})
}

func (suite *fileTestSuite) TestFileCreateLongName() {
	fileName := filepath.Join(
		suite.testPath,
		"Higher Call_ An Incredible True Story of Combat and Chivalry in the War-Torn Skies of World War II, A - Adam Makos & Larry Alexander.epub",
	)
	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()

	suite.fileTestCleanup([]string{fileName})
}

func (suite *fileTestSuite) TestFileCreateSlashName() {
	// Backslashes are not allowed in filenames on Windows.
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestFileCreateSlashName on Windows")
		return
	}

	fileName := filepath.Join(suite.testPath, "abcd\\efg.txt")

	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()

	suite.fileTestCleanup([]string{fileName, filepath.Join(suite.testPath, "abcd")})
}

func (suite *fileTestSuite) TestFileCreateLabel() {
	fileName := filepath.Join(suite.testPath, "chunk_f13c48d4-5c1e-11ea-b41d-000d3afe1867.label")

	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()

	suite.fileTestCleanup([]string{fileName})
}

func (suite *fileTestSuite) TestFileAppend() {
	fileName := filepath.Join(suite.testPath, "append_test.txt")
	initialContent := []byte("Initial content\n")
	appendContent := []byte("Appended content\n")

	// Create and write initial content to the file
	srcFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0777)
	suite.NoError(err)
	_, err = srcFile.Write(initialContent)
	suite.NoError(err)
	srcFile.Close()

	// Open the file with O_APPEND and append new content
	appendFile, err := os.OpenFile(fileName, os.O_APPEND|os.O_WRONLY, 0777)
	suite.NoError(err)
	_, err = appendFile.Write(appendContent)
	suite.NoError(err)
	appendFile.Close()

	// Read the file and verify the content
	data, err := os.ReadFile(fileName)
	suite.NoError(err)
	expectedContent := append(initialContent, appendContent...)
	suite.Equal(expectedContent, data)

	suite.fileTestCleanup([]string{fileName})
}

// # Write a small file
func (suite *fileTestSuite) TestFileWriteSmall() {
	fileName := filepath.Join(suite.testPath, "small_write.txt")
	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()

	err = os.WriteFile(fileName, suite.minBuff, 0777)
	suite.NoError(err)

	suite.fileTestCleanup([]string{fileName})
}

// # Read a small file
func (suite *fileTestSuite) TestFileReadSmall() {
	fileName := filepath.Join(suite.testPath, "small_write.txt")
	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()

	err = os.WriteFile(fileName, suite.minBuff, 0777)
	suite.NoError(err)

	data, err := os.ReadFile(fileName)
	suite.NoError(err)
	suite.Len(data, len(suite.minBuff))

	suite.fileTestCleanup([]string{fileName})
}

// # Create duplicate file
func (suite *fileTestSuite) TestFileCreateDuplicate() {
	fileName := filepath.Join(suite.testPath, "small_write.txt")
	f, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.NoError(err)
	f.Close()

	f, err = os.Create(fileName)
	suite.NoError(err)
	f.Close()

	suite.fileTestCleanup([]string{fileName})
}

// # Truncate a file
func (suite *fileTestSuite) TestFileTruncate() {
	fileName := filepath.Join(suite.testPath, "small_write.txt")
	f, err := os.Create(fileName)
	suite.NoError(err)
	f.Close()

	err = os.Truncate(fileName, 2)
	suite.NoError(err)

	data, err := os.ReadFile(fileName)
	suite.NoError(err)
	suite.LessOrEqual(2, len(data))

	suite.fileTestCleanup([]string{fileName})
}

// # Create file matching directory name
func (suite *fileTestSuite) TestFileNameConflict() {
	dirName := filepath.Join(suite.testPath, "test")
	fileName := filepath.Join(suite.testPath, "test.txt")

	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)

	f, err := os.Create(fileName)
	suite.NoError(err)
	f.Close()

	err = os.RemoveAll(dirName)
	suite.NoError(err)
}

// # Copy file from once directory to another
func (suite *fileTestSuite) TestFileCopy() {
	dirName := filepath.Join(suite.testPath, "test123")
	fileName := filepath.Join(suite.testPath, "test")
	dstFileName := filepath.Join(dirName, "test_copy.txt")

	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)

	srcFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR, 0777)
	suite.NoError(err)
	defer srcFile.Close()

	dstFile, err := os.Create(dstFileName)
	suite.NoError(err)
	defer dstFile.Close()

	_, err = io.Copy(srcFile, dstFile)
	suite.NoError(err)
	dstFile.Close()

	suite.fileTestCleanup([]string{dirName})
}

// # Get stats of a file
func (suite *fileTestSuite) TestFileGetStat() {
	fileName := filepath.Join(suite.testPath, "test")
	f, err := os.Create(fileName)
	suite.NoError(err)
	f.Close()
	var stat os.FileInfo
    var statErr error
    suite.waitForCondition(defaultPollTimeout, defaultPollInterval, func() (bool, error) {
        stat, statErr = os.Stat(fileName)
        if statErr != nil {
            return false, statErr
        }
        return true, nil
    }, "file %s stat to update with non-zero ModTime", fileName)
    suite.NoError(statErr)

	stat, err = os.Stat(fileName)
	suite.NoError(err)
	modTineDiff := time.Since(stat.ModTime())

	suite.False(stat.IsDir())
	suite.Equal("test", stat.Name())
	suite.LessOrEqual(modTineDiff.Hours(), float64(1))

	suite.fileTestCleanup([]string{fileName})
}

// # Change mod of file
func (suite *fileTestSuite) TestFileChmod() {
	// File permissions don't work on Windows.
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestFileChmod on Windows")
		return
	}
	if suite.adlsTest {
		fileName := filepath.Join(suite.testPath, "test")
		f, err := os.Create(fileName)
		suite.NoError(err)
		f.Close()

		err = os.Chmod(fileName, 0744)
		suite.NoError(err)
		stat, err := os.Stat(fileName)
		suite.NoError(err)
		suite.Equal("-rwxr--r--", stat.Mode().Perm().String())

		suite.fileTestCleanup([]string{fileName})
	}
}

// # Create multiple med files
func (suite *fileTestSuite) TestFileCreateMulti() {
	if strings.ToLower(fileTestStreamDirectPtr) == "true" &&
		strings.ToLower(fileTestDistroName) == "ubuntu-20.04" {
		fmt.Println("Skipping this test case for stream direct")
		return
	}
	dirName := filepath.Join(suite.testPath, "multi_dir")
	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)
	fileName := filepath.Join(dirName, "multi")
	for i := 0; i < 10; i++ {
		newFile := fileName + strconv.Itoa(i)
		err := os.WriteFile(newFile, suite.minBuff, 0777)
		suite.NoError(err)
	}
	suite.fileTestCleanup([]string{dirName})
}

// TODO: this test would always pass since its dependent on above tests - resources should be created only for it
// # Delete single files
func (suite *fileTestSuite) TestFileDeleteSingle() {
	fileName := filepath.Join(suite.testPath, "multi0")
	f, err := os.Create(fileName)
	suite.NoError(err)
	f.Close()
	suite.fileTestCleanup([]string{fileName})
}

// // -------------- SymLink Tests -------------------

// # Create a symlink to a file
func (suite *fileTestSuite) TestLinkCreate() {
	// Symbolic link creation requires admin rights on Windows.
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestLinkCreate on Windows")
		return
	}

	fileName := filepath.Join(suite.testPath, "small_write1.txt")
	f, err := os.Create(fileName)
	suite.NoError(err)
	f.Close()
	symName := filepath.Join(suite.testPath, "small.lnk")
	err = os.WriteFile(fileName, suite.minBuff, 0777)
	suite.NoError(err)

	err = os.Symlink(fileName, symName)
	suite.NoError(err)
	suite.fileTestCleanup([]string{fileName})
	err = os.Remove(symName)
	suite.NoError(err)
}

// # Read a small file using symlink
func (suite *fileTestSuite) TestLinkRead() {
	// Symbolic link creation requires admin rights on Windows.
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestLinkRead on Windows")
		return
	}

	fileName := filepath.Join(suite.testPath, "small_write1.txt")
	f, err := os.Create(fileName)
	suite.NoError(err)
	f.Close()

	symName := filepath.Join(suite.testPath, "small.lnk")
	err = os.Symlink(fileName, symName)
	suite.NoError(err)

	err = os.WriteFile(fileName, suite.minBuff, 0777)
	suite.NoError(err)
	data, err := os.ReadFile(fileName)
	suite.NoError(err)
	suite.Len(suite.minBuff, len(data))
	suite.fileTestCleanup([]string{fileName})
	err = os.Remove(symName)
	suite.NoError(err)
}

// # Write a small file using symlink
func (suite *fileTestSuite) TestLinkWrite() {
	// Symbolic link creation requires admin rights on Windows.
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestLinkWrite on Windows")
		return
	}

	targetName := filepath.Join(suite.testPath, "small_write1.txt")
	f, err := os.Create(targetName)
	suite.NoError(err)
	f.Close()
	symName := filepath.Join(suite.testPath, "small.lnk")
	err = os.Symlink(targetName, symName)
	suite.NoError(err)

	stat, err := os.Stat(targetName)
	modTineDiff := time.Since(stat.ModTime())
	suite.NoError(err)
	suite.LessOrEqual(modTineDiff.Minutes(), float64(1))
	suite.fileTestCleanup([]string{targetName})
	err = os.Remove(symName)
	suite.NoError(err)
}

// # Rename the target file and validate read on symlink fails
func (suite *fileTestSuite) TestLinkRenameTarget() {
	// Symbolic link creation requires admin rights on Windows.
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestLinkRenameTarget on Windows")
		return
	}

	fileName := filepath.Join(suite.testPath, "small_write1.txt")
	symName := filepath.Join(suite.testPath, "small.lnk")
	f, err := os.Create(fileName)
	suite.NoError(err)
	f.Close()
	err = os.Symlink(fileName, symName)
	suite.NoError(err)

	fileNameNew := filepath.Join(suite.testPath, "small_write_new.txt")
	err = os.Rename(fileName, fileNameNew)
	suite.NoError(err)

	_, err = os.ReadFile(symName)
	// we expect that to fail
	suite.Error(err)

	// rename back to original name
	err = os.Rename(fileNameNew, fileName)
	suite.NoError(err)

	suite.fileTestCleanup([]string{fileName})
	err = os.Remove(symName)
	suite.NoError(err)
}

// # Delete the symklink and check target file is still intact
func (suite *fileTestSuite) TestLinkDeleteReadTarget() {
	// Symbolic link creation requires admin rights on Windows.
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestLinkDeleteReadTarget on Windows")
		return
	}

	fileName := filepath.Join(suite.testPath, "small_write1.txt")
	symName := filepath.Join(suite.testPath, "small.lnk")
	f, err := os.Create(fileName)
	suite.NoError(err)
	f.Close()
	err = os.Symlink(fileName, symName)
	suite.NoError(err)
	err = os.WriteFile(fileName, suite.minBuff, 0777)
	suite.NoError(err)

	err = os.Remove(symName)
	suite.NoError(err)

	data, err := os.ReadFile(fileName)
	suite.NoError(err)
	suite.Len(suite.minBuff, len(data))

	err = os.Symlink(fileName, symName)
	suite.NoError(err)
	suite.fileTestCleanup([]string{fileName})
	err = os.Remove(symName)
	suite.NoError(err)
}

func (suite *fileTestSuite) TestListDirReadLink() {
	// Symbolic link creation requires admin rights on Windows.
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestListDirReadLink on Windows")
		return
	}
	if suite.adlsTest && strings.ToLower(fileTestEnableSymlinkADLS) != "true" {
		fmt.Printf(
			"Skipping this test case for adls : %v, enable-symlink-adls : %v\n",
			suite.adlsTest,
			fileTestEnableSymlinkADLS,
		)
		return
	}

	fileName := filepath.Join(suite.testPath, "small_hns.txt")
	f, err := os.Create(fileName)
	suite.NoError(err)
	f.Close()

	err = os.WriteFile(fileName, suite.minBuff, 0777)
	suite.NoError(err)

	symName := filepath.Join(suite.testPath, "small_hns.lnk")
	err = os.Symlink(fileName, symName)
	suite.NoError(err)

	dl, err := os.ReadDir(suite.testPath)
	suite.NoError(err)
	suite.NotEmpty(dl)

	// temp cache cleanup
	suite.fileTestCleanup(
		[]string{
			filepath.Join(suite.testCachePath, "small_hns.txt"),
			filepath.Join(suite.testCachePath, "small_hns.lnk"),
		},
	)

	data1, err := os.ReadFile(symName)
	suite.NoError(err)
	suite.Len(suite.minBuff, len(data1))

	// temp cache cleanup
	suite.fileTestCleanup(
		[]string{
			filepath.Join(suite.testCachePath, "small_hns.txt"),
			filepath.Join(suite.testCachePath, "small_hns.lnk"),
		},
	)

	data2, err := os.ReadFile(fileName)
	suite.NoError(err)
	suite.Len(suite.minBuff, len(data2))

	// validating data
	suite.Equal(data1, data2)

	suite.fileTestCleanup([]string{fileName})
	err = os.Remove(symName)
	suite.NoError(err)
}

/*
func (suite *fileTestSuite) TestReadOnlyFile() {
	if suite.adlsTest == true {
		fileName := filepath.Join(suite.testPath, "readOnlyFile.txt")
		srcFile, err := os.Create(fileName)
		suite.Equal(nil, err)
		srcFile.Close()
		// make it read only permissions
		err = os.Chmod(fileName, 0444)
		suite.Equal(nil, err)
		_, err = os.OpenFile(fileName, os.O_RDONLY, 0444)
		suite.Equal(nil, err)
		_, err = os.OpenFile(fileName, os.O_RDWR, 0444)
		suite.NotNil(err)
		suite.fileTestCleanup([]string{fileName})
	}
} */

func (suite *fileTestSuite) TestCreateReadOnlyFile() {
	// File permissions not working on Windows.
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestCreateReadOnlyFile on Windows")
		return
	}
	if suite.adlsTest == true {
		fileName := filepath.Join(suite.testPath, "createReadOnlyFile.txt")
		srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0444)
		srcFile.Close()
		suite.NoError(err)
		_, err = os.OpenFile(fileName, os.O_RDONLY, 0444)
		suite.NoError(err)
		suite.fileTestCleanup([]string{fileName})
	}
}

// # Rename with special character in name
func (suite *fileTestSuite) TestRenameSpecial() {
	// This test is flaky on GitHub actions, often, but not always failing.
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestRenameSpecial on Windows")
		return
	}

	dirName := filepath.Join(suite.testPath, "Alcaldía")
	newDirName := filepath.Join(suite.testPath, "Alδaδcaldía")
	fileName := filepath.Join(dirName, "भारत.txt")
	newFileName := filepath.Join(dirName, "भारतabcd.txt")

	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)

	f, err := os.Create(fileName)
	suite.NoError(err)
	f.Close()

	err = os.Rename(fileName, newFileName)
	suite.NoError(err)

	err = os.Rename(newFileName, fileName)
	suite.NoError(err)

	err = os.Rename(dirName, newDirName)
	suite.NoError(err)

	err = os.RemoveAll(newDirName)
	suite.NoError(err)
}

// -------------- Main Method -------------------
func TestFileTestSuite(t *testing.T) {
	initFileFlags()
	fileTest := fileTestSuite{
		minBuff:  make([]byte, 1024),
		medBuff:  make([]byte, (10 * 1024 * 1024)),
		hugeBuff: make([]byte, (500 * 1024 * 1024)),
	}

	// Generate random test dir name where our End to End test run is contained
	testDirName := getFileTestDirName(10)

	// Create directory for testing the End to End test on mount path
	fileTest.testPath = filepath.Join(fileTestPathPtr, testDirName)
	fmt.Println(fileTest.testPath)

	fileTest.testCachePath = filepath.Join(fileTestTempPathPtr, testDirName)
	fmt.Println(fileTest.testCachePath)

	if fileTestAdlsPtr == "true" || fileTestAdlsPtr == "True" {
		fmt.Println("ADLS Testing...")
		fileTest.adlsTest = true
	} else {
		fmt.Println("BLOCK Blob Testing...")
	}

	// Sanity check in the off chance the same random name was generated twice and was still around somehow
	err := os.RemoveAll(fileTest.testPath)
	if err != nil {
		fmt.Printf("Could not cleanup feature dir before testing [%s]\n", err.Error())
	}

	err = os.Mkdir(fileTest.testPath, 0777)
	if err != nil {
		t.Errorf("Failed to create test directory [%s]\n", err.Error())
	}
	rand.Read(fileTest.minBuff)
	rand.Read(fileTest.medBuff)

	// Run the actual End to End test
	suite.Run(t, &fileTest)

	//  Wipe out the test directory created for End to End test
	os.RemoveAll(fileTest.testPath)
}

func init() {
	regFileTestFlag(&fileTestPathPtr, "mnt-path", "", "Mount Path of Container")
	regFileTestFlag(&fileTestAdlsPtr, "adls", "", "Account is ADLS or not")
	regFileTestFlag(&fileTestTempPathPtr, "tmp-path", "", "Cache dir path")
	regFileTestFlag(&fileTestGitClonePtr, "clone", "", "Git clone test is enable or not")
	regFileTestFlag(
		&fileTestStreamDirectPtr,
		"stream-direct-test",
		"false",
		"Run stream direct tests",
	)
	regFileTestFlag(&fileTestDistroName, "distro-name", "", "Name of the distro")
	regFileTestFlag(
		&fileTestEnableSymlinkADLS,
		"enable-symlink-adls",
		"false",
		"Enable symlink support for ADLS accounts",
	)
}
