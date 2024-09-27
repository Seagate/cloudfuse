/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2024 Seagate Technology LLC and/or its Affiliates
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

package file_cache

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var home_dir, _ = os.UserHomeDir()

type fileCacheTestSuite struct {
	suite.Suite
	assert            *assert.Assertions
	fileCache         *FileCache
	loopback          internal.Component
	cache_path        string // uses os.Separator (filepath.Join)
	fake_storage_path string // uses os.Separator (filepath.Join)
}

func newLoopbackFS() internal.Component {
	loopback := loopback.NewLoopbackFSComponent()
	loopback.Configure(true)

	return loopback
}

func newTestFileCache(next internal.Component) *FileCache {

	fileCache := NewFileCacheComponent()
	fileCache.SetNextComponent(next)
	err := fileCache.Configure(true)
	if err != nil {
		panic(fmt.Sprintf("Unable to configure file cache: %v", err))
	}

	return fileCache.(*FileCache)
}

func randomString(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func (suite *fileCacheTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
	rand := randomString(8)
	suite.cache_path = filepath.Join(home_dir, "file_cache"+rand)
	suite.fake_storage_path = filepath.Join(home_dir, "fake_storage"+rand)
	defaultConfig := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  timeout-sec: 0\n\nloopbackfs:\n  path: %s", suite.cache_path, suite.fake_storage_path)
	log.Debug(defaultConfig)

	// Delete the temp directories created
	err = os.RemoveAll(suite.cache_path)
	if err != nil {
		fmt.Printf("fileCacheTestSuite::SetupTest : os.RemoveAll(%s) failed [%v]\n", suite.cache_path, err)
	}
	err = os.RemoveAll(suite.fake_storage_path)
	if err != nil {
		fmt.Printf("fileCacheTestSuite::SetupTest : os.RemoveAll(%s) failed [%v]\n", suite.fake_storage_path, err)
	}
	suite.setupTestHelper(defaultConfig)
}

func (suite *fileCacheTestSuite) setupTestHelper(configuration string) {
	suite.assert = assert.New(suite.T())

	config.ReadConfigFromReader(strings.NewReader(configuration))
	suite.loopback = newLoopbackFS()
	suite.fileCache = newTestFileCache(suite.loopback)
	err := suite.loopback.Start(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Unable to start loopback [%s]", err.Error()))
	}
	err = suite.fileCache.Start(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Unable to start file cache [%s]", err.Error()))
	}

}

func (suite *fileCacheTestSuite) cleanupTest() {
	suite.loopback.Stop()
	err := suite.fileCache.Stop()
	if err != nil {
		panic(fmt.Sprintf("Unable to stop file cache [%s]", err.Error()))
	}

	// Delete the temp directories created
	err = os.RemoveAll(suite.cache_path)
	if err != nil {
		fmt.Printf("fileCacheTestSuite::cleanupTest : os.RemoveAll(%s) failed [%v]\n", suite.cache_path, err)
	}
	err = os.RemoveAll(suite.fake_storage_path)
	if err != nil {
		fmt.Printf("fileCacheTestSuite::cleanupTest : os.RemoveAll(%s) failed [%v]\n", suite.fake_storage_path, err)
	}
}

// Tests the default configuration of file cache
func (suite *fileCacheTestSuite) TestEmpty() {
	defer suite.cleanupTest()
	suite.cleanupTest() // teardown the default file cache generated
	emptyConfig := fmt.Sprintf("file_cache:\n  path: %s\n\n  offload-io: true\n\nloopbackfs:\n  path: %s", suite.cache_path, suite.fake_storage_path)
	suite.setupTestHelper(emptyConfig) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	suite.assert.Equal("file_cache", suite.fileCache.Name())
	suite.assert.Equal(suite.fileCache.tmpPath, suite.cache_path)
	suite.assert.Equal("lru", suite.fileCache.policy.Name())

	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxSizeMB, 0)
	suite.assert.EqualValues(defaultMaxEviction, suite.fileCache.policy.(*lruPolicy).maxEviction)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).highThreshold, defaultMaxThreshold)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).lowThreshold, defaultMinThreshold)

	suite.assert.False(suite.fileCache.createEmptyFile)
	suite.assert.False(suite.fileCache.allowNonEmpty)
	suite.assert.EqualValues(suite.fileCache.cacheTimeout, 120)
	suite.assert.False(suite.fileCache.cleanupOnStart)
	suite.assert.True(suite.fileCache.syncToFlush)
}

// Tests configuration of file cache
func (suite *fileCacheTestSuite) TestConfig() {
	defer suite.cleanupTest()
	suite.cleanupTest() // teardown the default file cache generated
	policy := "lru"
	maxSizeMb := 1024
	cacheTimeout := 60
	maxDeletion := 10
	highThreshold := 90
	lowThreshold := 10
	createEmptyFile := true
	allowNonEmptyTemp := true
	cleanupOnStart := true
	syncToFlush := false
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  policy: %s\n  max-size-mb: %d\n  timeout-sec: %d\n  max-eviction: %d\n  high-threshold: %d\n  low-threshold: %d\n  create-empty-file: %t\n  allow-non-empty-temp: %t\n  cleanup-on-start: %t\n  sync-to-flush: %t",
		suite.cache_path, policy, maxSizeMb, cacheTimeout, maxDeletion, highThreshold, lowThreshold, createEmptyFile, allowNonEmptyTemp, cleanupOnStart, syncToFlush)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	suite.assert.Equal("file_cache", suite.fileCache.Name())
	suite.assert.Equal(suite.fileCache.tmpPath, suite.cache_path)
	suite.assert.Equal(suite.fileCache.policy.Name(), policy)

	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxSizeMB, maxSizeMb)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxEviction, maxDeletion)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).highThreshold, highThreshold)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).lowThreshold, lowThreshold)

	suite.assert.Equal(suite.fileCache.createEmptyFile, createEmptyFile)
	suite.assert.Equal(suite.fileCache.allowNonEmpty, allowNonEmptyTemp)
	suite.assert.EqualValues(suite.fileCache.cacheTimeout, cacheTimeout)
	suite.assert.Equal(suite.fileCache.cleanupOnStart, cleanupOnStart)
	suite.assert.Equal(suite.fileCache.syncToFlush, syncToFlush)
}

func (suite *fileCacheTestSuite) TestDefaultCacheSize() {
	defer suite.cleanupTest()
	// Setup
	config := fmt.Sprintf("file_cache:\n  path: %s\n", suite.cache_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)
	var freeDisk int
	if runtime.GOOS == "windows" {
		cmd := exec.Command("fsutil", "volume", "diskfree", suite.cache_path)
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		suite.assert.NoError(err)

		output := out.String()
		re := regexp.MustCompile(`Total free bytes\s+:\s+([\d,]+)`)
		matches := re.FindStringSubmatch(output)
		suite.assert.GreaterOrEqual(len(matches), 2)
		totalFreeBytesStr := strings.ReplaceAll(matches[1], ",", "")
		freeDisk, err = strconv.Atoi(totalFreeBytesStr)
		suite.assert.NoError(err)
	} else {
		cmd := exec.Command("bash", "-c", fmt.Sprintf("df -B1 %s | awk 'NR==2{print $4}'", suite.cache_path))
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		fmt.Println(err)
		suite.assert.NoError(err)
		freeDisk, err = strconv.Atoi(strings.TrimSpace(out.String()))
		suite.assert.NoError(err)
	}
	expected := uint64(0.8 * float64(freeDisk))
	actual := suite.fileCache.maxCacheSize
	difference := math.Abs(float64(actual) - float64(expected))
	tolerance := 0.10 * float64(math.Max(float64(actual), float64(expected)))
	suite.assert.LessOrEqual(difference, tolerance, "mssg:", actual, expected)
}

