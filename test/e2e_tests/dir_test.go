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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

const (
    defaultPollTimeout  = 10 * time.Second
    defaultPollInterval = 100 * time.Millisecond
)

type dirTestSuite struct {
	suite.Suite
	testPath      string
	adlsTest      bool
	sizeTracker   bool
	testCachePath string
	minBuff       []byte
	medBuff       []byte
	hugeBuff      []byte
}

var pathPtr string
var tempPathPtr string
var adlsPtr string
var clonePtr string
var streamDirectPtr string
var enableSymlinkADLS string
var sizeTrackerPtr string

func regDirTestFlag(p *string, name string, value string, usage string) {
	if flag.Lookup(name) == nil {
		flag.StringVar(p, name, value, usage)
	}
}

func getDirTestFlag(name string) string {
	return flag.Lookup(name).Value.(flag.Getter).Get().(string)
}

func initDirFlags() {
	pathPtr = getDirTestFlag("mnt-path")
	adlsPtr = getDirTestFlag("adls")
	tempPathPtr = getDirTestFlag("tmp-path")
	clonePtr = getDirTestFlag("clone")
	streamDirectPtr = getDirTestFlag("stream-direct-test")
	enableSymlinkADLS = getDirTestFlag("enable-symlink-adls")
	sizeTrackerPtr = getDirTestFlag("size-tracker")
}

func getTestDirName(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:n]
}

func (suite *dirTestSuite) dirTestCleanup(toRemove []string) {
	for _, path := range toRemove {
		err := os.RemoveAll(path)
		if err != nil {
			fmt.Printf("dirTestCleanup : %s failed [%v]\n", path, err)
		}
	}
}

func formatMessage(msgAndArgs ...interface{}) string {
    if len(msgAndArgs) == 0 {
        return ""
    }
    if len(msgAndArgs) == 1 {
        if msg, ok := msgAndArgs[0].(string); ok {
            return msg
        }
        return fmt.Sprintf("%v", msgAndArgs[0])
    }
    if msgFormat, ok := msgAndArgs[0].(string); ok {
        return fmt.Sprintf(msgFormat, msgAndArgs[1:]...)
    }
    return fmt.Sprintf("invalid message format: first argument not a string with multiple arguments. Args: %v", msgAndArgs)
}

// waitForCondition polls for a condition to be true, failing the test on timeout.
func (suite *dirTestSuite) waitForCondition(timeout time.Duration, interval time.Duration, condition func() (bool, error), msgAndArgs ...interface{}) {
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

// -------------- Directory Tests -------------------

// # Create Directory with a simple name
func (suite *dirTestSuite) TestDirCreateSimple() {
	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	dirName := filepath.Join(suite.testPath, "test1")
	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)

	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	// cleanup
	suite.dirTestCleanup([]string{dirName})
}

// # Create Directory that already exists
func (suite *dirTestSuite) TestDirCreateDuplicate() {
	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	dirName := filepath.Join(suite.testPath, "test1")
	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)
	// duplicate dir - we expect to throw
	err = os.Mkdir(dirName, 0777)

	if runtime.GOOS == "windows" {
		suite.Contains(err.Error(), "file already exists")
	} else {
		suite.Contains(err.Error(), "file exists")
	}

	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	// cleanup
	suite.dirTestCleanup([]string{dirName})
}

// # Create Directory with special characters in name
func (suite *dirTestSuite) TestDirCreateSplChar() {
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestDirCreateSplChar on Windows")
		return
	}
	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	dirName := filepath.Join(suite.testPath, "@#$^&*()_+=-{}[]|?><.,~")
	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)

	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	// cleanup
	suite.dirTestCleanup([]string{dirName})
}

// # Create Directory with slash in name
func (suite *dirTestSuite) TestDirCreateSlashChar() {
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestDirCreateSlashChar on Windows")
		return
	}
	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	dirName := filepath.Join(suite.testPath, "PRQ\\STUV")
	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)

	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	// cleanup
	suite.dirTestCleanup([]string{dirName})
}

// # Rename a directory
func (suite *dirTestSuite) TestDirRename() {
	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	dirName := filepath.Join(suite.testPath, "test1")
	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)

	newName := filepath.Join(suite.testPath, "test1_new")
	err = os.Rename(dirName, newName)
	suite.NoError(err)

	suite.NoDirExists(dirName)

	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	// cleanup
	suite.dirTestCleanup([]string{newName})
}

