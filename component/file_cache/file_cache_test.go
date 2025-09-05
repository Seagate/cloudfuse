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
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path"
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
	"github.com/golang/mock/gomock"

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
	useMock           bool
	mockCtrl          *gomock.Controller
	mock              *internal.MockComponent
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
	defaultConfig := fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true\n\nloopbackfs:\n  path: %s",
		suite.cache_path,
		suite.fake_storage_path,
	)
	suite.useMock = false
	log.Debug(defaultConfig)

	// Delete the temp directories created
	err = os.RemoveAll(suite.cache_path)
	if err != nil {
		fmt.Printf(
			"fileCacheTestSuite::SetupTest : os.RemoveAll(%s) failed [%v]\n",
			suite.cache_path,
			err,
		)
	}
	err = os.RemoveAll(suite.fake_storage_path)
	if err != nil {
		fmt.Printf(
			"fileCacheTestSuite::SetupTest : os.RemoveAll(%s) failed [%v]\n",
			suite.fake_storage_path,
			err,
		)
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
	suite.assert.NoError(err)
	err = os.RemoveAll(suite.fake_storage_path)
	suite.assert.NoError(err)
}

// Tests the default configuration of file cache
func (suite *fileCacheTestSuite) TestEmpty() {
	defer suite.cleanupTest()
	suite.cleanupTest() // teardown the default file cache generated
	emptyConfig := fmt.Sprintf(
		"file_cache:\n  path: %s\n\n  offload-io: true\n\nloopbackfs:\n  path: %s",
		suite.cache_path,
		suite.fake_storage_path,
	)
	suite.setupTestHelper(
		emptyConfig,
	) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	suite.assert.Equal("file_cache", suite.fileCache.Name())
	suite.assert.Equal(suite.fileCache.tmpPath, suite.cache_path)
	suite.assert.Equal("lru", suite.fileCache.policy.Name())

	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxEviction, defaultMaxEviction)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).highThreshold, defaultMaxThreshold)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).lowThreshold, defaultMinThreshold)

	suite.assert.False(suite.fileCache.createEmptyFile)
	suite.assert.False(suite.fileCache.allowNonEmpty)
	suite.assert.EqualValues(suite.fileCache.cacheTimeout, 216000)
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
	config := fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true\n  policy: %s\n  max-size-mb: %d\n  timeout-sec: %d\n  max-eviction: %d\n  high-threshold: %d\n  low-threshold: %d\n  create-empty-file: %t\n  allow-non-empty-temp: %t\n  cleanup-on-start: %t\n  sync-to-flush: %t",
		suite.cache_path,
		policy,
		maxSizeMb,
		cacheTimeout,
		maxDeletion,
		highThreshold,
		lowThreshold,
		createEmptyFile,
		allowNonEmptyTemp,
		cleanupOnStart,
		syncToFlush,
	)
	suite.setupTestHelper(
		config,
	) // setup a new file cache with a custom config (teardown will occur after the test as usual)

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
}

func (suite *fileCacheTestSuite) TestDefaultCacheSize() {
	defer suite.cleanupTest()
	// Setup
	config := fmt.Sprintf("file_cache:\n  path: %s\n", suite.cache_path)
	suite.setupTestHelper(
		config,
	) // setup a new file cache with a custom config (teardown will occur after the test as usual)
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
		suite.assert.NoError(err)
		freeDisk, err = strconv.Atoi(strings.TrimSpace(out.String()))
		suite.assert.NoError(err)
	}
	expected := uint64(0.8 * float64(freeDisk))
	actual := suite.fileCache.maxCacheSize * MB
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
	config := fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true\n  policy: %s\n  max-size-mb: %d\n  timeout-sec: %d\n  max-eviction: %d\n  high-threshold: %d\n  low-threshold: %d\n  create-empty-file: %t\n  allow-non-empty-temp: %t\n  cleanup-on-start: %t",
		suite.cache_path,
		policy,
		maxSizeMb,
		cacheTimeout,
		maxDeletion,
		highThreshold,
		lowThreshold,
		createEmptyFile,
		allowNonEmptyTemp,
		cleanupOnStart,
	)
	suite.setupTestHelper(
		config,
	) // setup a new file cache with a custom config (teardown will occur after the test as usual)

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
	config := fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true\n  policy: %s\n  max-size-mb: %d\n  max-eviction: %d\n  high-threshold: %d\n  low-threshold: %d\n  create-empty-file: %t\n  allow-non-empty-temp: %t\n  cleanup-on-start: %t",
		suite.cache_path,
		policy,
		maxSizeMb,
		maxDeletion,
		highThreshold,
		lowThreshold,
		createEmptyFile,
		allowNonEmptyTemp,
		cleanupOnStart,
	)
	suite.setupTestHelper(
		config,
	) // setup a new file cache with a custom config (teardown will occur after the test as usual)

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
	config := fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true\n  policy: %s\n  max-size-mb: %d\n  timeout-sec: %d\n  max-eviction: %d\n  high-threshold: %d\n  low-threshold: %d\n  create-empty-file: %t\n  allow-non-empty-temp: %t\n  cleanup-on-start: %t",
		suite.cache_path,
		policy,
		maxSizeMb,
		cacheTimeout,
		maxDeletion,
		highThreshold,
		lowThreshold,
		createEmptyFile,
		allowNonEmptyTemp,
		cleanupOnStart,
	)
	suite.setupTestHelper(
		config,
	) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	suite.assert.Equal("file_cache", suite.fileCache.Name())
	suite.assert.Equal(suite.fileCache.tmpPath, suite.cache_path)
	suite.assert.Equal(suite.fileCache.policy.Name(), policy)

	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxSizeMB, maxSizeMb)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxEviction, maxDeletion)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).highThreshold, highThreshold)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).lowThreshold, lowThreshold)

	suite.assert.Equal(suite.fileCache.createEmptyFile, createEmptyFile)
	suite.assert.Equal(suite.fileCache.allowNonEmpty, allowNonEmptyTemp)
	suite.assert.EqualValues(suite.fileCache.cacheTimeout, minimumFileCacheTimeout)
}

func (suite *fileCacheTestSuite) TestDefaultFilePath() {
	defer suite.cleanupTest()
	suite.cleanupTest() // teardown the default file cache generated
	config := "file_cache:\n  offload-io: true"
	suite.setupTestHelper(
		config,
	)

	suite.assert.Equal("file_cache", suite.fileCache.Name())
	homeDir, err := os.UserHomeDir()
	suite.assert.NoError(err)
	suite.assert.Equal(filepath.Join(homeDir, ".cloudfuse", "file_cache"), suite.fileCache.tmpPath)
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
	handle, _ := suite.loopback.CreateFile(internal.CreateFileOptions{Name: file1})
	suite.loopback.CloseFile(internal.CloseFileOptions{Handle: handle})
	handle, _ = suite.loopback.CreateFile(internal.CreateFileOptions{Name: file2})
	suite.loopback.CloseFile(internal.CloseFileOptions{Handle: handle})
	handle, _ = suite.loopback.CreateFile(internal.CreateFileOptions{Name: file3})
	suite.loopback.CloseFile(internal.CloseFileOptions{Handle: handle})

	// Read the Directory
	dir, _, err := suite.fileCache.StreamDir(internal.StreamDirOptions{Name: name})
	suite.assert.NoError(err)
	suite.assert.NotEmpty(dir)
	suite.assert.Len(dir, 4)
	suite.assert.Equal(file1, dir[0].Path)
	suite.assert.Equal(file2, dir[1].Path)
	suite.assert.Equal(file3, dir[2].Path)
	suite.assert.Equal(subdir, dir[3].Path)
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
	suite.assert.Equal(subdir, dir[0].Path)
	suite.assert.Equal(file1, dir[1].Path)
	suite.assert.Equal(file2, dir[2].Path)
	suite.assert.Equal(file3, dir[3].Path)
}