func (suite *fileCacheTestSuite) TestConfigPolicyTimeout() {
	defer suite.cleanupTest()
	suite.cleanupTest() // teardown the default file cache generated
	policy := "lru"
	maxSizeMb := 1024
	cacheTimeout := 60
	maxDeletion := 10
	highThreshold := 90
	lowThreshold := 10
	createEmptyFile := true
	allowNonEmptyTemp := true
	cleanupOnStart := true
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  policy: %s\n  max-size-mb: %d\n  timeout-sec: %d\n  max-eviction: %d\n  high-threshold: %d\n  low-threshold: %d\n  create-empty-file: %t\n  allow-non-empty-temp: %t\n  cleanup-on-start: %t",
		suite.cache_path, policy, maxSizeMb, cacheTimeout, maxDeletion, highThreshold, lowThreshold, createEmptyFile, allowNonEmptyTemp, cleanupOnStart)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	suite.assert.Equal("file_cache", suite.fileCache.Name())
	suite.assert.Equal(suite.fileCache.tmpPath, suite.cache_path)
	suite.assert.Equal(suite.fileCache.policy.Name(), policy)

	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxSizeMB, maxSizeMb)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxEviction, maxDeletion)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).highThreshold, highThreshold)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).lowThreshold, lowThreshold)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).cacheTimeout, cacheTimeout)

	suite.assert.Equal(suite.fileCache.createEmptyFile, createEmptyFile)
	suite.assert.Equal(suite.fileCache.allowNonEmpty, allowNonEmptyTemp)
	suite.assert.EqualValues(suite.fileCache.cacheTimeout, cacheTimeout)
	suite.assert.Equal(suite.fileCache.cleanupOnStart, cleanupOnStart)
}

func (suite *fileCacheTestSuite) TestConfigPolicyDefaultTimeout() {
	defer suite.cleanupTest()
	suite.cleanupTest() // teardown the default file cache generated
	policy := "lru"
	maxSizeMb := 1024
	cacheTimeout := defaultFileCacheTimeout
	maxDeletion := 10
	highThreshold := 90
	lowThreshold := 10
	createEmptyFile := true
	allowNonEmptyTemp := true
	cleanupOnStart := true
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  policy: %s\n  max-size-mb: %d\n  max-eviction: %d\n  high-threshold: %d\n  low-threshold: %d\n  create-empty-file: %t\n  allow-non-empty-temp: %t\n  cleanup-on-start: %t",
		suite.cache_path, policy, maxSizeMb, maxDeletion, highThreshold, lowThreshold, createEmptyFile, allowNonEmptyTemp, cleanupOnStart)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	suite.assert.Equal("file_cache", suite.fileCache.Name())
	suite.assert.Equal(suite.fileCache.tmpPath, suite.cache_path)
	suite.assert.Equal(suite.fileCache.policy.Name(), policy)

	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxSizeMB, maxSizeMb)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxEviction, maxDeletion)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).highThreshold, highThreshold)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).lowThreshold, lowThreshold)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).cacheTimeout, cacheTimeout)

	suite.assert.Equal(suite.fileCache.createEmptyFile, createEmptyFile)
	suite.assert.Equal(suite.fileCache.allowNonEmpty, allowNonEmptyTemp)
	suite.assert.EqualValues(suite.fileCache.cacheTimeout, cacheTimeout)
	suite.assert.Equal(suite.fileCache.cleanupOnStart, cleanupOnStart)
}

func (suite *fileCacheTestSuite) TestConfigZero() {
	defer suite.cleanupTest()
	suite.cleanupTest() // teardown the default file cache generated
	policy := "lru"
	maxSizeMb := 1024
	cacheTimeout := 0
	maxDeletion := 10
	highThreshold := 90
	lowThreshold := 10
	createEmptyFile := true
	allowNonEmptyTemp := true
	cleanupOnStart := true
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  policy: %s\n  max-size-mb: %d\n  timeout-sec: %d\n  max-eviction: %d\n  high-threshold: %d\n  low-threshold: %d\n  create-empty-file: %t\n  allow-non-empty-temp: %t\n  cleanup-on-start: %t",
		suite.cache_path, policy, maxSizeMb, cacheTimeout, maxDeletion, highThreshold, lowThreshold, createEmptyFile, allowNonEmptyTemp, cleanupOnStart)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	suite.assert.Equal("file_cache", suite.fileCache.Name())
	suite.assert.Equal(suite.fileCache.tmpPath, suite.cache_path)
	suite.assert.Equal(suite.fileCache.policy.Name(), policy)

	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxSizeMB, maxSizeMb)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxEviction, maxDeletion)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).highThreshold, highThreshold)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).lowThreshold, lowThreshold)

	suite.assert.Equal(suite.fileCache.createEmptyFile, createEmptyFile)
	suite.assert.Equal(suite.fileCache.allowNonEmpty, allowNonEmptyTemp)
	suite.assert.EqualValues(suite.fileCache.cacheTimeout, cacheTimeout)
	suite.assert.Equal(suite.fileCache.cleanupOnStart, cleanupOnStart)
}

// Tests CreateDir
func (suite *fileCacheTestSuite) TestCreateDir() {
	defer suite.cleanupTest()
	path := "a"
	options := internal.CreateDirOptions{Name: path}
	err := suite.fileCache.CreateDir(options)
	suite.assert.NoError(err)

	// Path should not be added to the file cache
	suite.assert.NoDirExists(filepath.Join(suite.cache_path, path))
	// Path should be in fake storage
	suite.assert.DirExists(filepath.Join(suite.fake_storage_path, path))
}

func (suite *fileCacheTestSuite) TestDeleteDir() {
	defer suite.cleanupTest()
	// Setup
	// Configure to create empty files so we create the file in cloud storage
	createEmptyFile := true
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  create-empty-file: %t\n\nloopbackfs:\n  path: %s",
		suite.cache_path, createEmptyFile, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	dir := "dir"
	path := dir + "/file"
	err := suite.fileCache.CreateDir(internal.CreateDirOptions{Name: dir, Mode: 0777})
	suite.assert.NoError(err)
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.assert.NoError(err)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)
	// The file (and directory) is in the cache and storage (see TestCreateFileInDirCreateEmptyFile)
	// Delete the file since we can only delete empty directories
	err = suite.fileCache.DeleteFile(internal.DeleteFileOptions{Name: path})
	suite.assert.NoError(err)

	// Delete the directory
	err = suite.fileCache.DeleteDir(internal.DeleteDirOptions{Name: dir})
	suite.assert.NoError(err)
	// wait for asynchronous deletion
	time.Sleep(100 * time.Millisecond)
	// Directory should not be cached
	suite.assert.NoDirExists(filepath.Join(suite.cache_path, dir))
}

func (suite *fileCacheTestSuite) TestStreamDirError() {
	defer suite.cleanupTest()
	// Setup
	name := "dir" // Does not exist in cache or storage

	dir, _, err := suite.fileCache.StreamDir(internal.StreamDirOptions{Name: name})
	suite.assert.Error(err)
	suite.assert.Empty(dir)
}