// # Move an empty directory
func (suite *dirTestSuite) TestDirMoveEmpty() {
	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	dir2Name := filepath.Join(suite.testPath, "test2")
	err := os.Mkdir(dir2Name, 0777)
	suite.NoError(err)

	dir3Name := filepath.Join(suite.testPath, "test3")
	err = os.Mkdir(dir3Name, 0777)
	suite.NoError(err)

	err = os.Rename(dir2Name, filepath.Join(dir3Name, "test2"))
	suite.NoError(err)

	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	// cleanup
	suite.dirTestCleanup([]string{dir3Name})
}

// # Move an non-empty directory
func (suite *dirTestSuite) TestDirMoveNonEmpty() {
	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	dir2Name := filepath.Join(suite.testPath, "test2NE")
	err := os.Mkdir(dir2Name, 0777)
	suite.NoError(err)

	file1Name := filepath.Join(dir2Name, "test.txt")
	f, err := os.Create(file1Name)
	suite.NoError(err)
	f.Close()

	dir3Name := filepath.Join(suite.testPath, "test3NE")
	err = os.Mkdir(dir3Name, 0777)
	suite.NoError(err)

	err = os.Mkdir(filepath.Join(dir3Name, "abcdTest"), 0777)
	suite.NoError(err)

	err = os.Rename(dir2Name, filepath.Join(dir3Name, "test2"))
	suite.NoError(err)

	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	// cleanup
	suite.dirTestCleanup([]string{file1Name, dir3Name})
}

// # Delete non-empty directory
func (suite *dirTestSuite) TestDirDeleteEmpty() {
	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	dirName := filepath.Join(suite.testPath, "test1_new")
	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)

	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	suite.dirTestCleanup([]string{dirName})
}

// # Delete non-empty directory
func (suite *dirTestSuite) TestDirDeleteNonEmpty() {
	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	dir3Name := filepath.Join(suite.testPath, "test3NE")
	err := os.Mkdir(dir3Name, 0777)
	suite.NoError(err)

	err = os.Mkdir(filepath.Join(dir3Name, "abcdTest"), 0777)
	suite.NoError(err)

	err = os.Remove(dir3Name)
	suite.Error(err)
	// Error message is different on Windows
	if runtime.GOOS == "windows" {
		suite.Contains(err.Error(), "directory is not empty")
	} else {
		suite.Contains(err.Error(), "directory not empty")
	}

	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	// cleanup
	suite.dirTestCleanup([]string{dir3Name})
}

// // # Delete non-empty directory recursively
// func (suite *dirTestSuite) TestDirDeleteRecursive() {
// 	dirName := filepath.Join(suite.testPath, "testREC")

// 	err := os.Mkdir(dirName, 0777)
// 	suite.Equal(nil, err)

// 	err = os.Mkdir(filepath.Join(dirName, "level1"), 0777)
// 	suite.Equal(nil, err)

// 	err = os.Mkdir(filepath.Join(dirName, "level2"), 0777)
// 	suite.Equal(nil, err)

// 	err = os.Mkdir(filepath.Join(dirName, "level1", "l1"), 0777)
// 	suite.Equal(nil, err)

// 	srcFile, err := os.OpenFile(filepath.Join(dirName, "level2", "abc.txt"), os.O_CREATE, 0777)
// 	suite.Equal(nil, err)
// 	srcFile.Close()

// 	suite.dirTestCleanup([]string{dirName})
// }

// # Get stats of a directory
func (suite *dirTestSuite) TestDirGetStats() {
	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	dirName := filepath.Join(suite.testPath, "test3")
	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)

	stat, err := os.Stat(dirName)
	suite.NoError(err)
	modTineDiff := time.Since(stat.ModTime())

	// for directory block blob may still return timestamp as 0
	// So compare the time only if epoch is non-zero
	if stat.ModTime().Unix() != 0 {
		suite.True(stat.IsDir())
		suite.Equal("test3", stat.Name())
		suite.GreaterOrEqual(float64(1), modTineDiff.Hours())
	}

	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	// Cleanup
	suite.dirTestCleanup([]string{dirName})
}