func (suite *fileCacheTestSuite) TestStreamDirCase3() {
	defer suite.cleanupTest()
	suite.fileCache.createEmptyFile = true
	// Setup
	name := "dir"
	subdir := name + "/subdir"
	file1 := name + "/file1"
	file2 := name + "/file2"
	file3 := name + "/file3"
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: name, Mode: 0777})
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})
	// Truncate causes these files to be written to fake storage
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file1, Mode: 0777})
	suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: file1, Size: 1024})
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file2, Mode: 0777})
	suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: file2, Size: 1024})
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file3, Mode: 0777})
	suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: file3, Size: 1024})
	// Change the sizes directly in fake storage
	suite.loopback.TruncateFile(internal.TruncateFileOptions{Name: file1}) // Length is default 0
	suite.loopback.TruncateFile(internal.TruncateFileOptions{Name: file2})
	suite.loopback.TruncateFile(internal.TruncateFileOptions{Name: file3})

	// Read the Directory
	dir, _, err := suite.fileCache.StreamDir(internal.StreamDirOptions{Name: name})
	suite.assert.NoError(err)
	suite.assert.NotEmpty(dir)
	suite.assert.Len(dir, 4)
	suite.assert.Equal(file1, dir[0].Path)
	suite.assert.EqualValues(1024, dir[0].Size)
	suite.assert.Equal(file2, dir[1].Path)
	suite.assert.EqualValues(1024, dir[1].Size)
	suite.assert.Equal(file3, dir[2].Path)
	suite.assert.EqualValues(1024, dir[2].Size)
	suite.assert.Equal(subdir, dir[3].Path)
	suite.fileCache.createEmptyFile = false
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
	handle, _ := suite.loopback.CreateFile(
		internal.CreateFileOptions{Name: file1, Mode: 0777},
	) // Length is default 0
	suite.loopback.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.loopback.TruncateFile(internal.TruncateFileOptions{Name: file3})
	handle, _ = suite.loopback.CreateFile(
		internal.CreateFileOptions{Name: file4, Mode: 0777},
	) // Length is default 0
	suite.loopback.CloseFile(internal.CloseFileOptions{Handle: handle})
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
	src := "src"
	dst := "dst"
	err := suite.fileCache.CreateDir(internal.CreateDirOptions{Name: src, Mode: 0777})
	suite.assert.NoError(err)
	path := src + "/file"
	for i := range 5 {
		handle, err := suite.fileCache.CreateFile(
			internal.CreateFileOptions{Name: path + strconv.Itoa(i), Mode: 0777},
		)
		suite.assert.NoError(err)
		err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
		suite.assert.NoError(err)
	}
	// The file (and directory) is in the cache and storage (see TestCreateFileInDirCreateEmptyFile)

	// Rename the directory
	err = suite.fileCache.RenameDir(internal.RenameDirOptions{Src: src, Dst: dst})
	suite.assert.NoError(err)
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

// Combined test for all three cases
func (suite *fileCacheTestSuite) TestRenameDirOpenFile() {
	defer suite.cleanupTest()

	// Setup
	srcDir := "src"
	dstDir := "dst"
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: srcDir, Mode: 0777})
	//
	// Case 1
	case1src := srcDir + "/fileCase1"
	case1dst := dstDir + "/fileCase1"
	// create file in cloud
	tempHandle, _ := suite.loopback.CreateFile(
		internal.CreateFileOptions{Name: case1src, Mode: 0777},
	)
	suite.loopback.CloseFile(internal.CloseFileOptions{Handle: tempHandle})
	// open file for writing
	handle1, err := suite.fileCache.OpenFile(
		internal.OpenFileOptions{Name: case1src, Flags: os.O_RDWR, Mode: 0777},
	)
	suite.assert.NoError(err)
	handlemap.Add(handle1)
	// Path should not be in the file cache (lazy open)
	suite.assert.NoFileExists(suite.cache_path + "/" + case1src)
	//
	// Case 2
	case2src := srcDir + "/fileCase2"
	case2dst := dstDir + "/fileCase2"
	// create source file
	handle2, err := suite.fileCache.CreateFile(
		internal.CreateFileOptions{Name: case2src, Mode: 0666},
	)
	suite.assert.NoError(err)
	handlemap.Add(handle2)
	// Path should only be in the file cache
	suite.assert.FileExists(suite.cache_path + "/" + case2src)
	suite.assert.NoFileExists(suite.fake_storage_path + "/" + case2src)
	//
	// Case 3
	case3src := srcDir + "/fileCase3"
	case3dst := dstDir + "/fileCase3"
	// create source file
	handle3, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: case3src, Mode: 0666})
	handlemap.Add(handle3)
	// Path should be in the file cache
	suite.assert.FileExists(suite.cache_path + "/" + case3src)
	// write and flush to cloud
	initialData := []byte("initialData")
	n, err := suite.fileCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle3,
		Data:   initialData,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(len(initialData), n)
	err = suite.fileCache.FlushFile(internal.FlushFileOptions{
		Handle: handle3,
	})
	suite.assert.NoError(err)
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, case3src))

	// Test: Rename the directory
	err = suite.fileCache.RenameDir(internal.RenameDirOptions{Src: srcDir, Dst: dstDir})
	suite.assert.NoError(err)
	//
	// Case 1
	// rename succeeded in cloud
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, case1src))
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, case1dst))
	// still in lazy open state
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, case1src))
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, case1dst))
	//
	// Case 2
	// local rename succeeded
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, case2src))
	suite.assert.FileExists(filepath.Join(suite.cache_path, case2dst))
	// file still in case 2
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, case2src))
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, case2dst))
	//
	// Case 3
	// local rename succeeded
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, case3src))
	suite.assert.FileExists(filepath.Join(suite.cache_path, case3dst))
	// cloud rename succeeded
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, case3src))
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, case3dst))

	// Test: write new data
	data := []byte("newdata")
	//
	// Case 1
	// write to file handle
	n, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle1,
		Data:   data,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(len(data), n)
	// open is completed (file is downloaded), and writes go to the correct file
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, case1src))
	suite.assert.FileExists(filepath.Join(suite.cache_path, case1dst))
	//
	// Case 2
	n, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle2,
		Data:   data,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(len(data), n)
	//
	// Case 3
	n, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle3,
		Data:   data,
		Offset: int64(len(initialData)),
	})
	suite.assert.NoError(err)
	suite.assert.Equal(len(data), n)

	// Test: Close handle
	//
	// Case 1
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{
		Handle: handle1,
	})
	suite.assert.NoError(err)
	// check cloud data
	dstData, err := os.ReadFile(path.Join(suite.fake_storage_path, case1dst))
	suite.assert.NoError(err)
	suite.assert.Equal(data, dstData)
	//
	// Case 2
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{
		Handle: handle2,
	})
	suite.assert.NoError(err)
	// check cloud data
	dstData, err = os.ReadFile(path.Join(suite.fake_storage_path, case2dst))
	suite.assert.NoError(err)
	suite.assert.Equal(data, dstData)
	//
	// Case 3
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{
		Handle: handle3,
	})
	suite.assert.NoError(err)
	// check cloud data
	dstData, err = os.ReadFile(path.Join(suite.fake_storage_path, case3dst))
	suite.assert.NoError(err)
	suite.assert.Equal(append(initialData, data...), dstData)
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
	config := fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true\n  create-empty-file: %t\n\nloopbackfs:\n  path: %s",
		suite.cache_path,
		createEmptyFile,
		suite.fake_storage_path,
	)
	suite.setupTestHelper(
		config,
	) // setup a new file cache with a custom config (teardown will occur after the test as usual)

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
	config := fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true\n  create-empty-file: %t\n\nloopbackfs:\n  path: %s",
		suite.cache_path,
		createEmptyFile,
		suite.fake_storage_path,
	)
	suite.setupTestHelper(
		config,
	) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	dir := "dir"
	path := dir + "/file"
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: dir, Mode: 0777})
	f, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.assert.NoError(err)
	suite.assert.False(
		f.Dirty(),
	) // Handle should be dirty since it was not created in cloud storage

	// Path should be added to the file cache, including directory
	suite.assert.DirExists(filepath.Join(suite.cache_path, dir))
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))
	// Path should be in fake storage, including directory
	suite.assert.DirExists(filepath.Join(suite.fake_storage_path, dir))
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, path))
}

func (suite *fileCacheTestSuite) TestChmodNonexistentCreateEmptyFile() {
	defer suite.cleanupTest()
	// Set flag high to test bugfix
	createEmptyFile := true
	config := fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true\n  create-empty-file: %t\n\nloopbackfs:\n  path: %s",
		suite.cache_path,
		createEmptyFile,
		suite.fake_storage_path,
	)
	suite.setupTestHelper(
		config,
	) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	path := "file"
	err := suite.fileCache.Chmod(internal.ChmodOptions{Name: path, Mode: 0777})
	suite.assert.ErrorIs(err, os.ErrNotExist)
}