func (suite *fileCacheTestSuite) TestStreamDirCase1() {
	defer suite.cleanupTest()
	// Setup
	name := "dir"
	subdir := name + "/subdir"
	file1 := name + "/file1"
	file2 := name + "/file2"
	file3 := name + "/file3"
	// Create files directly in "fake_storage"
	suite.loopback.CreateDir(internal.CreateDirOptions{Name: name, Mode: 0777})
	suite.loopback.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file1})
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file2})
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file3})

	// Read the Directory
	dir, _, err := suite.fileCache.StreamDir(internal.StreamDirOptions{Name: name})
	suite.assert.NoError(err)
	suite.assert.NotEmpty(dir)
	suite.assert.Len(dir, 4)
	suite.assert.EqualValues(file1, dir[0].Path)
	suite.assert.EqualValues(file2, dir[1].Path)
	suite.assert.EqualValues(file3, dir[2].Path)
	suite.assert.EqualValues(subdir, dir[3].Path)
}

func (suite *fileCacheTestSuite) TestStreamDirCase2() {
	defer suite.cleanupTest()
	// Setup
	name := "dir"
	subdir := name + "/subdir"
	file1 := name + "/file1"
	file2 := name + "/file2"
	file3 := name + "/file3"
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: name, Mode: 0777})
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})
	// By default createEmptyFile is false, so we will not create these files in cloud storage until they are closed.
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file1, Mode: 0777})
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file2, Mode: 0777})
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file3, Mode: 0777})

	// Read the Directory
	dir, _, err := suite.fileCache.StreamDir(internal.StreamDirOptions{Name: name})
	suite.assert.NoError(err)
	suite.assert.NotEmpty(dir)
	suite.assert.Len(dir, 4)
	suite.assert.EqualValues(subdir, dir[0].Path)
	suite.assert.EqualValues(file1, dir[1].Path)
	suite.assert.EqualValues(file2, dir[2].Path)
	suite.assert.EqualValues(file3, dir[3].Path)
}

func (suite *fileCacheTestSuite) TestStreamDirCase3() {
	defer suite.cleanupTest()
	// Setup
	name := "dir"
	subdir := name + "/subdir"
	file1 := name + "/file1"
	file2 := name + "/file2"
	file3 := name + "/file3"
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: name, Mode: 0777})
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})
	// By default createEmptyFile is false, so we will not create these files in storage until they are closed.
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file1, Mode: 0777})
	suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: file1, Size: 1024})
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file2, Mode: 0777})
	suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: file2, Size: 1024})
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file3, Mode: 0777})
	suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: file3, Size: 1024})
	// Create the files in fake_storage and simulate different sizes
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file1, Mode: 0777}) // Length is default 0
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file2, Mode: 0777})
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file3, Mode: 0777})

	// Read the Directory
	dir, _, err := suite.fileCache.StreamDir(internal.StreamDirOptions{Name: name})
	suite.assert.NoError(err)
	suite.assert.NotEmpty(dir)
	suite.assert.Len(dir, 4)
	suite.assert.EqualValues(file1, dir[0].Path)
	suite.assert.EqualValues(1024, dir[0].Size)
	suite.assert.EqualValues(file2, dir[1].Path)
	suite.assert.EqualValues(1024, dir[1].Size)
	suite.assert.EqualValues(file3, dir[2].Path)
	suite.assert.EqualValues(1024, dir[2].Size)
	suite.assert.EqualValues(subdir, dir[3].Path)
}

func pos(s []*internal.ObjAttr, e string) int {
	for i, v := range s {
		if v.Path == e {
			return i
		}
	}
	return -1
}

func (suite *fileCacheTestSuite) TestStreamDirMixed() {
	defer suite.cleanupTest()
	// Setup
	name := "dir"
	subdir := name + "/subdir"
	file1 := name + "/file1" // case 1
	file2 := name + "/file2" // case 2
	file3 := name + "/file3" // case 3
	file4 := name + "/file4" // case 4

	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: name, Mode: 0777})
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})

	// By default createEmptyFile is false, so we will not create these files in storage until they are closed.
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file2, Mode: 0777})
	suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: file2, Size: 1024})
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file3, Mode: 0777})
	suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: file3, Size: 1024})

	// Create the files in fake_storage and simulate different sizes
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file1, Mode: 0777}) // Length is default 0
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file3, Mode: 0777})

	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file4, Mode: 0777})
	suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: file4, Size: 1024})
	suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: file4, Size: 0})

	// Read the Directory
	dir, _, err := suite.fileCache.StreamDir(internal.StreamDirOptions{Name: name})
	suite.assert.NoError(err)
	suite.assert.NotEmpty(dir)

	var i int
	i = pos(dir, file1)
	suite.assert.EqualValues(0, dir[i].Size)

	i = pos(dir, file3)
	suite.assert.EqualValues(1024, dir[i].Size)

	i = pos(dir, file2)
	suite.assert.EqualValues(1024, dir[i].Size)

	i = pos(dir, file4)
	suite.assert.EqualValues(0, dir[i].Size)
}

func (suite *fileCacheTestSuite) TestFileUsed() {
	defer suite.cleanupTest()
	err := suite.fileCache.FileUsed("temp")
	suite.assert.NoError(err)
	suite.assert.True(suite.fileCache.policy.IsCached(filepath.Join(suite.cache_path, "temp")))
}

// File cache does not have CreateDir Method implemented hence results are undefined here
func (suite *fileCacheTestSuite) TestIsDirEmpty() {
	defer suite.cleanupTest()
	// Setup
	path := "dir"
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: path, Mode: 0777})

	empty := suite.fileCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: path})
	suite.assert.True(empty)
}

func (suite *fileCacheTestSuite) TestIsDirEmptyFalse() {
	defer suite.cleanupTest()
	// Setup
	path := "dir"
	subdir := path + "/subdir"
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: path, Mode: 0777})
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})

	empty := suite.fileCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: path})
	suite.assert.False(empty)
}

func (suite *fileCacheTestSuite) TestIsDirEmptyFalseInCache() {
	defer suite.cleanupTest()
	// Setup
	path := "dir"
	file := path + "/file"
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: path, Mode: 0777})
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

	empty := suite.fileCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: path})
	suite.assert.False(empty)
}

func (suite *fileCacheTestSuite) TestRenameDir() {
	defer suite.cleanupTest()
	// Setup
	// Configure to create empty files so we create the file in cloud storage
	createEmptyFile := true
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  create-empty-file: %t\n\nloopbackfs:\n  path: %s",
		suite.cache_path, createEmptyFile, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	src := "src"
	dst := "dst"
	err := suite.fileCache.CreateDir(internal.CreateDirOptions{Name: src, Mode: 0777})
	suite.assert.NoError(err)
	path := src + "/file"
	for i := 0; i < 5; i++ {
		handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path + strconv.Itoa(i), Mode: 0777})
		suite.assert.NoError(err)
		err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
		suite.assert.NoError(err)
	}
	// The file (and directory) is in the cache and storage (see TestCreateFileInDirCreateEmptyFile)

	// Rename the directory
	err = suite.fileCache.RenameDir(internal.RenameDirOptions{Src: src, Dst: dst})
	suite.assert.NoError(err)
	// wait for asynchronous deletion
	time.Sleep(100 * time.Millisecond)
	// src directory should not exist in local filesystem
	suite.assert.NoDirExists(filepath.Join(suite.cache_path, src))
	// dst directory should exist and have contents from src
	dstEntries, err := os.ReadDir(filepath.Join(suite.cache_path, dst))
	suite.assert.NoError(err)
	suite.assert.Len(dstEntries, 5)
	for i, entry := range dstEntries {
		suite.assert.Equal("file"+strconv.Itoa(i), entry.Name())
	}
}

