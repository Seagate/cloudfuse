//go:build linux

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

package file_cache

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type fileCacheLinuxTestSuite struct {
	suite.Suite
	assert            *assert.Assertions
	fileCache         *FileCache
	loopback          internal.Component
	cache_path        string
	fake_storage_path string
}

func (suite *fileCacheLinuxTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
	rand := randomString(8)
	suite.cache_path = common.JoinUnixFilepath(home_dir, "file_cache"+rand)
	suite.fake_storage_path = common.JoinUnixFilepath(home_dir, "fake_storage"+rand)
	defaultConfig := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  timeout-sec: 1\n\nloopbackfs:\n  path: %s", suite.cache_path, suite.fake_storage_path)
	log.Debug(defaultConfig)

	// Delete the temp directories created
	err = os.RemoveAll(suite.cache_path)
	if err != nil {
		fmt.Printf("fileCacheLinuxTestSuite::SetupTest : os.RemoveAll(%s) failed [%v]\n", suite.cache_path, err)
	}
	err = os.RemoveAll(suite.fake_storage_path)
	if err != nil {
		fmt.Printf("fileCacheLinuxTestSuite::SetupTest : os.RemoveAll(%s) failed [%v]\n", suite.fake_storage_path, err)
	}
	suite.setupTestHelper(defaultConfig)
}

func (suite *fileCacheLinuxTestSuite) setupTestHelper(configuration string) {
	suite.assert = assert.New(suite.T())

	config.ReadConfigFromReader(strings.NewReader(configuration))
	suite.loopback = newLoopbackFS()
	suite.fileCache = newTestFileCache(suite.loopback)
	suite.loopback.Start(context.Background())
	err := suite.fileCache.Start(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Unable to start file cache [%s]", err.Error()))
	}

}

func (suite *fileCacheLinuxTestSuite) cleanupTest() {
	suite.loopback.Stop()
	err := suite.fileCache.Stop()
	if err != nil {
		panic(fmt.Sprintf("Unable to stop file cache [%s]", err.Error()))
	}

	// Delete the temp directories created
	err = os.RemoveAll(suite.cache_path)
	if err != nil {
		fmt.Printf("fileCacheLinuxTestSuite::cleanupTest : os.RemoveAll(%s) failed [%v]\n", suite.cache_path, err)
	}
	err = os.RemoveAll(suite.fake_storage_path)
	if err != nil {
		fmt.Printf("fileCacheLinuxTestSuite::cleanupTest : os.RemoveAll(%s) failed [%v]\n", suite.fake_storage_path, err)
	}
}

func (suite *fileCacheLinuxTestSuite) TestChmodNotInCache() {
	defer suite.cleanupTest()
	// Setup - create file directly in fake storage
	path := "file33"
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})

	// Path should be in fake storage
	suite.assert.FileExists(suite.fake_storage_path + "/" + path)

	// Chmod
	err := suite.fileCache.Chmod(internal.ChmodOptions{Name: path, Mode: os.FileMode(0666)})
	suite.assert.NoError(err)

	// Path in fake storage should be updated
	info, _ := os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.EqualValues(0666, info.Mode())
}

func (suite *fileCacheLinuxTestSuite) TestChmodInCache() {
	defer suite.cleanupTest()
	// Setup
	path := "file34"
	createHandle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0666})
	openHandle, _ := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0666})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})

	// Path should be in the file cache
	suite.assert.FileExists(suite.cache_path + "/" + path)
	// Path should be in fake storage
	suite.assert.FileExists(suite.fake_storage_path + "/" + path)

	// Chmod
	err := suite.fileCache.Chmod(internal.ChmodOptions{Name: path, Mode: os.FileMode(0755)})
	suite.assert.NoError(err)
	// Path in fake storage and file cache should be updated
	info, err := os.Stat(suite.cache_path + "/" + path)
	suite.assert.NoError(err)
	suite.assert.EqualValues(0755, info.Mode())
	info, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.NoError(err)
	suite.assert.EqualValues(0755, info.Mode())

	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: openHandle})
}