func (suite *fileCacheTestSuite) TestSyncFile() {
	defer suite.cleanupTest()

	suite.fileCache.syncToFlush = false
	path := "file3"

	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})

	// On a sync we open, sync, flush and close
	handle, err := suite.fileCache.OpenFile(
		internal.OpenFileOptions{Name: path, Flags: os.O_RDWR, Mode: 0777},
	)
	handlemap.Add(handle)
	suite.assert.NoError(err)
	err = suite.fileCache.SyncFile(internal.SyncFileOptions{Handle: handle})
	suite.assert.NoError(err)
	testData := "test data"
	data := []byte(testData)

	suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
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
	_, err = suite.fileCache.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)
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

func (suite *fileCacheTestSuite) TestDeleteOpenFileCase1() {
	defer suite.cleanupTest()
	path := "file"

	// setup
	// Create file directly in "fake_storage" and open in case 1 (lazy open)
	handle, _ := suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.loopback.CloseFile(internal.CloseFileOptions{Handle: handle})
	handle, _ = suite.fileCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0777})

	// Test
	err := suite.fileCache.DeleteFile(internal.DeleteFileOptions{Name: path})
	suite.assert.NoError(err)
	// Path should not be in fake storage
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, path))

	// cleanup
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
}

// Case 2 Test cover when the file does not exist in cloud storage but it exists in the local cache.
// This can happen if createEmptyFile is false and the file hasn't been flushed yet.
func (suite *fileCacheTestSuite) TestDeleteOpenFileCase2() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	path := "file5"
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})

	err := suite.fileCache.DeleteFile(internal.DeleteFileOptions{Name: path})
	suite.assert.NoError(err)

	// Path should not be in local cache (the delete succeeded)
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, path))
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
	handle, _ := suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	suite.loopback.WriteFile(&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.loopback.CloseFile(internal.CloseFileOptions{Handle: handle})

	handle, err := suite.fileCache.OpenFile(
		internal.OpenFileOptions{
			Name:  path,
			Flags: os.O_RDWR,
			Mode:  suite.fileCache.defaultPermission,
		},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(path, handle.Path)
	suite.assert.False(handle.Dirty())

	// File should not exist in cache
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, path))
}

func (suite *fileCacheTestSuite) TestOpenFileInCache() {
	defer suite.cleanupTest()
	path := "file8"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})

	// Download is required
	handle, err := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0777})
	suite.assert.NoError(err)
	suite.assert.Equal(path, handle.Path)
	suite.assert.False(handle.Dirty())

	// File should exist in cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))
}

func (suite *fileCacheTestSuite) TestOpenCreateGetAttr() {
	defer suite.cleanupTest()
	path := "file8a"

	// we report file does not exist before it is created
	attr, err := suite.fileCache.GetAttr(internal.GetAttrOptions{Name: path})
	suite.assert.Nil(attr)
	suite.assert.ErrorIs(err, os.ErrNotExist)
	// since it does not exist, we allow the file to be created using OpenFile
	handle, err := suite.fileCache.OpenFile(
		internal.OpenFileOptions{Name: path, Flags: os.O_CREATE, Mode: 0777},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(path, handle.Path)
	// we should report that the file exists now
	attr, err = suite.fileCache.GetAttr(internal.GetAttrOptions{Name: path})
	suite.assert.NoError(err)
	suite.NotNil(attr)
}

// Tests for GetProperties in OpenFile should be done in E2E tests
// - there is no good way to test it here with a loopback FS without a mock component.

func (suite *fileCacheTestSuite) TestCloseFileAndEvict() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	configuration := fmt.Sprintf(
		"file_cache:\n  path: %s\n  timeout-sec: %d\n\nloopbackfs:\n  path: %s",
		suite.cache_path,
		minimumFileCacheTimeout,
		suite.fake_storage_path,
	)
	suite.setupTestHelper(configuration)

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

	time.Sleep(minimumFileCacheTimeout * time.Second)
	// loop until file does not exist - done due to async nature of eviction
	_, err = os.Stat(filepath.Join(suite.cache_path, path))
	for i := 0; i < 30*minimumFileCacheTimeout && !os.IsNotExist(err); i++ {
		time.Sleep(100 * time.Millisecond)
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

	suite.cleanupTest()
	configuration := fmt.Sprintf(
		"file_cache:\n  path: %s\n  timeout-sec: %d\n\nloopbackfs:\n  path: %s",
		suite.cache_path,
		minimumFileCacheTimeout,
		suite.fake_storage_path,
	)
	suite.setupTestHelper(configuration)

	// Setup
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
	time.Sleep(2*minimumFileCacheTimeout*time.Second + 100*time.Millisecond)

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
	length, err := suite.fileCache.ReadInBuffer(
		&internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(0, length)
	suite.assert.Empty(data)
}

func (suite *fileCacheTestSuite) TestReadInBufferNoFlush() {
	defer suite.cleanupTest()
	// Setup
	file := "file16"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})

	handle, _ = suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

	output := make([]byte, 9)
	length, err := suite.fileCache.ReadInBuffer(
		&internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: output},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(data, output)
	suite.assert.Equal(len(data), length)
}

func (suite *fileCacheTestSuite) TestReadInBuffer() {
	defer suite.cleanupTest()
	// Setup
	file := "file17"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})

	handle, _ = suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

	output := make([]byte, 9)
	length, err := suite.fileCache.ReadInBuffer(
		&internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: output},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(data, output)
	suite.assert.Equal(len(data), length)
}

func (suite *fileCacheTestSuite) TestReadInBufferErrorBadFd() {
	defer suite.cleanupTest()
	// Setup
	file := "file18"
	handle := handlemap.NewHandle(file)
	length, err := suite.fileCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: handle})
	suite.assert.Error(err)
	suite.assert.EqualValues(syscall.EBADF, err)
	suite.assert.Equal(0, length)
}

func (suite *fileCacheTestSuite) TestWriteFile() {
	defer suite.cleanupTest()
	// Setup
	file := "file19"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

	handle.Flags.Clear(
		handlemap.HandleFlagDirty,
	) // Technically create file will mark it as dirty, we just want to check write file updates the dirty flag, so temporarily set this to false
	testData := "test data"
	data := []byte(testData)
	length, err := suite.fileCache.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)

	suite.assert.NoError(err)
	suite.assert.Equal(len(data), length)
	// Check that the local cache updated with data
	d, _ := os.ReadFile(filepath.Join(suite.cache_path, file))
	suite.assert.Equal(data, d)
	suite.assert.True(handle.Dirty())
}

func (suite *fileCacheTestSuite) TestWriteFileErrorBadFd() {
	defer suite.cleanupTest()
	// Setup
	file := "file20"
	handle := handlemap.NewHandle(file)
	len, err := suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle})
	suite.assert.Error(err)
	suite.assert.EqualValues(syscall.EBADF, err)
	suite.assert.Equal(0, len)
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
	suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})

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
	suite.assert.Equal(data, d)
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

func (suite *fileCacheTestSuite) TestCronOffToONUpload() {
	defer suite.cleanupTest()

	testStartTime := time.Now()
	second := (testStartTime.Second() + 2) % 60
	cronExpr := fmt.Sprintf("%d * * * * *", second)

	configContent := fmt.Sprintf(`file_cache:
  path: %s
  offload-io: true
  create-empty-file: false
  schedule:
    - name: "Test"
      cron: %s
      duration: "30s"

loopbackfs:
  path: %s`,
		suite.cache_path,
		cronExpr,
		suite.fake_storage_path,
	)

	// Setup the file cache with this configuration
	suite.setupTestHelper(configContent)

	file := "simple_scheduledlol.txt"
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.assert.NoError(err)

	data := []byte("simple scheduled upload test data")
	_, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle, Data: data})
	suite.assert.NoError(err)

	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	suite.assert.FileExists(filepath.Join(suite.cache_path, file))
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, file))

	_, exists := suite.fileCache.scheduleOps.Load(file)
	suite.assert.True(exists, "File should be in scheduleOps after creation")

	// wait for uploads to start
	time.Sleep(time.Until(testStartTime.Add(2 * time.Second).Truncate(time.Second)))
	_, err = os.Stat(filepath.Join(suite.fake_storage_path, file))
	for i := 0; i < 200 && os.IsNotExist(err); i++ {
		time.Sleep(10 * time.Millisecond)
		_, err = os.Stat(filepath.Join(suite.fake_storage_path, file))
	}
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, file))
	_, exists = suite.fileCache.scheduleOps.Load(file)
	suite.assert.False(exists, "File should have been removed from scheduleOps after upload")
	suite.assert.False(
		suite.fileCache.fileLocks.Get(file).SyncPending,
		"SyncPending flag should be cleared after upload",
	)
}