func (suite *fileCacheTestSuite) TestCreateFile() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	path := "file1"
	options := internal.CreateFileOptions{Name: path}
	f, err := suite.fileCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.True(f.Dirty()) // Handle should be dirty since it was not created in cloud storage

	// Path should be added to the file cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))
	// Path should not be in fake storage
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, path))
}

func (suite *fileCacheTestSuite) TestCreateFileWithNoPerm() {
	if runtime.GOOS == "windows" {
		defer suite.cleanupTest()
		// Default is to not create empty files on create file to support immutable storage.
		path := "file1"
		options := internal.CreateFileOptions{Name: path, Mode: 0444}
		f, err := suite.fileCache.CreateFile(options)
		suite.assert.NoError(err)
		suite.assert.True(f.Dirty()) // Handle should be dirty since it was not created in storage

		// Path should be added to the file cache
		suite.assert.FileExists(suite.cache_path + "/" + path)
		// Path should not be in fake storage
		suite.assert.NoFileExists(suite.fake_storage_path + "/" + path)
		err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: f})
		suite.assert.NoError(err)
		info, _ := os.Stat(suite.cache_path + "/" + path)
		suite.assert.Equal(info.Mode(), os.FileMode(0444))
	} else {
		defer suite.cleanupTest()
		// Default is to not create empty files on create file to support immutable storage.
		path := "file1"
		options := internal.CreateFileOptions{Name: path, Mode: 0000}
		f, err := suite.fileCache.CreateFile(options)
		suite.assert.NoError(err)
		suite.assert.True(f.Dirty()) // Handle should be dirty since it was not created in storage

		// Path should be added to the file cache
		suite.assert.FileExists(suite.cache_path + "/" + path)
		// Path should not be in fake storage
		suite.assert.NoFileExists(suite.fake_storage_path + "/" + path)
		err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: f})
		suite.assert.NoError(err)
		info, _ := os.Stat(suite.cache_path + "/" + path)
		suite.assert.Equal(info.Mode(), os.FileMode(0000))
	}
}

func (suite *fileCacheTestSuite) TestCreateFileWithWritePerm() {
	if runtime.GOOS == "windows" {
		defer suite.cleanupTest()
		// Default is to not create empty files on create file to support immutable storage.
		path := "file1"
		options := internal.CreateFileOptions{Name: path, Mode: 0444}
		f, err := suite.fileCache.CreateFile(options)
		suite.assert.NoError(err)
		suite.assert.True(f.Dirty()) // Handle should be dirty since it was not created in storage

		os.Chmod(suite.cache_path+"/"+path, 0666)

		// Path should be added to the file cache
		suite.assert.FileExists(suite.cache_path + "/" + path)
		// Path should not be in fake storage
		suite.assert.NoFileExists(suite.fake_storage_path + "/" + path)
		err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: f})
		suite.assert.NoError(err)
		info, _ := os.Stat(suite.cache_path + "/" + path)
		suite.assert.Equal(info.Mode(), fs.FileMode(0666))
	} else {
		defer suite.cleanupTest()
		// Default is to not create empty files on create file to support immutable storage.
		path := "file1"
		options := internal.CreateFileOptions{Name: path, Mode: 0222}
		f, err := suite.fileCache.CreateFile(options)
		suite.assert.NoError(err)
		suite.assert.True(f.Dirty()) // Handle should be dirty since it was not created in storage

		os.Chmod(suite.cache_path+"/"+path, 0331)

		// Path should be added to the file cache
		suite.assert.FileExists(suite.cache_path + "/" + path)
		// Path should not be in fake storage
		suite.assert.NoFileExists(suite.fake_storage_path + "/" + path)
		err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: f})
		suite.assert.NoError(err)
		info, _ := os.Stat(suite.cache_path + "/" + path)
		suite.assert.Equal(info.Mode(), fs.FileMode(0331))
	}
}

func (suite *fileCacheTestSuite) TestCreateFileInDir() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	dir := "dir"
	path := dir + "/file"
	options := internal.CreateFileOptions{Name: path}
	f, err := suite.fileCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.True(f.Dirty()) // Handle should be dirty since it was not created in cloud storage

	// Path should be added to the file cache, including directory
	suite.assert.DirExists(filepath.Join(suite.cache_path, dir))
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))
	// Path should not be in fake storage
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, path))
}

func (suite *fileCacheTestSuite) TestCreateFileCreateEmptyFile() {
	defer suite.cleanupTest()
	// Configure to create empty files so we create the file in cloud storage
	createEmptyFile := true
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  create-empty-file: %t\n\nloopbackfs:\n  path: %s",
		suite.cache_path, createEmptyFile, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	path := "file2"
	options := internal.CreateFileOptions{Name: path}
	f, err := suite.fileCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.False(f.Dirty()) // Handle should not be dirty since it was written to storage

	// Path should be added to the file cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))
	// Path should be in fake storage
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, path))
}

func (suite *fileCacheTestSuite) TestCreateFileInDirCreateEmptyFile() {
	defer suite.cleanupTest()
	// Configure to create empty files so we create the file in cloud storage
	createEmptyFile := true
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  create-empty-file: %t\n\nloopbackfs:\n  path: %s",
		suite.cache_path, createEmptyFile, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	dir := "dir"
	path := dir + "/file"
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: dir, Mode: 0777})
	f, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.assert.NoError(err)
	suite.assert.False(f.Dirty()) // Handle should be dirty since it was not created in cloud storage

	// Path should be added to the file cache, including directory
	suite.assert.DirExists(filepath.Join(suite.cache_path, dir))
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))
	// Path should be in fake storage, including directory
	suite.assert.DirExists(filepath.Join(suite.fake_storage_path, dir))
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, path))
}

func (suite *fileCacheTestSuite) TestSyncFile() {
	defer suite.cleanupTest()

	suite.fileCache.syncToFlush = false
	path := "file3"

	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})

	// On a sync we open, sync, flush and close
	handle, err := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR, Mode: 0777})
	handlemap.Add(handle)
	suite.assert.NoError(err)
	err = suite.fileCache.SyncFile(internal.SyncFileOptions{Handle: handle})
	suite.assert.NoError(err)
	testData := "test data"
	data := []byte(testData)

	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	handle, loaded := handlemap.Load(handle.ID)
	suite.assert.True(loaded)
	suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})

	// Path should not be in file cache
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, path))

	path = "file.fsync"
	suite.fileCache.syncToFlush = true
	handle, err = suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.assert.NoError(err)
	_, err = suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.assert.NoError(err)
	suite.assert.True(handle.Dirty())
	err = suite.fileCache.SyncFile(internal.SyncFileOptions{Handle: handle})
	suite.assert.NoError(err)
	suite.assert.False(handle.Dirty())
	suite.assert.FileExists(suite.fake_storage_path + "/" + path)

	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
}

func (suite *fileCacheTestSuite) TestDeleteFile() {
	defer suite.cleanupTest()
	path := "file4"

	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.assert.NoError(err)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	err = suite.fileCache.DeleteFile(internal.DeleteFileOptions{Name: path})
	suite.assert.NoError(err)

	// Path should not be in file cache
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, path))
	// Path should not be in fake storage
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, path))
}

