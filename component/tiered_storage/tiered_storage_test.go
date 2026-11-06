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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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
	"github.com/stretchr/testify/require"
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

// localFile returns where the component keeps a local-owned file.
func (suite *tieredStorageTestSuite) localFile(name string) string {
	return filepath.Join(suite.cache_path, localDirName, filepath.FromSlash(name))
}

// cacheFile returns where the component keeps an open cloud-owned file.
func (suite *tieredStorageTestSuite) cacheFile(name string) string {
	return filepath.Join(suite.cache_path, cacheDirName, filepath.FromSlash(name))
}

func TestLocalPath(t *testing.T) {
	storage := &TieredStorage{tmpPath: filepath.Join(string(os.PathSeparator), "root")}
	localRoot := filepath.Join(storage.tmpPath, localDirName)
	absoluteName, err := filepath.Abs("file")
	require.NoError(t, err)

	tests := []struct {
		name     string
		expected string
		valid    bool
	}{
		{name: "", expected: localRoot, valid: true},
		{name: "dir/file", expected: filepath.Join(localRoot, "dir", "file"), valid: true},
		{name: "../file", valid: false},
		{name: "dir/../../file", valid: false},
		{name: absoluteName, valid: false},
	}
	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			name     string
			expected string
			valid    bool
		}{name: `dir\file`, expected: filepath.Join(localRoot, "dir", "file"), valid: true})
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path, err := storage.localPath(test.name)
			if test.valid {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, path)
				return
			}
			assert.ErrorIs(t, err, syscall.EINVAL)
		})
	}
}

func TestCachePath(t *testing.T) {
	storage := &TieredStorage{tmpPath: filepath.Join(string(os.PathSeparator), "root")}

	path, err := storage.cachePath("dir/file")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(storage.tmpPath, cacheDirName, "dir", "file"), path)

	_, err = storage.cachePath("../file")
	assert.ErrorIs(t, err, syscall.EINVAL)
}

// restart stops the component and starts a fresh instance on the same root.
func (suite *tieredStorageTestSuite) restart() error {
	suite.Require().NoError(suite.tieredStorage.Stop())
	suite.tieredStorage = newTestTieredStorage(suite.loopback)
	return suite.tieredStorage.Start(context.Background())
}

func (suite *tieredStorageTestSuite) TestStartClearsCacheAndKeepsLocal() {
	defer suite.cleanupTest()

	suite.Require().NoError(os.MkdirAll(filepath.Dir(suite.cacheFile("dir/cached")), 0700))
	suite.Require().NoError(os.WriteFile(suite.cacheFile("dir/cached"), []byte("c"), 0644))
	suite.Require().NoError(os.WriteFile(suite.localFile("owned"), []byte("l"), 0644))

	suite.Require().NoError(suite.restart())

	suite.assert.NoFileExists(suite.cacheFile("dir/cached"))
	suite.assert.DirExists(suite.cacheFile(""))
	suite.assert.FileExists(suite.localFile("owned"))
}

func (suite *tieredStorageTestSuite) TestStartRejectsUnexpectedRootEntries() {
	defer suite.cleanupTest()

	stray := filepath.Join(suite.cache_path, "stray")
	suite.Require().NoError(os.WriteFile(stray, []byte("old layout"), 0644))
	suite.Require().NoError(os.WriteFile(suite.cacheFile("cached"), []byte("c"), 0644))

	err := suite.restart()
	suite.Require().Error(err)
	suite.assert.Contains(err.Error(), "stray")
	suite.assert.FileExists(stray)
	// Nothing is changed before the check fails.
	suite.assert.FileExists(suite.cacheFile("cached"))
}

func TestTieredStoragePolicyConfig(t *testing.T) {
	tests := []struct {
		name         string
		config       string
		wantErr      bool
		high         float64
		low          float64
		maxEviction  uint32
		parallelism  int
		pollInterval time.Duration
	}{
		{
			name:   "defaults",
			config: "max-size-mb: 1",
			high:   0.9, low: 0.8, maxEviction: 5000, parallelism: 8,
			pollInterval: time.Second,
		},
		{
			name: "custom",
			config: "max-size-mb: 2\n  high-threshold: 90\n  low-threshold: 70\n" +
				"  parallelism: 3",
			high: 0.9, low: 0.7, maxEviction: 5000, parallelism: 3,
			pollInterval: time.Second,
		},
		{name: "missing capacity", config: "high-threshold: 80", wantErr: true},
		{
			name:    "reversed thresholds",
			config:  "max-size-mb: 1\n  high-threshold: 60\n  low-threshold: 80",
			wantErr: true,
		},
		{
			name:    "high threshold over 100",
			config:  "max-size-mb: 1\n  high-threshold: 101",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configuration := fmt.Sprintf(
				"tiered_storage:\n  path: %s\n  %s",
				t.TempDir(),
				test.config,
			)
			require.NoError(t, config.ReadConfigFromReader(strings.NewReader(configuration)))

			storage := NewTieredStorageComponent().(*TieredStorage)
			err := storage.Configure(true)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.InEpsilon(t, test.high, storage.policy.threshold, 0.0001)
			assert.InEpsilon(t, test.low, storage.policy.targetRatio, 0.0001)
			assert.Equal(t, test.maxEviction, storage.policy.maxEviction)
			assert.Equal(t, test.parallelism, storage.policy.numWorkers)
			assert.Equal(t, test.pollInterval, storage.policy.pollInterval)
		})
	}
}

func (suite *tieredStorageTestSuite) TestGetAttrLocalOnly() {
	defer suite.cleanupTest()

	const path = "local-only"
	handle, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: path, Mode: 0644},
	)
	suite.Require().NoError(err)
	_, err = suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Data: []byte("local data")},
	)
	suite.Require().NoError(err)

	attrs, err := suite.tieredStorage.GetAttr(internal.GetAttrOptions{Name: path})
	suite.Require().NoError(err)
	suite.assert.Equal(path, attrs.Path)
	suite.assert.EqualValues(len("local data"), attrs.Size)
}

func (suite *tieredStorageTestSuite) TestGetAttrCloudOnly() {
	defer suite.cleanupTest()

	const path = "cloud-only"
	handle, err := suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0644})
	suite.Require().NoError(err)
	_, err = suite.loopback.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Data: []byte("cloud data")},
	)
	suite.Require().NoError(err)
	suite.Require().NoError(suite.loopback.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))

	attrs, err := suite.tieredStorage.GetAttr(internal.GetAttrOptions{Name: path})
	suite.Require().NoError(err)
	suite.assert.EqualValues(len("cloud data"), attrs.Size)
	suite.assert.NoFileExists(suite.localFile(path))
}

func (suite *tieredStorageTestSuite) TestGetAttrPrefersLocalData() {
	defer suite.cleanupTest()

	const path = "local-and-cloud"
	cloudHandle, err := suite.loopback.CreateFile(
		internal.CreateFileOptions{Name: path, Mode: 0644},
	)
	suite.Require().NoError(err)
	_, err = suite.loopback.WriteFile(
		&internal.WriteFileOptions{Handle: cloudHandle, Data: []byte("cloud")},
	)
	suite.Require().NoError(err)
	suite.Require().NoError(
		suite.loopback.ReleaseFile(internal.ReleaseFileOptions{Handle: cloudHandle}),
	)

	handle, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path, Flags: os.O_RDWR, Mode: 0644},
	)
	suite.Require().NoError(err)
	_, err = suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: []byte("local data")},
	)
	suite.Require().NoError(err)

	attrs, err := suite.tieredStorage.GetAttr(internal.GetAttrOptions{Name: path})
	suite.Require().NoError(err)
	suite.assert.EqualValues(len("local data"), attrs.Size)

	cloudAttrs, err := suite.loopback.GetAttr(internal.GetAttrOptions{Name: path})
	suite.Require().NoError(err)
	suite.assert.EqualValues(len("cloud"), cloudAttrs.Size)
}

// isCloudStale reads a name's CloudStale flag under its file lock.
func (suite *tieredStorageTestSuite) isCloudStale(name string) bool {
	flock := suite.tieredStorage.fileLocks.Get(name)
	flock.Lock()
	defer flock.Unlock()
	return flock.CloudStale
}

// lruOrder lists the LRU from tail (next to evict) to head.
func lruOrder(q *lruQueue) []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	var names []string
	for node := q.tail; node != nil; node = node.prev {
		names = append(names, node.name)
	}
	return names
}

func (suite *tieredStorageTestSuite) TestStartRecoversLocalFilesByMtime() {
	defer suite.cleanupTest()

	now := time.Now()
	files := map[string]time.Duration{
		"newest":     0,
		"dir/oldest": -2 * time.Hour,
		"middle":     -time.Hour,
	}
	for name, age := range files {
		path := suite.localFile(name)
		suite.Require().NoError(os.MkdirAll(filepath.Dir(path), 0700))
		suite.Require().NoError(os.WriteFile(path, []byte(name), 0644))
		suite.Require().NoError(os.Chtimes(path, now.Add(age), now.Add(age)))
	}

	suite.Require().NoError(suite.restart())

	suite.assert.Equal(
		[]string{"dir/oldest", "middle", "newest"},
		lruOrder(suite.tieredStorage.policy),
	)
}

func TestUploadFailureKeepsLocalFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	next := internal.NewMockComponent(ctrl)
	cachePath := t.TempDir()
	localPath := filepath.Join(cachePath, localDirName, "file")
	require.NoError(t, os.MkdirAll(filepath.Dir(localPath), 0700))
	require.NoError(t, os.WriteFile(localPath, []byte("data"), 0644))

	storage := &TieredStorage{
		tmpPath:   cachePath,
		fileLocks: common.NewLockMap(),
		cacheSize: newCacheSizeTracker(cachePath, 0),
	}
	storage.SetNextComponent(next)
	storage.fileLocks.Get("file").CloudStale = true
	next.EXPECT().CopyFromFile(gomock.Any()).Return(errors.New("upload failed"))

	err := storage.uploadandCleanFile("file")
	require.Error(t, err)
	assert.FileExists(t, localPath)
	stale := storage.fileLocks.Get("file").CloudStale
	assert.True(t, stale)
}

func TestGetAttrLocalMakesNoCloudCall(t *testing.T) {
	ctrl := gomock.NewController(t)
	// Any call on next fails the test.
	next := internal.NewMockComponent(ctrl)
	root := t.TempDir()
	localPath := filepath.Join(root, localDirName, "file")
	require.NoError(t, os.MkdirAll(filepath.Dir(localPath), 0700))
	require.NoError(t, os.WriteFile(localPath, []byte("data"), 0644))

	storage := &TieredStorage{tmpPath: root}
	storage.SetNextComponent(next)

	attrs, err := storage.GetAttr(internal.GetAttrOptions{Name: "file"})
	require.NoError(t, err)
	assert.EqualValues(t, 4, attrs.Size)
}

func TestCheckCloudReachable(t *testing.T) {
	tests := []struct {
		name      string
		connected bool
		getAttr   error // nil means the object exists
		wantErr   bool
	}{
		{name: "connected", connected: true},
		{name: "offline, not in cloud", getAttr: syscall.ENOENT},
		{name: "offline, in cloud", wantErr: true},
		{
			name:    "offline, unknown",
			getAttr: common.NewNoCachedDataError(syscall.ENOENT),
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			next := internal.NewMockComponent(ctrl)
			next.EXPECT().CloudConnected().Return(test.connected)
			if !test.connected {
				next.EXPECT().GetAttr(internal.GetAttrOptions{Name: "file"}).
					Return(&internal.ObjAttr{}, test.getAttr)
			}
			storage := &TieredStorage{}
			storage.SetNextComponent(next)

			err := storage.checkCloudReachable("file")
			if !test.wantErr {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, &common.CloudUnreachableError{})
			assert.NotErrorIs(t, err, os.ErrNotExist)
		})
	}
}

func (suite *tieredStorageTestSuite) TestCreateFileRejectsExistingCloudObject() {
	defer suite.cleanupTest()

	const path = "existing-cloud"
	handle, err := suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0644})
	suite.Require().NoError(err)
	suite.Require().NoError(suite.loopback.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))

	_, err = suite.tieredStorage.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0644})
	suite.assert.ErrorIs(err, syscall.EEXIST)
}

func (suite *tieredStorageTestSuite) TestCreateFileRejectsExistingLocalObject() {
	defer suite.cleanupTest()

	const path = "existing-local"
	_, err := suite.tieredStorage.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0644})
	suite.Require().NoError(err)

	_, err = suite.tieredStorage.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0644})
	suite.assert.ErrorIs(err, syscall.EEXIST)
}

func (suite *tieredStorageTestSuite) TestOpenFileExclusiveCreateRejectsExistingCloudObject() {
	defer suite.cleanupTest()

	const path = "exclusive-cloud"
	handle, err := suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0644})
	suite.Require().NoError(err)
	suite.Require().NoError(suite.loopback.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))

	_, err = suite.tieredStorage.OpenFile(internal.OpenFileOptions{
		Name: path, Flags: os.O_CREATE | os.O_EXCL | os.O_RDWR, Mode: 0644,
	})
	suite.assert.ErrorIs(err, syscall.EEXIST)
}

func (suite *tieredStorageTestSuite) TestOpenFileTruncatesCloudObjectWithoutDownloading() {
	defer suite.cleanupTest()

	const path = "truncate-cloud"
	handle, err := suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0644})
	suite.Require().NoError(err)
	_, err = suite.loopback.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Data: []byte("old cloud data")},
	)
	suite.Require().NoError(err)
	suite.Require().NoError(suite.loopback.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))

	handle, err = suite.tieredStorage.OpenFile(internal.OpenFileOptions{
		Name: path, Flags: os.O_WRONLY | os.O_TRUNC, Mode: 0644,
	})
	suite.Require().NoError(err)
	suite.assert.True(handle.Dirty())
	suite.assert.False(suite.tieredStorage.onCacheCopy(handle))
	info, err := os.Stat(suite.localFile(path))
	suite.Require().NoError(err)
	suite.assert.Zero(info.Size())
	suite.assert.NoFileExists(suite.cacheFile(path))
	stale := suite.isCloudStale(path)
	suite.assert.True(stale)
	suite.Require().NoError(
		suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
	)
}

func (suite *tieredStorageTestSuite) TestOpenFileSharesCacheCopy() {
	defer suite.cleanupTest()

	const path = "shared-cache"
	suite.writeCloudFile(path, "cloud data")

	first, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path, Flags: os.O_RDONLY},
	)
	suite.Require().NoError(err)
	// Make a second download detectable.
	suite.writeCloudFile(path, "changed in cloud")

	second, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path, Flags: os.O_RDONLY},
	)
	suite.Require().NoError(err)
	suite.assert.True(suite.tieredStorage.onCacheCopy(second))
	buffer := make([]byte, len("cloud data"))
	_, err = suite.tieredStorage.ReadInBuffer(
		&internal.ReadInBufferOptions{Handle: second, Data: buffer},
	)
	suite.Require().NoError(err)
	suite.assert.Equal("cloud data", string(buffer))
	suite.assert.EqualValues(2, suite.tieredStorage.fileLocks.Get(path).Count())

	for _, handle := range []*handlemap.Handle{first, second} {
		suite.Require().NoError(
			suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
		)
	}
}

func (suite *tieredStorageTestSuite) TestOpenFileHonorsReadOnlyFlag() {
	defer suite.cleanupTest()

	const path = "read-only"
	data := []byte("cloud data")
	handle, err := suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0644})
	suite.Require().NoError(err)
	_, err = suite.loopback.WriteFile(&internal.WriteFileOptions{Handle: handle, Data: data})
	suite.Require().NoError(err)
	suite.Require().NoError(suite.loopback.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))

	handle, err = suite.tieredStorage.OpenFile(internal.OpenFileOptions{
		Name: path, Flags: os.O_RDONLY,
	})
	suite.Require().NoError(err)
	buffer := make([]byte, len(data))
	_, err = suite.tieredStorage.ReadInBuffer(
		&internal.ReadInBufferOptions{Handle: handle, Data: buffer},
	)
	suite.Require().NoError(err)
	suite.assert.Equal(data, buffer)
	suite.Require().NoError(
		suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
	)
}

func (suite *tieredStorageTestSuite) TestTruncateOpenLocalFile() {
	defer suite.cleanupTest()

	handle, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: "local-truncate", Mode: 0644},
	)
	suite.Require().NoError(err)
	_, err = suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Data: []byte("original")},
	)
	suite.Require().NoError(err)
	used := suite.tieredStorage.cacheSize.Used()

	err = suite.tieredStorage.TruncateFile(internal.TruncateFileOptions{
		Name: "local-truncate", Handle: handle, NewSize: 3,
	})
	suite.Require().NoError(err)
	info, err := handle.GetFileObject().Stat()
	suite.Require().NoError(err)
	suite.assert.EqualValues(3, info.Size())
	suite.assert.True(handle.Dirty())
	suite.assert.Equal(used-int64(len("original")-3), suite.tieredStorage.cacheSize.Used())
}

func (suite *tieredStorageTestSuite) TestTruncateCloudFileByPath() {
	defer suite.cleanupTest()

	const path = "cloud-truncate"
	handle, err := suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0644})
	suite.Require().NoError(err)
	_, err = suite.loopback.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Data: []byte("cloud data")},
	)
	suite.Require().NoError(err)
	suite.Require().NoError(suite.loopback.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))

	suite.Require().NoError(suite.tieredStorage.TruncateFile(
		internal.TruncateFileOptions{Name: path, NewSize: 4},
	))
	suite.waitEvictOnClose()
	attrs, err := suite.loopback.GetAttr(internal.GetAttrOptions{Name: path})
	suite.Require().NoError(err)
	suite.assert.EqualValues(4, attrs.Size)
	suite.assert.NoFileExists(suite.localFile(path))
}

