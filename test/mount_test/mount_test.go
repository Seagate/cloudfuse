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

package mount_test

import (
	"bytes"
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
)

var cloudfuseBinary string = "cloudfuse"
var mntDir string = "mntdir"
var configFile, tags string

const (
    pollTimeout  = 10 * time.Second
    pollInterval = 100 * time.Millisecond
)

type mountSuite struct {
	suite.Suite
}

func remountCheck(suite *mountSuite) {
	mountCmd := exec.Command(cloudfuseBinary, "mount", mntDir, "--config-file="+configFile)
	var errb bytes.Buffer
	mountCmd.Stderr = &errb
	_, err := mountCmd.Output()
	suite.Error(err)
	suite.NotEmpty(errb.String())
	suite.Contains(errb.String(), "directory is already mounted")
}

// list cloudfuse mounted directories
func listCloudfuseMounts(suite *mountSuite) []byte {
	mntListCmd := exec.Command(cloudfuseBinary, "mount", "list")
	cliOut, err := mntListCmd.Output()
	suite.NoError(err)
	return cliOut
}

// unmount cloudfuse
func cloudfuseUnmount(suite *mountSuite, unmountOutput string) {
	unmountCmd := exec.Command(cloudfuseBinary, "unmount", "all")
	cliOut, err := unmountCmd.Output()
	suite.NotEmpty(cliOut)
	suite.NoError(err)
	suite.Contains(string(cliOut), unmountOutput)

	// wait after unmount
	unmountVerified := false
    for startTime := time.Now(); time.Since(startTime) < pollTimeout; {
        currentMountsList := listCloudfuseMounts(suite)
        if len(bytes.TrimSpace(currentMountsList)) == 0 {
            unmountVerified = true
            break
        }
        time.Sleep(pollInterval)
    }
	suite.True(unmountVerified)
}

// mount command test along with remount on the same path
func (suite *mountSuite) TestMountCmd() {
	// TODO: Allow test to run on Windows.
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestMountCmd on windows. We should enable this in the future.")
		return
	}

	// run mount command
	mountCmd := exec.Command(cloudfuseBinary, "mount", mntDir, "--config-file="+configFile)
	cliOut, err := mountCmd.Output()
	fmt.Println(string(cliOut))
	suite.Empty(cliOut)
	suite.NoError(err)

	// wait for mount
	mounted := false
    var listOutput []byte
    for startTime := time.Now(); time.Since(startTime) < pollTimeout; {
        listOutput = listCloudfuseMounts(suite)
        if strings.Contains(string(listOutput), mntDir) {
            mounted = true
            break
        }
        time.Sleep(pollInterval)
    }
	suite.True(mounted)

	// validate mount
	cliOut = listCloudfuseMounts(suite)
	suite.NotEmpty(cliOut)
	suite.Contains(string(cliOut), mntDir)

	remountCheck(suite)

	// unmount
	cloudfuseUnmount(suite, mntDir)
}

// mount failure test where the mount directory does not exists on Linux
// or does exist on Windows
func (suite *mountSuite) TestMountDirNotExists() {
	if runtime.GOOS == "windows" {
		os.Mkdir(mntDir, 0777)
		mountCmd := exec.Command(cloudfuseBinary, "mount", mntDir, "--config-file="+configFile)
		var errb bytes.Buffer
		mountCmd.Stderr = &errb
		_, err := mountCmd.Output()
		suite.Error(err)
		suite.NotEmpty(errb.String())
		suite.Contains(errb.String(), "Cannot create WinFsp-FUSE file system")

		// list cloudfuse mounted directories
		cliOut := listCloudfuseMounts(suite)
		suite.Empty(cliOut)

		// unmount
		cloudfuseUnmount(suite, "Nothing to unmount")
		os.RemoveAll(mntDir)
	} else {
		tempDir := filepath.Join(mntDir, "tempdir")
		mountCmd := exec.Command(cloudfuseBinary, "mount", tempDir, "--config-file="+configFile)
		var errb bytes.Buffer
		mountCmd.Stderr = &errb
		_, err := mountCmd.Output()
		suite.Error(err)
		suite.NotEmpty(errb.String())
		suite.Contains(errb.String(), "mount directory does not exists")

		// list cloudfuse mounted directories
		cliOut := listCloudfuseMounts(suite)
		suite.Empty(cliOut)

		// unmount
		cloudfuseUnmount(suite, "Nothing to unmount")
	}
}