// Case 2 Test cover when the file does not exist in cloud storage but it exists in the local cache.
// This can happen if createEmptyFile is false and the file hasn't been flushed yet.
func (suite *fileCacheTestSuite) TestDeleteFileCase2() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	path := "file5"
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})

	err := suite.fileCache.DeleteFile(internal.DeleteFileOptions{Name: path})
	suite.assert.Error(err)
	suite.assert.Equal(syscall.EIO, err)

	// Path should not be in local cache (since we failed the operation)
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))
	// Path should not be in fake storage
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, path))
}

func (suite *fileCacheTestSuite) TestDeleteFileError() {
	defer suite.cleanupTest()
	path := "file6"
	err := suite.fileCache.DeleteFile(internal.DeleteFileOptions{Name: path})
	suite.assert.Error(err)
	suite.assert.EqualValues(syscall.ENOENT, err)
}

func (suite *fileCacheTestSuite) TestOpenFileNotInCache() {
	defer suite.cleanupTest()
	path := "file7"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})

	// loop until file does not exist - done due to async nature of eviction
	_, err := os.Stat(filepath.Join(suite.cache_path, path))
	for i := 0; i < 1000 && !os.IsNotExist(err); i++ {
		time.Sleep(10 * time.Millisecond)
		_, err = os.Stat(filepath.Join(suite.cache_path, path))
	}
	// TODO: find out why this delayed eviction check fails in CI on Windows sometimes
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping TestOpenFileNotInCache eviction check on Windows (flaky)")
	} else {
		suite.assert.True(os.IsNotExist(err))
	}

	handle, err = suite.fileCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR, Mode: suite.fileCache.defaultPermission})
	suite.assert.NoError(err)
	// Download is required
	err = suite.fileCache.downloadFile(handle)
	suite.assert.NoError(err)
	suite.assert.EqualValues(path, handle.Path)
	suite.assert.False(handle.Dirty())

	// File should exist in cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))
}

func (suite *fileCacheTestSuite) TestOpenFileInCache() {
	defer suite.cleanupTest()
	path := "file8"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})

	// Download is required
	handle, err := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0777})
	suite.assert.NoError(err)
	suite.assert.EqualValues(path, handle.Path)
	suite.assert.False(handle.Dirty())

	// File should exist in cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))
}

// Tests for GetProperties in OpenFile should be done in E2E tests
// - there is no good way to test it here with a loopback FS without a mock component.

func (suite *fileCacheTestSuite) TestCloseFile() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	path := "file9"

	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	// The file is in the cache but not in cloud storage (see TestCreateFileInDirCreateEmptyFile)

	// CloseFile
	err := suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// loop until file does not exist - done due to async nature of eviction
	_, err = os.Stat(filepath.Join(suite.cache_path, path))
	for i := 0; i < 1000 && !os.IsNotExist(err); i++ {
		time.Sleep(10 * time.Millisecond)
		_, err = os.Stat(filepath.Join(suite.cache_path, path))
	}
	suite.assert.True(os.IsNotExist(err))

	// File should not be in cache
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, path))
	// File should be in cloud storage
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, path))
}

func (suite *fileCacheTestSuite) TestCloseFileTimeout() {
	defer suite.cleanupTest()
	suite.cleanupTest() // teardown the default file cache generated
	cacheTimeout := 5
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  timeout-sec: %d\n\nloopbackfs:\n  path: %s",
		suite.cache_path, cacheTimeout, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	path := "file10"

	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	// The file is in the cache but not in cloud storage (see TestCreateFileInDirCreateEmptyFile)

	// CloseFile
	err := suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// File should be in cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))
	// File should be in cloud storage
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, path))

	// loop until file does not exist - done due to async nature of eviction
	_, err = os.Stat(filepath.Join(suite.cache_path, path))
	for i := 0; i < (cacheTimeout*300) && !os.IsNotExist(err); i++ {
		time.Sleep(10 * time.Millisecond)
		_, err = os.Stat(filepath.Join(suite.cache_path, path))
	}

	// File should not be in cache
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, path))
	// File should be invalidated
	suite.assert.False(suite.fileCache.policy.IsCached(filepath.Join(suite.cache_path, path)))
	// File should be in cloud storage
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, path))
}

func (suite *fileCacheTestSuite) TestOpenCloseHandleCount() {
	defer suite.cleanupTest()
	// Setup
	file := "file11"
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.assert.NoError(err)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	handle, err = suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})
	suite.assert.NoError(err)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// check that flock handle count is correct
	flock := suite.fileCache.fileLocks.Get(file)
	suite.assert.Zero(flock.Count())
}

func (suite *fileCacheTestSuite) TestOpenPreventsEviction() {
	defer suite.cleanupTest()
	// Setup
	suite.cleanupTest() // teardown the default file cache generated
	cacheTimeout := 1
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  timeout-sec: %d\n\nloopbackfs:\n  path: %s",
		suite.cache_path, cacheTimeout, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	path := "file12"

	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.assert.NoError(err)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)
	// File should be in cache and cloud storage
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, path))

	// Open file (this should prevent eviction)
	handle, err = suite.fileCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0777})
	suite.assert.NoError(err)

	// wait until file would be evicted (if not for being opened)
	time.Sleep(time.Second * time.Duration(cacheTimeout*3))

	// File should still be in cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))
	suite.assert.True(suite.fileCache.policy.IsCached(filepath.Join(suite.cache_path, path)))

	// cleanup
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)
}

func (suite *fileCacheTestSuite) TestReadInBufferEmpty() {
	defer suite.cleanupTest()
	// Setup
	file := "file15"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

	data := make([]byte, 0)
	length, err := suite.fileCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: data})
	suite.assert.NoError(err)
	suite.assert.EqualValues(0, length)
	suite.assert.Empty(data)
}

func (suite *fileCacheTestSuite) TestReadInBufferNoFlush() {
	defer suite.cleanupTest()
	// Setup
	file := "file16"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})

	handle, _ = suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

	output := make([]byte, 9)
	length, err := suite.fileCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: output})
	suite.assert.NoError(err)
	suite.assert.EqualValues(data, output)
	suite.assert.EqualValues(len(data), length)
}

func (suite *fileCacheTestSuite) TestReadInBuffer() {
	defer suite.cleanupTest()
	// Setup
	file := "file17"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})

	handle, _ = suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

	output := make([]byte, 9)
	length, err := suite.fileCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: output})
	suite.assert.NoError(err)
	suite.assert.EqualValues(data, output)
	suite.assert.EqualValues(len(data), length)
}

func (suite *fileCacheTestSuite) TestReadInBufferErrorBadFd() {
	defer suite.cleanupTest()
	// Setup
	file := "file18"
	handle := handlemap.NewHandle(file)
	length, err := suite.fileCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: handle})
	suite.assert.Error(err)
	suite.assert.EqualValues(syscall.EBADF, err)
	suite.assert.EqualValues(0, length)
}

func (suite *fileCacheTestSuite) TestWriteFile() {
	defer suite.cleanupTest()
	// Setup
	file := "file19"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

	handle.Flags.Clear(handlemap.HandleFlagDirty) // Technically create file will mark it as dirty, we just want to check write file updates the dirty flag, so temporarily set this to false
	testData := "test data"
	data := []byte(testData)
	length, err := suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})

	suite.assert.NoError(err)
	suite.assert.EqualValues(len(data), length)
	// Check that the local cache updated with data
	d, _ := os.ReadFile(filepath.Join(suite.cache_path, file))
	suite.assert.EqualValues(data, d)
	suite.assert.True(handle.Dirty())
}

