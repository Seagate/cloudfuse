/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2026 Seagate Technology LLC and/or its Affiliates

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

package tiered_storage

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/component/loopback"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/handlemap"
	"go.uber.org/mock/gomock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var home_dir, _ = os.UserHomeDir()

type tieredStorageTestSuite struct {
	suite.Suite
	assert            *assert.Assertions
	tieredStorage     *TieredStorage
	loopback          internal.Component
	cache_path        string // uses os.Separator (filepath.Join)
	fake_storage_path string // uses os.Separator (filepath.Join)
	useMock           bool
	mockCtrl          *gomock.Controller
	mock              *internal.MockComponent
}

func newLoopbackFS(cachePath string) internal.Component {
	loopback := loopback.NewLoopbackFSComponent()
	_ = loopback.Configure(true)
	return loopback
}

func newTestTieredStorage(next internal.Component) *TieredStorage {

	tieredStorage := NewTieredStorageComponent()
	tieredStorage.SetNextComponent(next)
	err := tieredStorage.Configure(true)
	if err != nil {
		panic(fmt.Sprintf("Unable to configure tiered storage: %v", err))
	}
	return tieredStorage.(*TieredStorage)
}

func randomString(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", b)[:length]
}

func (suite *tieredStorageTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
	rand := randomString(8)
	suite.cache_path = filepath.Join(home_dir, "file_cache"+rand)
	suite.fake_storage_path = filepath.Join(home_dir, "fake_storage"+rand)
	defaultConfig := fmt.Sprintf(
		"tiered_storage:\n  path: %s\n  max-size-mb: 1.0\n  offload-io: true\n\nloopbackfs:\n  path: %s",
		suite.cache_path,
		suite.fake_storage_path,
	)
	suite.useMock = false
	log.Debug("%s", defaultConfig)

	// Delete the temp directories created
	err = os.RemoveAll(suite.cache_path)
	if err != nil {
		fmt.Printf(
			"tieredStorageTestSuite::SetupTest : os.RemoveAll(%s) failed [%v]\n",
			suite.cache_path,
			err,
		)
	}
	err = os.RemoveAll(suite.fake_storage_path)
	if err != nil {
		fmt.Printf(
			"tieredStorageTestSuite::SetupTest : os.RemoveAll(%s) failed [%v]\n",
			suite.fake_storage_path,
			err,
		)
	}
	suite.setupTestHelper(defaultConfig)
}

func (suite *tieredStorageTestSuite) setupTestHelper(configuration string) {
	suite.assert = assert.New(suite.T())

	err := config.ReadConfigFromReader(strings.NewReader(configuration))
	suite.assert.NoError(err)
	if suite.useMock {
		suite.mockCtrl = gomock.NewController(suite.T())
		suite.mock = internal.NewMockComponent(suite.mockCtrl)
		suite.tieredStorage = newTestTieredStorage(suite.mock)
		// always simulate being offline
		suite.mock.EXPECT().CloudConnected().AnyTimes().Return(false)
	} else {
		suite.loopback = newLoopbackFS(suite.fake_storage_path)
		suite.tieredStorage = newTestTieredStorage(suite.loopback)
		err = suite.loopback.Start(context.Background())
		suite.assert.NoError(err)
	}
	err = suite.tieredStorage.Start(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Unable to start tiered storage [%s]", err.Error()))
	}

}

func (suite *tieredStorageTestSuite) cleanupTest() {
	err := suite.tieredStorage.Stop()
	if err != nil {
		panic(fmt.Sprintf("Unable to stop tiered storage [%s]", err.Error()))
	}
	if suite.useMock {
		suite.mockCtrl.Finish()
	} else {
		err = suite.loopback.Stop()
		suite.assert.NoError(err)
	}

	// Delete the temp directories created
	err = os.RemoveAll(suite.cache_path)
	suite.assert.NoError(err)
	err = os.RemoveAll(suite.fake_storage_path)
	suite.assert.NoError(err)
}

//Testing OpenFile