func (suite *fileCacheTestSuite) TestCronOnToOFFUpload() {
	defer suite.cleanupTest()

	testStartTime := time.Now()
	second := testStartTime.Second() % 60
	duration := 2 * time.Second
	cronExpr := fmt.Sprintf("%d * * * * *", second)

	configContent := fmt.Sprintf(`file_cache:
  path: %s
  offload-io: true
  create-empty-file: false
  schedule:
    - name: "Test"
      cron: %s
      duration: "%ds"

loopbackfs:
  path: %s`,
		suite.cache_path,
		cronExpr,
		int(duration.Seconds()),
		suite.fake_storage_path,
	)

	// Setup the file cache with this configuration
	suite.setupTestHelper(configContent)

	file1 := "scheduled_on_window.txt"
	handle1, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file1, Mode: 0777})
	suite.assert.NoError(err)
	data1 := []byte("file created during scheduler ON window")
	_, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle1, Data: data1})
	suite.assert.NoError(err)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle1})
	suite.assert.NoError(err)
	suite.assert.FileExists(filepath.Join(suite.cache_path, file1))

	_, err = os.Stat(filepath.Join(suite.fake_storage_path, file1))
	for i := 0; i < 200 && os.IsNotExist(err); i++ {
		time.Sleep(10 * time.Millisecond)
		_, err = os.Stat(filepath.Join(suite.fake_storage_path, file1))
	}
	suite.FileExists(
		filepath.Join(suite.fake_storage_path, file1),
		"First file should be uploaded when scheduler is ON",
	)

	// wait until the window closes
	time.Sleep(time.Since(testStartTime.Add(duration + 10*time.Millisecond)))

	file2 := "scheduled_off_window.txt"
	handle2, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file2, Mode: 0777})
	suite.assert.NoError(err)
	data2 := []byte("file created during scheduler OFF window")
	_, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle2, Data: data2})
	suite.assert.NoError(err)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle2})
	suite.assert.NoError(err)
	suite.assert.FileExists(filepath.Join(suite.cache_path, file2))
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, file2))
	_, scheduled := suite.fileCache.scheduleOps.Load(file2)
	suite.assert.True(scheduled, "File should be scheduled when scheduler is OFF")
	flock := suite.fileCache.fileLocks.Get(file2)
	suite.assert.True(flock.SyncPending, "SyncPending flag should be set")
}

func (suite *fileCacheTestSuite) TestNoScheduleAlwaysOn() {
	defer suite.cleanupTest()

	configContent := fmt.Sprintf(`file_cache:
  path: %s
  offload-io: true
  create-empty-file: false

loopbackfs:
  path: %s`,
		suite.cache_path,
		suite.fake_storage_path,
	)

	suite.setupTestHelper(configContent)
	suite.assert.Empty(suite.fileCache.schedule, "Should have no schedule entries")

	file := "no_schedule_test.txt"
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.assert.NoError(err)
	data := []byte("testing default scheduler behavior")
	_, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle, Data: data})
	suite.assert.NoError(err)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, file),
		"File should be uploaded immediately with no schedule (always-on mode)")
	_, exists := suite.fileCache.scheduleOps.Load(file)
	suite.assert.False(exists, "File should not be in scheduleOps map")

	uploadedData, err := os.ReadFile(filepath.Join(suite.fake_storage_path, file))
	suite.assert.NoError(err)
	suite.assert.Equal(data, uploadedData, "Uploaded file content should match original")

	flock := suite.fileCache.fileLocks.Get(file)
	if flock != nil {
		suite.assert.False(flock.SyncPending, "SyncPending flag should be clear")
	}
}

func (suite *fileCacheTestSuite) TestExistingCloudFileImmediateUpload() {
	defer suite.cleanupTest()

	// 1. Initialize variables and files / call setuptesthelper
	// Set up scheduler with a time far in the future (ensuring we're in OFF state initially)
	now := time.Now()
	second := (now.Second() + 30) % 60
	cronExpr := fmt.Sprintf("%d * * * * *", second)

	configContent := fmt.Sprintf(`file_cache:
  path: %s
  offload-io: true
  create-empty-file: false
  schedule:
    - name: "Test"
      cron: %s
      duration: "5s"

loopbackfs:
  path: %s`,
		suite.cache_path,
		cronExpr,
		suite.fake_storage_path,
	)

	suite.setupTestHelper(configContent)

	// Create a file that will be "already in cloud"
	originalFile := "existing_cloud_file.txt"
	originalContent := []byte("original cloud content")

	// Create the file in the cloud storage directly
	err := os.MkdirAll(suite.fake_storage_path, 0777)
	err = os.WriteFile(filepath.Join(suite.fake_storage_path, originalFile), originalContent, 0777)
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, originalFile))
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, originalFile))

	// Write to the file and close the file
	handle, err := suite.fileCache.OpenFile(internal.OpenFileOptions{
		Name:  originalFile,
		Flags: os.O_RDWR,
		Mode:  0777,
	})
	suite.assert.NoError(err)
	// Write new content to the file
	modifiedContent := []byte("modified cloud file content")
	_, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle,
		Data:   modifiedContent,
		Offset: 0,
	})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})

	// Confirm cloud storage copy is updated
	fInfo, err := os.Stat(filepath.Join(suite.fake_storage_path, originalFile))
	suite.NoError(err)
	suite.assert.Len(modifiedContent, int(fInfo.Size()))
}

func (suite *fileCacheTestSuite) TestCreateFileAndRename() {
	defer suite.cleanupTest()

	now := time.Now()
	second := (now.Second() + 30) % 60
	cronExpr := fmt.Sprintf("%d * * * * *", second)

	configContent := fmt.Sprintf(`file_cache:
  path: %s
  offload-io: true
  create-empty-file: false
  schedule:
    - name: "Test"
      cron: %s
      duration: "5s"

loopbackfs:
  path: %s`,
		suite.cache_path,
		cronExpr,
		suite.fake_storage_path,
	)
	suite.setupTestHelper(configContent)

	srcFile := "source_rename_test.txt"
	dstFile := "destination_rename_test.txt"

	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: srcFile, Mode: 0777})
	suite.assert.NoError(err)

	data := []byte("file to be renamed while in scheduler OFF state")
	_, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle, Data: data})
	suite.assert.NoError(err)

	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Check if the file exists(local and Cloud)(Local == True, Cloud == False)
	suite.assert.FileExists(filepath.Join(suite.cache_path, srcFile),
		"File should exist in local cache")
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, srcFile),
		"File should not exist in cloud storage when scheduler is OFF")

	// Check if file is in scheduleOps with original name
	_, existsInSchedule := suite.fileCache.scheduleOps.Load(srcFile)
	suite.assert.True(existsInSchedule, "File should be in scheduleOps before rename")

	// Rename the file
	err = suite.fileCache.RenameFile(internal.RenameFileOptions{Src: srcFile, Dst: dstFile})
	suite.assert.NoError(err)

	// Verify renamed file paths
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, srcFile),
		"Source file should not exist in local cache after rename")
	suite.assert.FileExists(filepath.Join(suite.cache_path, dstFile),
		"Destination file should exist in local cache after rename")

	// Check if the file has been renamed in scheduleOps
	_, existsInScheduleOld := suite.fileCache.scheduleOps.Load(srcFile)
	suite.assert.False(
		existsInScheduleOld,
		"Old file name should not be in scheduleOps after rename",
	)

	_, existsInScheduleNew := suite.fileCache.scheduleOps.Load(dstFile)
	suite.assert.True(existsInScheduleNew, "New file name should be in scheduleOps after rename")

	// Check that file lock status was properly transferred
	flock := suite.fileCache.fileLocks.Get(dstFile)
	if flock != nil {
		suite.assert.True(flock.SyncPending, "SyncPending flag should be set on renamed file")
	}
}