// mount failure test where the mount directory is not empty
func (suite *mountSuite) TestMountDirNotEmptyFailure() {
	tempDir := filepath.Join(mntDir, "tempdir")
	_ = os.Mkdir(tempDir, 0777)
	mountCmd := exec.Command(cloudfuseBinary, "mount", mntDir, "--config-file="+configFile)
	var errb bytes.Buffer
	mountCmd.Stderr = &errb
	_, err := mountCmd.Output()
	suite.Error(err)
	suite.NotEmpty(errb.String())

	if runtime.GOOS == "windows" {
		suite.Contains(errb.String(), "Cannot create WinFsp-FUSE file system")
	} else {
		suite.Contains(errb.String(), "mount directory is not empty")
	}

	// list cloudfuse mounted directories
	cliOut := listCloudfuseMounts(suite)
	suite.Empty(cliOut)

	os.RemoveAll(tempDir)

	// Mount directory cannot exist on Windows so we need to remove it
	if runtime.GOOS == "windows" {
		os.RemoveAll(mntDir)
	}

	// unmount
	cloudfuseUnmount(suite, "Nothing to unmount")
}

// mount non-empty directory using nonempty flag
func (suite *mountSuite) TestMountDirNotEmptySuccess() {
	tempDir := filepath.Join(mntDir, "tempdir")
	_ = os.Mkdir(tempDir, 0777)

	mountCmd := exec.Command(
		cloudfuseBinary,
		"mount",
		mntDir,
		"--config-file="+configFile,
		"-o",
		"nonempty",
	)
	cliOut, err := mountCmd.Output()
	suite.Empty(cliOut)
	suite.NoError(err)

	// wait for mount
	mounted := false
    var listOutput []byte
    for startTime := time.Now(); time.Since(startTime) < pollTimeout; {
        listOutput = listCloudfuseMounts(suite)
        if strings.Contains(string(listOutput), mntDir) {
            mounted = true
            break
        }
        time.Sleep(pollInterval)
    }
	suite.True(mounted)

	// validate mount
	cliOut = listCloudfuseMounts(suite)
	suite.NotEmpty(cliOut)
	suite.Contains(string(cliOut), mntDir)

	remountCheck(suite)

	// unmount
	cloudfuseUnmount(suite, mntDir)

	os.RemoveAll(tempDir)
}

// mount failure test where the mount path is not provided
func (suite *mountSuite) TestMountPathNotProvided() {
	mountCmd := exec.Command(cloudfuseBinary, "mount", "", "--config-file="+configFile)
	var errb bytes.Buffer
	mountCmd.Stderr = &errb
	_, err := mountCmd.Output()
	suite.Error(err)
	suite.NotEmpty(errb.String())
	suite.Contains(errb.String(), "mount path not provided")

	// list cloudfuse mounted directories
	cliOut := listCloudfuseMounts(suite)
	suite.Empty(cliOut)

	// unmount
	cloudfuseUnmount(suite, "Nothing to unmount")
}

// mount failure test where config file is not provided
func (suite *mountSuite) TestConfigFileNotProvided() {
	mountCmd := exec.Command(cloudfuseBinary, "mount", mntDir)
	var errb bytes.Buffer
	mountCmd.Stderr = &errb
	_, err := mountCmd.Output()
	suite.Error(err)
	suite.NotEmpty(errb.String())
	suite.Contains(errb.String(), "failed to initialize new pipeline")

	// list cloudfuse mounted directories
	cliOut := listCloudfuseMounts(suite)
	suite.Empty(cliOut)

	// unmount
	cloudfuseUnmount(suite, "Nothing to unmount")
}

// mount failure test where config file is not provided and environment variables have incorrect credentials
func (suite *mountSuite) TestEnvVarMountFailure() {
	tempDir := filepath.Join(mntDir, "..", "tempdir")
	os.Mkdir(tempDir, 0777)

	// create environment variables
	os.Setenv("AZURE_STORAGE_ACCOUNT", "myAccount")
	os.Setenv("AZURE_STORAGE_ACCESS_KEY", "myKey")
	os.Setenv("AZURE_STORAGE_BLOB_ENDPOINT", "https://myAccount.dfs.core.windows.net")

	mountCmd := exec.Command(
		cloudfuseBinary,
		"mount",
		mntDir,
		"--tmp-path="+tempDir,
		"--container-name=myContainer",
	)
	cliOut, err := mountCmd.Output()
	suite.Error(err)

	// list cloudfuse mounted directories
	cliOut = listCloudfuseMounts(suite)
	suite.Empty(cliOut)

	// unmount
	cloudfuseUnmount(suite, "Nothing to unmount")

	os.Unsetenv("AZURE_STORAGE_ACCOUNT")
	os.Unsetenv("AZURE_STORAGE_ACCESS_KEY")
	os.Unsetenv("AZURE_STORAGE_BLOB_ENDPOINT")

	os.RemoveAll(tempDir)
}