func (suite *tieredStorageTestSuite) TestHandlesNeverSetLibfuseCachedFlag() {
	defer suite.cleanupTest()

	// HandleFlagCached makes libfuse read the file object directly; tiered
	// storage reads go through ReadInBuffer whatever tree the file is in.
	suite.writeCloudFile("cloud", "data")
	cloud, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: "cloud", Flags: os.O_RDWR},
	)
	suite.Require().NoError(err)
	local, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: "local", Mode: 0644},
	)
	suite.Require().NoError(err)
	suite.assert.True(suite.tieredStorage.onCacheCopy(cloud))
	suite.assert.False(suite.tieredStorage.onCacheCopy(local))
	for _, handle := range []*handlemap.Handle{cloud, local} {
		suite.assert.False(handle.Cached())
	}

	for _, handle := range []*handlemap.Handle{cloud, local} {
		suite.Require().NoError(
			suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
		)
	}
	suite.assert.False(suite.tieredStorage.onCacheCopy(cloud))
}

func (suite *tieredStorageTestSuite) TestWritePromotesCachedFile() {
	defer suite.cleanupTest()

	const path = "dir/promote"
	suite.writeCloudFile(path, "cloud data")
	openRW := internal.OpenFileOptions{Name: path, Flags: os.O_RDWR}
	writer, err := suite.tieredStorage.OpenFile(openRW)
	suite.Require().NoError(err)
	reader, err := suite.tieredStorage.OpenFile(openRW)
	suite.Require().NoError(err)

	_, err = suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: writer, Data: []byte("LOCAL")},
	)
	suite.Require().NoError(err)

	suite.assert.NoFileExists(suite.cacheFile(path))
	suite.assert.FileExists(suite.localFile(path))
	suite.assert.False(suite.tieredStorage.onCacheCopy(writer))
	stale := suite.isCloudStale(path)
	suite.assert.True(stale)
	suite.assert.Contains(lruOrder(suite.tieredStorage.policy), path)

	// The other handle still works, and sees the promoted file.
	buffer := make([]byte, len("LOCAL data"))
	_, err = suite.tieredStorage.ReadInBuffer(
		&internal.ReadInBufferOptions{Handle: reader, Data: buffer},
	)
	suite.Require().NoError(err)
	suite.assert.Equal("LOCAL data", string(buffer))
	_, err = suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: reader, Offset: 5, Data: []byte("!")},
	)
	suite.Require().NoError(err)
	suite.assert.False(suite.tieredStorage.onCacheCopy(reader))

	data, err := os.ReadFile(suite.localFile(path))
	suite.Require().NoError(err)
	suite.assert.Equal("LOCAL!data", string(data))
	for _, handle := range []*handlemap.Handle{writer, reader} {
		suite.Require().NoError(
			suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
		)
	}
}

func (suite *tieredStorageTestSuite) TestPromoteNeverOverwritesLocalFile() {
	defer suite.cleanupTest()

	const path = "both-trees"
	suite.writeCloudFile(path, "old cloud")
	handle, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path, Flags: os.O_RDWR},
	)
	suite.Require().NoError(err)
	// Simulate a failed cleanup that left a stray cache/ copy beside local/x.
	suite.Require().NoError(os.WriteFile(suite.localFile(path), []byte("authoritative"), 0644))

	_, err = suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Data: []byte("X")},
	)
	suite.Require().NoError(err)

	suite.assert.NoFileExists(suite.cacheFile(path))
	data, err := os.ReadFile(suite.localFile(path))
	suite.Require().NoError(err)
	suite.assert.Equal("authoritative", string(data))
	suite.Require().NoError(
		suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
	)
	suite.assert.FileExists(suite.localFile(path))
	suite.assert.Contains(lruOrder(suite.tieredStorage.policy), path)
}

func (suite *tieredStorageTestSuite) TestTruncatePromotesCachedFile() {
	defer suite.cleanupTest()

	const path = "truncate-promote"
	suite.writeCloudFile(path, "cloud data")
	handle, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path, Flags: os.O_RDWR},
	)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.tieredStorage.TruncateFile(
		internal.TruncateFileOptions{Name: path, Handle: handle, NewSize: 5},
	))

	suite.assert.NoFileExists(suite.cacheFile(path))
	data, err := os.ReadFile(suite.localFile(path))
	suite.Require().NoError(err)
	suite.assert.Equal("cloud", string(data))
	stale := suite.isCloudStale(path)
	suite.assert.True(stale)
	suite.Require().NoError(
		suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
	)
}

func (suite *tieredStorageTestSuite) TestOpenTruncPromotesOpenCachedFile() {
	defer suite.cleanupTest()

	const path = "trunc-open-cached"
	suite.writeCloudFile(path, "cloud data")
	reader, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path, Flags: os.O_RDONLY},
	)
	suite.Require().NoError(err)

	writer, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path, Flags: os.O_WRONLY | os.O_TRUNC},
	)
	suite.Require().NoError(err)

	suite.assert.False(suite.tieredStorage.onCacheCopy(writer))
	suite.assert.NoFileExists(suite.cacheFile(path))
	info, err := os.Stat(suite.localFile(path))
	suite.Require().NoError(err)
	suite.assert.Zero(info.Size())
	stale := suite.isCloudStale(path)
	suite.assert.True(stale)
	for _, handle := range []*handlemap.Handle{reader, writer} {
		suite.Require().NoError(
			suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
		)
	}
}

func (suite *tieredStorageTestSuite) TestOpenFileExclusiveChangesNothing() {
	defer suite.cleanupTest()

	suite.writeCloudFile("cached", "cloud data")
	reader, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: "cached", Flags: os.O_RDONLY},
	)
	suite.Require().NoError(err)
	local, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: "local", Mode: 0644},
	)
	suite.Require().NoError(err)
	_, err = suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: local, Data: []byte("local")},
	)
	suite.Require().NoError(err)
	used := suite.tieredStorage.cacheSize.Used()

	for _, name := range []string{"cached", "local"} {
		_, err = suite.tieredStorage.OpenFile(internal.OpenFileOptions{
			Name: name, Flags: os.O_CREATE | os.O_EXCL | os.O_TRUNC | os.O_RDWR, Mode: 0644,
		})
		suite.assert.ErrorIs(err, syscall.EEXIST)
	}
	suite.assert.FileExists(suite.cacheFile("cached"))
	stale := suite.isCloudStale("cached")
	suite.assert.False(stale)
	suite.assert.Equal(used, suite.tieredStorage.cacheSize.Used())
	for _, handle := range []*handlemap.Handle{reader, local} {
		suite.Require().NoError(
			suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
		)
	}
}

func (suite *tieredStorageTestSuite) TestRenameFileOverwriteUpdatesUsage() {
	defer suite.cleanupTest()

	for name, data := range map[string]string{"src": "s", "dst": "replaced data"} {
		handle, err := suite.tieredStorage.CreateFile(
			internal.CreateFileOptions{Name: name, Mode: 0644},
		)
		suite.Require().NoError(err)
		_, err = suite.tieredStorage.WriteFile(
			&internal.WriteFileOptions{Handle: handle, Data: []byte(data)},
		)
		suite.Require().NoError(err)
		suite.Require().NoError(
			suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
		)
	}
	used := suite.tieredStorage.cacheSize.Used()

	suite.Require().NoError(
		suite.tieredStorage.RenameFile(internal.RenameFileOptions{Src: "src", Dst: "dst"}),
	)
	suite.assert.Equal(used-int64(len("replaced data")), suite.tieredStorage.cacheSize.Used())
}

func (suite *tieredStorageTestSuite) TestDownloadToCache() {
	defer suite.cleanupTest()

	suite.writeCloudFile("dir/cloud", "cloud data")
	suite.Require().NoError(suite.tieredStorage.downloadToCache("dir/cloud", 0640))
	data, err := os.ReadFile(suite.cacheFile("dir/cloud"))
	suite.Require().NoError(err)
	suite.assert.Equal("cloud data", string(data))

	err = suite.tieredStorage.downloadToCache("missing", 0640)
	suite.assert.Error(err)
	suite.assert.NoFileExists(suite.cacheFile("missing"))
}

func (suite *tieredStorageTestSuite) TestWriteIsNotLimitedByMaxSize() {
	defer suite.cleanupTest()

	handle, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: "big", Mode: 0644},
	)
	suite.Require().NoError(err)
	data := make([]byte, 2*int(suite.tieredStorage.maxCacheSize))
	n, err := suite.tieredStorage.WriteFile(&internal.WriteFileOptions{Handle: handle, Data: data})
	suite.Require().NoError(err)
	suite.assert.Equal(len(data), n)
	suite.assert.Greater(
		float64(suite.tieredStorage.cacheSize.Used()),
		suite.tieredStorage.maxCacheSize,
	)

	suite.Require().NoError(suite.tieredStorage.TruncateFile(internal.TruncateFileOptions{
		Name: "big", Handle: handle, NewSize: int64(3 * suite.tieredStorage.maxCacheSize),
	}))
	suite.Require().NoError(
		suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
	)
}

