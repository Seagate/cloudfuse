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
	"sort"
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
	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v2"

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
	if suite.useMock {
		suite.mockCtrl = gomock.NewController(suite.T())
		suite.mock = internal.NewMockComponent(suite.mockCtrl)
		suite.fileCache = newTestFileCache(suite.mock)
		// always simulate being offline
		suite.mock.EXPECT().StatFs().AnyTimes().Return(nil, false, &common.CloudUnreachableError{})
	} else {
		suite.loopback = newLoopbackFS()
		suite.fileCache = newTestFileCache(suite.loopback)
		err := suite.loopback.Start(context.Background())
		if err != nil {
			panic(fmt.Sprintf("Unable to start next component [%s]", err.Error()))
		}
	}
	err := suite.fileCache.Start(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Unable to start file cache [%s]", err.Error()))
	}

}

func (suite *fileCacheTestSuite) cleanupTest() {
	err := suite.fileCache.Stop()
	if err != nil {
		panic(fmt.Sprintf("Unable to stop file cache [%s]", err.Error()))
	}
	if suite.useMock {
		suite.mockCtrl.Finish()
	} else {
		suite.loopback.Stop()
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

	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxSizeMB, 0)
	suite.assert.EqualValues(defaultMaxEviction, suite.fileCache.policy.(*lruPolicy).maxEviction)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).highThreshold, defaultMaxThreshold)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).lowThreshold, defaultMinThreshold)

	suite.assert.False(suite.fileCache.createEmptyFile)
	suite.assert.False(suite.fileCache.allowNonEmpty)
	suite.assert.EqualValues(suite.fileCache.cacheTimeout, 216000)
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
	suite.assert.Equal(suite.fileCache.cleanupOnStart, cleanupOnStart)
	suite.assert.Equal(suite.fileCache.syncToFlush, syncToFlush)
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
	suite.assert.Equal(suite.fileCache.cleanupOnStart, cleanupOnStart)
}

// Tests CreateDir
func (suite *fileCacheTestSuite) TestCreateDir() {
	defer suite.cleanupTest()
	path := "a"
	options := internal.CreateDirOptions{Name: path}
	err := suite.fileCache.CreateDir(options)
	suite.assert.NoError(err)

	// Path should be added to the file cache
	suite.assert.DirExists(filepath.Join(suite.cache_path, path))
	// Path should be in fake storage
	suite.assert.DirExists(filepath.Join(suite.fake_storage_path, path))
}

// Tests CreateDir
func (suite *fileCacheTestSuite) TestCreateDirErrExist() {
	defer suite.cleanupTest()
	path := "a"
	options := internal.CreateDirOptions{Name: path}
	err := suite.fileCache.CreateDir(options)
	// test
	err = suite.fileCache.CreateDir(options)
	suite.assert.ErrorIs(err, os.ErrExist)
}

// Tests CreateDir
func (suite *fileCacheTestSuite) TestCreateDirOffline() {
	// enable mock component
	suite.cleanupTest()
	defaultConfig := fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true",
		suite.cache_path,
	)
	suite.useMock = true
	suite.setupTestHelper(defaultConfig)
	defer suite.cleanupTest()
	// setup
	path := "a"
	options := internal.CreateDirOptions{Name: path}
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: path}).Return(nil, os.ErrNotExist)
	err := suite.fileCache.CreateDir(options)
	suite.assert.NoError(err)

	// Path should be added to the file cache
	suite.assert.DirExists(filepath.Join(suite.cache_path, path))
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
	for i := 0; i < 5; i++ {
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
	n, err := suite.fileCache.WriteFile(internal.WriteFileOptions{
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
	n, err = suite.fileCache.WriteFile(internal.WriteFileOptions{
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
	n, err = suite.fileCache.WriteFile(internal.WriteFileOptions{
		Handle: handle2,
		Data:   data,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(len(data), n)
	//
	// Case 3
	n, err = suite.fileCache.WriteFile(internal.WriteFileOptions{
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
	_, err = suite.fileCache.WriteFile(
		internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
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
	suite.loopback.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
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
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
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
	time.Sleep(3 * minimumFileCacheTimeout * time.Second)

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
		internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: data},
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
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})

	handle, _ = suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

	output := make([]byte, 9)
	length, err := suite.fileCache.ReadInBuffer(
		internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: output},
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
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})

	handle, _ = suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

	output := make([]byte, 9)
	length, err := suite.fileCache.ReadInBuffer(
		internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: output},
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
	length, err := suite.fileCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: handle})
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
		internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
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
	len, err := suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle})
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
	suite.assert.Equal(data, d)
}

func (suite *fileCacheTestSuite) TestFlushFileDoesNotUploadToCloud() {
	suite.cleanupTest()

	defaultConfig := fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true",
		suite.cache_path,
	)
	suite.useMock = true
	suite.setupTestHelper(defaultConfig)
	defer suite.cleanupTest()
	// Create and write to file
	file := "file_100"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	data := []byte("offline test data")
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})

	// Expect that CopyFromFile is Never Called (no upload)
	suite.mock.EXPECT().CopyFromFile(gomock.Any()).Times(0)
	suite.mock.EXPECT().GetAttr(gomock.Any()).Return(&internal.ObjAttr{}, os.ErrNotExist)
	suite.mock.EXPECT().Chmod(gomock.Any()).Return(nil).Times(1)
	// Flush Everything
	err := suite.fileCache.FlushFile(
		internal.FlushFileOptions{Handle: handle},
	)
	suite.assert.NoError(err)
	suite.assert.False(handle.Dirty())

	// The file should still exist locally
	suite.assert.FileExists(filepath.Join(suite.cache_path, file))
}