// mount test using environment variables for mounting
func (suite *mountSuite) TestEnvVarMount() {
	// TODO: Allow test to run on Windows.
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestEnvVarMount on windows. We should enable this in the future.")
		return
	}

	// read config file
	configData, err := os.ReadFile(configFile)
	suite.NoError(err)

	viper.SetConfigType("yaml")
	viper.ReadConfig(bytes.NewBuffer(configData))

	// create environment variables
	os.Setenv("AZURE_STORAGE_ACCOUNT", viper.GetString("azstorage.account-name"))
	os.Setenv("AZURE_STORAGE_ACCESS_KEY", viper.GetString("azstorage.account-key"))
	os.Setenv("AZURE_STORAGE_BLOB_ENDPOINT", viper.GetString("azstorage.endpoint"))
	os.Setenv("AZURE_STORAGE_ACCOUNT_CONTAINER", viper.GetString("azstorage.container"))
	os.Setenv("AZURE_STORAGE_ACCOUNT_TYPE", viper.GetString("azstorage.type"))

	tempCachePath := viper.GetString("file_cache.path")

	mountCmd := exec.Command(cloudfuseBinary, "mount", mntDir, "--tmp-path="+tempCachePath)
	cliOut, err := mountCmd.Output()
	fmt.Println(string(cliOut))
	fmt.Println(err)
	suite.Empty(cliOut)
	suite.NoError(err)

	// wait for mount
	mounted := false
    var listOutput []byte
    for startTime := time.Now(); time.Since(startTime) < pollTimeout; {
        listOutput = listCloudfuseMounts(suite)
        if strings.Contains(string(listOutput), mntDir) {
            mounted = true
            break
        }
        time.Sleep(pollInterval)
    }
	suite.True(mounted)

	// list cloudfuse mounted directories
	cliOut = listCloudfuseMounts(suite)
	suite.NotEmpty(cliOut)
	suite.Contains(string(cliOut), mntDir)

	// unmount
	cloudfuseUnmount(suite, mntDir)

	mountAllCmd := exec.Command(
		cloudfuseBinary,
		"mount",
		"all",
		mntDir,
		"--tmp-path="+tempCachePath,
	)
	cliOut, err = mountAllCmd.Output()
	fmt.Println(string(cliOut))
	suite.NotEmpty(cliOut)
	suite.NoError(err)

	// wait for mount
	time.Sleep(5 * time.Second)

	// list cloudfuse mounted directories
	cliOut = listCloudfuseMounts(suite)
	suite.NotEmpty(cliOut)
	suite.Contains(string(cliOut), mntDir)

	// unmount
	cloudfuseUnmount(suite, mntDir)

	err = os.RemoveAll(mntDir)
	suite.NoError(err)

	err = os.RemoveAll(tempCachePath)
	suite.NoError(err)

	err = os.Mkdir(mntDir, 0777)
	suite.NoError(err)

	err = os.Mkdir(tempCachePath, 0777)
	suite.NoError(err)

	os.Unsetenv("AZURE_STORAGE_ACCOUNT")
	os.Unsetenv("AZURE_STORAGE_ACCESS_KEY")
	os.Unsetenv("AZURE_STORAGE_BLOB_ENDPOINT")
	os.Unsetenv("AZURE_STORAGE_ACCOUNT_CONTAINER")
	os.Unsetenv("AZURE_STORAGE_ACCOUNT_TYPE")
}

// mount test using environment variables for mounting with cli options
// func (suite *mountSuite) TestEnvVarMountCliParams() {
// 	// read config file
// 	configData, err := os.ReadFile(configFile)
// 	suite.Equal(nil, err)

// 	viper.SetConfigType("yaml")
// 	viper.ReadConfig(bytes.NewBuffer(configData))

// 	// create environment variables
// 	os.Setenv("AZURE_STORAGE_ACCOUNT", viper.GetString("azstorage.account-name"))
// 	os.Setenv("AZURE_STORAGE_ACCESS_KEY", viper.GetString("azstorage.account-key"))
// 	os.Setenv("AZURE_STORAGE_BLOB_ENDPOINT", viper.GetString("azstorage.endpoint"))
// 	os.Setenv("AZURE_STORAGE_ACCOUNT_CONTAINER", viper.GetString("azstorage.container"))
// 	os.Setenv("AZURE_STORAGE_ACCOUNT_TYPE", viper.GetString("azstorage.type"))

// 	tempFile := viper.GetString("file_cache.path")

// 	mountCmd := exec.Command(cloudfuseBinary, "mount", mntDir, "--tmp-path="+tempFile, "--allow-other",
// 		"--file-cache-timeout=120", "--cancel-list-on-mount-seconds=10", "--attr-timeout=120", "--entry-timeout=120",
// 		"--negative-timeout=120", "--log-level=LOG_WARNING", "--cache-size-mb=1000")
// 	cliOut, err := mountCmd.Output()
// 	fmt.Println(string(cliOut))
// 	suite.Equal(0, len(cliOut))
// 	suite.Equal(nil, err)