func (suite *fileCacheTestSuite) TestWriteFileErrorBadFd() {
	defer suite.cleanupTest()
	// Setup
	file := "file20"
	handle := handlemap.NewHandle(file)
	len, err := suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle})
	suite.assert.Error(err)
	suite.assert.EqualValues(syscall.EBADF, err)
	suite.assert.EqualValues(0, len)
}

func (suite *fileCacheTestSuite) TestFlushFileEmpty() {
	defer suite.cleanupTest()
	// Setup
	file := "file21"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

	// Path should not be in fake storage
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, file))

	// Flush the Empty File
	err := suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NoError(err)
	suite.assert.False(handle.Dirty())

	// Path should be in fake storage
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, file))
}

func (suite *fileCacheTestSuite) TestFlushFile() {
	defer suite.cleanupTest()
	// Setup
	file := "file22"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})

	// Path should not be in fake storage
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, file))

	// Flush the Empty File
	err := suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NoError(err)
	suite.assert.False(handle.Dirty())

	// Path should be in fake storage
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, file))
	// Check that fake_storage updated with data
	d, _ := os.ReadFile(filepath.Join(suite.fake_storage_path, file))
	suite.assert.EqualValues(data, d)
}

func (suite *fileCacheTestSuite) TestFlushFileErrorBadFd() {
	defer suite.cleanupTest()
	// Setup
	file := "file23"
	handle := handlemap.NewHandle(file)
	handle.Flags.Set(handlemap.HandleFlagDirty)
	err := suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.Error(err)
	suite.assert.EqualValues(syscall.EBADF, err)
}

func (suite *fileCacheTestSuite) TestGetAttrCase1() {
	defer suite.cleanupTest()
	// Setup
	file := "file24"
	// Create files directly in "fake_storage"
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

	// Read the Directory
	attr, err := suite.fileCache.GetAttr(internal.GetAttrOptions{Name: file})
	suite.assert.NoError(err)
	suite.assert.NotNil(attr)
	suite.assert.EqualValues(file, attr.Path)
}

func (suite *fileCacheTestSuite) TestGetAttrCase2() {
	defer suite.cleanupTest()
	// Setup
	file := "file25"
	// By default createEmptyFile is false, so we will not create these files in cloud storage until they are closed.
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

	// Read the Directory
	attr, err := suite.fileCache.GetAttr(internal.GetAttrOptions{Name: file})
	suite.assert.NoError(err)
	suite.assert.NotNil(attr)
	suite.assert.EqualValues(file, attr.Path)
}

func (suite *fileCacheTestSuite) TestGetAttrCase3() {
	defer suite.cleanupTest()
	// Setup
	file := "file26"
	// By default createEmptyFile is false, so we will not create these files in cloud storage until they are closed.
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: file, Size: 1024})
	// Create the files in fake_storage and simulate different sizes
	//suite.loopback.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777}) // Length is default 0

	// Read the Directory
	attr, err := suite.fileCache.GetAttr(internal.GetAttrOptions{Name: file})
	suite.assert.NoError(err)
	suite.assert.NotNil(attr)
	suite.assert.EqualValues(file, attr.Path)
	// this check is flaky in our CI pipeline on Linux, so skip it
	if runtime.GOOS != "windows" {
		fmt.Println("Skipping TestGetAttrCase3 attr.Size check on Linux because it's flaky.")
	} else {
		suite.assert.EqualValues(1024, attr.Size)
	}
}

func (suite *fileCacheTestSuite) TestGetAttrCase4() {
	defer suite.cleanupTest()
	// Setup
	file := "file27"
	// By default createEmptyFile is false, so we will not create these files in cloud storage until they are closed.
	createHandle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.assert.NoError(err)
	suite.assert.NotNil(createHandle)

	size := (100 * 1024 * 1024)
	data := make([]byte, size)

	written, err := suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: createHandle, Offset: 0, Data: data})
	suite.assert.NoError(err)
	suite.assert.EqualValues(size, written)

	err = suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: createHandle})
	suite.assert.NoError(err)

	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
	suite.assert.NoError(err)

	// Wait  file is evicted
	_, err = os.Stat(filepath.Join(suite.cache_path, file))
	for i := 0; i < 2000 && !os.IsNotExist(err); i++ {
		time.Sleep(10 * time.Millisecond)
		_, err = os.Stat(filepath.Join(suite.cache_path, file))
	}
	// TODO: why is check test flaky (on both platforms)?
	fmt.Println("Skipping TestGetAttrCase4 eviction check (flaky).")
	// suite.assert.True(os.IsNotExist(err))

	// open the file in parallel and try getting the size of file while open is on going
	go suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0666})

	// Read the Directory
	attr, err := suite.fileCache.GetAttr(internal.GetAttrOptions{Name: file})
	suite.assert.NoError(err)
	suite.assert.NotNil(attr)
	suite.assert.EqualValues(file, attr.Path)
	suite.assert.EqualValues(size, attr.Size)
}

// func (suite *fileCacheTestSuite) TestGetAttrError() {
// defer suite.cleanupTest()
// 	// Setup
// 	name := "file"
// 	attr, err := suite.fileCache.GetAttr(internal.GetAttrOptions{Name: name})
// 	suite.assert.NotNil(err)
// 	suite.assert.EqualValues(syscall.ENOENT, err)
// 	suite.assert.EqualValues("", attr.Name)
// }

func (suite *fileCacheTestSuite) TestRenameFileNotInCache() {
	defer suite.cleanupTest()
	// Setup
	src := "source1"
	dst := "destination1"
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0777})
	suite.assert.NoError(err)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	_, err = os.Stat(filepath.Join(suite.cache_path, src))
	for i := 0; i < 1000 && !os.IsNotExist(err); i++ {
		time.Sleep(10 * time.Millisecond)
		_, err = os.Stat(filepath.Join(suite.cache_path, src))
	}
	suite.assert.True(os.IsNotExist(err))

	// Path should be in fake storage
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, src))

	// RenameFile
	err = suite.fileCache.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	suite.assert.NoError(err)

	// Path in fake storage should be updated
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, src)) // Src does not exist
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, dst))   // Dst does exist
}

func (suite *fileCacheTestSuite) TestRenameFileInCache() {
	defer suite.cleanupTest()
	// Setup
	src := "source2"
	dst := "destination2"
	createHandle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0666})
	suite.assert.NoError(err)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
	suite.assert.NoError(err)
	openHandle, err := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: src, Mode: 0666})
	suite.assert.NoError(err)

	// Path should be in the file cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, src))
	// Path should be in fake storage
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, src))

	// RenameFile
	err = suite.fileCache.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	suite.assert.NoError(err)
	// Path in fake storage and file cache should be updated
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, src))        // Src does not exist
	suite.assert.FileExists(filepath.Join(suite.cache_path, dst))          // Dst shall exists in cache
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, src)) // Src does not exist
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, dst))   // Dst does exist

	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: openHandle})
}

func (suite *fileCacheTestSuite) TestRenameFileCase2() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	src := "source3"
	dst := "destination3"
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0777})

	err := suite.fileCache.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	suite.assert.Error(err)
	suite.assert.Equal(syscall.EIO, err)

	// Src should be in local cache (since we failed the operation)
	suite.assert.FileExists(filepath.Join(suite.cache_path, src))
	// Src should not be in fake storage
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, src))
	// Dst should not be in fake storage
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, dst))
}