func (suite *tieredStorageTestSuite) TestTruncateInvalidSize() {
	defer suite.cleanupTest()

	err := suite.tieredStorage.TruncateFile(
		internal.TruncateFileOptions{Name: "invalid", NewSize: -1},
	)
	suite.assert.ErrorIs(err, syscall.EINVAL)
}

func (suite *tieredStorageTestSuite) TestStatFs() {
	defer suite.cleanupTest()

	handle, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: "usage", Mode: 0644},
	)
	suite.Require().NoError(err)
	_, err = suite.tieredStorage.WriteFile(&internal.WriteFileOptions{
		Handle: handle,
		Data:   make([]byte, 4096),
	})
	suite.Require().NoError(err)

	stat, populated, err := suite.tieredStorage.StatFs()
	suite.Require().NoError(err)
	suite.assert.True(populated)
	suite.assert.EqualValues(4096, stat.Bsize)
	suite.assert.Equal(uint64(suite.tieredStorage.maxCacheSize)/4096, stat.Blocks)
	expectedAvailable := max(
		int64(suite.tieredStorage.maxCacheSize)-suite.tieredStorage.cacheSize.Used(),
		0,
	)
	suite.assert.Equal(uint64(expectedAvailable)/4096, stat.Bavail)
	suite.assert.Positive(stat.Bfree)
}

func (suite *tieredStorageTestSuite) TestSymlink() {
	defer suite.cleanupTest()

	suite.Require().NoError(suite.tieredStorage.CreateLink(
		internal.CreateLinkOptions{Name: "link", Target: "target"},
	))
	target, err := suite.tieredStorage.ReadLink(
		internal.ReadLinkOptions{Name: "link"},
	)
	suite.Require().NoError(err)
	suite.assert.Equal("target", target)
	suite.assert.NoFileExists(suite.localFile("link"))
}

func (suite *tieredStorageTestSuite) TestChmodLocalFilePersistsOnUpload() {
	defer suite.cleanupTest()
	if runtime.GOOS == "windows" {
		suite.T().Skip("Windows only supports the read-only mode bit")
	}

	const path = "local-chmod"
	handle, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: path, Mode: 0644},
	)
	suite.Require().NoError(err)
	suite.Require().NoError(suite.tieredStorage.Chmod(
		internal.ChmodOptions{Name: path, Mode: 0600},
	))
	suite.Require().NoError(
		suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
	)

	flock := suite.tieredStorage.fileLocks.Get(path)
	flock.Lock()
	suite.Require().NoError(suite.tieredStorage.uploadandCleanFile(path))
	flock.Unlock()
	info, err := os.Stat(filepath.Join(suite.fake_storage_path, path))
	suite.Require().NoError(err)
	suite.assert.Equal(os.FileMode(0600), info.Mode().Perm())
}

func (suite *tieredStorageTestSuite) TestChmodCloudBackedFileUpdatesBothTiers() {
	defer suite.cleanupTest()
	if runtime.GOOS == "windows" {
		suite.T().Skip("Windows only supports the read-only mode bit")
	}

	const path = "cloud-chmod"
	handle, err := suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0644})
	suite.Require().NoError(err)
	suite.Require().NoError(suite.loopback.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))
	handle, err = suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path, Flags: os.O_RDWR, Mode: 0644},
	)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.tieredStorage.Chmod(
		internal.ChmodOptions{Name: path, Mode: 0600},
	))
	localInfo, err := os.Stat(suite.cacheFile(path))
	suite.Require().NoError(err)
	cloudInfo, err := os.Stat(filepath.Join(suite.fake_storage_path, path))
	suite.Require().NoError(err)
	suite.assert.Equal(os.FileMode(0600), localInfo.Mode().Perm())
	suite.assert.Equal(os.FileMode(0600), cloudInfo.Mode().Perm())
	suite.Require().NoError(
		suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
	)
}

func (suite *tieredStorageTestSuite) TestChmodLocalFileSkipsCloud() {
	defer suite.cleanupTest()
	if runtime.GOOS == "windows" {
		suite.T().Skip("Windows only supports the read-only mode bit")
	}

	// A stale cloud copy must not be touched; the local file is authoritative.
	const path = "local-over-stale"
	suite.writeCloudFile(path, "stale")
	cloudPath := filepath.Join(suite.fake_storage_path, path)
	suite.Require().NoError(os.Chmod(cloudPath, 0644))
	suite.Require().NoError(os.WriteFile(suite.localFile(path), []byte("local"), 0644))

	suite.Require().NoError(suite.tieredStorage.Chmod(
		internal.ChmodOptions{Name: path, Mode: 0600},
	))
	localInfo, err := os.Stat(suite.localFile(path))
	suite.Require().NoError(err)
	suite.assert.Equal(os.FileMode(0600), localInfo.Mode().Perm())
	cloudInfo, err := os.Stat(cloudPath)
	suite.Require().NoError(err)
	suite.assert.Equal(os.FileMode(0644), cloudInfo.Mode().Perm())
}

func (suite *tieredStorageTestSuite) TestChmodCloudOnlyFile() {
	defer suite.cleanupTest()
	if runtime.GOOS == "windows" {
		suite.T().Skip("Windows only supports the read-only mode bit")
	}

	const path = "cloud-only-chmod"
	handle, err := suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0644})
	suite.Require().NoError(err)
	suite.Require().NoError(suite.loopback.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))

	suite.Require().NoError(suite.tieredStorage.Chmod(
		internal.ChmodOptions{Name: path, Mode: 0600},
	))
	info, err := os.Stat(filepath.Join(suite.fake_storage_path, path))
	suite.Require().NoError(err)
	suite.assert.Equal(os.FileMode(0600), info.Mode().Perm())
}

func (suite *tieredStorageTestSuite) TestStreamDirMergesLocalAndCloudEntries() {
	defer suite.cleanupTest()

	localHandle, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: "local", Mode: 0644},
	)
	suite.Require().NoError(err)
	_, err = suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: localHandle, Data: []byte("local data")},
	)
	suite.Require().NoError(err)

	for _, path := range []string{"cloud", "shared"} {
		handle, err := suite.loopback.CreateFile(
			internal.CreateFileOptions{Name: path, Mode: 0644},
		)
		suite.Require().NoError(err)
		_, err = suite.loopback.WriteFile(
			&internal.WriteFileOptions{Handle: handle, Data: []byte("cloud")},
		)
		suite.Require().NoError(err)
		suite.Require().NoError(
			suite.loopback.ReleaseFile(
				internal.ReleaseFileOptions{Handle: handle},
			),
		)
	}
	sharedHandle, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: "shared", Flags: os.O_RDWR, Mode: 0644},
	)
	suite.Require().NoError(err)
	_, err = suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: sharedHandle, Data: []byte("local data")},
	)
	suite.Require().NoError(err)

	attrs := listDir(suite.T(), suite.tieredStorage, "")
	suite.Require().Len(attrs, 3)
	suite.assert.ElementsMatch([]string{"cloud", "local", "shared"}, []string{
		attrs[0].Name, attrs[1].Name, attrs[2].Name,
	})
	sizes := map[string]int64{}
	for _, attr := range attrs {
		sizes[attr.Name] = attr.Size
	}
	suite.assert.EqualValues(len("local data"), sizes["local"])
	suite.assert.EqualValues(len("local data"), sizes["shared"])
}

func (suite *tieredStorageTestSuite) TestCreateAndDeleteLocalDirectory() {
	defer suite.cleanupTest()

	const path = "directory"
	suite.Require().NoError(
		suite.tieredStorage.CreateDir(internal.CreateDirOptions{Name: path, Mode: 0755}),
	)
	attrs, err := suite.tieredStorage.GetAttr(internal.GetAttrOptions{Name: path})
	suite.Require().NoError(err)
	suite.assert.True(attrs.IsDir())
	suite.assert.True(suite.tieredStorage.IsDirEmpty(internal.IsDirEmptyOptions{Name: path}))
	suite.Require().NoError(suite.tieredStorage.DeleteDir(internal.DeleteDirOptions{Name: path}))
	_, err = suite.tieredStorage.GetAttr(internal.GetAttrOptions{Name: path})
	suite.assert.ErrorIs(err, syscall.ENOENT)
}

func (suite *tieredStorageTestSuite) TestCreateDirUnderCloudOnlyParent() {
	defer suite.cleanupTest()

	suite.Require().
		NoError(suite.loopback.CreateDir(internal.CreateDirOptions{Name: "parent", Mode: 0755}))
	suite.Require().NoError(
		suite.tieredStorage.CreateDir(internal.CreateDirOptions{Name: "parent/child", Mode: 0755}),
	)
	suite.assert.DirExists(suite.localFile("parent/child"))
	suite.assert.DirExists(filepath.Join(suite.fake_storage_path, "parent", "child"))
}