func (suite *tieredStorageTestSuite) TestOpenFileNotInCache() {
	defer suite.cleanupTest()
	path := "file7"

	//put file in cloud
	handle, _ := suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	_, err := suite.loopback.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)
	err = suite.loopback.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	//open file through tiered storage, should succeed and return a handle with correct path
	handle, err = suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{
			Name:  path,
			Flags: os.O_RDWR,
			Mode:  0666, //random mode, since we didn't do the other stuff yet
		},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(path, handle.Path)

	// Verify it was now downloaded to the local tiered storage cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))
}

func (suite *tieredStorageTestSuite) TestOpenFileInCache() {
	defer suite.cleanupTest()
	path := "file8"
	handle, _ := suite.tieredStorage.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	_, err := suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)
	err = suite.tieredStorage.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Download is required
	handle, err = suite.tieredStorage.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0777})
	suite.assert.NoError(err)
	suite.assert.Equal(path, handle.Path)
	suite.assert.False(handle.Dirty())

	// File should exist in cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))
}

func (suite *tieredStorageTestSuite) TestOpenFileOCreate() {
	defer suite.cleanupTest()
	path := "file9"
	handle, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path, Flags: os.O_CREATE, Mode: 0777},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(path, handle.Path)
	suite.assert.True(handle.Dirty())
	// File should exist in cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))

}

func (suite *tieredStorageTestSuite) TestOpenFileOCreateExistsLocal() {
	defer suite.cleanupTest()
	path := "file10"
	handle, _ := suite.tieredStorage.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	_, err := suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)
	err = suite.tieredStorage.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Download is required
	handle, err = suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path, Flags: os.O_CREATE, Mode: 0777},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(path, handle.Path)
	suite.assert.False(handle.Dirty())

	// File should exist in cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))

	//Make sure data didn't get modified
	d, err := os.ReadFile(filepath.Join(suite.cache_path, path))
	suite.assert.NoError(err)
	suite.assert.Equal(data, d)

}

func (suite *tieredStorageTestSuite) TestOpenFileOCreateExistsCloud() {
	defer suite.cleanupTest()
	path := "file11"

	//put file in cloud
	handle, _ := suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	_, err := suite.loopback.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)
	err = suite.loopback.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	//open file through tiered storage, should succeed and return a handle with correct path
	handle, err = suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{
			Name:  path,
			Flags: os.O_CREATE,
			Mode:  0777,
		},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(path, handle.Path)

	// Verify it was now downloaded to the local tiered storage cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))
}

//Testing WriteFile

func (suite *tieredStorageTestSuite) TestWriteFile() {
	defer suite.cleanupTest()
	path := "file11"
	handle, _ := suite.tieredStorage.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	handle.Flags.Clear(
		handlemap.HandleFlagDirty,
	) // Technically create file will mark it as dirty, we just want to check write file updates the dirty flag, so temporarily set this to false
	testData := "test data"
	data := []byte(testData)
	length, err := suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)

	suite.assert.NoError(err)
	suite.assert.Equal(len(data), length)
	// Check that the local cache updated with data
	d, _ := os.ReadFile(filepath.Join(suite.cache_path, path))
	suite.assert.Equal(data, d)
	suite.assert.True(handle.Dirty())
}

func (suite *tieredStorageTestSuite) TestWriteFileErrorBadFd() {
	defer suite.cleanupTest()
	// Setup
	file := "file20"
	//bad handle
	handle := handlemap.NewHandle(file)
	bytesWrittength, err := suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: handle},
	)
	suite.assert.Error(err)
	suite.assert.EqualValues(syscall.EBADF, err)
	suite.assert.Equal(0, bytesWrittength)
}

// Testing Create File
func (suite *tieredStorageTestSuite) TestCreateFile() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	path := "file12"
	options := internal.CreateFileOptions{Name: path}
	f, err := suite.tieredStorage.CreateFile(options)

	suite.assert.NoError(err)
	suite.assert.True(f.Dirty()) // Handle should be dirty since it was not created in cloud storage

	// Path should be added to the file cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))
	// Path should not be in fake storage
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, path))
}