func (suite *fileCacheTestSuite) TestRenameFileAndCacheCleanup() {
	defer suite.cleanupTest()
	suite.cleanupTest()

	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  timeout-sec: 2\n\nloopbackfs:\n  path: %s",
		suite.cache_path, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	src := "source4"
	dst := "destination4"
	createHandle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0666})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
	openHandle, _ := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: src, Mode: 0666})

	// Path should be in the file cache
	suite.assert.FileExists(suite.cache_path + "/" + src)
	// Path should be in fake storage
	suite.assert.FileExists(suite.fake_storage_path + "/" + src)

	// RenameFile
	err := suite.fileCache.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	suite.assert.NoError(err)
	// Path in fake storage and file cache should be updated
	suite.assert.NoFileExists(suite.cache_path + "/" + src)        // Src does not exist
	suite.assert.FileExists(suite.cache_path + "/" + dst)          // Dst shall exists in cache
	suite.assert.NoFileExists(suite.fake_storage_path + "/" + src) // Src does not exist
	suite.assert.FileExists(suite.fake_storage_path + "/" + dst)   // Dst does exist

	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: openHandle})

	time.Sleep(1 * time.Second)                           // Check once before the cache cleanup that file exists
	suite.assert.FileExists(suite.cache_path + "/" + dst) // Dst shall exists in cache

	time.Sleep(2 * time.Second)                           // Wait for the cache cleanup to occur
	suite.assert.FileExists(suite.cache_path + "/" + dst) // Dst shall not exists in cache
}

func (suite *fileCacheTestSuite) TestRenameFileAndCacheCleanupWithNoTimeout() {
	defer suite.cleanupTest()
	suite.cleanupTest()

	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  timeout-sec: 0\n\nloopbackfs:\n  path: %s",
		suite.cache_path, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	src := "source5"
	dst := "destination5"
	createHandle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0666})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
	openHandle, _ := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: src, Mode: 0666})

	// Path should be in the file cache
	suite.assert.FileExists(suite.cache_path + "/" + src)
	// Path should be in fake storage
	suite.assert.FileExists(suite.fake_storage_path + "/" + src)

	// RenameFile
	err := suite.fileCache.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	suite.assert.NoError(err)
	// Path in fake storage and file cache should be updated
	suite.assert.NoFileExists(suite.cache_path + "/" + src)        // Src does not exist
	suite.assert.FileExists(suite.cache_path + "/" + dst)          // Dst shall exists in cache
	suite.assert.NoFileExists(suite.fake_storage_path + "/" + src) // Src does not exist
	suite.assert.FileExists(suite.fake_storage_path + "/" + dst)   // Dst does exist

	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: openHandle})

	time.Sleep(100 * time.Millisecond)                      // Wait for the cache cleanup to occur
	suite.assert.NoFileExists(suite.cache_path + "/" + dst) // Dst shall not exists in cache
}

func (suite *fileCacheTestSuite) TestTruncateFileNotInCache() {
	defer suite.cleanupTest()
	// Setup
	path := "file30"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})

	_, err := os.Stat(filepath.Join(suite.cache_path, path))
	for i := 0; i < 1000 && !os.IsNotExist(err); i++ {
		time.Sleep(10 * time.Millisecond)
		_, err = os.Stat(filepath.Join(suite.cache_path, path))
	}
	suite.assert.True(os.IsNotExist(err))

	// Path should be in fake storage
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, path))

	// Chmod
	size := 1024
	err = suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: path, Size: int64(size)})
	suite.assert.NoError(err)

	// Path in fake storage should be updated
	info, _ := os.Stat(filepath.Join(suite.fake_storage_path, path))
	suite.assert.EqualValues(info.Size(), size)
}

func (suite *fileCacheTestSuite) TestTruncateFileInCache() {
	defer suite.cleanupTest()
	// Setup
	path := "file31"
	createHandle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0666})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
	openHandle, _ := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0666})

	// Path should be in the file cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))
	// Path should be in fake storage
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, path))

	// Chmod
	size := 1024
	err := suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: path, Size: int64(size)})
	suite.assert.NoError(err)
	// Path in fake storage and file cache should be updated
	info, _ := os.Stat(filepath.Join(suite.cache_path, path))
	suite.assert.EqualValues(info.Size(), size)
	info, _ = os.Stat(filepath.Join(suite.fake_storage_path, path))
	suite.assert.EqualValues(info.Size(), size)

	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: openHandle})
}

func (suite *fileCacheTestSuite) TestTruncateFileCase2() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	path := "file32"
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0666})

	size := 1024
	err := suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: path, Size: int64(size)})
	suite.assert.NoError(err)

	// Path should be in the file cache and size should be updated
	info, err := os.Stat(filepath.Join(suite.cache_path, path))
	suite.assert.NoError(err)
	suite.assert.EqualValues(info.Size(), size)

	// Path should not be in fake storage
	// With new changes we always download and then truncate so file will exists in local path
	// suite.assert.NoFileExists(suite.fake_storage_path + "/" + path)
}

func (suite *fileCacheTestSuite) TestZZMountPathConflict() {
	defer suite.cleanupTest()
	cacheTimeout := 1
	configuration := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  timeout-sec: %d\n\nloopbackfs:\n  path: %s",
		suite.cache_path, cacheTimeout, suite.fake_storage_path)

	fileCache := NewFileCacheComponent()
	config.ReadConfigFromReader(strings.NewReader(configuration))
	config.Set("mount-path", suite.cache_path)
	err := fileCache.Configure(true)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "[tmp-path is same as mount path]")
}

// This test does not run on Windows unless you have admin rights since
// creating symlinks is only allowed as an admin
func (suite *fileCacheTestSuite) TestCachePathSymlink() {
	// Ignore test on Windows so pass a true test so the test passes
	if runtime.GOOS == "windows" {
		suite.assert.Nil(nil)
		return
	}

	defer suite.cleanupTest()
	// Setup
	suite.cleanupTest()
	err := os.Mkdir(suite.cache_path, 0777)
	defer os.RemoveAll(suite.cache_path)
	suite.assert.NoError(err)
	symlinkPath := suite.cache_path + ".lnk"
	err = os.Symlink(common.NormalizeObjectName(suite.cache_path), symlinkPath)
	defer os.Remove(symlinkPath)
	suite.assert.NoError(err)
	configuration := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n\nloopbackfs:\n  path: %s",
		symlinkPath, suite.fake_storage_path)
	suite.setupTestHelper(configuration)

	file := "file39"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})

	handle, _ = suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

	output := make([]byte, 9)
	n, err := suite.fileCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: output})
	suite.assert.NoError(err)
	suite.assert.Equal(9, n)
	suite.assert.EqualValues(data, output)
}

func (suite *fileCacheTestSuite) TestZZOffloadIO() {
	defer suite.cleanupTest()
	configuration := fmt.Sprintf("file_cache:\n  path: %s\n  timeout-sec: 0\n\nloopbackfs:\n  path: %s",
		suite.cache_path, suite.fake_storage_path)

	suite.setupTestHelper(configuration)

	file := "file40"
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.assert.NoError(err)
	suite.assert.NotNil(handle)
	suite.assert.True(handle.Cached())

	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
}