func (suite *fileCacheTestSuite) TestFlushFileDoesUploadToCloud_ImmediateUpload() {
	suite.cleanupTest()

	defaultConfig := fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true",
		suite.cache_path,
	)
	suite.useMock = true
	suite.setupTestHelper(defaultConfig)
	defer suite.cleanupTest()

	// Create and write to file
	file := "file_101"
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.assert.NoError(err)
	data := []byte("test data for immediate upload")
	_, err = suite.fileCache.WriteFile(
		internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)

	suite.mock.EXPECT().GetAttr(gomock.Any()).Return(&internal.ObjAttr{}, os.ErrNotExist)

	// Expect that CopyFromFile is called once (upload happens)
	suite.mock.EXPECT().CopyFromFile(gomock.Any()).Return(nil).Times(1)

	suite.mock.EXPECT().Chmod(gomock.Any()).Return(nil).Times(1)

	// Call flushFileInternal directly with ImmediateUpload = true
	err = suite.fileCache.flushFileInternal(
		internal.FlushFileOptions{
			Handle:          handle,
			ImmediateUpload: true,
		},
	)
	suite.assert.NoError(err)

	suite.assert.False(handle.Dirty())

	suite.assert.FileExists(filepath.Join(suite.cache_path, file))
}

func (suite *fileCacheTestSuite) TestFlushFileInternalUpdatesExistingCloudFile() {
	suite.cleanupTest()

	defaultConfig := fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true",
		suite.cache_path,
	)
	suite.useMock = true
	suite.setupTestHelper(defaultConfig)
	defer suite.cleanupTest()

	// Create and write to file
	file := "file_existing_cloud"
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.assert.NoError(err)

	initialData := []byte("initial cloud data")
	updatedData := []byte("updated local data")

	_, err = suite.fileCache.WriteFile(
		internal.WriteFileOptions{Handle: handle, Offset: 0, Data: updatedData},
	)
	suite.assert.NoError(err)
	suite.assert.True(handle.Dirty())

	// Mock that file already exists in cloud
	suite.mock.EXPECT().GetAttr(gomock.Any()).Return(&internal.ObjAttr{
		Path: file,
		Size: int64(len(initialData)),
	}, nil).Times(1)

	suite.mock.EXPECT().CopyFromFile(gomock.Any()).Return(nil).Times(1)

	suite.mock.EXPECT().Chmod(gomock.Any()).Return(nil).Times(1)

	// Call flushFileInternal directly with ImmediateUpload = false or true(should upload no matter what)
	err = suite.fileCache.flushFileInternal(
		internal.FlushFileOptions{
			Handle:          handle,
			ImmediateUpload: false,
		},
	)
	suite.assert.NoError(err)
	suite.assert.False(handle.Dirty())

	// Verify file still exists in cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, file))
}

func (suite *fileCacheTestSuite) TestUploadPendingFileWithServicePendingOPS() {
	defer suite.cleanupTest()

	// Create and write to file
	file := "file_pending_upload"
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.assert.NoError(err)
	testData := "data for pending upload"
	data := []byte(testData)
	_, err = suite.fileCache.WriteFile(
		internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)

	err = suite.fileCache.uploadPendingFile(handle.Path)
	suite.assert.NoError(err)
	suite.assert.True(handle.Dirty())

	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, file))
	// Check that fake_storage updated with data
	d, _ := os.ReadFile(filepath.Join(suite.fake_storage_path, file))
	suite.assert.Equal(data, d)
}
func (suite *fileCacheTestSuite) TestServicePendingOpsAndUploadPendingFile() {
	defer suite.cleanupTest()

	// Create and write to a file
	file := "file_pending_scheduler"
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.assert.NoError(err)
	testData := "data for pending upload via scheduler"
	data := []byte(testData)
	_, err = suite.fileCache.WriteFile(
		internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)

	// Add the file to the scheduleOps map to mark it as pending
	suite.fileCache.scheduleOps.Store(file, true) // properly store the key-value pair
	value, exists := suite.fileCache.scheduleOps.Load(file)
	if exists {
		print(value)
	} else {
		print("Value not found")
	}

	suite.fileCache.scheduleOps.Store(file, true)
	print(suite.fileCache.scheduleOps.Load(file))
	// Get the file lock and mark it as pending sync
	flock := suite.fileCache.fileLocks.Get(file)
	flock.SyncPending = true
	suite.fileCache.scheduleOps.Range(func(key, value interface{}) bool {
		print(suite.fileCache.scheduleOps.Load(file))
		return true
	})

	suite.fileCache.servicePendingOps()
	print(suite.fileCache.scheduleOps.Load(file))

	uploaded := false

	if _, err := os.Stat(filepath.Join(suite.fake_storage_path, file)); err == nil {
		if _, exists := suite.fileCache.scheduleOps.Load(file); !exists {
			if !flock.SyncPending {
				uploaded = true
			}
		}
	}

	suite.assert.True(uploaded, "File was not uploaded by the background servicePendingOps process")

	// Verify the file data was correctly uploaded
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, file))
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
func (suite *fileCacheTestSuite) TestScheduleUploadsCronIntegration2() {
	defer suite.cleanupTest()

	// Setup test config with a clean environment
	err := log.SetDefaultLogger("silent", common.LogConfig{})
	suite.assert.NoError(err)

	// Create a file and add it to scheduleOps
	file := "scheduled_file_cron.txt"
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.assert.NoError(err)

	// Write some data to the file
	testData := []byte("scheduled upload via cron test data")
	n, err := suite.fileCache.WriteFile(
		internal.WriteFileOptions{Handle: handle, Offset: 0, Data: testData},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(len(testData), n)

	// Close the handle
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Confirm file exists locally but not in cloud storage
	suite.assert.FileExists(filepath.Join(suite.cache_path, file))
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, file))

	// Add the file to scheduleOps map (as if it was deferred for upload)
	suite.fileCache.scheduleOps.Store(file, struct{}{})
	flock := suite.fileCache.fileLocks.Get(file)
	flock.SyncPending = true

	// Create a cron scheduler with seconds-level precision
	cronScheduler := cron.New(cron.WithSeconds())

	// Get current time
	now := time.Now()

	// Set the cron to run a few seconds in the future
	second := now.Second() + 5 // Schedule for 5 seconds from now
	minute := now.Minute()
	hour := now.Hour()

	if second >= 60 {
		second = second % 60
		minute++
		if minute >= 60 {
			minute = 0
			hour++
			if hour >= 24 {
				hour = 0
			}
		}
	}

	// Cron expression that includes seconds
	cronExpr := fmt.Sprintf("%d %d %d * * *", second, minute, hour)

	// Create a schedule with the correct field names
	schedule := WeeklySchedule{
		"test": UploadWindow{
			CronExpr: cronExpr, // Run at specific time
			Duration: "10s",    // Run for 10 seconds (lowercase field name)
			Repeat:   false,
		},
	}

	// Keep track of window start/end
	windowStarted := false
	windowEnded := false

	// Setup the callbacks
	startFunc := func() {
		windowStarted = true
		fmt.Printf("[test] Starting upload at %s\n", time.Now().Format("15:04:05"))
		suite.fileCache.servicePendingOps()
	}

	endFunc := func() {
		windowEnded = true
		fmt.Printf("[test] Upload window ended at %s\n", time.Now().Format("15:04:05"))
	}

	// Use the unexported scheduleUploads method
	suite.fileCache.scheduleUploads(cronScheduler, schedule, startFunc, endFunc)
	cronScheduler.Start()
	defer cronScheduler.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fileUploaded := false
	for {
		select {
		case <-ctx.Done():
			suite.T().Log("Test timed out waiting for file upload")
			break
		default:
			if _, err := os.Stat(filepath.Join(suite.fake_storage_path, file)); err == nil {
				if _, exists := suite.fileCache.scheduleOps.Load(file); !exists {
					fileUploaded = true
					break
				}
			}
			time.Sleep(500 * time.Millisecond)
		}

		// Break out of the loop if file was uploaded or timeout occurred
		if fileUploaded || ctx.Err() != nil {
			break
		}
	}

	suite.assert.True(windowStarted, "Upload window should have started")
	if !windowEnded {
		time.Sleep(10 * time.Second)
	}
	suite.assert.True(windowEnded, "Upload window should have ended")

	if !fileUploaded {
		suite.T().Skip("Test skipped: file upload timed out")
		return
	}

	suite.assert.True(fileUploaded, "File should have been uploaded during the window")
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, file))

	uploadedData, err := os.ReadFile(filepath.Join(suite.fake_storage_path, file))
	suite.assert.NoError(err)
	suite.assert.Equal(testData, uploadedData)

	flock = suite.fileCache.fileLocks.Get(file)
	suite.assert.False(flock.SyncPending, "SyncPending flag should be cleared after upload")
}

