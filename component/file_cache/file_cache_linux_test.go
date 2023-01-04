//go:build linux

package file_cache

import (
	"os"
	"syscall"
	"time"

	"lyvecloudfuse/internal"
)

func (suite *fileCacheTestSuite) TestChownNotInCache() {
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
	info, err := os.Stat(suite.fake_storage_path + "/" + path)
	stat := info.Sys().(*syscall.Stat_t)
	suite.assert.True(err == nil || os.IsExist(err))
	suite.assert.EqualValues(owner, stat.Uid)
	suite.assert.EqualValues(group, stat.Gid)
}

func (suite *fileCacheTestSuite) TestChownInCache() {
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
	info, err := os.Stat(suite.cache_path + "/" + path)
	stat := info.Sys().(*syscall.Stat_t)
	suite.assert.True(err == nil || os.IsExist(err))
	suite.assert.EqualValues(owner, stat.Uid)
	suite.assert.EqualValues(group, stat.Gid)
	info, err = os.Stat(suite.fake_storage_path + "/" + path)
	stat = info.Sys().(*syscall.Stat_t)
	suite.assert.True(err == nil || os.IsExist(err))
	suite.assert.EqualValues(owner, stat.Uid)
	suite.assert.EqualValues(group, stat.Gid)

	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: openHandle})
}

func (suite *fileCacheTestSuite) TestChownCase2() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	path := "file"
	oldMode := os.FileMode(0511)
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: oldMode})
	info, _ := os.Stat(suite.cache_path + "/" + path)
	stat := info.Sys().(*syscall.Stat_t)
	oldOwner := stat.Uid
	oldGroup := stat.Gid

	owner := os.Getuid()
	group := os.Getgid()
	err := suite.fileCache.Chown(internal.ChownOptions{Name: path, Owner: owner, Group: group})
	suite.assert.NotNil(err)
	suite.assert.Equal(err, syscall.EIO)

	// Path should be in the file cache with old group and owner (since we failed the operation)
	info, err = os.Stat(suite.cache_path + "/" + path)
	stat = info.Sys().(*syscall.Stat_t)
	suite.assert.True(err == nil || os.IsExist(err))
	suite.assert.EqualValues(oldOwner, stat.Uid)
	suite.assert.EqualValues(oldGroup, stat.Gid)
	// Path should not be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(os.IsNotExist(err))
}