func (suite *fileCacheTestSuite) TestDeleteFileAndScheduleOps() {
	defer suite.cleanupTest()

	now := time.Now()
	second := (now.Second() + 30) % 60
	cronExpr := fmt.Sprintf("%d * * * * *", second)

	configContent := fmt.Sprintf(`file_cache:
  path: %s
  offload-io: true
  create-empty-file: false
  schedule:
    - name: "Test"
      cron: %s
      duration: "5s"

loopbackfs:
  path: %s`,
		suite.cache_path,
		cronExpr,
		suite.fake_storage_path,
	)
	suite.setupTestHelper(configContent)

	// Create file that will be local only (not in cloud) due to scheduler being OFF
	testFile := "delete_test_file.txt"

	handle, err := suite.fileCache.CreateFile(
		internal.CreateFileOptions{Name: testFile, Mode: 0777},
	)
	suite.assert.NoError(err)

	data := []byte("file to be deleted while in scheduler OFF state")
	_, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle, Data: data})
	suite.assert.NoError(err)

	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Check if the file exists(local and Cloud)
	suite.assert.FileExists(filepath.Join(suite.cache_path, testFile),
		"File should exist in local cache")
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, testFile),
		"File should not exist in cloud storage when scheduler is OFF")

	// Check if file is in scheduleOps before deletion
	_, existsInSchedule := suite.fileCache.scheduleOps.Load(testFile)
	suite.assert.True(existsInSchedule, "File should be in scheduleOps before deletion")

	err = suite.fileCache.DeleteFile(internal.DeleteFileOptions{Name: testFile})
	suite.assert.NoError(err)

	// Check if file has been deleted in local cache
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, testFile),
		"File should not exist in local cache after deletion")

	// Check if the file has been deleted in scheduleOps
	_, existsInScheduleAfterDelete := suite.fileCache.scheduleOps.Load(testFile)
	suite.assert.False(existsInScheduleAfterDelete,
		"File should not be in scheduleOps after deletion")
}

func (suite *fileCacheTestSuite) TestCreateEmptyFileEqualTrue() {
	defer suite.cleanupTest()

	now := time.Now()
	second := (now.Second() + 30) % 60
	cronExpr := fmt.Sprintf("%d * * * * *", second)

	configContent := fmt.Sprintf(`file_cache:
  path: %s
  offload-io: true
  create-empty-file: true
  schedule:
    - name: "Test"
      cron: %s
      duration: "5s"

loopbackfs:
  path: %s`,
		suite.cache_path,
		cronExpr,
		suite.fake_storage_path,
	)
	suite.setupTestHelper(configContent)

	testFile := "empty_create_test_file.txt"
	handle, err := suite.fileCache.CreateFile(
		internal.CreateFileOptions{Name: testFile, Mode: 0777},
	)
	suite.assert.NoError(err)

	// Check if file exists in cloud storage immediately (without closing or flushing)
	// When create-empty-file is true, the file should be created in cloud storage immediately
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, testFile),
		"File should exist in cloud storage immediately when create-empty-file is true")

	// Verify the handle is not marked as dirty
	suite.assert.False(
		handle.Dirty(),
		"Handle should not be marked as dirty when create-empty-file is true",
	)

	// The file shouldn't be in scheduleOps because it's already in cloud storage
	_, existsInSchedule := suite.fileCache.scheduleOps.Load(testFile)
	suite.assert.False(existsInSchedule,
		"File should not be in scheduleOps because it's already in cloud storage")

	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)
}

func (suite *fileCacheTestSuite) TestReadWriteLocalFile() {
	defer suite.cleanupTest()

	now := time.Now()
	second := (now.Second() + 30) % 60
	cronExpr := fmt.Sprintf("%d * * * * *", second)

	configContent := fmt.Sprintf(`file_cache:
  path: %s
  offload-io: true
  create-empty-file: false
  schedule:
    - name: "Test"
      cron: %s
      duration: "5s"

loopbackfs:
  path: %s`,
		suite.cache_path,
		cronExpr,
		suite.fake_storage_path,
	)
	suite.setupTestHelper(configContent)

	testFile := "read_write_local_test.txt"
	initialContent := []byte("initial file content")

	handle, err := suite.fileCache.CreateFile(
		internal.CreateFileOptions{Name: testFile, Mode: 0777},
	)
	suite.assert.NoError(err)

	_, err = suite.fileCache.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Data: initialContent},
	)
	suite.assert.NoError(err)

	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Check if file is in local cache but not in cloud storage
	suite.assert.FileExists(filepath.Join(suite.cache_path, testFile),
		"File should exist in local cache")
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, testFile),
		"File should not exist in cloud storage when scheduler is OFF")

	// Check if file is in scheduleOps initially
	_, existsInSchedule := suite.fileCache.scheduleOps.Load(testFile)
	suite.assert.True(existsInSchedule, "File should be in scheduleOps after creation")

	// Write to file again with updated content
	newContent := []byte("updated file content")

	handle, err = suite.fileCache.OpenFile(
		internal.OpenFileOptions{Name: testFile, Flags: os.O_RDWR, Mode: 0777},
	)
	suite.assert.NoError(err)

	_, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle, Data: newContent})
	suite.assert.NoError(err)

	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Check scheduleOps to verify changes
	_, stillInSchedule := suite.fileCache.scheduleOps.Load(testFile)
	suite.assert.True(stillInSchedule, "File should remain in scheduleOps after modification")

	// Verify the local content was updated
	localData, err := os.ReadFile(filepath.Join(suite.cache_path, testFile))
	suite.assert.NoError(err)
	suite.assert.Equal(newContent, localData, "Local file content should be updated")

	// Check if file exists in cloud (should be false)
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, testFile),
		"File should still not exist in cloud storage after modification")
}

func (suite *fileCacheTestSuite) TestInvalidCronExpression() {
	defer suite.cleanupTest()

	// Set up a configuration with an invalid cron expression
	configContent := fmt.Sprintf(`file_cache:
  path: %s
  offload-io: true
  create-empty-file: false
  schedule:
    - name: "InvalidTest"
      cron: "invalid cron format"
      duration: "5s"
    - name: "ValidTest"
      cron: "0 * * * * *"
      duration: "5s"

loopbackfs:
  path: %s`,
		suite.cache_path,
		suite.fake_storage_path,
	)
	suite.setupTestHelper(configContent)

	// The invalid schedule should be skipped but valid one should be there
	hasValidSchedule := false
	for _, sched := range suite.fileCache.schedule {
		if sched.Name == "InvalidTest" {
			suite.assert.Fail("Invalid schedule should not be added")
		}
		if sched.Name == "ValidTest" {
			hasValidSchedule = true
		}
	}

	suite.assert.True(hasValidSchedule, "Valid schedule entry should be processed")

	// Test that operations still work with the valid schedule
	file := "test_after_invalid_cron.txt"
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.assert.NoError(err)

	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)
	suite.assert.FileExists(filepath.Join(suite.cache_path, file),
		"File should be created successfully despite invalid cron expression")
}

func (suite *fileCacheTestSuite) TestOverlappingSchedules() {
	defer suite.cleanupTest()

	now := time.Now()
	// Create two schedules that will run in close succession (3 seconds apart)
	firstSecond := now.Second()
	secondSecond := (now.Second() + 2) % 60

	configContent := fmt.Sprintf(`file_cache:
  path: %s
  offload-io: true
  create-empty-file: false
  schedule:
    - name: "FirstWindow"
      cron: "%d * * * * *"
      duration: "10s"
    - name: "SecondWindow"
      cron: "%d * * * * *"
      duration: "10s"

loopbackfs:
  path: %s`,
		suite.cache_path,
		firstSecond,
		secondSecond,
		suite.fake_storage_path,
	)
	suite.setupTestHelper(configContent)

	file1 := "overlap_test_first_window.txt"
	handle1, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file1, Mode: 0777})
	suite.assert.NoError(err)
	data1 := []byte("file created during first window")
	_, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle1, Data: data1})
	suite.assert.NoError(err)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle1})
	suite.assert.NoError(err)

	// File should be uploaded immediately as we're in an upload window
	time.Sleep(100 * time.Millisecond)
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, file1),
		"File should be uploaded immediately during first window")

	// Create another file to verify we're still in an upload window
	file2 := "overlap_test_second_window.txt"
	handle2, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file2, Mode: 0777})
	suite.assert.NoError(err)
	data2 := []byte("file created during second window")
	_, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle2, Data: data2})
	suite.assert.NoError(err)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle2})
	suite.assert.NoError(err)

	time.Sleep(100 * time.Millisecond)
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, file2),
		"File should be uploaded immediately during second window")

	// Modify the first file and verify it still uploads immediately
	handle1, err = suite.fileCache.OpenFile(internal.OpenFileOptions{
		Name:  file1,
		Flags: os.O_RDWR,
		Mode:  0777,
	})
	suite.assert.NoError(err)
	updatedData := []byte(" - updated in second window")
	_, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle1,
		Data:   updatedData,
		Offset: int64(len(data1)),
	})
	suite.assert.NoError(err)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle1})
	suite.assert.NoError(err)

	// Verify updated data was uploaded
	time.Sleep(100 * time.Millisecond)
	cloudData, err := os.ReadFile(filepath.Join(suite.fake_storage_path, file1))
	suite.assert.NoError(err)
	suite.assert.Equal(append(data1, updatedData...), cloudData,
		"File should be updated immediately in the second window")
}

