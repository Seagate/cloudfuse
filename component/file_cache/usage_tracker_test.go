/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2026 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.

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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type usageTrackerTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (s *usageTrackerTestSuite) SetupTest() {
	s.assert = assert.New(s.T())
}

// newTestTracker returns a tracker with a fixed block size, independent of the platform
func newTestTracker(blockSize int64) *usageTracker {
	u := newUsageTracker()
	u.blockSize = blockSize
	return u
}

func (s *usageTrackerTestSuite) TestPlatformBlockSize() {
	want := int64(1)
	if runtime.GOOS == "windows" {
		want = common.SectorSize
	}
	s.Equal(want, newUsageTracker().blockSize)
}

func (s *usageTrackerTestSuite) TestDiskSize() {
	tests := []struct {
		blockSize int64
		size      int64
		want      int64
	}{
		{1, 0, 0},
		{1, 4097, 4097},
		{4096, 0, 0},
		{4096, 1, 4096},
		{4096, 4096, 4096},
		{4096, 4097, 8192},
	}
	for _, tt := range tests {
		s.Run(fmt.Sprintf("block %d size %d", tt.blockSize, tt.size), func() {
			s.Equal(tt.want, newTestTracker(tt.blockSize).diskSize(tt.size))
		})
	}
}

func (s *usageTrackerTestSuite) TestOperationsRoundToBlocks() {
	u := newTestTracker(4096)
	u.set("/c/a", 1)
	s.EqualValues(4096, u.bytes())
	u.grow("/c/a", 4096) // same block
	s.EqualValues(4096, u.bytes())
	u.grow("/c/a", 4097)
	s.EqualValues(8192, u.bytes())
	size, _ := u.size("/c/a")
	s.EqualValues(4097, size, "the apparent size is kept")

	u.set("/c/b", 10)
	u.rename("/c/b", "/c/a") // replaces a
	s.EqualValues(4096, u.bytes())
	u.set("/c/a", 0) // truncate
	s.Zero(u.bytes())
	u.set("/c/a", 5000)
	u.remove("/c/a")
	s.Zero(u.bytes())
}

func (s *usageTrackerTestSuite) TestReconcileRoundsToBlocks() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "x"), make([]byte, 4097), 0o644))
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "y"), make([]byte, 3), 0o644))
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "empty"), nil, 0o644))

	total, err := newTestTracker(4096).reconcile(dir)
	s.Require().NoError(err)
	s.EqualValues(3*4096, total)
}

func (s *usageTrackerTestSuite) TestOperations() {
	u := newTestTracker(1)
	u.set("/c/a", 100)
	u.grow("/c/a", 50) // never shrinks
	u.grow("/c/b", 30)
	s.assert.EqualValues(130, u.bytes())

	u.set("/c/a", 10) // truncate
	s.assert.EqualValues(40, u.bytes())

	u.rename("/c/a", "/c/b") // replaces b
	s.assert.EqualValues(10, u.bytes())
	_, ok := u.size("/c/a")
	s.assert.False(ok)
	size, ok := u.size("/c/b")
	s.assert.True(ok)
	s.assert.EqualValues(10, size)

	u.rename("/c/missing", "/c/x") // untracked source is a no-op
	u.remove("/c/b")
	u.remove("/c/b")
	s.assert.Zero(u.bytes())
}

func (s *usageTrackerTestSuite) TestConcurrentGrow() {
	u := newTestTracker(1)
	var wg sync.WaitGroup
	for w := range 8 {
		wg.Go(func() {
			path := fmt.Sprintf("/c/f%d", w)
			for i := range 1000 {
				u.grow(path, int64(i+1)*10)
			}
		})
	}
	wg.Wait()
	s.assert.EqualValues(8*10000, u.bytes())
}

func (s *usageTrackerTestSuite) TestReconcileMatchesDisk() {
	dir := s.T().TempDir()
	s.Require().NoError(os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755))
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "x"), make([]byte, 4097), 0o644))
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "a", "b", "y"), make([]byte, 3), 0o644))

	u := newTestTracker(1)
	u.set(filepath.Join(dir, "stale"), 1<<30) // drift that reconcile must correct
	total, err := u.reconcile(dir)

	s.Require().NoError(err)
	s.assert.EqualValues(4100, total)
	s.assert.EqualValues(4100, u.bytes())
	_, ok := u.size(filepath.Join(dir, "stale"))
	s.assert.False(ok)
	size, _ := u.size(filepath.Join(dir, "a", "b", "y"))
	s.assert.EqualValues(3, size)
}