// # Change mod of directory
func (suite *dirTestSuite) TestDirChmod() {
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestDirChmod on Windows")
		return
	}
	if suite.adlsTest == true {
		if suite.sizeTracker {
			suite.Equal(0, DiskSize(pathPtr))
		}
		dirName := filepath.Join(suite.testPath, "testchmod")
		err := os.Mkdir(dirName, 0777)
		suite.NoError(err)

		err = os.Chmod(dirName, 0744)
		suite.NoError(err)

		stat, err := os.Stat(dirName)
		suite.NoError(err)
		suite.Equal("-rwxr--r--", stat.Mode().Perm().String())

		if suite.sizeTracker {
			suite.Equal(0, DiskSize(pathPtr))
		}
		suite.dirTestCleanup([]string{dirName})
	}
}

// # List directory
func (suite *dirTestSuite) TestDirList() {
	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	testDir := filepath.Join(suite.testPath, "bigTestDir")
	err := os.Mkdir(testDir, 0777)
	suite.NoError(err)

	dir := filepath.Join(testDir, "Dir1")
	err = os.Mkdir(dir, 0777)
	suite.NoError(err)
	dir = filepath.Join(testDir, "Dir2")
	err = os.Mkdir(dir, 0777)
	suite.NoError(err)
	dir = filepath.Join(testDir, "Dir3")
	err = os.Mkdir(dir, 0777)
	suite.NoError(err)

	srcFile, err := os.OpenFile(filepath.Join(testDir, "abc.txt"), os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()

	time.Sleep(1 * time.Second)
	files, err := os.ReadDir(testDir)
	suite.NoError(err)
	suite.Len(files, 4)

	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	// Cleanup
	suite.dirTestCleanup([]string{testDir})
}

// // # List directory recursively
// func (suite *dirTestSuite) TestDirListRecursive() {
// 	testDir := filepath.Join(suite.testPath, "bigTestDir")
// 	err := os.Mkdir(testDir, 0777)
// 	suite.Equal(nil, err)

// 	dir := filepath.Join(testDir, "Dir1")
// 	err = os.Mkdir(dir, 0777)
// 	suite.Equal(nil, err)

// 	dir = filepath.Join(testDir, "Dir2")
// 	err = os.Mkdir(dir, 0777)
// 	suite.Equal(nil, err)

// 	dir = filepath.Join(testDir, "Dir3")
// 	err = os.Mkdir(dir, 0777)
// 	suite.Equal(nil, err)

// 	srcFile, err := os.OpenFile(filepath.Join(testDir, "abc.txt"), os.O_CREATE, 0777)
// 	suite.Equal(nil, err)
// 	srcFile.Close()

// 	var files []string
// 	err = filepath.Walk(testDir, func(path string, info os.FileInfo, err error) error {
// 		files = append(files, path)
// 		return nil
// 	})
// 	suite.Equal(nil, err)

// 	testFiles, err := os.ReadDir(testDir)
// 	suite.Equal(nil, err)
// 	suite.Equal(4, len(testFiles))

// 	// Cleanup
// 	suite.dirTestCleanup([]string{testDir})
// }

// // # Rename directory with data
func (suite *dirTestSuite) TestDirRenameFull() {
	if strings.ToLower(streamDirectPtr) == "true" {
		fmt.Println("Skipping this test case for stream direct")
		return
	}

	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	dirName := filepath.Join(suite.testPath, "full_dir")
	newName := filepath.Join(suite.testPath, "full_dir_rename")
	fileName := filepath.Join(dirName, "test_file_")

	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)

	err = os.Mkdir(filepath.Join(dirName, "tmp"), 0777)
	suite.NoError(err)

	for i := 0; i < 10; i++ {
		newFile := fileName + strconv.Itoa(i)
		err := os.WriteFile(newFile, suite.medBuff, 0777)
		suite.NoError(err)
	}

	if suite.sizeTracker {
		suite.Equal(10*len(suite.medBuff), DiskSize(pathPtr))
	}

	err = os.Rename(dirName, newName)
	suite.NoError(err)

	if suite.sizeTracker {
		suite.Equal(10*len(suite.medBuff), DiskSize(pathPtr))
	}

	//  Deleted directory shall not be present in the container now
	suite.NoDirExists(dirName)

	suite.DirExists(newName)

	// this should fail as the new dir should be filled
	err = os.Remove(newName)
	suite.Error(err)

	// cleanup
	suite.dirTestCleanup([]string{newName})

}