func (suite *fileCacheTestSuite) TestGetAttrCase1() {
	defer suite.cleanupTest()
	// Setup
	file := "file24"
	// Create files directly in "fake_storage"
	handle, _ := suite.loopback.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.loopback.CloseFile(internal.CloseFileOptions{Handle: handle})

	// Read the Directory
	attr, err := suite.fileCache.GetAttr(internal.GetAttrOptions{Name: file})
	suite.assert.NoError(err)
	suite.assert.NotNil(attr)
	suite.assert.Equal(file, attr.Path)
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
	suite.assert.Equal(file, attr.Path)
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
	suite.assert.Equal(file, attr.Path)
	suite.assert.EqualValues(1024, attr.Size)
}

func (suite *fileCacheTestSuite) TestGetAttrCase4() {
	defer suite.cleanupTest()

	suite.cleanupTest()
	configuration := fmt.Sprintf(
		"file_cache:\n  path: %s\n  timeout-sec: %d\n\nloopbackfs:\n  path: %s",
		suite.cache_path,
		minimumFileCacheTimeout,
		suite.fake_storage_path,
	)
	suite.setupTestHelper(configuration)

	// Setup
	file := "file27"
	// By default createEmptyFile is false, so we will not create these files in cloud storage until they are closed.
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.assert.NoError(err)
	suite.assert.NotNil(handle)

	size := (100 * 1024 * 1024)
	data := make([]byte, size)

	written, err := suite.fileCache.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(size, written)

	err = suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NoError(err)

	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Wait  file is evicted
	time.Sleep(minimumFileCacheTimeout * time.Second)
	_, err = os.Stat(filepath.Join(suite.cache_path, file))
	for i := 0; i < 20*minimumFileCacheTimeout && !os.IsNotExist(err); i++ {
		time.Sleep(100 * time.Millisecond)
		_, err = os.Stat(filepath.Join(suite.cache_path, file))
	}
	suite.assert.True(os.IsNotExist(err))

	// open the file in parallel and try getting the size of file while open is on going
	go suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0666})

	// Read the Directory
	attr, err := suite.fileCache.GetAttr(internal.GetAttrOptions{Name: file})
	suite.assert.NoError(err)
	suite.assert.NotNil(attr)
	suite.assert.Equal(file, attr.Path)
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
	handle, _ := suite.loopback.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0777})
	suite.loopback.CloseFile(internal.CloseFileOptions{Handle: handle})

	// Path should be in fake storage
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, src))

	// RenameFile
	err := suite.fileCache.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
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
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0666})
	suite.assert.NoError(err)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Path should be in the file cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, src))
	// Path should be in fake storage
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, src))

	// RenameFile
	err = suite.fileCache.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	suite.assert.NoError(err)
	// Path in fake storage and file cache should be updated
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, src)) // Src does not exist
	suite.assert.FileExists(
		filepath.Join(suite.cache_path, dst),
	) // Dst shall exists in cache
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, src)) // Src does not exist
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, dst))   // Dst does exist
}

func (suite *fileCacheTestSuite) TestRenameFileAndCacheCleanup() {
	defer suite.cleanupTest()

	suite.cleanupTest()
	configuration := fmt.Sprintf(
		"file_cache:\n  path: %s\n  timeout-sec: %d\n\nloopbackfs:\n  path: %s",
		suite.cache_path,
		minimumFileCacheTimeout,
		suite.fake_storage_path,
	)
	suite.setupTestHelper(configuration)

	src := "source4"
	dst := "destination4"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0666})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})

	// Path should be in the file cache
	suite.assert.FileExists(suite.cache_path + "/" + src)
	// Path should be in fake storage
	suite.assert.FileExists(suite.fake_storage_path + "/" + src)

	// RenameFile
	err := suite.fileCache.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	suite.assert.NoError(err)
	// Path in fake storage and file cache should be updated
	suite.assert.False(suite.fileCache.policy.IsCached(filepath.Join(suite.cache_path, src)))
	suite.assert.NoFileExists(suite.cache_path + "/" + src)        // Src does not exist
	suite.assert.FileExists(suite.cache_path + "/" + dst)          // Dst shall exists in cache
	suite.assert.NoFileExists(suite.fake_storage_path + "/" + src) // Src does not exist
	suite.assert.FileExists(suite.fake_storage_path + "/" + dst)   // Dst does exist

	suite.assert.FileExists(suite.cache_path + "/" + dst) // Dst shall exists in cache

	// Wait for the cache cleanup to occur
	time.Sleep(minimumFileCacheTimeout * time.Second)
	_, err = os.Stat(filepath.Join(suite.cache_path, dst))
	for i := 0; i < 20*minimumFileCacheTimeout && !os.IsNotExist(err); i++ {
		time.Sleep(100 * time.Millisecond)
		_, err = os.Stat(filepath.Join(suite.cache_path, dst))
	}
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, dst)) // Dst shall not exists in cache
}

func (suite *fileCacheTestSuite) TestRenameOpenFileCase1() {
	defer suite.cleanupTest()

	src := "source5"
	dst := "destination5"

	// create file in cloud
	handle, _ := suite.loopback.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0777})
	suite.loopback.CloseFile(internal.CloseFileOptions{Handle: handle})

	// open file for writing
	handle, err := suite.fileCache.OpenFile(
		internal.OpenFileOptions{Name: src, Flags: os.O_RDWR, Mode: 0777},
	)
	suite.assert.NoError(err)
	handlemap.Add(handle)
	// Path should not be in the file cache (lazy open)
	suite.assert.NoFileExists(suite.cache_path + "/" + src)

	// rename open file
	err = suite.fileCache.RenameFile(internal.RenameFileOptions{
		Src: src,
		Dst: dst,
	})
	suite.assert.NoError(err)
	// rename succeeded in cloud
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, src))
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, dst))
	// still in lazy open state
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, src))
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, dst))

	// write to file handle
	data := []byte("newdata")
	n, err := suite.fileCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle,
		Data:   data,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(len(data), n)
	// open is completed (file is downloaded), and writes go to the correct file
	suite.assert.NoFileExists(filepath.Join(suite.cache_path, src))
	suite.assert.FileExists(filepath.Join(suite.cache_path, dst))

	// Close file handle
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{
		Handle: handle,
	})
	suite.assert.NoError(err)

	// Check cloud storage
	suite.assert.NoFileExists(path.Join(suite.fake_storage_path, src)) // Src does not exist
	suite.assert.FileExists(path.Join(suite.fake_storage_path, dst))   // Dst does exist
	dstData, err := os.ReadFile(path.Join(suite.fake_storage_path, dst))
	suite.assert.NoError(err)
	suite.assert.Equal(data, dstData)
}

func (suite *fileCacheTestSuite) TestRenameOpenFileCase2() {
	defer suite.cleanupTest()

	src := "source6"
	dst := "destination6"

	// create source file
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0666})
	suite.assert.NoError(err)
	handlemap.Add(handle)
	// Path should be in the file cache
	suite.assert.FileExists(suite.cache_path + "/" + src)

	// rename open file
	err = suite.fileCache.RenameFile(internal.RenameFileOptions{
		Src: src,
		Dst: dst,
	})
	suite.assert.NoError(err)

	// write to file handle
	data := []byte("newdata")
	n, err := suite.fileCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle,
		Data:   data,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(len(data), n)

	// Close file handle
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{
		Handle: handle,
	})
	suite.assert.NoError(err)

	// Check cloud storage
	suite.assert.NoFileExists(path.Join(suite.fake_storage_path, src)) // Src does not exist
	suite.assert.FileExists(path.Join(suite.fake_storage_path, dst))   // Dst does exist
	dstData, err := os.ReadFile(path.Join(suite.fake_storage_path, dst))
	suite.assert.NoError(err)
	suite.assert.Equal(data, dstData)
}