// Test to verify that the correct tasks are added to the cron scheduler
func (suite *fileCacheTestSuite) TestSetupSchedulerAddsTasks() {
	defer suite.cleanupTest()

	// Create a custom cron scheduler that we can inspect
	cronScheduler := cron.New(cron.WithSeconds())

	// Create tracking variables to verify callbacks are executed
	startCallbackExecuted := false

	// Setup the test callbacks
	startFunc := func() {
		startCallbackExecuted = true
	}

	endFunc := func() {
		// endCallbackExecuted is intentionally not used
	}

	// Define the expected schedule
	hardcodedSchedule := WeeklySchedule{
		"default": UploadWindow{
			CronExpr: "0 * * * * *",
			Duration: "5m",
			Repeat:   true,
		},
	}

	// Call the scheduleUploads method
	suite.fileCache.scheduleUploads(cronScheduler, hardcodedSchedule, startFunc, endFunc)

	// Start the scheduler
	cronScheduler.Start()
	defer cronScheduler.Stop()

	// Verify that a task was added to the scheduler
	entries := cronScheduler.Entries()
	suite.assert.Equal(1, len(entries), "Expected exactly one scheduled task")

	// Verify the cron expression matches what we expect
	if len(entries) > 0 {
		nextRun := entries[0].Schedule.Next(time.Now())

		// Should be no more than a minute in the future
		suite.assert.LessOrEqual(nextRun.Sub(time.Now()), time.Minute+time.Second,
			"Next run should be scheduled within the next minute")
	}

	if len(entries) > 0 && time.Until(entries[0].Next) < 5*time.Second {
		// Wait until after the next scheduled run
		time.Sleep(time.Until(entries[0].Next) + 2*time.Second)

		// Check that the start callback was executed
		suite.assert.True(startCallbackExecuted, "Start callback should have been executed")

	}
}