func (suite *fileCacheLinuxTestSuite) TestChmodCase2() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	path := "file35"
	oldMode := os.FileMode(0511)

	createHandle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: oldMode})
	suite.assert.NoError(err)

	newMode := os.FileMode(0666)
	err = suite.fileCache.Chmod(internal.ChmodOptions{Name: path, Mode: newMode})
	suite.assert.NoError(err)

	err = suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: createHandle})
	suite.assert.NoError(err)

	info, err := os.Stat(suite.cache_path + "/" + path)
	suite.assert.NoError(err)
	suite.assert.EqualValues(info.Mode(), newMode)

	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
	suite.assert.NoError(err)

	// loop until file does not exist - done due to async nature of eviction
	time.Sleep(time.Second)
	_, err = os.Stat(suite.cache_path + "/" + path)
	for i := 0; i < 1000 && !os.IsNotExist(err); i++ {
		time.Sleep(10 * time.Millisecond)
		_, err = os.Stat(suite.cache_path + "/" + path)
	}
	suite.assert.True(os.IsNotExist(err))

	// Get the attributes and now and check file mode is set correctly or not
	attr, err := suite.fileCache.GetAttr(internal.GetAttrOptions{Name: path})
	suite.assert.NoError(err)
	suite.assert.NotNil(attr)
	suite.assert.EqualValues(path, attr.Path)
	suite.assert.EqualValues(attr.Mode, newMode)
}

func (suite *fileCacheLinuxTestSuite) TestChownNotInCache() {
	defer suite.cleanupTest()
	// Setup
	path := "file36"
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})

	// Path should be in fake storage
	suite.assert.FileExists(suite.fake_storage_path + "/" + path)

	// Chown
	owner := os.Getuid()
	group := os.Getgid()
	err := suite.fileCache.Chown(internal.ChownOptions{Name: path, Owner: owner, Group: group})
	suite.assert.NoError(err)

	// Path in fake storage should be updated
	info, err := os.Stat(suite.fake_storage_path + "/" + path)
	stat := info.Sys().(*syscall.Stat_t)
	suite.assert.NoError(err)
	suite.assert.EqualValues(owner, stat.Uid)
	suite.assert.EqualValues(group, stat.Gid)
}

func (suite *fileCacheLinuxTestSuite) TestChownInCache() {
	defer suite.cleanupTest()
	// Setup
	path := "file37"
	createHandle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	openHandle, _ := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0777})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})

	// Path should be in the file cache
	suite.assert.FileExists(suite.cache_path + "/" + path)
	// Path should be in fake storage
	suite.assert.FileExists(suite.fake_storage_path + "/" + path)

	// Chown
	owner := os.Getuid()
	group := os.Getgid()
	err := suite.fileCache.Chown(internal.ChownOptions{Name: path, Owner: owner, Group: group})
	suite.assert.NoError(err)
	// Path in fake storage and file cache should be updated
	info, err := os.Stat(suite.cache_path + "/" + path)
	stat := info.Sys().(*syscall.Stat_t)
	suite.assert.NoError(err)
	suite.assert.EqualValues(owner, stat.Uid)
	suite.assert.EqualValues(group, stat.Gid)
	info, err = os.Stat(suite.fake_storage_path + "/" + path)
	stat = info.Sys().(*syscall.Stat_t)
	suite.assert.NoError(err)
	suite.assert.EqualValues(owner, stat.Uid)
	suite.assert.EqualValues(group, stat.Gid)

	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: openHandle})
}

func (suite *fileCacheLinuxTestSuite) TestChownCase2() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	path := "file38"
	oldMode := os.FileMode(0511)
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: oldMode})
	info, _ := os.Stat(suite.cache_path + "/" + path)
	stat := info.Sys().(*syscall.Stat_t)
	oldOwner := stat.Uid
	oldGroup := stat.Gid

	owner := os.Getuid()
	group := os.Getgid()
	err := suite.fileCache.Chown(internal.ChownOptions{Name: path, Owner: owner, Group: group})
	suite.assert.Error(err)
	suite.assert.Equal(syscall.EIO, err)

	// Path should be in the file cache with old group and owner (since we failed the operation)
	info, err = os.Stat(suite.cache_path + "/" + path)
	stat = info.Sys().(*syscall.Stat_t)
	suite.assert.NoError(err)
	suite.assert.EqualValues(oldOwner, stat.Uid)
	suite.assert.EqualValues(oldGroup, stat.Gid)
	// Path should not be in fake storage
	suite.assert.NoFileExists(suite.fake_storage_path + "/" + path)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestFileCacheLinuxTestSuite(t *testing.T) {
	suite.Run(t, new(fileCacheLinuxTestSuite))
}