func (suite *fileCacheTestSuite) TestRenameOpenFileCase3() {
	defer suite.cleanupTest()

	// Setup
	src := "source7"
	dst := "destination7"
	// create source file
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0666})
	suite.assert.NoError(err)
	handlemap.Add(handle)
	// Path should be in the file cache
	suite.assert.FileExists(suite.cache_path + "/" + src)
	// write to file handle
	initialData := []byte("initialData")
	n, err := suite.fileCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle,
		Data:   initialData,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(len(initialData), n)
	// flush to cloud
	err = suite.fileCache.FlushFile(internal.FlushFileOptions{
		Handle: handle,
	})
	suite.assert.NoError(err)
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, src))

	// rename open file
	err = suite.fileCache.RenameFile(internal.RenameFileOptions{
		Src: src,
		Dst: dst,
	})
	suite.assert.NoError(err)
	// write to file handle
	newData := []byte("newData")
	n, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle,
		Data:   newData,
		Offset: int64(len(initialData)),
	})
	suite.assert.NoError(err)
	suite.assert.Equal(len(newData), n)
	// Close file handle
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{
		Handle: handle,
	})
	suite.assert.NoError(err)

	// Check that cloud storage got all data and file was renamed properly
	suite.assert.NoFileExists(path.Join(suite.fake_storage_path, src)) // Src does not exist
	suite.assert.FileExists(path.Join(suite.fake_storage_path, dst))   // Dst does exist
	dstData, err := os.ReadFile(path.Join(suite.fake_storage_path, dst))
	suite.assert.NoError(err)
	suite.assert.Equal(append(initialData, newData...), dstData)
}

func (suite *fileCacheTestSuite) TestTruncateFileNotInCache() {
	defer suite.cleanupTest()
	// Setup
	path := "file30"
	handle, _ := suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.loopback.CloseFile(internal.CloseFileOptions{Handle: handle})

	// Path should be in fake storage
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, path))

	// Chmod
	size := 1024
	err := suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: path, Size: int64(size)})
	suite.assert.NoError(err)

	// Path in fake storage should be updated
	info, _ := os.Stat(filepath.Join(suite.fake_storage_path, path))
	suite.assert.EqualValues(info.Size(), size)
}

func (suite *fileCacheTestSuite) TestTruncateFileCase3() {
	defer suite.cleanupTest()
	// Setup
	path := "file31"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0666})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})

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
	configuration := fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true\n  timeout-sec: %d\n\nloopbackfs:\n  path: %s",
		suite.cache_path,
		minimumFileCacheTimeout,
		suite.fake_storage_path,
	)

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
	err = os.Symlink(suite.cache_path, symlinkPath)
	defer os.Remove(symlinkPath)
	suite.assert.NoError(err)
	configuration := fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true\n\nloopbackfs:\n  path: %s",
		symlinkPath,
		suite.fake_storage_path,
	)
	suite.setupTestHelper(configuration)
	file := "file39"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})

	handle, _ = suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

	output := make([]byte, 9)
	n, err := suite.fileCache.ReadInBuffer(
		&internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: output},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(9, n)
	suite.assert.Equal(data, output)
}

func (suite *fileCacheTestSuite) TestZZOffloadIO() {
	defer suite.cleanupTest()
	configuration := fmt.Sprintf(
		"file_cache:\n  path: %s\n  timeout-sec: %d\n\nloopbackfs:\n  path: %s",
		suite.cache_path,
		minimumFileCacheTimeout,
		suite.fake_storage_path,
	)

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
	suite.fileCache.lazyWrite = true

	file := "file101"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	data := make([]byte, 10*1024*1024)
	_, _ = suite.fileCache.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)
	_ = suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})

	// As lazy write is enabled flush shall not upload the file
	suite.assert.True(handle.Dirty())

	// File is uploaded async on close
	_ = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	// Wait for the upload
	for i := 0; i < 50 && handle.Dirty(); i++ {
		time.Sleep(100 * time.Millisecond)
	}

	suite.assert.False(handle.Dirty())

	// cleanup
	suite.fileCache.lazyWrite = false
}

func (suite *fileCacheTestSuite) TestStatFS() {
	defer suite.cleanupTest()
	cacheTimeout := 5
	maxSizeMb := 2
	config := fmt.Sprintf(
		"file_cache:\n  path: %s\n  max-size-mb: %d\n  offload-io: true\n  timeout-sec: %d\n\nloopbackfs:\n  path: %s",
		suite.cache_path,
		maxSizeMb,
		cacheTimeout,
		suite.fake_storage_path,
	)
	os.Mkdir(suite.cache_path, 0777)
	suite.setupTestHelper(
		config,
	) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	file := "file41"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	data := make([]byte, 1024*1024)
	suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
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
	config := fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true\n  refresh-sec: 1\n\nloopbackfs:\n  path: %s",
		suite.cache_path,
		suite.fake_storage_path,
	)
	suite.setupTestHelper(
		config,
	) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	path := "file42"
	byteArr := []byte("test data")
	os.WriteFile(filepath.Join(suite.fake_storage_path, path), byteArr, 0777)

	data := make([]byte, 20)
	options := internal.OpenFileOptions{Name: path, Mode: 0777}
	// Read file once and we shall get the same data
	handle, err := suite.fileCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.False(handle.Dirty())
	n, err := suite.fileCache.ReadInBuffer(
		&internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(len(byteArr), n)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Modify the file in background but we shall still get the old data
	byteArr = []byte("test data1")
	err = os.WriteFile(suite.fake_storage_path+"/"+path, byteArr, 0777)
	suite.assert.NoError(err)
	handle, err = suite.fileCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.False(handle.Dirty())
	n, err = suite.fileCache.ReadInBuffer(
		&internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(9, n)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Now wait for refresh timeout and we shall get the updated content on next read
	byteArr = []byte("test data123456")
	err = os.WriteFile(suite.fake_storage_path+"/"+path, byteArr, 0777)
	suite.assert.NoError(err)
	time.Sleep(1 * time.Second)
	handle, err = suite.fileCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.False(handle.Dirty())
	n, err = suite.fileCache.ReadInBuffer(
		&internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(15, n)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)
}

func (suite *fileCacheTestSuite) TestHardLimitOnSize() {
	defer suite.cleanupTest()
	// Configure to create empty files so we create the file in cloud storage
	config := fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true\n  hard-limit: true\n  max-size-mb: 2\n\nloopbackfs:\n  path: %s",
		suite.cache_path,
		suite.fake_storage_path,
	)
	suite.setupTestHelper(
		config,
	) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	data := make([]byte, 3*MB)
	pathbig := "filebig"
	err := os.WriteFile(suite.fake_storage_path+"/"+pathbig, data, 0777)
	suite.assert.NoError(err)

	data = make([]byte, 1*MB)
	pathsmall := "filesmall"
	err = os.WriteFile(suite.fake_storage_path+"/"+pathsmall, data, 0777)
	suite.assert.NoError(err)

	smallHandle, err := suite.fileCache.OpenFile(
		internal.OpenFileOptions{
			Name:  pathsmall,
			Flags: os.O_RDONLY,
			Mode:  suite.fileCache.defaultPermission,
		},
	)
	suite.assert.NoError(err)
	// try opening small file
	suite.assert.False(smallHandle.Dirty())
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: smallHandle})
	suite.assert.NoError(err)

	// try opening bigger file which shall fail due to hardlimit
	bigHandle, err := suite.fileCache.OpenFile(
		internal.OpenFileOptions{
			Name:  pathbig,
			Flags: os.O_RDONLY,
			Mode:  suite.fileCache.defaultPermission,
		},
	)
	suite.assert.Error(err)
	suite.assert.Nil(bigHandle)
	suite.assert.Equal(syscall.ENOSPC, err)

	// try writing a small file
	options1 := internal.CreateFileOptions{Name: pathsmall + "_new", Mode: 0777}
	f, err := suite.fileCache.CreateFile(options1)
	suite.assert.NoError(err)
	data = make([]byte, 1*MB)
	n, err := suite.fileCache.WriteFile(
		&internal.WriteFileOptions{Handle: f, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(1*MB, n)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: f})
	suite.assert.NoError(err)

	// try writing a bigger file
	options1 = internal.CreateFileOptions{Name: pathbig + "_new", Mode: 0777}
	f, err = suite.fileCache.CreateFile(options1)
	suite.assert.NoError(err)
	data = make([]byte, 3*MB)
	n, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: f, Offset: 0, Data: data})
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

	path := "file43"
	err := os.WriteFile(suite.fake_storage_path+"/"+path, []byte("test data"), 0777)
	suite.assert.NoError(err)

	data := make([]byte, 20)
	options := internal.OpenFileOptions{Name: path, Flags: os.O_RDONLY, Mode: 0777}

	// Read file once and we shall get the same data
	handle, err := suite.fileCache.OpenFile(options)
	handlemap.Add(handle)
	suite.assert.NoError(err)
	suite.assert.False(handle.Dirty())
	n, err := suite.fileCache.ReadInBuffer(
		&internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: data},
	)
	handle, loaded := handlemap.Load(handle.ID)
	suite.assert.True(loaded)
	suite.assert.NoError(err)
	suite.assert.Equal(9, n)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)
}