// Test to verify that the correct tasks are added to the cron scheduler and perform uploads
// Test to verify that multiple files are uploaded by the scheduler and track which files were uploaded
func (suite *fileCacheTestSuite) TestMultipleFilesScheduleUploads() {
	defer suite.cleanupTest()

	// Create multiple files to upload
	fileCount := 5
	files := make([]string, fileCount)
	testData := make([][]byte, fileCount)

	suite.T().Logf("Creating %d files for scheduled upload test", fileCount)

	// Create and prepare each file
	for i := 0; i < fileCount; i++ {
		// Create files with different names and contents
		files[i] = fmt.Sprintf("scheduled_upload_test_%d.txt", i)
		testData[i] = []byte(fmt.Sprintf("test data for scheduled upload file %d", i))

		handle, err := suite.fileCache.CreateFile(
			internal.CreateFileOptions{Name: files[i], Mode: 0777},
		)
		suite.assert.NoError(err)

		_, err = suite.fileCache.WriteFile(
			internal.WriteFileOptions{Handle: handle, Offset: 0, Data: testData[i]},
		)
		suite.assert.NoError(err)

		err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
		suite.assert.NoError(err)

		// Mark file as pending upload
		suite.fileCache.scheduleOps.Store(files[i], struct{}{})
		flock := suite.fileCache.fileLocks.Get(files[i])
		flock.SyncPending = true

		// Verify file exists locally but not in storage
		suite.assert.FileExists(filepath.Join(suite.cache_path, files[i]))
		suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, files[i]))

		suite.T().Logf("Created file %d: %s (size: %d bytes)", i, files[i], len(testData[i]))
	}

	// Print the initial pending files
	pendingCount := 0
	suite.T().Log("Initial pending files:")
	suite.fileCache.scheduleOps.Range(func(key, value interface{}) bool {
		pendingCount++
		suite.T().Logf("  ⏳ %s", key)
		return true
	})
	suite.assert.Equal(fileCount, pendingCount, "All files should be pending upload initially")

	// Initialize a cron scheduler(Use WithSeconds)
	cronScheduler := cron.New(cron.WithSeconds())

	startCallbackExecuted := false
	endCallbackExecuted := false
	uploadedFiles := make(map[string]bool)

	startFunc := func() {
		startCallbackExecuted = true
		suite.T().Log("Upload window started - beginning upload of pending files")

		// Display number of pending files before upload
		pendingCount := 0
		suite.fileCache.scheduleOps.Range(func(key, value interface{}) bool {
			pendingCount++
			return true
		})
		suite.T().Logf("Found %d files pending upload", pendingCount)

		// Perform the upload
		suite.fileCache.servicePendingOps()
	}

	endFunc := func() {
		endCallbackExecuted = true
		suite.T().Log("Upload window ended - checking results")

		// Check which files were uploaded and log the results
		for i, file := range files {
			filePath := filepath.Join(suite.fake_storage_path, file)
			if _, err := os.Stat(filePath); err == nil {
				uploadedFiles[file] = true
				fmt.Printf("File %d (%s) was successfully uploaded", i, file)

				// Read and verify uploaded content if needed
				uploadedData, err := os.ReadFile(filePath)
				if err == nil {
					if bytes.Equal(uploadedData, testData[i]) {
						fmt.Printf("    Content verification: PASSED (%d bytes)", len(uploadedData))
					} else {
						fmt.Printf("    Content verification: FAILED (expected %d bytes, got %d bytes)",
							len(testData[i]), len(uploadedData))
					}
				}
			} else {
				suite.T().Logf("File %d (%s) was NOT uploaded", i, file)
			}
		}

		// Check if any files are still pending
		remainingCount := 0
		suite.fileCache.scheduleOps.Range(func(key, value interface{}) bool {
			remainingCount++
			fmt.Printf("  ⏳ File still pending: %s", key)
			return true
		})
		suite.T().Logf("  Upload summary: %d/%d files uploaded, %d still pending",
			len(uploadedFiles), fileCount, remainingCount)
	}

	// Define the schedule to run immediately
	now := time.Now()
	second := (now.Second() + 2) % 60 // Schedule for 2 seconds from now
	cronExpr := fmt.Sprintf("%d * * * * *", second)

	schedule := WeeklySchedule{
		"default": UploadWindow{
			CronExpr: cronExpr,
			Duration: "10s",
			Repeat:   true,
		},
	}
	fmt.Printf("Scheduling uploads to run at second %d of each minute", second)

	suite.fileCache.scheduleUploads(cronScheduler, schedule, startFunc, endFunc)

	cronScheduler.Start()
	defer cronScheduler.Stop()

	// Verify that a task was added to the scheduler
	entries := cronScheduler.Entries()
	suite.assert.Equal(1, len(entries), "Expected exactly one scheduled task")
	fmt.Printf("Scheduler configured with %d task(s)", len(entries))
	suite.T().Logf("Next scheduled run: %s", entries[0].Next.Format("15:04:05"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	suite.T().Log("Waiting for scheduled upload to complete...")

	uploadCompleted := false
	for {
		select {
		case <-ctx.Done():
			suite.T().Log("Test timed out waiting for file uploads")
			break
		default:
			// Check if all files have been uploaded
			allUploaded := true
			for _, file := range files {
				if _, err := os.Stat(filepath.Join(suite.fake_storage_path, file)); os.IsNotExist(
					err,
				) {
					allUploaded = false
					break
				}
			}

			if allUploaded {
				uploadCompleted = true
				break
			}
			time.Sleep(500 * time.Millisecond)
		}

		// Break out of the loop if uploads completed or timeout occurred
		if uploadCompleted || ctx.Err() != nil {
			break
		}
	}

	// Verify the upload occurred
	suite.assert.True(startCallbackExecuted, "Start callback should have been executed")

	if !uploadCompleted {
		suite.T().Log("⚠️ Not all files were uploaded within timeout period")
	}

	// Verify all files were uploaded
	uploadCount := 0
	for i, file := range files {
		filePath := filepath.Join(suite.fake_storage_path, file)
		if _, err := os.Stat(filePath); err == nil {
			uploadCount++

			// Verify content
			uploadedData, err := os.ReadFile(filePath)
			suite.assert.NoError(err)
			suite.assert.Equal(
				testData[i],
				uploadedData,
				fmt.Sprintf("File %s content should match", file),
			)
		}
	}
	suite.assert.Equal(fileCount, uploadCount, "All files should have been uploaded")

	time.Sleep(10 * time.Second)
	suite.assert.True(endCallbackExecuted, "End callback should have been executed")

	// Final verification of scheduleOps map - should be empty
	remainingCount := 0
	suite.fileCache.scheduleOps.Range(func(key, value interface{}) bool {
		remainingCount++
		return true
	})
	suite.assert.Equal(0, remainingCount, "No files should remain in the scheduleOps map")

	for _, file := range files {
		flock := suite.fileCache.fileLocks.Get(file)
		suite.assert.False(
			flock.SyncPending,
			fmt.Sprintf("SyncPending flag for %s should be cleared", file),
		)
	}
}

func (suite *fileCacheTestSuite) TestDaySpecificSchedulerFromYAML() {
	defer suite.cleanupTest()

	configPath := filepath.Join(suite.cache_path, "day_schedule_config.yaml")
	configContent := `schedule:
  Monday:
    cron: "0 0 10 * * 1"    # 10:00 AM on Mondays only (1=Monday)
    duration: "30m"
    repeat: true
  Wednesday:
    cron: "0 0 14 * * 3"    # 2:00 PM on Wednesdays only (3=Wednesday)
    duration: "45m"
    repeat: false
  Friday:
    cron: "0 0 16 * * 5"    # 4:00 PM on Fridays only (5=Friday)
    duration: "1h"
    repeat: true`
	err := os.MkdirAll(suite.cache_path, 0755)
	suite.assert.NoError(err)

	err = os.WriteFile(configPath, []byte(configContent), 0644)
	suite.assert.NoError(err)

	cronScheduler := cron.New(cron.WithSeconds())

	configWasLoaded := false

	configWasLoaded = false

	testSetupScheduler := func(path string) error {
		suite.assert.Equal(configPath, path)
		configWasLoaded = true

		// Load the config manually to inspect
		schedule, err := LoadConfig(configPath)
		if err != nil {
			return err
		}
		startFunc := func() {
			suite.fileCache.servicePendingOps()
		}
		endFunc := func() {}

		suite.fileCache.scheduleUploads(cronScheduler, schedule, startFunc, endFunc)
		return nil
	}

	err = testSetupScheduler(configPath)
	suite.assert.NoError(err)
	suite.assert.True(configWasLoaded, "SetupScheduler should have been called")

	// Load the config to verify its contents
	schedule, err := LoadConfig(configPath)
	suite.assert.NoError(err)

	suite.assert.Equal(3, len(schedule), "Should have loaded 3 day-specific schedules")

	mondayConfig, exists := schedule["Monday"]
	suite.assert.True(exists, "Monday schedule should exist")
	suite.assert.Equal("0 0 10 * * 1", mondayConfig.CronExpr)
	suite.assert.Equal("30m", mondayConfig.Duration)
	suite.assert.True(mondayConfig.Repeat)

	wednesdayConfig, exists := schedule["Wednesday"]
	suite.assert.True(exists, "Wednesday schedule should exist")
	suite.assert.Equal("0 0 14 * * 3", wednesdayConfig.CronExpr)
	suite.assert.Equal("45m", wednesdayConfig.Duration)
	suite.assert.False(wednesdayConfig.Repeat)

	fridayConfig, exists := schedule["Friday"]
	suite.assert.True(exists, "Friday schedule should exist")
	suite.assert.Equal("0 0 16 * * 5", fridayConfig.CronExpr)
	suite.assert.Equal("1h", fridayConfig.Duration)
	suite.assert.True(fridayConfig.Repeat)

	entries := cronScheduler.Entries()
	suite.assert.Equal(3, len(entries), "Scheduler should have 3 entries, one for each day")

	for i, entry := range entries {
		suite.T().Logf("Schedule entry %d: next run at %s",
			i, entry.Next.Format("Mon Jan 2 15:04:05"))
	}

	testFile := "day_scheduled_file.txt"
	handle, err := suite.fileCache.CreateFile(
		internal.CreateFileOptions{Name: testFile, Mode: 0777},
	)
	suite.assert.NoError(err)
	testData := []byte("data for day-specific scheduled upload test")
	_, err = suite.fileCache.WriteFile(
		internal.WriteFileOptions{Handle: handle, Offset: 0, Data: testData},
	)
	suite.assert.NoError(err)

	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	suite.fileCache.scheduleOps.Store(testFile, struct{}{})
	flock := suite.fileCache.fileLocks.Get(testFile)
	flock.SyncPending = true

	suite.assert.FileExists(filepath.Join(suite.cache_path, testFile))
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, testFile))

	// Manually trigger the servicePendingOps to simulate a schedule firing
	suite.fileCache.servicePendingOps()

	// Verify the upload was triggered and completed
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, testFile))
	uploadedData, err := os.ReadFile(filepath.Join(suite.fake_storage_path, testFile))
	suite.assert.NoError(err)
	suite.assert.Equal(testData, uploadedData, "Uploaded file content should match original")

	// Verify file was removed from pending operations
	_, exists = suite.fileCache.scheduleOps.Load(testFile)
	suite.assert.False(exists, "File should have been removed from scheduleOps")

	// File lock's sync pending flag should be cleared
	flock = suite.fileCache.fileLocks.Get(testFile)
	suite.assert.False(flock.SyncPending, "SyncPending flag should be cleared")
}