// Testing Release File
func (suite *tieredStorageTestSuite) TestReleaseCloudNoDirtyFile() {
	defer suite.cleanupTest()
	path := "file13"

	//put file in cloud
	handle, _ := suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	err := suite.loopback.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	//open file through tiered storage, should succeed and return a handle with correct path
	handle, openErr := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{
			Name:  path,
			Flags: os.O_RDWR,
			Mode:  0666, //random mode, since we didn't do the other stuff yet
		},
	)
	suite.assert.NoError(openErr)

	// Verify it was now downloaded to the local tiered storage cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))

	//As of now, the file would be cloudbacked and exist in map
	// suite.tieredStorage.mu.Lock()
	// node, exists := suite.tieredStorage.fileMap[path]
	// suite.tieredStorage.mu.Unlock()

	val, ok := suite.tieredStorage.fileMap.Load(path)
	node := val.(*FileNode)

	suite.assert.True(node.cloudBacked, "File should be marked as cloud-backed")
	suite.assert.True(ok, "File should be tracked in the fileMap")

	//File should be "cloudBacked" and not dirty so on release the file should be deleted from local and the handle clean
	err = suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)
	_, err = os.Stat(filepath.Join(suite.cache_path, path))
	suite.assert.True(os.IsNotExist(err), "File should be deleted from cache after release")

}

func (suite *tieredStorageTestSuite) TestReleaseCloudDirtyFile() {
	defer suite.cleanupTest()
	path := "file13"

	//put file in cloud
	handle, _ := suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	_, err := suite.loopback.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)
	err = suite.loopback.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	//open file through tiered storage, should succeed and return a handle with correct path
	handle, openErr := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{
			Name:  path,
			Flags: os.O_RDWR,
			Mode:  0666, //random mode, since we didn't do the other stuff yet
		},
	)
	suite.assert.NoError(openErr)

	// Verify it was now downloaded to the local tiered storage cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))

	_, err = suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)

	// Handle should be dirty since it was not created in cloud storage
	suite.assert.True(handle.Dirty())

	//As of now, the file would be cloudbacked and exist in map
	// suite.tieredStorage.mu.Lock()
	// node, exists := suite.tieredStorage.fileMap[path]
	// suite.tieredStorage.mu.Unlock()

	val, exists := suite.tieredStorage.fileMap.Load(path)
	node := val.(*FileNode)

	suite.assert.True(node.cloudBacked, "File should be marked as cloud-backed")
	suite.assert.True(exists, "File should be tracked in the fileMap")

	//File should be "cloudBacked" and dirty so on release the file should be deleted from local and the handle clean
	err = suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)
	_, err = os.Stat(filepath.Join(suite.cache_path, path))
	suite.assert.True(os.IsNotExist(err), "File should be deleted from cache after release")

	//Must check that file by its data is actually in the cloud
	_, err = suite.tieredStorage.NextComponent().GetAttr(
		internal.GetAttrOptions{Name: path, RetrieveMetadata: true})
	suite.assert.NoError(err)

	//tmpFile to hold cloud data || WARNING AI SLOP BELOW, I did not write below this
	//It just checks if the data is preserved
	tmpFile, err := os.CreateTemp("", "cloud_verify")
	suite.assert.NoError(err)
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// 2. Copy from the cloud (loopback) to the temporary file
	err = suite.loopback.CopyToFile(internal.CopyToFileOptions{
		Name:   path,
		Offset: 0,
		Count:  0, // 0 usually means the whole file
		File:   tmpFile,
	})
	suite.assert.NoError(err)

	// 3. Read the data back from the temp file and verify
	dataFromCloud, err := os.ReadFile(tmpFile.Name())
	suite.assert.NoError(err)
	suite.assert.Equal(
		data,
		dataFromCloud,
		"The cloud version should match the modified local version",
	)

}

func (suite *tieredStorageTestSuite) TestReadInBuffer() {
	defer suite.cleanupTest()
	// Setup
	file := "file14"

	//put file in cloud abd write to it
	handle, _ := suite.loopback.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	_, err := suite.loopback.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)
	err = suite.loopback.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	//Must check that file by its data is actually in the cloud
	_, err = suite.tieredStorage.NextComponent().GetAttr(
		internal.GetAttrOptions{Name: file, RetrieveMetadata: true})
	suite.assert.NoError(err)

	handle, _ = suite.tieredStorage.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

	output := make([]byte, 9)
	length, err := suite.tieredStorage.ReadInBuffer(
		&internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: output},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(data, output)
	suite.assert.Equal(len(data), length)
}