func (s *usageTrackerTestSuite) TestRenameSamePath() {
	u := newTestTracker(1)
	u.set("/c/a", 100)

	s.True(u.rename("/c/a", "/c/a"))
	s.EqualValues(100, u.bytes())
	size, ok := u.size("/c/a")
	s.True(ok)
	s.EqualValues(100, size)
}

func (s *usageTrackerTestSuite) TestRenameUntrackedSourceDropsDestination() {
	u := newTestTracker(1)
	u.set("/c/dst", 100)

	// dst was overwritten with contents of unknown size
	s.False(u.rename("/c/src", "/c/dst"))
	s.Zero(u.bytes())
	_, ok := u.size("/c/dst")
	s.False(ok)
}

func (s *usageTrackerTestSuite) TestReconcileMissingRootKeepsTotal() {
	u := newTestTracker(1)
	u.set("/c/a", 100)

	total, err := u.reconcile(filepath.Join(s.T().TempDir(), "missing"))
	s.Error(err)
	s.EqualValues(100, total)
	s.EqualValues(100, u.bytes())
	_, ok := u.size("/c/a")
	s.True(ok)
}

func (s *usageTrackerTestSuite) TestReconcileUnreadableDirKeepsTotal() {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		s.T().Skip("directory permissions are not enforced")
	}
	dir := s.T().TempDir()
	locked := filepath.Join(dir, "locked")
	s.Require().NoError(os.Mkdir(locked, 0o755))
	s.Require().NoError(os.WriteFile(filepath.Join(locked, "f"), make([]byte, 10), 0o644))
	s.Require().NoError(os.Chmod(locked, 0o000))
	defer func() { _ = os.Chmod(locked, 0o755) }()

	u := newTestTracker(1)
	u.set("/c/a", 100)
	_, err := u.reconcile(dir)
	s.ErrorIs(err, fs.ErrPermission)
	s.EqualValues(100, u.bytes())
}

func (s *usageTrackerTestSuite) TestReconcileIgnoresNonRegularFiles() {
	dir := s.T().TempDir()
	target := filepath.Join(dir, "f")
	s.Require().NoError(os.WriteFile(target, make([]byte, 10), 0o644))
	if err := os.Symlink(target, filepath.Join(dir, "link")); err != nil {
		s.T().Skipf("symlinks unsupported: %v", err)
	}

	total, err := newTestTracker(1).reconcile(dir)
	s.Require().NoError(err)
	s.EqualValues(10, total)
}

func TestUsageTrackerTestSuite(t *testing.T) {
	suite.Run(t, new(usageTrackerTestSuite))
}

// diskUsage returns the apparent size of all files under root
func (suite *fileCacheTestSuite) diskUsage() int64 {
	total, err := newUsageTracker().reconcile(suite.cache_path)
	suite.Require().NoError(err)
	return total
}

func (suite *fileCacheTestSuite) TestUsageTrackedAcrossOperations() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	// a cache that already holds files, as after a restart
	suite.Require().NoError(os.MkdirAll(suite.cache_path, 0o755))
	for i := range 10 {
		path := filepath.Join(suite.cache_path, fmt.Sprintf("old%d", i))
		suite.Require().NoError(os.WriteFile(path, make([]byte, 4096), 0o644))
	}
	suite.setupTestHelper(fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true\n  hard-limit: true\n  allow-non-empty-temp: true\n\nloopbackfs:\n  path: %s",
		suite.cache_path,
		suite.fake_storage_path,
	))
	fc := suite.fileCache
	check := func(step string) {
		suite.assert.Equal(suite.diskUsage(), fc.usage.bytes(), step)
	}
	check("start (pre-existing files)")
	suite.assert.EqualValues(10*4096, fc.usage.bytes())

	h, err := fc.CreateFile(internal.CreateFileOptions{Name: "a", Mode: 0o644})
	suite.Require().NoError(err)
	_, err = fc.WriteFile(
		&internal.WriteFileOptions{Handle: h, Offset: 0, Data: make([]byte, 1000)},
	)
	suite.Require().NoError(err)
	_, err = fc.WriteFile(
		&internal.WriteFileOptions{Handle: h, Offset: 5000, Data: make([]byte, 10)},
	)
	suite.Require().NoError(err)
	check("sparse write")
	_, err = fc.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: make([]byte, 10)})
	suite.Require().NoError(err)
	check("overwrite")
	suite.Require().NoError(fc.TruncateFile(
		internal.TruncateFileOptions{Name: "a", Handle: h, NewSize: 2000},
	))
	check("truncate via handle")
	suite.Require().NoError(fc.ReleaseFile(internal.ReleaseFileOptions{Handle: h}))

	suite.Require().NoError(fc.RenameFile(internal.RenameFileOptions{Src: "a", Dst: "b"}))
	check("rename")

	remote := filepath.Join(suite.fake_storage_path, "remote")
	suite.Require().NoError(os.WriteFile(remote, make([]byte, 12345), 0o644))
	h, err = fc.OpenFile(
		internal.OpenFileOptions{Name: "remote", Flags: os.O_RDONLY, Mode: 0o644},
	)
	suite.Require().NoError(err)
	_, err = fc.ReadInBuffer(
		&internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: make([]byte, 10)},
	)
	suite.Require().NoError(err)
	check("download")
	suite.Require().NoError(fc.ReleaseFile(internal.ReleaseFileOptions{Handle: h}))

	suite.Require().NoError(fc.TruncateFile(
		internal.TruncateFileOptions{Name: "remote", NewSize: 100},
	))
	check("truncate by path")

	suite.Require().NoError(fc.DeleteFile(internal.DeleteFileOptions{Name: "b"}))
	suite.Require().NoError(fc.DeleteFile(internal.DeleteFileOptions{Name: "remote"}))
	check("delete")
	suite.assert.EqualValues(10*4096, fc.usage.bytes())
}