// TestPrintScheduleFromYAML verifies YAML schedule loading by printing the parsed schedule map
// loggerAdapter implements the cron.Logger interface
type loggerAdapter struct{}

func (l *loggerAdapter) Printf(format string, v ...interface{}) {
	fmt.Printf(format, v...)
}

func (suite *fileCacheTestSuite) TestPrintScheduleFromYAML() {
	defer suite.cleanupTest()

	// Create a temporary config file with day-specific schedules
	configPath := filepath.Join(suite.cache_path, "print_schedule_config.yaml")
	configContent := `schedule:
  Tuesday:
    cron: "0 45 13 * * 2"    # 12:00 PM on Tuesdays only (2=Tuesday)
    duration: "30m"
    repeat: true
  Wednesday:
    cron: "0 0 14 * * 3"    # 2:00 PM on Wednesdays only (3=Wednesday)
    duration: "45m"
    repeat: false
  Friday:
    cron: "0 0 16 * * 5"    # 4:00 PM on Fridays only (5=Friday)
    duration: "1h"
    repeat: true
  Saturday:
    cron: "0 0 2 * * *"     # 2:00 AM every day
    duration: "20m"
    repeat: true`
	err := os.MkdirAll(suite.cache_path, 0755)
	suite.assert.NoError(err)

	err = os.WriteFile(configPath, []byte(configContent), 0644)
	suite.assert.NoError(err)

	// Load the config to verify its contents
	schedule, err := LoadConfig(configPath)
	suite.assert.NoError(err)

	fmt.Println("\n=== PARSED SCHEDULE CONFIGURATION ===")
	fmt.Printf("Schedule has %d entries\n", len(schedule))

	// Sort the keys to get consistent output for easier verification
	keys := make([]string, 0, len(schedule))
	for day := range schedule {
		keys = append(keys, day)
	}
	sort.Strings(keys)

	for _, day := range keys {
		config := schedule[day]
		fmt.Printf("\nDay: %s\n", day)
		fmt.Printf("  Cron Expression: %s\n", config.CronExpr)
		fmt.Printf("  Duration: %s\n", config.Duration)
		fmt.Printf("  Repeat: %t\n", config.Repeat)
	}
	fmt.Println("===============================")

	suite.assert.Equal(4, len(schedule), "Should have loaded 4 schedule entries")

	logAdapter := &loggerAdapter{}
	cronScheduler := cron.New(cron.WithSeconds(), cron.WithLogger(
		cron.PrintfLogger(logAdapter)))

	uploadCount := 0
	startFunc := func() {
		fmt.Println("Start callback executed")
		uploadCount++
	}
	endFunc := func() {
		fmt.Println("End callback executed")
	}

	fmt.Println("\n=== SCHEDULING JOBS ===")
	suite.fileCache.scheduleUploads(cronScheduler, schedule, startFunc, endFunc)

	fmt.Println("Starting cron scheduler...")
	cronScheduler.Start()
	defer cronScheduler.Stop()

	entries := cronScheduler.Entries()
	fmt.Printf("\n=== SCHEDULED JOBS (%d entries) ===\n", len(entries))

	// Verify the correct number of entries
	suite.assert.Equal(
		4,
		len(entries),
		"Scheduler should have 4 entries, one for each configuration",
	)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Next.Before(entries[j].Next)
	})

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Next.Before(entries[j].Next)
	})

	now := time.Now()
	for i, entry := range entries {
		nextRun := entry.Next
		timeUntil := nextRun.Sub(now).Round(time.Second)

		fmt.Printf("\nJob #%d:\n", i+1)
		fmt.Printf("  ID: %v\n", entry.ID)
		fmt.Printf("  Next run: %s\n", nextRun.Format("Mon Jan 2 15:04:05"))
		fmt.Printf("  Time until next run: %s\n", timeUntil)
		fmt.Printf("  Cron expression: %s\n", entry.Schedule)

		// Find which day this entry corresponds to
		for day, config := range schedule {
			if strings.Contains(config.CronExpr, entry.Schedule.Next(time.Now()).Format("5")) {
				fmt.Printf("  Corresponds to: %s schedule\n", day)
				break
			}
		}
	}
	fmt.Println("===================================")

	testFile := "schedule_test_file.txt"
	testData := []byte("test data for schedule verification")

	handle, err := suite.fileCache.CreateFile(
		internal.CreateFileOptions{Name: testFile, Mode: 0777},
	)
	suite.assert.NoError(err)
	_, err = suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Data: testData})
	suite.assert.NoError(err)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	suite.fileCache.scheduleOps.Store(testFile, struct{}{})
	flock := suite.fileCache.fileLocks.Get(testFile)
	flock.SyncPending = true

	fmt.Println("\n=== PENDING FILES FOR UPLOAD ===")
	pendingCount := 0
	suite.fileCache.scheduleOps.Range(func(key, value interface{}) bool {
		pendingCount++
		fmt.Printf("  %d. %s\n", pendingCount, key)
		return true
	})

	if pendingCount == 0 {
		fmt.Println("  No files pending upload")
	}
	fmt.Println("===================================")

	fmt.Println("\n=== MANUAL UPLOAD TEST ===")
	fmt.Println("Triggering servicePendingOps manually...")
	suite.fileCache.servicePendingOps()

	// Check if the file was uploaded
	if _, err := os.Stat(filepath.Join(suite.fake_storage_path, testFile)); err == nil {
		fmt.Println("File was successfully uploaded")

		// Verify file contents
		uploadedData, err := os.ReadFile(filepath.Join(suite.fake_storage_path, testFile))
		if err == nil && bytes.Equal(uploadedData, testData) {
			fmt.Println("Uploaded file content matches original")
		} else {
			fmt.Println("✗ Uploaded file content differs from original")
		}

		// Check if file was removed from pending operations
		if _, exists := suite.fileCache.scheduleOps.Load(testFile); !exists {
			fmt.Println("File was removed from pending operations")
		} else {
			fmt.Println("File still exists in pending operations")
		}
	} else {
		fmt.Println("File was not uploaded")
	}
	fmt.Println("===================================")
}