func (suite *tieredStorageTestSuite) TestStreamDirSkipsCacheAndPrefersLocal() {
	defer suite.cleanupTest()

	suite.writeCloudFile("cached", "cloud data")
	suite.writeCloudFile("promoted", "cloud data")
	cached, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: "cached", Flags: os.O_RDONLY},
	)
	suite.Require().NoError(err)
	promoted, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: "promoted", Flags: os.O_RDWR},
	)
	suite.Require().NoError(err)
	_, err = suite.tieredStorage.WriteFile(&internal.WriteFileOptions{
		Handle: promoted, Offset: 10, Data: []byte(" and more"),
	})
	suite.Require().NoError(err)

	attrs := listDir(suite.T(), suite.tieredStorage, "")
	sizes := map[string]int64{}
	for _, attr := range attrs {
		sizes[attr.Name] = attr.Size
	}
	suite.assert.Equal(map[string]int64{
		"cached":   int64(len("cloud data")),
		"promoted": int64(len("cloud data and more")),
	}, sizes)
	for _, handle := range []*handlemap.Handle{cached, promoted} {
		suite.Require().NoError(
			suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
		)
	}
}

func TestDeleteDirOfflineChangesNothing(t *testing.T) {
	ctrl := gomock.NewController(t)
	next := internal.NewMockComponent(ctrl)
	root := t.TempDir()
	localPath := filepath.Join(root, localDirName, "dir")
	require.NoError(t, os.MkdirAll(localPath, 0700))

	storage := &TieredStorage{tmpPath: root}
	storage.SetNextComponent(next)
	next.EXPECT().CloudConnected().Return(false)
	next.EXPECT().GetAttr(gomock.Any()).Return(&internal.ObjAttr{}, nil)

	err := storage.DeleteDir(internal.DeleteDirOptions{Name: "dir"})
	assert.ErrorIs(t, err, &common.CloudUnreachableError{})
	assert.DirExists(t, localPath)
}

func (suite *tieredStorageTestSuite) TestStreamDirListsImplicitLocalDirectory() {
	defer suite.cleanupTest()

	handle, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: "directory/file", Mode: 0644},
	)
	suite.Require().NoError(err)
	suite.Require().NoError(
		suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
	)
	suite.Require().NoError(
		suite.tieredStorage.OpenDir(internal.OpenDirOptions{Name: "directory"}),
	)

	attrs := listDir(suite.T(), suite.tieredStorage, "directory")
	suite.Require().Len(attrs, 1)
	suite.assert.Equal("directory/file", attrs[0].Path)
	suite.Require().NoError(
		suite.tieredStorage.CloseDir(internal.CloseDirOptions{Name: "directory"}),
	)
}

// listDir pages through StreamDir as libfuse does.
func listDir(t *testing.T, storage *TieredStorage, name string) []*internal.ObjAttr {
	var attrs []*internal.ObjAttr
	token := ""
	for {
		page, nextToken, err := storage.StreamDir(
			internal.StreamDirOptions{Name: name, Token: token},
		)
		require.NoError(t, err)
		// libfuse takes an empty page as the end of the listing.
		require.True(t, len(page) > 0 || nextToken == "", "empty page before the end")
		attrs = append(attrs, page...)
		if nextToken == "" {
			return attrs
		}
		token = nextToken
	}
}

func TestStreamDirListsLocalEntriesOnceAcrossPages(t *testing.T) {
	ctrl := gomock.NewController(t)
	next := internal.NewMockComponent(ctrl)
	root := t.TempDir()
	localDir := filepath.Join(root, localDirName)
	require.NoError(t, os.MkdirAll(localDir, 0700))
	for _, name := range []string{"a", "c"} {
		require.NoError(t, os.WriteFile(filepath.Join(localDir, name), []byte("local"), 0600))
	}

	storage := &TieredStorage{tmpPath: root}
	storage.SetNextComponent(next)
	// Every name on the second cloud page is local, so that page must be
	// skipped rather than returned empty. attr_cache returns its cached
	// listing, which must not be modified.
	secondPage := []*internal.ObjAttr{{Path: "a", Name: "a"}, {Path: "c", Name: "c"}}
	secondPageCopy := slices.Clone(secondPage)
	next.EXPECT().StreamDir(internal.StreamDirOptions{}).
		Return([]*internal.ObjAttr{{Path: "b", Name: "b"}}, "page2", nil)
	next.EXPECT().StreamDir(internal.StreamDirOptions{Token: "page2"}).
		Return(secondPage, "page3", nil)
	next.EXPECT().StreamDir(internal.StreamDirOptions{Token: "page3"}).
		Return([]*internal.ObjAttr{{Path: "d", Name: "d"}}, "", nil)

	sizes := map[string]int64{}
	var names []string
	for _, attr := range listDir(t, storage, "") {
		names = append(names, attr.Name)
		sizes[attr.Name] = attr.Size
	}
	assert.ElementsMatch(t, []string{"a", "b", "c", "d"}, names)
	assert.EqualValues(t, len("local"), sizes["a"])
	assert.EqualValues(t, len("local"), sizes["c"])
	assert.Equal(t, secondPageCopy, secondPage)
}

func TestStreamDirWithoutLocalEntriesStartsWithCloud(t *testing.T) {
	ctrl := gomock.NewController(t)
	next := internal.NewMockComponent(ctrl)
	storage := &TieredStorage{tmpPath: t.TempDir()}
	storage.SetNextComponent(next)
	next.EXPECT().StreamDir(internal.StreamDirOptions{Name: "dir"}).
		Return([]*internal.ObjAttr{{Path: "dir/a", Name: "a"}}, "", nil)

	attrs, token, err := storage.StreamDir(internal.StreamDirOptions{Name: "dir"})
	require.NoError(t, err)
	assert.Empty(t, token)
	require.Len(t, attrs, 1)
	assert.Equal(t, "dir/a", attrs[0].Path)
}

func (suite *tieredStorageTestSuite) TestRenameLocalOnlyDirectory() {
	defer suite.cleanupTest()

	handle, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: "src/file", Mode: 0644},
	)
	suite.Require().NoError(err)
	suite.Require().NoError(
		suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
	)

	suite.Require().NoError(suite.tieredStorage.RenameDir(
		internal.RenameDirOptions{Src: "src", Dst: "dst"},
	))
	suite.assert.NoFileExists(suite.localFile("src/file"))
	suite.assert.FileExists(suite.localFile("dst/file"))
	_, oldQueued := suite.tieredStorage.policy.nodeMap.Load("src/file")
	_, newQueued := suite.tieredStorage.policy.nodeMap.Load("dst/file")
	suite.assert.False(oldQueued)
	suite.assert.True(newQueued)
}

func (suite *tieredStorageTestSuite) TestRenameMixedDirectory() {
	defer suite.cleanupTest()

	suite.Require().NoError(suite.loopback.CreateDir(
		internal.CreateDirOptions{Name: "src", Mode: 0755},
	))
	cloudHandle, err := suite.loopback.CreateFile(
		internal.CreateFileOptions{Name: "src/cloud", Mode: 0644},
	)
	suite.Require().NoError(err)
	suite.Require().NoError(
		suite.loopback.ReleaseFile(internal.ReleaseFileOptions{Handle: cloudHandle}),
	)
	localHandle, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: "src/local", Mode: 0644},
	)
	suite.Require().NoError(err)
	suite.Require().NoError(
		suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: localHandle}),
	)

	suite.Require().NoError(suite.tieredStorage.RenameDir(
		internal.RenameDirOptions{Src: "src", Dst: "dst"},
	))
	suite.assert.FileExists(suite.localFile("dst/local"))
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, "dst", "cloud"))
	suite.assert.NoDirExists(suite.localFile("src"))
	suite.assert.NoDirExists(filepath.Join(suite.fake_storage_path, "src"))
}

func (suite *tieredStorageTestSuite) TestRenameDirectoryUpdatesOpenHandle() {
	defer suite.cleanupTest()

	handle, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: "src/open", Mode: 0644},
	)
	suite.Require().NoError(err)
	handlemap.Add(handle)
	err = suite.tieredStorage.RenameDir(internal.RenameDirOptions{Src: "src", Dst: "dst"})
	if runtime.GOOS == "windows" {
		// Windows refuses to rename a directory containing open files.
		suite.Require().ErrorIs(err, syscall.EBUSY)
		suite.assert.Equal("src/open", handle.Path)
		suite.assert.FileExists(suite.localFile("src/open"))
		suite.Require().NoError(
			suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
		)
		return
	}
	suite.Require().NoError(err)
	suite.assert.Equal("dst/open", handle.Path)
	suite.Require().NoError(
		suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
	)
	_, queued := suite.tieredStorage.policy.nodeMap.Load("dst/open")
	suite.assert.True(queued)
}