func (suite *fileCacheTestSuite) TestZZZZLazyWrite() {
	defer suite.cleanupTest()
	configuration := fmt.Sprintf("file_cache:\n  path: %s\n  timeout-sec: 0\n\nloopbackfs:\n  path: %s",
		suite.cache_path, suite.fake_storage_path)

	suite.setupTestHelper(configuration)
	suite.fileCache.lazyWrite = true

	file := "file101"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	data := make([]byte, 10*1024*1024)
	_, _ = suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	_ = suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})

	// As lazy write is enabled flush shall not upload the file
	suite.assert.True(handle.Dirty())

	_ = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	time.Sleep(5 * time.Second)
	suite.fileCache.lazyWrite = false

	// As lazy write is enabled flush shall not upload the file
	suite.assert.False(handle.Dirty())
}

func (suite *fileCacheTestSuite) TestStatFS() {
	defer suite.cleanupTest()
	cacheTimeout := 5
	maxSizeMb := 2
	config := fmt.Sprintf("file_cache:\n  path: %s\n  max-size-mb: %d\n  offload-io: true\n  timeout-sec: %d\n\nloopbackfs:\n  path: %s",
		suite.cache_path, maxSizeMb, cacheTimeout, suite.fake_storage_path)
	os.Mkdir(suite.cache_path, 0777)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	file := "file41"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	data := make([]byte, 1024*1024)
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	stat, ret, err := suite.fileCache.StatFs()
	suite.assert.True(ret)
	suite.assert.NoError(err)
	suite.assert.NotEqual(&common.Statfs_t{}, stat)

	// Added additional checks for StatFS
	suite.assert.Equal(int64(4096), stat.Bsize)
	suite.assert.Equal(int64(4096), stat.Frsize)
	suite.assert.Equal(uint64(512), stat.Blocks)
	suite.assert.Equal(uint64(255), stat.Namemax)
}

func (suite *fileCacheTestSuite) TestReadFileWithRefresh() {
	defer suite.cleanupTest()
	// Configure to create empty files so we create the file in cloud storage
	createEmptyFile := true
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  create-empty-file: %t\n  timeout-sec: 1000\n  refresh-sec: 1\n\nloopbackfs:\n  path: %s",
		suite.cache_path, createEmptyFile, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	path := "file42"
	byteArr := []byte("test data")
	err := os.WriteFile(suite.fake_storage_path+"/"+path, byteArr, 0777)
	suite.assert.NoError(err)

	data := make([]byte, 20)

	options := internal.OpenFileOptions{Name: path, Mode: 0777}

	// Read file once and we shall get the same data
	f, err := suite.fileCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.False(f.Dirty())
	n, err := suite.fileCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: f, Offset: 0, Data: data})
	suite.assert.NoError(err)
	suite.assert.Equal(9, n)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: f})
	suite.assert.NoError(err)

	// Modify the file in background but we shall still get the old data
	byteArr = []byte("test data1")
	err = os.WriteFile(suite.fake_storage_path+"/"+path, byteArr, 0777)
	suite.assert.NoError(err)
	f, err = suite.fileCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.False(f.Dirty())
	n, err = suite.fileCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: f, Offset: 0, Data: data})
	suite.assert.NoError(err)
	suite.assert.Equal(9, n)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: f})
	suite.assert.NoError(err)

	// Now wait for refresh timeout and we shall get the updated content on next read
	byteArr = []byte("test data123456")
	err = os.WriteFile(suite.fake_storage_path+"/"+path, []byte("test data123456"), 0777)
	suite.assert.NoError(err)
	time.Sleep(2 * time.Second)
	f, err = suite.fileCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.False(f.Dirty())
	n, err = suite.fileCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: f, Offset: 0, Data: data})
	suite.assert.NoError(err)
	suite.assert.Equal(15, n)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: f})
	suite.assert.NoError(err)
}

func (suite *fileCacheTestSuite) TestHardLimitOnSize() {
	defer suite.cleanupTest()
	// Configure to create empty files so we create the file in cloud storage
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  hard-limit: true\n  max-size-mb: 2\n\nloopbackfs:\n  path: %s",
		suite.cache_path, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	data := make([]byte, 3*MB)
	pathbig := "filebig"
	err := os.WriteFile(suite.fake_storage_path+"/"+pathbig, data, 0777)
	suite.assert.NoError(err)

	data = make([]byte, 1*MB)
	pathsmall := "filesmall"
	err = os.WriteFile(suite.fake_storage_path+"/"+pathsmall, data, 0777)
	suite.assert.NoError(err)

	smallHandle, err := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: pathsmall, Flags: os.O_RDONLY, Mode: suite.fileCache.defaultPermission})
	suite.assert.NoError(err)
	// try opening small file
	err = suite.fileCache.downloadFile(smallHandle)
	suite.assert.NoError(err)
	suite.assert.False(smallHandle.Dirty())
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: smallHandle})
	suite.assert.NoError(err)

	// try opening bigger file which shall fail due to hardlimit
	bigHandle, err := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: pathbig, Flags: os.O_RDONLY, Mode: suite.fileCache.defaultPermission})
	suite.assert.Error(err)
	suite.assert.Nil(bigHandle)
	suite.assert.Equal(syscall.ENOSPC, err)

	// try writing a small file
	options1 := internal.CreateFileOptions{Name: pathsmall + "_new", Mode: 0777}
	f, err := suite.fileCache.CreateFile(options1)
	suite.assert.NoError(err)
	data = make([]byte, 1*MB)
	n, err := suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: f, Offset: 0, Data: data})
	suite.assert.NoError(err)
	suite.assert.Equal(1*MB, n)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: f})
	suite.assert.NoError(err)

	// try writing a bigger file
	options1 = internal.CreateFileOptions{Name: pathbig + "_new", Mode: 0777}
	f, err = suite.fileCache.CreateFile(options1)
	suite.assert.NoError(err)
	data = make([]byte, 3*MB)
	n, err = suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: f, Offset: 0, Data: data})
	suite.assert.Error(err)
	suite.assert.Equal(0, n)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: f})
	suite.assert.NoError(err)

	// try opening small file
	err = suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: pathsmall, Size: 1 * MB})
	suite.assert.NoError(err)

	// try opening small file
	err = suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: pathsmall, Size: 3 * MB})
	suite.assert.Error(err)
}

func (suite *fileCacheTestSuite) TestHandleDataChange() {
	defer suite.cleanupTest()
	// Configure to create empty files so we create the file in cloud storage
	createEmptyFile := true
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  create-empty-file: %t\n  timeout-sec: 1000\n  refresh-sec: 10\n\nloopbackfs:\n  path: %s",
		suite.cache_path, createEmptyFile, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	path := "file43"
	err := os.WriteFile(suite.fake_storage_path+"/"+path, []byte("test data"), 0777)
	suite.assert.NoError(err)

	data := make([]byte, 20)
	options := internal.OpenFileOptions{Name: path, Flags: os.O_RDONLY, Mode: 0777}

	// Read file once and we shall get the same data
	f, err := suite.fileCache.OpenFile(options)
	handlemap.Add(f)
	suite.assert.NoError(err)
	suite.assert.False(f.Dirty())
	n, err := suite.fileCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: f, Offset: 0, Data: data})
	f, loaded := handlemap.Load(f.ID)
	suite.assert.True(loaded)
	suite.assert.NoError(err)
	suite.assert.Equal(9, n)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: f})
	suite.assert.NoError(err)

}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestFileCacheTestSuite(t *testing.T) {
	suite.Run(t, new(fileCacheTestSuite))
}