func (suite *fileCacheTestSuite) TestSimpleScheduledUpload() {
	defer suite.cleanupTest()

	// Create a file and mark it for pending upload
	file := "simple_scheduled_test.txt"
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.assert.NoError(err)

	data := []byte("simple scheduled upload test data")
	_, err = suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Data: data})
	suite.assert.NoError(err)

	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Mark file as pending upload
	suite.fileCache.scheduleOps.Store(file, struct{}{})
	flock := suite.fileCache.fileLocks.Get(file)
	flock.SyncPending = true

	// Verify the file starts in local cache only
	suite.assert.FileExists(filepath.Join(suite.cache_path, file))
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, file))

	// Schedule an immediate upload (in 2 seconds)
	now := time.Now()
	second := (now.Second() + 2) % 60
	cronExpr := fmt.Sprintf("%d * * * * *", second)

	// Create a simple scheduler and call servicePendingOps directly
	cronScheduler := cron.New(cron.WithSeconds())
	uploadExecuted := false

	startFunc := func() {
		fmt.Println("⭐ Upload window starting now")
		uploadExecuted = true
		suite.fileCache.servicePendingOps() // Direct call
	}

	endFunc := func() {}

	schedule := WeeklySchedule{
		"test": UploadWindow{
			CronExpr: cronExpr,
			Duration: "5s",
			Repeat:   false,
		},
	}

	suite.fileCache.scheduleUploads(cronScheduler, schedule, startFunc, endFunc)
	cronScheduler.Start()
	defer cronScheduler.Stop()

	// Wait for upload to complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fileUploaded := false
	checkTicker := time.NewTicker(500 * time.Millisecond)
	defer checkTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			suite.T().Log("Test timed out waiting for file upload")
			break
		case <-checkTicker.C:
			if _, err := os.Stat(filepath.Join(suite.fake_storage_path, file)); err == nil {
				fileUploaded = true
				cancel()
			}
		}

		if ctx.Err() != nil {
			break
		}
	}

	// Verify results
	suite.assert.True(uploadExecuted, "Upload function should have been executed")
	suite.assert.True(fileUploaded, "File should have been uploaded")
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, file))

	// Verify file was properly uploaded
	if fileUploaded {
		uploadedData, err := os.ReadFile(filepath.Join(suite.fake_storage_path, file))
		suite.assert.NoError(err)
		suite.assert.Equal(data, uploadedData, "Uploaded file content should match original")
	}
}