func (suite *tieredStorageTestSuite) TestReadInBufferErrorBadFd() {
	defer suite.cleanupTest()
	// Setup
	file := "file15"
	handle := handlemap.NewHandle(file)
	length, err := suite.tieredStorage.ReadInBuffer(&internal.ReadInBufferOptions{Handle: handle})
	suite.assert.Error(err)
	suite.assert.EqualValues(syscall.EBADF, err)
	suite.assert.Equal(0, length)
}

func (suite *tieredStorageTestSuite) TestWriteReadDirtyState() {
	defer suite.cleanupTest()
	path := "file16"

	//put file in cloud
	handle, _ := suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	err := suite.loopback.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	//open file through tiered storage, should succeed and return a handle with correct path
	handle, openErr := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{
			Name:  path,
			Flags: os.O_RDWR,
			Mode:  0666, //random mode, since we didn't do the other stuff yet
		},
	)
	suite.assert.NoError(openErr)

	// Verify it was now downloaded to the local tiered storage cache + in map
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))
	// suite.tieredStorage.mu.Lock()
	// node, exists := suite.tieredStorage.fileMap[path]
	// suite.tieredStorage.mu.Unlock()

	val, exists := suite.tieredStorage.fileMap.Load(path)
	node := val.(*FileNode)

	suite.assert.True(node.cloudBacked, "File should be marked as cloud-backed")
	suite.assert.True(exists, "File should be tracked in the fileMap")

	//1. Write to handle
	testData := "test data"
	data := []byte(testData)
	length, err := suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(len(data), length)

	//check the handle is dirty
	suite.assert.True(handle.Dirty())

	//2. New Read Handle to same file
	handle2, openErr := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{
			Name:  path,
			Flags: os.O_RDWR,
			Mode:  0666, //random mode, since we didn't do the other stuff yet
		},
	)
	suite.assert.NoError(openErr)
	output := make([]byte, 9)
	length, err = suite.tieredStorage.ReadInBuffer(
		&internal.ReadInBufferOptions{Handle: handle2, Offset: 0, Data: output},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(data, output)
	suite.assert.Equal(len(data), length)

	//3. Release The write handle, should still be in local with a dirty handle
	err = suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)
	_, err = os.Stat(filepath.Join(suite.cache_path, path))
	suite.assert.False(os.IsNotExist(err), "File should not be uploaded")

	//check still dirty
	suite.assert.False(handle2.Dirty())
	suite.assert.False(handle.Dirty())

	// suite.tieredStorage.mu.Lock()
	// node, _ = suite.tieredStorage.fileMap[path]
	// suite.tieredStorage.mu.Unlock()

	val, _ = suite.tieredStorage.fileMap.Load(path)
	node = val.(*FileNode)

	suite.assert.True(node.isDirty, "File should be marked as dirty")

	//4. Release the read should upload to cloud
	err = suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle2})
	suite.assert.NoError(err)
	_, err = os.Stat(filepath.Join(suite.cache_path, path))
	suite.assert.True(os.IsNotExist(err), "File should be deleted from cache after release")

	//5. Check data
	//It just checks if the data is preserved
	tmpFile, err := os.CreateTemp("", "cloud_verify")
	suite.assert.NoError(err)
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// 2. Copy from the cloud (loopback) to the temporary file
	err = suite.loopback.CopyToFile(internal.CopyToFileOptions{
		Name:   path,
		Offset: 0,
		Count:  0, // 0 usually means the whole file
		File:   tmpFile,
	})
	suite.assert.NoError(err)

	// 3. Read the data back from the temp file and verify
	dataFromCloud, err := os.ReadFile(tmpFile.Name())
	suite.assert.NoError(err)
	suite.assert.Equal(
		data,
		dataFromCloud,
		"The cloud version should match the modified local version",
	)
}