func (suite *tieredStorageTestSuite) TestRenameDirMovesCacheCopiesAndState() {
	defer suite.cleanupTest()
	if runtime.GOOS == "windows" {
		suite.T().Skip("Windows refuses to rename a directory containing open files")
	}

	suite.writeCloudFile("src/cached", "cloud")
	suite.writeCloudFile("src/sub/promoted", "cloud")
	cached, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: "src/cached", Flags: os.O_RDONLY},
	)
	suite.Require().NoError(err)
	handlemap.Add(cached)
	promoted, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: "src/sub/promoted", Flags: os.O_RDWR},
	)
	suite.Require().NoError(err)
	handlemap.Add(promoted)
	_, err = suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: promoted, Data: []byte("L")},
	)
	suite.Require().NoError(err)

	suite.Require().
		NoError(suite.tieredStorage.RenameDir(internal.RenameDirOptions{Src: "src", Dst: "dst"}))

	suite.assert.FileExists(suite.cacheFile("dst/cached"))
	suite.assert.FileExists(suite.localFile("dst/sub/promoted"))
	suite.assert.NoDirExists(suite.cacheFile("src"))
	suite.assert.NoDirExists(suite.localFile("src"))
	suite.assert.Equal("dst/cached", cached.Path)
	suite.assert.Equal("dst/sub/promoted", promoted.Path)
	stale := suite.isCloudStale("dst/sub/promoted")
	suite.assert.True(stale)
	suite.assert.Equal([]string{"dst/sub/promoted"}, lruOrder(suite.tieredStorage.policy))
	suite.assert.EqualValues(1, suite.tieredStorage.fileLocks.Get("dst/cached").Count())
	suite.assert.EqualValues(0, suite.tieredStorage.fileLocks.Get("src/cached").Count())

	for _, handle := range []*handlemap.Handle{cached, promoted} {
		suite.Require().NoError(
			suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
		)
	}
	suite.assert.NoFileExists(suite.cacheFile("dst/cached"))
}

func (suite *tieredStorageTestSuite) TestRenameDirRefusesExistingLocalDestination() {
	defer suite.cleanupTest()

	suite.Require().
		NoError(suite.tieredStorage.CreateDir(internal.CreateDirOptions{Name: "src", Mode: 0755}))
	suite.Require().
		NoError(suite.tieredStorage.CreateDir(internal.CreateDirOptions{Name: "dst", Mode: 0755}))

	err := suite.tieredStorage.RenameDir(internal.RenameDirOptions{Src: "src", Dst: "dst"})
	suite.assert.ErrorIs(err, syscall.EEXIST)
	suite.assert.DirExists(suite.localFile("src"))
}

func TestRenameDirOfflineChangesNothing(t *testing.T) {
	ctrl := gomock.NewController(t)
	next := internal.NewMockComponent(ctrl)
	root := t.TempDir()
	localPath := filepath.Join(root, localDirName, "src", "file")
	require.NoError(t, os.MkdirAll(filepath.Dir(localPath), 0700))
	require.NoError(t, os.WriteFile(localPath, []byte("data"), 0644))

	storage := &TieredStorage{tmpPath: root, fileLocks: common.NewLockMap()}
	storage.SetNextComponent(next)
	next.EXPECT().CloudConnected().Return(false)
	next.EXPECT().GetAttr(gomock.Any()).Return(&internal.ObjAttr{}, nil)

	err := storage.RenameDir(internal.RenameDirOptions{Src: "src", Dst: "dst"})
	assert.ErrorIs(t, err, &common.CloudUnreachableError{})
	assert.FileExists(t, localPath)
	assert.NoDirExists(t, filepath.Join(root, localDirName, "dst"))
}

func (suite *tieredStorageTestSuite) writeCloudFile(name, data string) {
	cloudDir := filepath.Join(suite.fake_storage_path, filepath.Dir(filepath.FromSlash(name)))
	suite.Require().NoError(os.MkdirAll(cloudDir, 0755))
	handle, err := suite.loopback.CreateFile(internal.CreateFileOptions{Name: name, Mode: 0644})
	suite.Require().NoError(err)
	_, err = suite.loopback.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Data: []byte(data)},
	)
	suite.Require().NoError(err)
	suite.Require().NoError(suite.loopback.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))
}

func (suite *tieredStorageTestSuite) TestRenameFileCloudFailureRevertsLocalRename() {
	defer suite.cleanupTest()

	suite.writeCloudFile("src", "data")
	handle, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: "src", Flags: os.O_RDWR, Mode: 0644},
	)
	suite.Require().NoError(err)
	// Not added to the handlemap: loopback renames handles even when it fails.
	_, err = suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Data: []byte("D")},
	)
	suite.Require().NoError(err)
	// An existing cloud directory makes the cloud rename fail.
	suite.Require().NoError(os.Mkdir(filepath.Join(suite.fake_storage_path, "dst"), 0755))

	err = suite.tieredStorage.RenameFile(internal.RenameFileOptions{Src: "src", Dst: "dst"})
	suite.Require().Error(err)
	suite.assert.FileExists(suite.localFile("src"))
	suite.assert.NoFileExists(suite.localFile("dst"))
	suite.assert.Equal("src", handle.Path)
	stale := suite.isCloudStale("src")
	suite.assert.True(stale)
	suite.assert.Equal([]string{"src"}, lruOrder(suite.tieredStorage.policy))
	suite.Require().NoError(
		suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
	)
}

func (suite *tieredStorageTestSuite) TestRenameFileMovesStateWithFile() {
	defer suite.cleanupTest()

	suite.writeCloudFile("src", "data")
	handle, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: "src", Flags: os.O_RDWR, Mode: 0644},
	)
	suite.Require().NoError(err)
	handlemap.Add(handle)
	_, err = suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Data: []byte("D")},
	)
	suite.Require().NoError(err)
	// Loopback, unlike object storage, needs the parent to exist.
	suite.Require().NoError(os.Mkdir(filepath.Join(suite.fake_storage_path, "dir"), 0755))

	suite.Require().NoError(
		suite.tieredStorage.RenameFile(internal.RenameFileOptions{Src: "src", Dst: "dir/dst"}),
	)
	suite.assert.FileExists(suite.localFile("dir/dst"))
	suite.assert.NoFileExists(suite.localFile("src"))
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, "dir", "dst"))
	suite.assert.Equal("dir/dst", handle.Path)
	stale := suite.isCloudStale("dir/dst")
	suite.assert.True(stale)
	stale = suite.isCloudStale("src")
	suite.assert.False(stale)
	suite.assert.Equal([]string{"dir/dst"}, lruOrder(suite.tieredStorage.policy))
	suite.Require().NoError(
		suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
	)
}

func (suite *tieredStorageTestSuite) TestRenameOpenCachedFile() {
	defer suite.cleanupTest()

	suite.writeCloudFile("src", "data")
	handle, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: "src", Flags: os.O_RDONLY, Mode: 0644},
	)
	suite.Require().NoError(err)
	handlemap.Add(handle)

	suite.Require().NoError(
		suite.tieredStorage.RenameFile(internal.RenameFileOptions{Src: "src", Dst: "dst"}),
	)
	suite.assert.FileExists(suite.cacheFile("dst"))
	suite.assert.NoFileExists(suite.localFile("dst"))
	suite.assert.FileExists(filepath.Join(suite.fake_storage_path, "dst"))
	suite.Require().NoError(
		suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
	)
	suite.assert.NoFileExists(suite.cacheFile("dst"))
}

func (suite *tieredStorageTestSuite) TestRenameFileNeverUploadedSucceeds() {
	defer suite.cleanupTest()

	handle, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: "src", Mode: 0644},
	)
	suite.Require().NoError(err)
	suite.Require().NoError(
		suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
	)

	suite.Require().NoError(
		suite.tieredStorage.RenameFile(internal.RenameFileOptions{Src: "src", Dst: "dst"}),
	)
	suite.assert.FileExists(suite.localFile("dst"))
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, "dst"))
}

func (suite *tieredStorageTestSuite) TestRenameFileReplacesCopyInOtherTree() {
	defer suite.cleanupTest()

	// dst is local-owned; src is a cloud file that is open in cache/.
	suite.writeCloudFile("src", "cloud src")
	dstHandle, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: "dst", Mode: 0644},
	)
	suite.Require().NoError(err)
	suite.Require().NoError(
		suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: dstHandle}),
	)
	srcHandle, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: "src", Flags: os.O_RDONLY},
	)
	suite.Require().NoError(err)
	handlemap.Add(srcHandle)

	suite.Require().NoError(
		suite.tieredStorage.RenameFile(internal.RenameFileOptions{Src: "src", Dst: "dst"}),
	)
	// The cached copy replaced local/dst in place, so dst is in one tree.
	suite.assert.NoFileExists(suite.cacheFile("dst"))
	data, err := os.ReadFile(suite.localFile("dst"))
	suite.Require().NoError(err)
	suite.assert.Equal("cloud src", string(data))
	attrs, err := suite.tieredStorage.GetAttr(internal.GetAttrOptions{Name: "dst"})
	suite.Require().NoError(err)
	suite.assert.EqualValues(len("cloud src"), attrs.Size)
	suite.Require().NoError(
		suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: srcHandle}),
	)
}