func (suite *dirTestSuite) TestGitStash() {
	if strings.ToLower(streamDirectPtr) == "true" {
		fmt.Println("Skipping this test case for stream direct")
		return
	}
	if clonePtr == "true" || clonePtr == "True" {
		dirName := filepath.Join(suite.testPath, "stash")
		tarName := filepath.Join(suite.testPath, "tardir.tar.gz")

		cmd := exec.Command(
			"git",
			"clone",
			"https://github.com/wastore/azure-storage-samples-for-net",
			dirName,
		)
		_, err := cmd.Output()
		suite.NoError(err)

		suite.DirExists(dirName)

		err = os.Chdir(dirName)
		suite.NoError(err)

		cmd = exec.Command("git", "status")
		cliOut, err := cmd.Output()
		suite.NoError(err)
		if len(cliOut) > 0 {
			suite.Contains(string(cliOut), "nothing to commit, working")
		}

		f, err := os.OpenFile("README.md", os.O_WRONLY, 0644)
		suite.NoError(err)
		suite.NotZero(f)
		info, err := f.Stat()
		suite.NoError(err)
		_, err = f.WriteAt([]byte("TestString"), info.Size())
		suite.NoError(err)
		_ = f.Close()

		f, err = os.OpenFile("README.md", os.O_RDONLY, 0644)
		suite.NoError(err)
		suite.NotZero(f)
		new_info, err := f.Stat()
		suite.NoError(err)
		suite.Equal(info.Size()+10, new_info.Size())
		data := make([]byte, 10)
		n, err := f.ReadAt(data, info.Size())
		suite.NoError(err)
		suite.Equal(10, n)
		suite.Equal("TestString", string(data))
		_ = f.Close()

		cmd = exec.Command("git", "status")
		cliOut, err = cmd.Output()
		suite.NoError(err)
		if len(cliOut) > 0 {
			suite.Contains(string(cliOut), "Changes not staged for commit")
		}

		cmd = exec.Command("git", "stash")
		cliOut, err = cmd.Output()
		suite.NoError(err)
		if len(cliOut) > 0 {
			suite.Contains(string(cliOut), "Saved working directory and index state WIP")
		}

		cmd = exec.Command("git", "stash", "list")
		_, err = cmd.Output()
		suite.NoError(err)

		cmd = exec.Command("git", "stash", "pop")
		cliOut, err = cmd.Output()
		suite.NoError(err)
		if len(cliOut) > 0 {
			suite.Contains(string(cliOut), "Changes not staged for commit")
		}

		os.Chdir(suite.testPath)

		// As Tar is taking long time first to clone and then to tar just mixing both the test cases
		cmd = exec.Command("tar", "-zcvf", tarName, dirName)
		cliOut, _ = cmd.Output()
		if len(cliOut) > 0 {
			suite.NotContains(cliOut, "file changed as we read it")
		}

		cmd = exec.Command("tar", "-zxvf", tarName, "--directory", dirName)
		_, _ = cmd.Output()

		os.Remove(tarName)

		suite.dirTestCleanup([]string{dirName})
	}
}