// 	// wait for mount
// 	time.Sleep(5 * time.Second)

// 	// list cloudfuse mounted directories
// 	cliOut = listCloudfuseMounts(suite)
// 	suite.NotEqual(0, len(cliOut))
// 	suite.Contains(string(cliOut), mntDir)

// 	// unmount
// 	cloudfuseUnmount(suite, mntDir)

// 	os.Unsetenv("AZURE_STORAGE_ACCOUNT")
// 	os.Unsetenv("AZURE_STORAGE_ACCESS_KEY")
// 	os.Unsetenv("AZURE_STORAGE_BLOB_ENDPOINT")
// 	os.Unsetenv("AZURE_STORAGE_ACCOUNT_CONTAINER")
// 	os.Unsetenv("AZURE_STORAGE_ACCOUNT_TYPE")
// }

func mountAndValidate(suite *mountSuite, args ...string) {
	// run mount command
	args = append([]string{"mount", mntDir, "--config-file=" + configFile}, args...)
	mountCmd := exec.Command(cloudfuseBinary, args...)
	cliOut, err := mountCmd.Output()
	suite.Empty(cliOut)
	suite.NoError(err)

	// wait for mount
	mounted := false
    var listOutput []byte
    for startTime := time.Now(); time.Since(startTime) < pollTimeout; {
        listOutput = listCloudfuseMounts(suite)
        if strings.Contains(string(listOutput), mntDir) {
            mounted = true
            break
        }
        time.Sleep(pollInterval)
    }
	suite.True(mounted)

	// validate mount
	cliOut = listCloudfuseMounts(suite)
	suite.NotEmpty(cliOut)
	suite.Contains(string(cliOut), mntDir)
}

func (suite *mountSuite) TestWriteBackCacheAndIgnoreOpenFlags() {
	if tags != "fuse3" {
		return
	}

	mountAndValidate(suite)

	fileName := "testFile"
	remoteFilePath := mntDir + "/" + fileName

	// write to file in the local directory
	buff := make([]byte, 200)
	rand.Read(buff)
	err := os.WriteFile(remoteFilePath, buff, 0777)
	suite.NoError(err)

	// unmount
	cloudfuseUnmount(suite, mntDir)

	mountAndValidate(suite, "--disable-writeback-cache=false", "--ignore-open-flags=false")
	f, err := os.OpenFile(remoteFilePath, os.O_APPEND, 0777)
	suite.Error(err)
	suite.Nil(f)
	cloudfuseUnmount(suite, mntDir)

	mountAndValidate(suite, "--disable-writeback-cache=true", "--ignore-open-flags=false")
	f, err = os.OpenFile(remoteFilePath, os.O_APPEND, 0777)
	suite.NoError(err)
	suite.NotNil(f)
	f.Close()
	time.Sleep(1 * time.Second)
	cloudfuseUnmount(suite, mntDir)

	mountAndValidate(suite, "--disable-writeback-cache=false", "--ignore-open-flags=true")
	f, err = os.OpenFile(remoteFilePath, os.O_APPEND, 0777)
	suite.NoError(err)
	suite.NotNil(f)
	f.Close()
	time.Sleep(1 * time.Second)
	cloudfuseUnmount(suite, mntDir)

	mountAndValidate(suite)
	f, err = os.OpenFile(remoteFilePath, os.O_APPEND, 0777)
	suite.NoError(err)
	suite.NotNil(f)
	f.Close()
	time.Sleep(1 * time.Second)

	err = os.RemoveAll(remoteFilePath)
	suite.NoError(err)

	cloudfuseUnmount(suite, mntDir)
}

func TestMountSuite(t *testing.T) {
	suite.Run(t, new(mountSuite))
}

func TestMain(m *testing.M) {
	workingDirPtr := flag.String("working-dir", "", "Directory containing the cloudfuse binary")
	pathPtr := flag.String("mnt-path", ".", "Mount Path of Container")
	configPtr := flag.String("config-file", "", "Config file for mounting")
	tagsPtr := flag.String("tags", "", "fuse version")

	flag.Parse()

	cloudfuseBinary = filepath.Join(*workingDirPtr, cloudfuseBinary)
	mntDir = filepath.Join(*pathPtr, mntDir)
	configFile = *configPtr
	tags = *tagsPtr

	err := os.RemoveAll(mntDir)
	if err != nil {
		fmt.Printf(
			"MountTest : Could not cleanup mount directory before testing. Here's why: %v\n",
			err,
		)
	}

	// On Linux the folder must exist so we need to create it, on Windows it cannot exist.
	if runtime.GOOS != "windows" {
		os.Mkdir(mntDir, 0777)
	}

	m.Run()

	if runtime.GOOS != "windows" {
		os.RemoveAll(mntDir)
	}
}