func (suite *tieredStorageTestSuite) TestReleaseLocalToLRUQueue() {
	//Ok this next test is to essentially go through an iteration of LRU,

	//1. Initialize a local only file
	defer suite.cleanupTest()
	path := "file17"
	handle, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path, Flags: os.O_CREATE, Mode: 0777},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(path, handle.Path)
	suite.assert.True(handle.Dirty())
	// File should exist in cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))

	val, exists := suite.tieredStorage.fileMap.Load(path)
	node := val.(*FileNode)

	suite.assert.False(node.cloudBacked, "File should not be marked as cloud-backed")
	suite.assert.True(exists, "File should be tracked in the fileMap")

	// 2. Release this local file
	//File is local only so it shouldn't be deleted from local knowledge
	err = suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// 3. Check if its in the LRU Queue
	suite.assert.Equal(path, suite.tieredStorage.policy.head.name)
	suite.assert.Equal(path, suite.tieredStorage.policy.tail.name)

}

func (suite *tieredStorageTestSuite) TestReleaseToTriggerEviction() {
	// Ok this next test is to essentially go through an iteration of LRU,
	// 1. Initialize many local only file
	//2. Create files that exceed the 80% threshold, max set at 1MB
	data := make([]byte, 250*1024)
	path1 := "file18"
	handle, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path1, Flags: os.O_CREATE, Mode: 0777},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(path1, handle.Path)

	suite.tieredStorage.WriteFile(&internal.WriteFileOptions{Handle: handle, Data: data})
	suite.assert.True(handle.Dirty())

	err = suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	path2 := "file19"
	handle, err = suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path2, Flags: os.O_CREATE, Mode: 0777},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(path2, handle.Path)

	suite.tieredStorage.WriteFile(&internal.WriteFileOptions{Handle: handle, Data: data})
	suite.assert.True(handle.Dirty())

	err = suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	path3 := "file20"
	handle, err = suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path3, Flags: os.O_CREATE, Mode: 0777},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(path3, handle.Path)

	suite.tieredStorage.WriteFile(&internal.WriteFileOptions{Handle: handle, Data: data})
	suite.assert.True(handle.Dirty())

	err = suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	path4 := "file21"
	handle, err = suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path4, Flags: os.O_CREATE, Mode: 0777},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(path4, handle.Path)

	suite.tieredStorage.WriteFile(&internal.WriteFileOptions{Handle: handle, Data: data})
	suite.assert.True(handle.Dirty())

	err = suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// 3. Check if all in the LRU Queue initially
	suite.assert.Equal(path4, suite.tieredStorage.policy.head.name)
	suite.assert.Equal(path3, suite.tieredStorage.policy.head.next.name)
	suite.assert.Equal(path2, suite.tieredStorage.policy.head.next.next.name)
	suite.assert.Equal(path1, suite.tieredStorage.policy.tail.name)

	_, exists1 := suite.tieredStorage.policy.nodeMap.Load(path1)
	_, exists2 := suite.tieredStorage.policy.nodeMap.Load(path2)
	_, exists3 := suite.tieredStorage.policy.nodeMap.Load(path3)
	_, exists4 := suite.tieredStorage.policy.nodeMap.Load(path4)

	suite.assert.True(exists1)
	suite.assert.True(exists2)
	suite.assert.True(exists3)
	suite.assert.True(exists4)

	//4. Sleep to wait for eviction to kick in
	time.Sleep(100 * time.Millisecond)

	// 4. Some should then be released to the cloud essentially, the ones we wrote data to
	//And the local files should be gone (uploaded and cleaned up), not in either map

	// 4a. Check state of NodeMap
	_, exists1 = suite.tieredStorage.policy.nodeMap.Load(path1)
	_, exists2 = suite.tieredStorage.policy.nodeMap.Load(path2)
	_, exists3 = suite.tieredStorage.policy.nodeMap.Load(path3)
	_, exists4 = suite.tieredStorage.policy.nodeMap.Load(path4)

	suite.assert.False(exists1)
	suite.assert.False(exists2)
	suite.assert.True(exists3)
	suite.assert.True(exists4)

	//4b. Check state of fileMap
	_, exists1 = suite.tieredStorage.fileMap.Load(path1)
	_, exists2 = suite.tieredStorage.fileMap.Load(path2)
	_, exists3 = suite.tieredStorage.fileMap.Load(path3)
	_, exists4 = suite.tieredStorage.fileMap.Load(path4)

	suite.assert.False(exists1)
	suite.assert.False(exists2)
	suite.assert.True(exists3)
	suite.assert.True(exists4)

	// 4c.Check files for files 1 and 2 no longer exist local
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, path1))
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, path2))

	//5. We have to check that the files exist in the cloud
	//Must check that file is actually in the cloud
	_, err = suite.tieredStorage.NextComponent().GetAttr(
		internal.GetAttrOptions{Name: path1, RetrieveMetadata: true})
	suite.assert.NoError(err)

	//Must check that file is actually in the cloud
	_, err = suite.tieredStorage.NextComponent().GetAttr(
		internal.GetAttrOptions{Name: path2, RetrieveMetadata: true})
	suite.assert.NoError(err)

	//Validate the data matches what we have
	//It just checks if the data is preserved
	tmpFile, err := os.CreateTemp("", "cloud_verify")
	suite.assert.NoError(err)
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// 2. Copy from the cloud (loopback) to the temporary file
	err = suite.loopback.CopyToFile(internal.CopyToFileOptions{
		Name:   path1,
		Offset: 0,
		Count:  0, // 0 usually means the whole file
		File:   tmpFile,
	})
	suite.assert.NoError(err)

	// 3. Read the data back from the temp file and verify
	dataFromCloud, err := os.ReadFile(tmpFile.Name())
	suite.assert.NoError(err)
	suite.assert.Equal(
		data,
		dataFromCloud,
		"The cloud version should match the modified local version",
	)

	//It just checks if the data is preserved
	tmpFile, err = os.CreateTemp("", "cloud_verify")
	suite.assert.NoError(err)
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// 2. Copy from the cloud (loopback) to the temporary file
	err = suite.loopback.CopyToFile(internal.CopyToFileOptions{
		Name:   path2,
		Offset: 0,
		Count:  0, // 0 usually means the whole file
		File:   tmpFile,
	})
	suite.assert.NoError(err)

	// 3. Read the data back from the temp file and verify
	dataFromCloud, err = os.ReadFile(tmpFile.Name())
	suite.assert.NoError(err)
	suite.assert.Equal(
		data,
		dataFromCloud,
		"The cloud version should match the modified local version",
	)

}