func (suite *dirTestSuite) TestReadDirLink() {
	// Symbolic link creation requires admin rights on Windows.
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestReadDirLink on Windows")
		return
	}
	if suite.adlsTest && strings.ToLower(enableSymlinkADLS) != "true" {
		fmt.Printf(
			"Skipping this test case for adls : %v, enable-symlink-adls : %v\n",
			suite.adlsTest,
			enableSymlinkADLS,
		)
		return
	}

	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}
	dirName := filepath.Join(suite.testPath, "test_hns")
	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)

	fileName := filepath.Join(dirName, "small_file.txt")
	f, err := os.Create(fileName)
	suite.NoError(err)
	f.Close()

	err = os.WriteFile(fileName, suite.minBuff, 0777)
	suite.NoError(err)

	// Write three more files so one block, 4096 bytes, is filled
	for i := 0; i < 3; i++ {
		newFile := fileName + strconv.Itoa(i)
		err := os.WriteFile(newFile, suite.minBuff, 0777)
		suite.NoError(err)
	}

	if suite.sizeTracker {
		suite.Equal(4*len(suite.minBuff), DiskSize(pathPtr))
	}

	symName := filepath.Join(suite.testPath, "dirlink.lnk")
	err = os.Symlink(dirName, symName)
	suite.NoError(err)

	dl, err := os.ReadDir(suite.testPath)
	suite.NoError(err)
	suite.NotEmpty(dl)

	// list operation on symlink
	dirLinkList, err := os.ReadDir(symName)
	suite.NoError(err)
	suite.NotEmpty(dirLinkList)

	dirList, err := os.ReadDir(dirName)
	suite.NoError(err)
	suite.NotEmpty(dirList)

	suite.Len(dirList, len(dirLinkList))

	// comparing list values since they are sorted by file name
	for i := range dirLinkList {
		suite.Equal(dirLinkList[i].Name(), dirList[i].Name())
	}

	// temp cache cleanup
	suite.dirTestCleanup(
		[]string{
			filepath.Join(suite.testCachePath, "test_hns", "small_file.txt"),
			filepath.Join(suite.testCachePath, "dirlink.lnk"),
		},
	)

	data1, err := os.ReadFile(filepath.Join(symName, "small_file.txt"))
	suite.NoError(err)
	suite.Len(suite.minBuff, len(data1))

	// temp cache cleanup
	suite.dirTestCleanup(
		[]string{
			filepath.Join(suite.testCachePath, "test_hns", "small_file.txt"),
			filepath.Join(suite.testCachePath, "dirlink.lnk"),
		},
	)

	data2, err := os.ReadFile(fileName)
	suite.NoError(err)
	suite.Len(suite.minBuff, len(data2))

	// validating data
	suite.Equal(data1, data2)

	if suite.sizeTracker {
		suite.Equal(4*len(suite.minBuff), DiskSize(pathPtr))
	}

	suite.dirTestCleanup([]string{dirName})
	err = os.Remove(symName)
	suite.NoError(err)
}

func (suite *dirTestSuite) TestStatfs() {
	if suite.sizeTracker {
		suite.Equal(0, DiskSize(pathPtr))
	}

	numberOfFiles := 5

	dirName := filepath.Join(suite.testPath, "test_statfs")
	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)

	fileName := filepath.Join(dirName, "small_file_")
	for i := 0; i < numberOfFiles; i++ {
		newFile := fileName + strconv.Itoa(i)
		err := os.WriteFile(newFile, suite.minBuff, 0777)
		suite.NoError(err)
	}
	// flaky test
	// if suite.sizeTracker {
	// 	expectedSize := numberOfFiles * len(suite.minBuff)
    //     suite.waitForCondition(defaultPollTimeout, defaultPollInterval, func() (bool, error) {
    //         currentSize := DiskSize(pathPtr)
    //         return currentSize == numberOfFiles*len(suite.minBuff), fmt.Errorf("expected %d, got %d", expectedSize, currentSize)
    //     }, "DiskSize to be %d after initial writes", expectedSize)
    // }

	for i := 0; i < numberOfFiles; i++ {
		file := fileName + strconv.Itoa(i)
		err := os.Truncate(file, 4096)
		suite.NoError(err)
	}
	if suite.sizeTracker {
		expectedSize := numberOfFiles * 4096
        suite.waitForCondition(defaultPollTimeout, defaultPollInterval, func() (bool, error) {
            currentSize := DiskSize(pathPtr)
            return currentSize == expectedSize, fmt.Errorf("expected %d, got %d", expectedSize, currentSize)
        }, "DiskSize to be %d after first truncate", expectedSize)
    }

	for i := 0; i < numberOfFiles; i++ {
		file := fileName + strconv.Itoa(i)
		err := os.WriteFile(file, suite.medBuff, 0777)
		suite.NoError(err)
	}
	if suite.sizeTracker {
		expectedSize := numberOfFiles * len(suite.medBuff)
        suite.waitForCondition(defaultPollTimeout, defaultPollInterval, func() (bool, error) {
            currentSize := DiskSize(pathPtr)
            return currentSize == expectedSize, fmt.Errorf("expected %d, got %d", expectedSize, currentSize)
        }, "DiskSize to be %d after first truncate", expectedSize)
    }

	renameFile := filepath.Join(dirName, "small_file_rename")
	for i := 0; i < numberOfFiles; i++ {
		oldFile := fileName + strconv.Itoa(i)
		newFile := renameFile + strconv.Itoa(i)
		err := os.Rename(oldFile, newFile)
		suite.NoError(err)
	}
	if suite.sizeTracker {
		expectedSize := numberOfFiles * len(suite.medBuff)
        suite.waitForCondition(defaultPollTimeout, defaultPollInterval, func() (bool, error) {
            currentSize := DiskSize(pathPtr)
            return currentSize == expectedSize, fmt.Errorf("expected %d, got %d", expectedSize, currentSize)
        }, "DiskSize to be %d after first truncate", expectedSize)
    }

	for i := 0; i < numberOfFiles; i++ {
		file := renameFile + strconv.Itoa(i)
		err := os.Truncate(file, 4096)
		suite.NoError(err)
	}
	if suite.sizeTracker {
		expectedSize := numberOfFiles*4096
        suite.waitForCondition(defaultPollTimeout, defaultPollInterval, func() (bool, error) {
            currentSize := DiskSize(pathPtr)
            return currentSize == expectedSize, fmt.Errorf("expected %d, got %d", expectedSize, currentSize)
        }, "DiskSize to be %d after first truncate", expectedSize)
    }

	suite.dirTestCleanup([]string{dirName})
}