func (suite *tieredStorageTestSuite) TestRenameDirCloudFailureRevertsLocalRename() {
	defer suite.cleanupTest()
	if runtime.GOOS == "windows" {
		suite.T().Skip("Windows refuses to rename a directory containing open files")
	}

	suite.Require().NoError(suite.loopback.CreateDir(
		internal.CreateDirOptions{Name: "src", Mode: 0755},
	))
	suite.writeCloudFile("src/cloud", "data")
	cloudHandle, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: "src/cloud", Flags: os.O_RDONLY, Mode: 0644},
	)
	suite.Require().NoError(err)
	handlemap.Add(cloudHandle)
	localHandle, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: "src/local", Mode: 0644},
	)
	suite.Require().NoError(err)
	suite.Require().NoError(
		suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: localHandle}),
	)
	// A non-empty cloud directory makes the cloud rename fail.
	suite.Require().NoError(os.Mkdir(filepath.Join(suite.fake_storage_path, "dst"), 0755))
	suite.Require().NoError(os.WriteFile(
		filepath.Join(suite.fake_storage_path, "dst", "existing"),
		[]byte("existing"),
		0644,
	))

	err = suite.tieredStorage.RenameDir(internal.RenameDirOptions{Src: "src", Dst: "dst"})
	suite.Require().Error(err)
	suite.assert.DirExists(suite.localFile("src"))
	suite.assert.NoDirExists(suite.localFile("dst"))
	suite.assert.NoFileExists(suite.localFile("dst/local"))
	suite.assert.FileExists(suite.localFile("src/local"))
	suite.assert.Equal("src/cloud", cloudHandle.Path)
	suite.assert.FileExists(suite.cacheFile("src/cloud"))
	suite.assert.NoDirExists(suite.cacheFile("dst"))
	suite.assert.Equal([]string{"src/local"}, lruOrder(suite.tieredStorage.policy))

	suite.Require().NoError(
		suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: cloudHandle}),
	)
	data, err := os.ReadFile(filepath.Join(suite.fake_storage_path, "src", "cloud"))
	suite.Require().NoError(err)
	suite.assert.Equal("data", string(data))
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, "dst", "cloud"))
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
	suite.Require().NoError(err)
	suite.assert.Equal(path, handle.Path)
	suite.assert.True(suite.tieredStorage.onCacheCopy(handle))

	// Cloud files are downloaded into cache/, not local/.
	suite.assert.FileExists(suite.cacheFile(path))
	suite.assert.NoFileExists(suite.localFile(path))
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
	suite.assert.FileExists(suite.localFile(path))
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
	suite.assert.FileExists(suite.localFile(path))

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
	suite.assert.FileExists(suite.localFile(path))

	//Make sure data didn't get modified
	d, err := os.ReadFile(suite.localFile(path))
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
	suite.Require().NoError(err)
	suite.assert.Equal(path, handle.Path)
	suite.assert.True(suite.tieredStorage.onCacheCopy(handle))

	// Cloud files are downloaded into cache/, not local/.
	suite.assert.FileExists(suite.cacheFile(path))
	suite.assert.NoFileExists(suite.localFile(path))
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
	d, _ := os.ReadFile(suite.localFile(path))
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
	suite.assert.FileExists(suite.localFile(path))
	// Path should not be in fake storage
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, path))
}

// Testing Release File
func (suite *tieredStorageTestSuite) TestReleaseDeletesCachedCopy() {
	defer suite.cleanupTest()
	const path = "dir/release-cached"
	suite.writeCloudFile(path, "cloud data")

	openRead := internal.OpenFileOptions{Name: path, Flags: os.O_RDONLY}
	first, err := suite.tieredStorage.OpenFile(openRead)
	suite.Require().NoError(err)
	second, err := suite.tieredStorage.OpenFile(openRead)
	suite.Require().NoError(err)
	used := suite.tieredStorage.cacheSize.Used()

	// The copy stays while any handle is open.
	suite.Require().
		NoError(suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: first}))
	suite.assert.FileExists(suite.cacheFile(path))

	suite.Require().
		NoError(suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: second}))
	suite.assert.NoFileExists(suite.cacheFile(path))
	suite.assert.NoFileExists(suite.localFile(path))
	suite.assert.NotContains(lruOrder(suite.tieredStorage.policy), path)
	suite.assert.Equal(used-int64(len("cloud data")), suite.tieredStorage.cacheSize.Used())
}

// waitEvictOnClose waits for evict-on-close goroutines started so far.
func (suite *tieredStorageTestSuite) waitEvictOnClose() {
	suite.tieredStorage.policy.closeWg.Wait()
}

// promoteCloudFile writes data to the cloud, then opens and modifies it
// through tiered storage, leaving it local-owned and CloudStale.
func (suite *tieredStorageTestSuite) promoteCloudFile(path, data string) *handlemap.Handle {
	suite.writeCloudFile(path, data)
	handle, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path, Flags: os.O_RDWR},
	)
	suite.Require().NoError(err)
	_, err = suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Data: []byte("LOCAL")},
	)
	suite.Require().NoError(err)
	return handle
}

func (suite *tieredStorageTestSuite) TestReleasePromotedFileEvictsIt() {
	defer suite.cleanupTest()
	const path = "release-promoted"
	handle := suite.promoteCloudFile(path, "cloud data")

	suite.Require().
		NoError(suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))
	suite.waitEvictOnClose()

	cloudData, err := os.ReadFile(filepath.Join(suite.fake_storage_path, path))
	suite.Require().NoError(err)
	suite.assert.Equal("LOCAL data", string(cloudData))
	suite.assert.NoFileExists(suite.localFile(path))
	suite.assert.Empty(lruOrder(suite.tieredStorage.policy))
	stale := suite.isCloudStale(path)
	suite.assert.False(stale)
}

func (suite *tieredStorageTestSuite) TestEvictOnCloseSkipsLocallyCreatedFile() {
	defer suite.cleanupTest()
	handle, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: "new", Mode: 0644},
	)
	suite.Require().NoError(err)
	_, err = suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Data: []byte("new")},
	)
	suite.Require().NoError(err)

	suite.Require().
		NoError(suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))
	suite.waitEvictOnClose()

	suite.assert.FileExists(suite.localFile("new"))
	suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, "new"))
}

func (suite *tieredStorageTestSuite) TestEvictOnCloseSkipsReopenedFile() {
	defer suite.cleanupTest()
	const path = "reopened"
	handle := suite.promoteCloudFile(path, "cloud data")

	// Hold eviction back until the file is open again.
	suite.tieredStorage.policy.evictMu.Lock()
	suite.Require().
		NoError(suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))
	handle, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path, Flags: os.O_RDONLY},
	)
	suite.Require().NoError(err)
	suite.tieredStorage.policy.evictMu.Unlock()
	suite.waitEvictOnClose()
	suite.assert.FileExists(suite.localFile(path))

	// The next close evicts it.
	suite.Require().
		NoError(suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))
	suite.waitEvictOnClose()
	suite.assert.NoFileExists(suite.localFile(path))
}

func (suite *tieredStorageTestSuite) TestEvictOnCloseUploadFailureKeepsFile() {
	defer suite.cleanupTest()
	const path = "upload-fails"
	handle := suite.promoteCloudFile(path, "cloud data")
	// A non-empty directory in place of the cloud object makes the upload fail.
	cloudPath := filepath.Join(suite.fake_storage_path, path)
	suite.Require().NoError(os.Remove(cloudPath))
	suite.Require().NoError(os.MkdirAll(filepath.Join(cloudPath, "blocker"), 0755))

	suite.Require().
		NoError(suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))
	suite.waitEvictOnClose()

	suite.assert.FileExists(suite.localFile(path))
	suite.assert.Equal([]string{path}, lruOrder(suite.tieredStorage.policy))
	stale := suite.isCloudStale(path)
	suite.assert.True(stale)
}

func (suite *tieredStorageTestSuite) TestReadInBuffer() {
	defer suite.cleanupTest()
	// Setup
	file := "file14"

	// put file in cloud and write to it
	handle, _ := suite.loopback.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	_, err := suite.loopback.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)
	err = suite.loopback.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Must check that file by its data is actually in the cloud
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

	// Verify it was now downloaded to the local tiered storage cache
	suite.assert.FileExists(suite.cacheFile(path))

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
	_, err = os.Stat(suite.localFile(path))
	suite.assert.False(os.IsNotExist(err), "File should not be uploaded")

	//check still dirty
	suite.assert.False(handle2.Dirty())
	suite.assert.False(handle.Dirty())

	stale := suite.isCloudStale(path)
	suite.assert.True(stale, "File should be marked as cloudStale")

	//4. Release the read should upload to cloud
	err = suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle2})
	suite.assert.NoError(err)
	suite.waitEvictOnClose()
	_, err = os.Stat(suite.localFile(path))
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
	suite.assert.FileExists(suite.localFile(path))

	// 2. Release this local file
	//File is local only so it shouldn't be deleted from local knowledge
	err = suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// 3. Check if its in the LRU Queue
	suite.assert.Equal(path, suite.tieredStorage.policy.head.name)
	suite.assert.Equal(path, suite.tieredStorage.policy.tail.name)

}

func (suite *tieredStorageTestSuite) TestFileUsedPromotesLocalFile() {
	defer suite.cleanupTest()

	for _, path := range []string{"first", "second"} {
		handle, err := suite.tieredStorage.CreateFile(
			internal.CreateFileOptions{Name: path, Mode: 0644},
		)
		suite.Require().NoError(err)
		suite.Require().NoError(
			suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
		)
	}
	suite.assert.Equal("first", suite.tieredStorage.policy.tail.name)
	suite.Require().NoError(suite.tieredStorage.FileUsed("first"))
	suite.assert.Equal("first", suite.tieredStorage.policy.head.name)
}