// ok we gonna do file in local, cloud, file doesnt exist
func (suite *tieredStorageTestSuite) TestDeleteFileCloud() {
	defer suite.cleanupTest()
	// Setup
	file := "file22"

	//put file in cloud abd write to it
	handle, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: file, Mode: 0777},
	)
	suite.assert.NoError(err)
	err = suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	err = suite.tieredStorage.DeleteFile(internal.DeleteFileOptions{Name: file})
	suite.assert.NoError(err)

	// Path should not be in file cache
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, file))

	//file should not exist in cloud
	_, err = suite.tieredStorage.NextComponent().GetAttr(
		internal.GetAttrOptions{Name: file, RetrieveMetadata: true})
	suite.assert.Error(err)

}

func (suite *tieredStorageTestSuite) TestDeleteFileLocal() {
	defer suite.cleanupTest()
	// Setup
	file := "file23"

	//create local file
	_, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: file, Mode: 0777},
	)
	suite.assert.NoError(err)

	err = suite.tieredStorage.DeleteFile(internal.DeleteFileOptions{Name: file})
	suite.assert.NoError(err)

	// Path should not be in file cache
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, file))

}

func (suite *tieredStorageTestSuite) TestDeleteFileNotExists() {
	defer suite.cleanupTest()
	// Setup
	file := "file24"

	err := suite.tieredStorage.DeleteFile(internal.DeleteFileOptions{Name: file})
	suite.assert.Error(err)
	suite.assert.EqualValues(syscall.ENOENT, err)
}

func (suite *tieredStorageTestSuite) TestFlushFile() {
	defer suite.cleanupTest()
	file := "file25"
	handle, _ := suite.tieredStorage.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

	testData := "test data"
	data := []byte(testData)
	_, err := suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)
	suite.assert.True(handle.Dirty())

	err = suite.tieredStorage.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NoError(err)

	//Verify Data is still on the disk
	d, _ := os.ReadFile(filepath.Join(suite.cache_path, file))
	suite.assert.Equal(data, d)
	//Check that handle is still dirty
	suite.assert.True(handle.Dirty())

}

func TestTieredStorageTestSuite(t *testing.T) {
	suite.Run(t, new(tieredStorageTestSuite))
}