// create a list of empty directories in local and storage and then try to delete those to validate empty directories
// are allowed be to deleted but non empty are not
func (suite *fileCacheTestSuite) TestDeleteDirectory() {
	defer suite.cleanupTest()

	config := fmt.Sprintf("file_cache:\n  path: %s\n  timeout-sec: 1000\n\nloopbackfs:\n  path: %s",
		suite.cache_path, suite.fake_storage_path)
	suite.setupTestHelper(
		config,
	) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	// Create local and remote dir structures
	suite.createLocalDirectoryStructure()
	suite.createRemoteDirectoryStructure()

	// Create a file in the some random directories
	file := "file43"
	h, err := suite.fileCache.CreateFile(
		internal.CreateFileOptions{Name: filepath.Join("a", "b", "c", "d", file), Mode: 0777},
	)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)

	h, err = suite.fileCache.CreateFile(
		internal.CreateFileOptions{Name: filepath.Join("a", "b", file), Mode: 0777},
	)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)

	h, err = suite.fileCache.CreateFile(
		internal.CreateFileOptions{Name: filepath.Join("h", "l", "m", file), Mode: 0777},
	)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)

	// Check directories are counted as non empty right now
	empty := suite.fileCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: filepath.Join("a")})
	suite.assert.False(empty)

	empty = suite.fileCache.IsDirEmpty(
		internal.IsDirEmptyOptions{Name: filepath.Join("a", "b", "c", "d")},
	)
	suite.assert.False(empty)

	// Validate one empty directory as well
	empty = suite.fileCache.IsDirEmpty(
		internal.IsDirEmptyOptions{Name: filepath.Join("a", "b", "e", "f")},
	)
	suite.assert.True(empty)

	// Delete file from one of the directory and validate its empty now, but its parent is not empty
	err = suite.fileCache.DeleteFile(
		internal.DeleteFileOptions{Name: filepath.Join("a", "b", "c", "d", file)},
	)
	suite.assert.NoError(err)

	empty = suite.fileCache.IsDirEmpty(
		internal.IsDirEmptyOptions{Name: filepath.Join("a", "b", "c", "d")},
	)
	suite.assert.True(empty)
	empty = suite.fileCache.IsDirEmpty(
		internal.IsDirEmptyOptions{Name: filepath.Join("a", "b", "c")},
	)
	suite.assert.False(empty)

	// Delete file only locally and not on remote and validate the directory is still not empty
	h, err = suite.fileCache.CreateFile(
		internal.CreateFileOptions{Name: filepath.Join("h", "l", "m", "n", file), Mode: 0777},
	)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)
	empty = suite.fileCache.IsDirEmpty(
		internal.IsDirEmptyOptions{Name: filepath.Join("h", "l", "m", "n")},
	)
	suite.assert.False(empty)

	os.Remove(filepath.Join(suite.cache_path, "h", "l", "m", "n", file))
	empty = suite.fileCache.IsDirEmpty(
		internal.IsDirEmptyOptions{Name: filepath.Join("h", "l", "m", "n")},
	)
	suite.assert.False(empty)
	os.Remove(filepath.Join(suite.fake_storage_path, "h", "l", "m", "n", file))
	empty = suite.fileCache.IsDirEmpty(
		internal.IsDirEmptyOptions{Name: filepath.Join("h", "l", "m", "n")},
	)
	suite.assert.True(empty)

	// Delete file only on remote and not on local and validate the directory is still not empty
	h, err = suite.fileCache.CreateFile(
		internal.CreateFileOptions{Name: filepath.Join("h", "l", "m", "n", file), Mode: 0777},
	)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)
	empty = suite.fileCache.IsDirEmpty(
		internal.IsDirEmptyOptions{Name: filepath.Join("h", "l", "m", "n")},
	)
	suite.assert.False(empty)

	os.Remove(filepath.Join(suite.fake_storage_path, "h", "l", "m", "n", file))
	empty = suite.fileCache.IsDirEmpty(
		internal.IsDirEmptyOptions{Name: filepath.Join("h", "l", "m", "n")},
	)
	suite.assert.False(empty)
	os.Remove(filepath.Join(suite.cache_path, "h", "l", "m", "n", file))
	empty = suite.fileCache.IsDirEmpty(
		internal.IsDirEmptyOptions{Name: filepath.Join("h", "l", "m", "n")},
	)
	suite.assert.True(empty)
}

func (suite *fileCacheTestSuite) createLocalDirectoryStructure() {
	err := os.MkdirAll(filepath.Join(suite.cache_path, "a", "b", "c", "d"), 0777)
	suite.assert.NoError(err)

	err = os.MkdirAll(filepath.Join(suite.cache_path, "a", "b", "e", "f"), 0777)
	suite.assert.NoError(err)

	err = os.MkdirAll(filepath.Join(suite.cache_path, "a", "b", "e", "g"), 0777)
	suite.assert.NoError(err)

	err = os.MkdirAll(filepath.Join(suite.cache_path, "h", "l", "m", "n"), 0777)
	suite.assert.NoError(err)
}

func (suite *fileCacheTestSuite) createRemoteDirectoryStructure() {
	err := os.MkdirAll(filepath.Join(suite.fake_storage_path, "a", "b", "c", "d"), 0777)
	suite.assert.NoError(err)

	err = os.MkdirAll(filepath.Join(suite.fake_storage_path, "a", "b", "e", "f"), 0777)
	suite.assert.NoError(err)

	err = os.MkdirAll(filepath.Join(suite.fake_storage_path, "a", "b", "e", "g"), 0777)
	suite.assert.NoError(err)

	err = os.MkdirAll(filepath.Join(suite.fake_storage_path, "h", "i", "j", "k"), 0777)
	suite.assert.NoError(err)

	err = os.MkdirAll(filepath.Join(suite.fake_storage_path, "h", "l", "m", "n"), 0777)
	suite.assert.NoError(err)
}

func (suite *fileCacheTestSuite) TestHardLimit() {
	defer suite.cleanupTest()
	cacheTimeout := 0
	maxSizeMb := 2
	config := fmt.Sprintf(
		"file_cache:\n  path: %s\n  max-size-mb: %d\n  timeout-sec: %d\n\nloopbackfs:\n  path: %s",
		suite.cache_path,
		maxSizeMb,
		cacheTimeout,
		suite.fake_storage_path,
	)
	os.Mkdir(suite.cache_path, 0777)
	suite.setupTestHelper(
		config,
	) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	file := "file96"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	data := make([]byte, 1024*1024)
	for i := range int64(5) {
		suite.fileCache.WriteFile(
			&internal.WriteFileOptions{Handle: handle, Offset: i * 1024 * 1024, Data: data},
		)
	}
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	time.Sleep(1)

	// Now try to open the file and validate we get an error due to hard limit
	handle, err := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})
	suite.assert.NotNil(err)
	suite.assert.Nil(handle)
	suite.assert.Equal(err, syscall.ENOSPC)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestFileCacheTestSuite(t *testing.T) {
	suite.Run(t, new(fileCacheTestSuite))
}