// -------------- Main Method -------------------
func TestDirTestSuite(t *testing.T) {
	initDirFlags()
	dirTest := dirTestSuite{
		minBuff:  make([]byte, 1024),
		medBuff:  make([]byte, (10 * 1024 * 1024)),
		hugeBuff: make([]byte, (500 * 1024 * 1024)),
	}

	// Generate random test dir name where our End to End test run is contained
	testDirName := getTestDirName(10)

	// Create directory for testing the End to End test on mount path
	dirTest.testPath = filepath.Join(pathPtr, testDirName)

	dirTest.testCachePath = filepath.Join(tempPathPtr, testDirName)

	if adlsPtr == "true" || adlsPtr == "True" {
		fmt.Println("ADLS Testing...")
		dirTest.adlsTest = true
	}

	if sizeTrackerPtr == "true" || adlsPtr == "True" {
		fmt.Println("Size Tracker Testing...")
		dirTest.sizeTracker = true
	}

	// Sanity check in the off chance the same random name was generated twice and was still around somehow
	err := os.RemoveAll(dirTest.testPath)
	if err != nil {
		fmt.Printf("Could not cleanup feature dir before testing [%s]\n", err.Error())
	}

	err = os.Mkdir(dirTest.testPath, 0777)
	if err != nil {
		t.Errorf("Failed to create test directory [%s]\n", err.Error())
	}
	rand.Read(dirTest.minBuff)
	rand.Read(dirTest.medBuff)
	rand.Read(dirTest.hugeBuff)

	// Run the actual End to End test
	suite.Run(t, &dirTest)

	//  Wipe out the test directory created for End to End test
	err = os.RemoveAll(dirTest.testPath)
	if err != nil {
		fmt.Printf(
			"TestDirTestSuite : Could not cleanup feature dir after testing. Here's why: %v\n",
			err,
		)
	}
}

func init() {
	regDirTestFlag(&pathPtr, "mnt-path", "", "Mount Path of Container")
	regDirTestFlag(&adlsPtr, "adls", "", "Account is ADLS or not")
	regDirTestFlag(&clonePtr, "clone", "", "Git clone test is enable or not")
	regDirTestFlag(&tempPathPtr, "tmp-path", "", "Cache dir path")
	regDirTestFlag(&streamDirectPtr, "stream-direct-test", "false", "Run stream direct tests")
	regDirTestFlag(
		&enableSymlinkADLS,
		"enable-symlink-adls",
		"false",
		"Enable symlink support for ADLS accounts",
	)
	regDirTestFlag(&sizeTrackerPtr, "size-tracker", "false", "Using size_tracker component")
}