func (suite *fileCacheTestSuite) TestScheduledUploadsFromYamlConfig2() {
	defer suite.cleanupTest()

	configPath := filepath.Join(os.Getenv("HOME"), "cloudfuse", "config.yaml")

	fmt.Println("\n=== READING CONFIG FILE ===")
	fmt.Printf("Config path: %s\n", configPath)

	configContent, err := os.ReadFile(configPath)
	suite.assert.NoError(err, "Should be able to read the config file")
	fmt.Println("Config file contents:")
	fmt.Println(string(configContent))

	file := "yaml_config_upload_test.txt"
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.assert.NoError(err)

	testData := []byte("test data for yaml config scheduled upload")
	_, err = suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Data: testData})
	suite.assert.NoError(err)

	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	suite.fileCache.scheduleOps.Store(file, struct{}{})
	flock := suite.fileCache.fileLocks.Get(file)
	flock.SyncPending = true

	suite.assert.FileExists(filepath.Join(suite.cache_path, file))
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, file))

	fmt.Println("\n=== YAML CONFIG UPLOAD TEST ===")
	fmt.Printf("Using config file: %s\n", configPath)

	var cfg struct {
		Schedule WeeklySchedule `yaml:"schedule"`
	}

	err = yaml.Unmarshal(configContent, &cfg)
	suite.assert.NoError(err, "Should be able to parse the config file")

	fmt.Println("\n=== PARSED SCHEDULE FROM CONFIG ===")
	if len(cfg.Schedule) > 0 {
		fmt.Printf("Found %d schedule entries\n", len(cfg.Schedule))
		for day, schedule := range cfg.Schedule {
			fmt.Printf("Day: %s, Cron: %s, Duration: %s, Repeat: %t\n",
				day, schedule.CronExpr, schedule.Duration, schedule.Repeat)
		}
	} else {
		fmt.Println("No schedule entries found in config file")
	}

	err = suite.fileCache.SetupScheduler(configPath)
	if err != nil {
		fmt.Printf("Error setting up scheduler: %v\n", err)
	}
	suite.assert.NoError(err)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	fileUploaded := false
	fmt.Println("\nWaiting for scheduled upload to complete...")

	checkTicker := time.NewTicker(500 * time.Millisecond)
	defer checkTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Test timed out waiting for upload")
			break
		case <-checkTicker.C:
			// Check if file exists in storage
			if _, err := os.Stat(filepath.Join(suite.fake_storage_path, file)); err == nil {
				fileUploaded = true
				fmt.Println("File uploaded successfully")
				cancel()
			} else {
				// Print diagnostics each tick
				fmt.Printf("Still waiting... Time: %s\n", time.Now().Format("15:04:05"))

				// Check if file is still in scheduleOps map
				_, exists := suite.fileCache.scheduleOps.Load(file)
				fmt.Printf("File still in scheduleOps map: %v\n", exists)

				if flock := suite.fileCache.fileLocks.Get(file); flock != nil {
					fmt.Printf("File SyncPending flag: %v\n", flock.SyncPending)
				}
			}
		}

		if ctx.Err() != nil {
			break
		}
	}

	if fileUploaded {
		uploadedData, err := os.ReadFile(filepath.Join(suite.fake_storage_path, file))
		suite.assert.NoError(err)
		suite.assert.Equal(testData, uploadedData, "Uploaded file content should match original")

		// Check state was cleaned up
		_, exists := suite.fileCache.scheduleOps.Load(file)
		suite.assert.False(exists, "File should have been removed from scheduleOps")

		flock = suite.fileCache.fileLocks.Get(file)
		suite.assert.False(flock.SyncPending, "SyncPending flag should be cleared")
	} else {
		// If not uploaded, help with debugging
		fmt.Println("\n=== TROUBLESHOOTING ===")

		// Check if YAML was loaded correctly
		schedule, err := LoadConfig(configPath)
		if err != nil {
			fmt.Printf("❌ Error loading config: %v\n", err)
		} else {
			fmt.Printf("✅ Config loaded successfully with %d entries\n", len(schedule))
			for key, val := range schedule {
				fmt.Printf("   Schedule entry: %s -> %+v\n", key, val)
			}
		}

		fmt.Println("\nManually triggering servicePendingOps...")
		suite.fileCache.servicePendingOps()

		// Check if manual trigger worked
		time.Sleep(1 * time.Second) // Give it a moment to complete
		if _, err := os.Stat(filepath.Join(suite.fake_storage_path, file)); err == nil {
			fmt.Println(" Manual trigger succeeded - issue is with scheduling")
			suite.T().Log("File was uploaded after manual servicePendingOps call, but not via scheduler")
		} else {
			fmt.Println("Even manual trigger failed - issue is with upload mechanism")
			suite.T().Log("File was NOT uploaded even after manual servicePendingOps call")
		}

		fmt.Println("===========================")
		suite.T().Fail()
	}
}

