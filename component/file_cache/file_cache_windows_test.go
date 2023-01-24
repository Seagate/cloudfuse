//go:build windows

package file_cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"lyvecloudfuse/common"
	"lyvecloudfuse/common/config"
	"lyvecloudfuse/common/log"
	"lyvecloudfuse/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type fileCacheWindowsTestSuite struct {
	suite.Suite
	assert            *assert.Assertions
	fileCache         *FileCache
	loopback          internal.Component
	cache_path        string
	fake_storage_path string
}

func (suite *fileCacheWindowsTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
	rand := randomString(8)
	suite.cache_path = filepath.Join(home_dir, "file_cache"+rand)
	suite.fake_storage_path = filepath.Join(home_dir, "fake_storage"+rand)
	defaultConfig := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n\nloopbackfs:\n  path: %s", suite.cache_path, suite.fake_storage_path)
	log.Debug(defaultConfig)

	// Delete the temp directories created
	os.RemoveAll(suite.cache_path)
	os.RemoveAll(suite.fake_storage_path)
	suite.setupTestHelper(defaultConfig)
}

func (suite *fileCacheWindowsTestSuite) setupTestHelper(configuration string) {
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

func (suite *fileCacheWindowsTestSuite) cleanupTest() {
	suite.loopback.Stop()
	err := suite.fileCache.Stop()
	if err != nil {
		panic(fmt.Sprintf("Unable to stop file cache [%s]", err.Error()))
	}

	// Delete the temp directories created
	os.RemoveAll(suite.cache_path)
	os.RemoveAll(suite.fake_storage_path)
}

func (suite *fileCacheWindowsTestSuite) TestChownNotInCache() {
	defer suite.cleanupTest()
	// Setup
	path := "file"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})

	_, err := os.Stat(suite.cache_path + "/" + path)
	for i := 0; i < 10 && !os.IsNotExist(err); i++ {
		time.Sleep(time.Second)
		_, err = os.Stat(suite.cache_path + "/" + path)
	}
	suite.assert.True(os.IsNotExist(err))

	// Path should be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))

	// Chown
	owner := os.Getuid()
	group := os.Getgid()
	err = suite.fileCache.Chown(internal.ChownOptions{Name: path, Owner: owner, Group: group})
	suite.assert.Nil(err)

	// Path in fake storage should be updated
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	//stat := info.Sys().(*syscall.Win32FileAttributeData)
	suite.assert.True(err == nil || os.IsExist(err))
	//suite.assert.EqualValues(owner, stat.Uid)
	//suite.assert.EqualValues(group, stat.Gid)
}

func (suite *fileCacheWindowsTestSuite) TestChownInCache() {
	defer suite.cleanupTest()
	// Setup
	path := "file"
	createHandle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
	openHandle, _ := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0777})

	// Path should be in the file cache
	_, err := os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
	// Path should be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))

	// Chown
	owner := os.Getuid()
	group := os.Getgid()
	err = suite.fileCache.Chown(internal.ChownOptions{Name: path, Owner: owner, Group: group})
	suite.assert.Nil(err)
	// Path in fake storage and file cache should be updated
	_, err = os.Stat(suite.cache_path + "/" + path)
	//stat := info.Sys().(*syscall.Win32FileAttributeData)
	suite.assert.True(err == nil || os.IsExist(err))
	//suite.assert.EqualValues(owner, stat.Uid)
	//suite.assert.EqualValues(group, stat.Gid)
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	//stat = info.Sys().(*syscall.Win32FileAttributeData)
	suite.assert.True(err == nil || os.IsExist(err))
	//suite.assert.EqualValues(owner, stat.Uid)
	//suite.assert.EqualValues(group, stat.Gid)

	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: openHandle})
}

func (suite *fileCacheWindowsTestSuite) TestChownCase2() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	path := "file"
	oldMode := os.FileMode(0511)
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: oldMode})
	_, _ = os.Stat(filepath.Join(suite.cache_path, path))
	//stat := info.Sys().(*syscall.Win32FileAttributeData)
	//oldOwner := stat.Uid
	//oldGroup := stat.Gid

	owner := os.Getuid()
	group := os.Getgid()
	err := suite.fileCache.Chown(internal.ChownOptions{Name: path, Owner: owner, Group: group})
	suite.assert.NotNil(err)
	suite.assert.Equal(err, syscall.EIO)

	// Path should be in the file cache with old group and owner (since we failed the operation)
	_, err = os.Stat(filepath.Join(suite.cache_path, path))
	//stat = info.Sys().(*syscall.Win32FileAttributeData)
	suite.assert.True(err == nil || os.IsExist(err))
	//suite.assert.EqualValues(oldOwner, stat.Uid)
	//suite.assert.EqualValues(oldGroup, stat.Gid)
	// Path should not be in fake storage
	_, err = os.Stat(filepath.Join(suite.fake_storage_path, path))
	suite.assert.True(os.IsNotExist(err))
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestFileCacheWindowsTestSuite(t *testing.T) {
	suite.Run(t, new(fileCacheWindowsTestSuite))
}