func (suite *fileCacheTestSuite) TestUsageTrackedRenameOfUntrackedFile() {
	defer suite.cleanupTest()
	fc := suite.fileCache
	// a file cached behind the tracker's back (e.g. between reconciles)
	suite.Require().NoError(os.WriteFile(
		filepath.Join(suite.fake_storage_path, "u"), make([]byte, 500), 0o644,
	))
	suite.Require().NoError(os.WriteFile(
		filepath.Join(suite.cache_path, "u"), make([]byte, 500), 0o644,
	))
	_, tracked := fc.usage.size(filepath.Join(suite.cache_path, "u"))
	suite.Require().False(tracked)

	suite.Require().NoError(fc.RenameFile(internal.RenameFileOptions{Src: "u", Dst: "v"}))
	suite.Equal(suite.diskUsage(), fc.usage.bytes())
	size, ok := fc.usage.size(filepath.Join(suite.cache_path, "v"))
	suite.True(ok)
	suite.EqualValues(500, size)
}

func (suite *fileCacheTestSuite) TestUsageTrackedOnTruncatingOpen() {
	defer suite.cleanupTest()
	fc := suite.fileCache
	suite.Require().NoError(os.WriteFile(
		filepath.Join(suite.fake_storage_path, "t"), make([]byte, 4000), 0o644,
	))

	// O_TRUNC skips the download, so the local file starts empty
	h, err := fc.OpenFile(
		internal.OpenFileOptions{Name: "t", Flags: os.O_RDWR | os.O_TRUNC, Mode: 0o644},
	)
	suite.Require().NoError(err)
	defer func() { _ = fc.ReleaseFile(internal.ReleaseFileOptions{Handle: h}) }()
	_, err = fc.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: make([]byte, 1)})
	suite.Require().NoError(err)

	suite.Equal(suite.diskUsage(), fc.usage.bytes())
	size, _ := fc.usage.size(filepath.Join(suite.cache_path, "t"))
	suite.EqualValues(1, size)
}

func (suite *fileCacheTestSuite) TestHardLimitUsesTrackedUsage() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	suite.setupTestHelper(fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true\n  hard-limit: true\n  max-size-mb: 1\n\nloopbackfs:\n  path: %s",
		suite.cache_path,
		suite.fake_storage_path,
	))
	fc := suite.fileCache
	// other cached files already use 900 KiB of the 1 MiB cache
	fc.usage.set(filepath.Join(fc.tmpPath, "other"), 900*1024)

	h, err := fc.CreateFile(internal.CreateFileOptions{Name: "f", Mode: 0o644})
	suite.Require().NoError(err)
	defer func() { _ = fc.ReleaseFile(internal.ReleaseFileOptions{Handle: h}) }()
	_, err = fc.WriteFile(
		&internal.WriteFileOptions{Handle: h, Offset: 0, Data: make([]byte, 100*1024)},
	)
	suite.Require().NoError(err)
	// rewriting existing bytes never needs more space
	_, err = fc.WriteFile(
		&internal.WriteFileOptions{Handle: h, Offset: 0, Data: make([]byte, 100*1024)},
	)
	suite.Require().NoError(err)
	_, err = fc.WriteFile(
		&internal.WriteFileOptions{Handle: h, Offset: 100 * 1024, Data: make([]byte, 64*1024)},
	)
	suite.assert.ErrorIs(err, syscall.ENOSPC)
}