func (suite *tieredStorageTestSuite) TestReleaseReturnsCloseError() {
	defer suite.cleanupTest()

	handle, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: "close-error", Mode: 0644},
	)
	suite.Require().NoError(err)
	suite.Require().NoError(handle.GetFileObject().Close())
	err = suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.Error(err)
	_, queued := suite.tieredStorage.policy.nodeMap.Load("close-error")
	suite.assert.True(queued)
}

func (suite *tieredStorageTestSuite) TestBlockAPIsUnsupported() {
	defer suite.cleanupTest()

	_, err := suite.tieredStorage.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{})
	suite.assert.ErrorIs(err, syscall.ENOTSUP)
	_, err = suite.tieredStorage.GetCommittedBlockList("file")
	suite.assert.ErrorIs(err, syscall.ENOTSUP)
	suite.assert.ErrorIs(
		suite.tieredStorage.StageData(internal.StageDataOptions{}),
		syscall.ENOTSUP,
	)
	suite.assert.ErrorIs(
		suite.tieredStorage.CommitData(internal.CommitDataOptions{}),
		syscall.ENOTSUP,
	)
}

func (suite *tieredStorageTestSuite) TestReleaseToTriggerEviction() {
	defer suite.cleanupTest()

	// Ok this next test is to essentially go through an iteration of LRU,
	// 1. Initialize many local only file
	//2. Create files that exceed the 80% threshold, max set at 1MB
	data := make([]byte, 250*1024)
	path1 := "file18"
	handle, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path1, Flags: os.O_CREATE | os.O_RDWR, Mode: 0777},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(path1, handle.Path)

	_, err = suite.tieredStorage.WriteFile(&internal.WriteFileOptions{Handle: handle, Data: data})
	suite.assert.NoError(err)
	suite.assert.True(handle.Dirty())

	err = suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	path2 := "file19"
	handle, err = suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path2, Flags: os.O_CREATE | os.O_RDWR, Mode: 0777},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(path2, handle.Path)

	_, err = suite.tieredStorage.WriteFile(&internal.WriteFileOptions{Handle: handle, Data: data})
	suite.assert.NoError(err)
	suite.assert.True(handle.Dirty())

	err = suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	path3 := "file20"
	handle, err = suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path3, Flags: os.O_CREATE | os.O_RDWR, Mode: 0777},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(path3, handle.Path)

	_, err = suite.tieredStorage.WriteFile(&internal.WriteFileOptions{Handle: handle, Data: data})
	suite.assert.NoError(err)
	suite.assert.True(handle.Dirty())

	err = suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	path4 := "file21"
	handle, err = suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: path4, Flags: os.O_CREATE | os.O_RDWR, Mode: 0777},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(path4, handle.Path)

	_, err = suite.tieredStorage.WriteFile(&internal.WriteFileOptions{Handle: handle, Data: data})
	suite.assert.NoError(err)
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

	// 4. Wait for one eviction pass to drop the oldest file.
	suite.assert.Eventually(func() bool {
		_, exists1 := suite.tieredStorage.policy.nodeMap.Load(path1)
		return !exists1
	}, 2*capacityPollInterval, 10*time.Millisecond)

	// 4. Some should then be released to the cloud essentially, the ones we wrote data to
	//And the local files should be gone (uploaded and cleaned up), not in either map

	// 4a. Check state of NodeMap
	_, exists1 = suite.tieredStorage.policy.nodeMap.Load(path1)
	_, exists2 = suite.tieredStorage.policy.nodeMap.Load(path2)
	_, exists3 = suite.tieredStorage.policy.nodeMap.Load(path3)
	_, exists4 = suite.tieredStorage.policy.nodeMap.Load(path4)

	suite.assert.False(exists1)
	suite.assert.True(exists2)
	suite.assert.True(exists3)
	suite.assert.True(exists4)

	// 4c. Only the oldest file should be uploaded and removed locally.
	suite.assert.NoFileExists(suite.localFile(path1))
	suite.assert.FileExists(suite.localFile(path2))

	// 5. The evicted file must now exist in the cloud.
	_, err = suite.tieredStorage.NextComponent().GetAttr(
		internal.GetAttrOptions{Name: path1, RetrieveMetadata: true})
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

}

// ok we gonna do file in local, cloud, file doesn't exist
func (suite *tieredStorageTestSuite) TestDeleteFileCloud() {
	defer suite.cleanupTest()
	// Setup
	file := "file22"

	// put file in cloud and write to it
	handle, err := suite.tieredStorage.CreateFile(
		internal.CreateFileOptions{Name: file, Mode: 0777},
	)
	suite.assert.NoError(err)
	err = suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	err = suite.tieredStorage.DeleteFile(internal.DeleteFileOptions{Name: file})
	suite.assert.NoError(err)

	// Path should not be in file cache
	suite.assert.NoFileExists(suite.localFile(file))

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
	suite.assert.NoFileExists(suite.localFile(file))

}

func (suite *tieredStorageTestSuite) TestDeleteFileOpenCachedAndPromoted() {
	defer suite.cleanupTest()

	suite.writeCloudFile("cached", "cloud")
	cached, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: "cached", Flags: os.O_RDONLY},
	)
	suite.Require().NoError(err)
	suite.writeCloudFile("promoted", "cloud")
	promoted, err := suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{Name: "promoted", Flags: os.O_RDWR},
	)
	suite.Require().NoError(err)
	_, err = suite.tieredStorage.WriteFile(
		&internal.WriteFileOptions{Handle: promoted, Data: []byte("L")},
	)
	suite.Require().NoError(err)

	for _, name := range []string{"cached", "promoted"} {
		suite.Require().
			NoError(suite.tieredStorage.DeleteFile(internal.DeleteFileOptions{Name: name}))
		suite.assert.NoFileExists(suite.cacheFile(name))
		suite.assert.NoFileExists(suite.localFile(name))
		suite.assert.NoFileExists(filepath.Join(suite.fake_storage_path, name))
	}
	suite.assert.NotContains(lruOrder(suite.tieredStorage.policy), "promoted")
	stale := suite.isCloudStale("promoted")
	suite.assert.False(stale)

	// Handles stay usable after delete.
	buffer := make([]byte, 5)
	_, err = suite.tieredStorage.ReadInBuffer(
		&internal.ReadInBufferOptions{Handle: cached, Data: buffer},
	)
	suite.Require().NoError(err)
	suite.assert.Equal("cloud", string(buffer))
	for _, handle := range []*handlemap.Handle{cached, promoted} {
		suite.Require().NoError(
			suite.tieredStorage.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}),
		)
	}
}

func TestDeleteFileOfflineChangesNothing(t *testing.T) {
	ctrl := gomock.NewController(t)
	next := internal.NewMockComponent(ctrl)
	root := t.TempDir()
	localPath := filepath.Join(root, localDirName, "file")
	require.NoError(t, os.MkdirAll(filepath.Dir(localPath), 0700))
	require.NoError(t, os.WriteFile(localPath, []byte("data"), 0644))

	storage := &TieredStorage{tmpPath: root, fileLocks: common.NewLockMap()}
	storage.SetNextComponent(next)
	next.EXPECT().CloudConnected().Return(false)
	next.EXPECT().GetAttr(gomock.Any()).Return(&internal.ObjAttr{}, nil)

	err := storage.DeleteFile(internal.DeleteFileOptions{Name: "file"})
	assert.ErrorIs(t, err, &common.CloudUnreachableError{})
	assert.FileExists(t, localPath)
}

func TestRenameFileOfflineChangesNothing(t *testing.T) {
	ctrl := gomock.NewController(t)
	next := internal.NewMockComponent(ctrl)
	root := t.TempDir()
	localPath := filepath.Join(root, localDirName, "src")
	require.NoError(t, os.MkdirAll(filepath.Dir(localPath), 0700))
	require.NoError(t, os.WriteFile(localPath, []byte("data"), 0644))

	storage := &TieredStorage{tmpPath: root, fileLocks: common.NewLockMap()}
	storage.SetNextComponent(next)
	next.EXPECT().CloudConnected().Return(false)
	next.EXPECT().GetAttr(gomock.Any()).Return(&internal.ObjAttr{}, nil)

	err := storage.RenameFile(internal.RenameFileOptions{Src: "src", Dst: "dst"})
	assert.ErrorIs(t, err, &common.CloudUnreachableError{})
	assert.FileExists(t, localPath)
	assert.NoFileExists(t, filepath.Join(root, localDirName, "dst"))
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
	d, _ := os.ReadFile(suite.localFile(file))
	suite.assert.Equal(data, d)
	//Check that handle is still dirty
	suite.assert.True(handle.Dirty())

}

func TestTieredStorageTestSuite(t *testing.T) {
	suite.Run(t, new(tieredStorageTestSuite))
}