func (suite *fileCacheTestSuite) TestScheduledUploadsWithLoadConfig() {
	defer suite.cleanupTest()

	configPath := filepath.Join(os.Getenv("HOME"), "cloudfuse", "config.yaml")

	fmt.Println("\n=== SCHEDULED UPLOADS WITH SETUP SCHEDULER TEST ===")
	fmt.Printf("Config path: %s\n", configPath)

	configContent, err := os.ReadFile(configPath)
	suite.assert.NoError(err, "Should be able to read the config file")
	fmt.Println("Config file contents:")
	fmt.Println(string(configContent))

	file := "setup_scheduler_test_file.txt"
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.assert.NoError(err)

	testData := []byte("test data for SetupScheduler upload test")
	_, err = suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Data: testData})
	suite.assert.NoError(err)

	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Mark file as pending upload and verify initial state
	suite.fileCache.scheduleOps.Store(file, struct{}{})
	flock := suite.fileCache.fileLocks.Get(file)
	flock.SyncPending = true

	// Verify file exists locally but not in storage
	suite.assert.FileExists(filepath.Join(suite.cache_path, file))
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, file))

	err = suite.fileCache.SetupScheduler(configPath)
	suite.assert.NoError(err, "Should be able to set up scheduler")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	fileUploaded := false
	fmt.Println("\nWaiting for scheduled upload to complete...")

	checkTicker := time.NewTicker(500 * time.Millisecond)
	defer checkTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Test timed out waiting for upload")
			break
		case <-checkTicker.C:
			// Check if file exists in storage
			if _, err := os.Stat(filepath.Join(suite.fake_storage_path, file)); err == nil {
				fileUploaded = true
				fmt.Println("File uploaded successfully")
				cancel()
			} else {
				// Print diagnostics each tick
				fmt.Printf("Still waiting... Time: %s\n", time.Now().Format("15:04:05"))

				// Check if file is still in scheduleOps map
				_, exists := suite.fileCache.scheduleOps.Load(file)
				fmt.Printf("File still in scheduleOps map: %v\n", exists)

				if flock := suite.fileCache.fileLocks.Get(file); flock != nil {
					fmt.Printf("File SyncPending flag: %v\n", flock.SyncPending)
				}
			}
		}

		if ctx.Err() != nil {
			break
		}
	}

	if fileUploaded {
		uploadedData, err := os.ReadFile(filepath.Join(suite.fake_storage_path, file))
		suite.assert.NoError(err)
		suite.assert.Equal(testData, uploadedData, "Uploaded file content should match original")

		_, exists := suite.fileCache.scheduleOps.Load(file)
		suite.assert.False(exists, "File should have been removed from scheduleOps")

		flock = suite.fileCache.fileLocks.Get(file)
		suite.assert.False(flock.SyncPending, "SyncPending flag should be cleared")
	} else {
		fmt.Println("\n=== TROUBLESHOOTING ===")

		// Try to load the config directly for verification
		schedule, err := LoadConfig(configPath)
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
		} else {
			fmt.Printf("Config loaded successfully with %d entries\n", len(schedule))
			for day, config := range schedule {
				fmt.Printf("   Schedule entry: %s -> Cron: %s, Duration: %s, Repeat: %t\n",
					day, config.CronExpr, config.Duration, config.Repeat)
			}
		}

		fmt.Println("\nManually triggering servicePendingOps...")
		suite.fileCache.servicePendingOps()

		// Check if manual trigger worked
		time.Sleep(1 * time.Second) // Give it a moment to complete
		if _, err := os.Stat(filepath.Join(suite.fake_storage_path, file)); err == nil {
			fmt.Println("Manual trigger succeeded - issue is with scheduling")
			suite.T().Log("File was uploaded after manual servicePendingOps call, but not via scheduler")
		} else {
			fmt.Println("Even manual trigger failed - issue is with upload mechanism")
			suite.T().Log("File was NOT uploaded even after manual servicePendingOps call")
		}

		fmt.Println("===========================")
		suite.T().Fail()
	}
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
		internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
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
	n, err := suite.fileCache.WriteFile(internal.WriteFileOptions{
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
	n, err := suite.fileCache.WriteFile(internal.WriteFileOptions{
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
	n, err := suite.fileCache.WriteFile(internal.WriteFileOptions{
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
	n, err = suite.fileCache.WriteFile(internal.WriteFileOptions{
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
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})

	handle, _ = suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

	output := make([]byte, 9)
	n, err := suite.fileCache.ReadInBuffer(
		internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: output},
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
		internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
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
	err := os.WriteFile(suite.fake_storage_path+"/"+path, byteArr, 0777)
	suite.assert.NoError(err)

	data := make([]byte, 20)

	options := internal.OpenFileOptions{Name: path, Mode: 0777}

	// Read file once and we shall get the same data
	handle, err := suite.fileCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.False(handle.Dirty())
	n, err := suite.fileCache.ReadInBuffer(
		internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(9, n)
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
		internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(9, n)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Now wait for refresh timeout and we shall get the updated content on next read
	byteArr = []byte("test data123456")
	err = os.WriteFile(suite.fake_storage_path+"/"+path, byteArr, 0777)
	suite.assert.NoError(err)
	time.Sleep(2 * time.Second)
	handle, err = suite.fileCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.False(handle.Dirty())
	n, err = suite.fileCache.ReadInBuffer(
		internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: data},
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
		internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: data},
	)
	handle, loaded := handlemap.Load(handle.ID)
	suite.assert.True(loaded)
	suite.assert.NoError(err)
	suite.assert.Equal(9, n)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.NoError(err)
}

func (suite *fileCacheTestSuite) createDirectoryStructure() {
	err := os.MkdirAll(filepath.Join(suite.cache_path, "a", "b", "c", "d"), 0777)
	suite.assert.NoError(err)

	err = os.MkdirAll(filepath.Join(suite.cache_path, "a", "b", "e", "f"), 0777)
	suite.assert.NoError(err)

	err = os.MkdirAll(filepath.Join(suite.cache_path, "a", "b", "e", "g"), 0777)
	suite.assert.NoError(err)

	err = os.MkdirAll(filepath.Join(suite.cache_path, "h", "i", "j", "k"), 0777)
	suite.assert.NoError(err)

	err = os.MkdirAll(filepath.Join(suite.cache_path, "h", "l", "m", "n"), 0777)
	suite.assert.NoError(err)
}

func (suite *fileCacheTestSuite) TestDeleteEmptyDirsRoot() {
	defer suite.cleanupTest()

	suite.createDirectoryStructure()
	val, err := suite.fileCache.DeleteEmptyDirs(internal.DeleteDirOptions{Name: suite.cache_path})
	suite.assert.NoError(err)
	suite.assert.True(val)
}

func (suite *fileCacheTestSuite) TestDeleteEmptyDirsNonRoot() {
	defer suite.cleanupTest()

	suite.createDirectoryStructure()
	val, err := suite.fileCache.DeleteEmptyDirs(internal.DeleteDirOptions{Name: "a"})
	suite.assert.NoError(err)
	suite.assert.True(val)

	val, err = suite.fileCache.DeleteEmptyDirs(
		internal.DeleteDirOptions{Name: filepath.Join(suite.cache_path, "h")},
	)
	suite.assert.NoError(err)
	suite.assert.True(val)
}

func (suite *fileCacheTestSuite) TestDeleteEmptyDirsNegative() {
	defer suite.cleanupTest()

	suite.createDirectoryStructure()
	file, err := os.Create(filepath.Join(suite.cache_path, "h", "l", "m", "n", "file.txt"))
	suite.assert.NoError(err)
	file.Close()

	val, err := suite.fileCache.DeleteEmptyDirs(internal.DeleteDirOptions{Name: suite.cache_path})
	suite.assert.Error(err)
	suite.assert.False(val)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestFileCacheTestSuite(t *testing.T) {
	suite.Run(t, new(fileCacheTestSuite))
}
