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

package libfuse

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"strings"
	"syscall"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/handlemap"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/winfsp/cgofuse/fuse"
	"go.uber.org/mock/gomock"
)

type libfuseTestSuite struct {
	suite.Suite
	assert   *assert.Assertions
	libfuse  *Libfuse
	cgofuse  *CgofuseFS
	mockCtrl *gomock.Controller
	mock     *internal.MockComponent
}

// Open and create call returns this kind of object
var emptyConfig = ""

// For fuse calls
var cfuseFS *CgofuseFS

func newTestLibfuse(next internal.Component, configuration string) *Libfuse {
	err := config.ReadConfigFromReader(strings.NewReader(configuration))
	if err != nil {
		panic(fmt.Sprintf("Unable to read config from reader: %v", err))
	}
	libfuse := NewLibfuseComponent()
	libfuse.SetNextComponent(next)
	err = libfuse.Configure(true)
	if err != nil {
		panic(fmt.Sprintf("Unable to configure for testing: %v", err))
	}

	return libfuse.(*Libfuse)
}

func (suite *libfuseTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
	suite.setupTestHelper(emptyConfig)
}

func (suite *libfuseTestSuite) setupTestHelper(config string) {
	suite.assert = assert.New(suite.T())

	suite.mockCtrl = gomock.NewController(suite.T())
	suite.mock = internal.NewMockComponent(suite.mockCtrl)
	suite.libfuse = newTestLibfuse(suite.mock, config)
	suite.cgofuse = &CgofuseFS{}
	cfuseFS = suite.cgofuse
	fuseFS = suite.libfuse
	// suite.libfuse.Start(context.Background())
}

func (suite *libfuseTestSuite) cleanupTest() {
	// suite.libfuse.Stop()
	suite.mockCtrl.Finish()
}

func testMkDir(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(0775)
	options := internal.CreateDirOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().CreateDir(options).Return(nil)

	// On Windows we test for directory creation, so we have a call to GetAttr
	if runtime.GOOS == "windows" {
		option := internal.GetAttrOptions{Name: name}
		suite.mock.EXPECT().GetAttr(option).Return(nil, syscall.ENOENT)
	}

	err := cfuseFS.Mkdir(path, 0775)
	suite.assert.Equal(0, err)
}

func testStatFs(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	path := "/"
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{Frsize: 1,
		Blocks: 2, Bavail: 3, Bfree: 4, Bsize: 5, Files: 6, Ffree: 7, Namemax: 8}, true, nil)
	buf := &fuse.Statfs_t{}
	ret := cfuseFS.Statfs(path, buf)

	suite.assert.Equal(0, ret)
	suite.assert.Equal(1, int(buf.Frsize))
	suite.assert.Equal(2, int(buf.Blocks))
	suite.assert.Equal(3, int(buf.Bavail))
	suite.assert.Equal(4, int(buf.Bfree))
	suite.assert.Equal(5, int(buf.Bsize))
	suite.assert.Equal(6, int(buf.Files))
	suite.assert.Equal(7, int(buf.Ffree))
	suite.assert.Equal(8, int(buf.Namemax))
}

func testStatFsNotPopulated(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	path := "/"
	suite.mock.EXPECT().StatFs().Return(nil, false, nil)
	buf := &fuse.Statfs_t{}
	ret := cfuseFS.Statfs(path, buf)

	suite.assert.Equal(0, ret)

	// By default these are all 0, so they should be populated by the system
	suite.assert.NotZero(buf.Frsize)
	suite.assert.NotZero(buf.Blocks)
	suite.assert.NotZero(buf.Bavail)
	suite.assert.NotZero(buf.Bfree)
	suite.assert.NotZero(buf.Bsize)
	suite.assert.NotZero(buf.Files)
	suite.assert.NotZero(buf.Ffree)
	suite.assert.NotZero(buf.Namemax)
}

func testStatFsCloudStorageCapacity(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	path := "/"

	fuseFS.displayCapacityMb = 1
	attr := &common.Statfs_t{
		Frsize:  1,
		Blocks:  10,
		Bavail:  0,
		Bfree:   0,
		Bsize:   1024,
		Files:   6,
		Ffree:   7,
		Namemax: 255,
	}
	suite.mock.EXPECT().StatFs().Return(attr, true, nil)

	buf := &fuse.Statfs_t{}
	ret := cfuseFS.Statfs(path, buf)

	suite.assert.Equal(0, ret)
	suite.assert.Equal(uint64(1024), buf.Blocks)
	suite.assert.Equal(uint64(1014), buf.Bavail)
	suite.assert.Equal(uint64(1014), buf.Bfree)
}

func testStatFsCloudStorageCapacityUsedExceedsDisplay(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	path := "/"

	fuseFS.displayCapacityMb = 1
	attr := &common.Statfs_t{
		Frsize:  1,
		Blocks:  5000,
		Bavail:  0,
		Bfree:   0,
		Bsize:   1024,
		Files:   6,
		Ffree:   7,
		Namemax: 255,
	}
	suite.mock.EXPECT().StatFs().Return(attr, true, nil)

	buf := &fuse.Statfs_t{}
	ret := cfuseFS.Statfs(path, buf)

	suite.assert.Equal(0, ret)
	suite.assert.Equal(uint64(5000), buf.Blocks)
	suite.assert.Equal(uint64(0), buf.Bavail)
	suite.assert.Equal(uint64(0), buf.Bfree)
}

func testStatFsError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	path := "/"
	suite.mock.EXPECT().StatFs().Return(nil, false, errors.New("error"))
	buf := &fuse.Statfs_t{}
	ret := cfuseFS.Statfs(path, buf)
	suite.assert.Equal(-fuse.EIO, ret)
}

func testMkDirError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(0775)
	options := internal.CreateDirOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().CreateDir(options).Return(errors.New("failed to create directory"))

	// On Windows we test for directory creation, so we have a call to GetAttr
	if runtime.GOOS == "windows" {
		option := internal.GetAttrOptions{Name: name}
		suite.mock.EXPECT().GetAttr(option).Return(nil, syscall.ENOENT)
	}

	err := cfuseFS.Mkdir(path, 0775)
	suite.assert.Equal(-fuse.EIO, err)
}

func testMkDirErrorPermission(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(0775)
	options := internal.CreateDirOptions{Name: name, Mode: mode}

	if runtime.GOOS == "windows" {
		option := internal.GetAttrOptions{Name: name}
		suite.mock.EXPECT().GetAttr(option).Return(nil, syscall.ENOENT)
	}

	suite.mock.EXPECT().CreateDir(options).Return(os.ErrPermission)

	err := cfuseFS.Mkdir(path, 0775)
	suite.assert.Equal(-fuse.EACCES, err)
}

func testMkDirErrorExist(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(0775)
	options := internal.CreateDirOptions{Name: name, Mode: mode}

	if runtime.GOOS == "windows" {
		option := internal.GetAttrOptions{Name: name}
		suite.mock.EXPECT().GetAttr(option).Return(nil, syscall.ENOENT)
	}

	suite.mock.EXPECT().CreateDir(options).Return(os.ErrExist)

	err := cfuseFS.Mkdir(path, 0775)
	suite.assert.Equal(-fuse.EEXIST, err)
}

// testMkDirErrorAttrExist only runs on Windows to test the case that the directory already exists
// and the attributes state it is a directory.
func testMkDirErrorAttrExist(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	if runtime.GOOS != "windows" {
		return
	}
	name := "path"
	path := "/" + name
	option := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().
		GetAttr(option).
		Return(&internal.ObjAttr{Flags: internal.NewDirBitMap()}, nil)
	err := cfuseFS.Mkdir(path, 0775)
	suite.assert.Equal(-fuse.EEXIST, err)
}

func testTrimFusePath(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	suite.assert.Empty(trimFusePath(""))
	suite.assert.Equal("path", trimFusePath("/path"))
	suite.assert.Equal("path", trimFusePath("path"))
}

func testNewCgofuseFS(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	cgofuse := NewcgofuseFS()
	suite.assert.NotNil(cgofuse)
}

func testGetAttrRoot(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	cfuseFS.uid = 1234
	cfuseFS.gid = 5678
	buf := &fuse.Stat_t{}
	err := cfuseFS.Getattr("/", buf, 0)
	suite.assert.Equal(0, err)
	suite.assert.Equal(uint32(1234), buf.Uid)
	suite.assert.Equal(uint32(5678), buf.Gid)
	suite.assert.Equal(int64(4096), buf.Size)
}

func testGetAttrIgnoredFile(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	buf := &fuse.Stat_t{}
	err := cfuseFS.Getattr("/.Trash", buf, 0)
	suite.assert.Equal(-fuse.ENOENT, err)
}

func testGetAttrErrors(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	option := internal.GetAttrOptions{Name: name}
	buf := &fuse.Stat_t{}

	suite.mock.EXPECT().GetAttr(option).Return(nil, syscall.ENOENT)
	suite.assert.Equal(-fuse.ENOENT, cfuseFS.Getattr("/"+name, buf, 0))

	suite.mock.EXPECT().GetAttr(option).Return(nil, syscall.EACCES)
	suite.assert.Equal(-fuse.EACCES, cfuseFS.Getattr("/"+name, buf, 0))

	suite.mock.EXPECT().GetAttr(option).Return(nil, errors.New("boom"))
	suite.assert.Equal(-fuse.EIO, cfuseFS.Getattr("/"+name, buf, 0))
}

func testFuseErrnoFromError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	testCases := []struct {
		err      error
		expected int
	}{
		{err: nil, expected: 0},
		{err: syscall.ENOTEMPTY, expected: -fuse.ENOTEMPTY},
		{err: fs.ErrNotExist, expected: -fuse.ENOENT},
		{err: os.ErrNotExist, expected: -fuse.ENOENT},
		{err: syscall.EPERM, expected: -fuse.EPERM},
		{err: syscall.EACCES, expected: -fuse.EACCES},
		{err: fs.ErrPermission, expected: -fuse.EACCES},
		{err: os.ErrPermission, expected: -fuse.EACCES},
		{err: syscall.EEXIST, expected: -fuse.EEXIST},
		{err: fs.ErrExist, expected: -fuse.EEXIST},
		{err: os.ErrExist, expected: -fuse.EEXIST},
		{err: syscall.EIO, expected: -fuse.EIO},
		{err: errors.New("boom"), expected: -fuse.EIO},
	}

	for _, tc := range testCases {
		suite.assert.Equal(tc.expected, fuseErrnoFromError(tc.err))
	}
}

func testOpendirAndReleasedir(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	err, fh := cfuseFS.Opendir("/dir")
	suite.assert.Equal(0, err)
	release := cfuseFS.Releasedir("/dir", fh)
	suite.assert.Equal(0, release)
}

func testReleasedirMissingHandle(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	release := cfuseFS.Releasedir("/dir", 999)
	suite.assert.Equal(-fuse.EBADF, release)
}

func testServeCachedEntries(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	cacheInfo := &dirChildCache{
		sIndex: 0,
		eIndex: 2,
		length: 2,
		children: []*internal.ObjAttr{
			{Name: "a", Flags: internal.NewFileBitMap()},
			{Name: "b", Flags: internal.NewFileBitMap()},
		},
		lastPage: true,
	}
	fillCount := 0
	fill := func(name string, stat *fuse.Stat_t, ofst int64) bool {
		fillCount++
		return true
	}

	nextOffset, done := serveCachedEntries(cacheInfo, 0, fill)
	suite.assert.Equal(uint64(2), nextOffset)
	suite.assert.True(done)
	suite.assert.Equal(2, fillCount)
}

func testServeCachedEntriesStopEarly(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	cacheInfo := &dirChildCache{
		sIndex: 0,
		eIndex: 2,
		length: 2,
		children: []*internal.ObjAttr{
			{Name: "a", Flags: internal.NewFileBitMap()},
			{Name: "b", Flags: internal.NewFileBitMap()},
		},
		lastPage: false,
	}
	fillCount := 0
	fill := func(name string, stat *fuse.Stat_t, ofst int64) bool {
		fillCount++
		return false
	}

	nextOffset, done := serveCachedEntries(cacheInfo, 0, fill)
	suite.assert.Equal(uint64(1), nextOffset)
	suite.assert.True(done)
	suite.assert.Equal(1, fillCount)
}

func testFillStatModes(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	lf := suite.libfuse
	lf.dirPermission = 0777
	lf.filePermission = 0666

	dirAttr := &internal.ObjAttr{Flags: internal.NewDirBitMap(), Mode: 0701}
	fileAttr := &internal.ObjAttr{Flags: internal.NewFileBitMap(), Mode: 0640}
	linkAttr := &internal.ObjAttr{Flags: internal.NewSymlinkBitMap(), Mode: 0777}

	dirStat := &fuse.Stat_t{}
	fileStat := &fuse.Stat_t{}
	linkStat := &fuse.Stat_t{}
	lf.fillStat(dirAttr, dirStat)
	lf.fillStat(fileAttr, fileStat)
	lf.fillStat(linkAttr, linkStat)

	suite.assert.NotEqual(0, dirStat.Mode&fuse.S_IFDIR)
	suite.assert.NotEqual(0, fileStat.Mode&fuse.S_IFREG)
	suite.assert.NotEqual(0, linkStat.Mode&fuse.S_IFLNK)
	suite.assert.Equal(uint32(0701), dirStat.Mode&0x1ff)
	suite.assert.Equal(uint32(0640), fileStat.Mode&0x1ff)
	suite.assert.Equal(uint32(0777), linkStat.Mode&0x1ff)
}

func testFillStatModeDefault(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	lf := suite.libfuse
	lf.dirPermission = 0755
	lf.filePermission = 0644

	attr := &internal.ObjAttr{Flags: internal.NewDirBitMap(), Mode: 0}
	attr.Flags.Set(internal.PropFlagModeDefault)

	st := &fuse.Stat_t{}
	lf.fillStat(attr, st)

	suite.assert.NotEqual(0, st.Mode&fuse.S_IFDIR)
	suite.assert.Equal(uint32(lf.dirPermission), st.Mode&0x1ff)
}

func testFillStatSyntheticInode(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	lf := suite.libfuse

	attr := &internal.ObjAttr{Path: "dir/file", Flags: internal.NewFileBitMap(), Mode: 0644}
	sameAttr := &internal.ObjAttr{Path: "dir/file", Flags: internal.NewFileBitMap(), Mode: 0644}
	otherAttr := &internal.ObjAttr{Path: "dir/other", Flags: internal.NewFileBitMap(), Mode: 0644}

	first := &fuse.Stat_t{}
	second := &fuse.Stat_t{}
	third := &fuse.Stat_t{}
	lf.fillStat(attr, first)
	lf.fillStat(sameAttr, second)
	lf.fillStat(otherAttr, third)

	suite.assert.NotZero(first.Ino)
	suite.assert.Equal(first.Ino, second.Ino)
	suite.assert.NotEqual(first.Ino, third.Ino)
}

func testFillStatSpecialPermissionBits(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	lf := suite.libfuse

	attr := &internal.ObjAttr{
		Path:  "file",
		Flags: internal.NewFileBitMap(),
		Mode:  os.FileMode(0751) | os.ModeSetuid | os.ModeSetgid | os.ModeSticky,
	}

	st := &fuse.Stat_t{}
	lf.fillStat(attr, st)

	suite.assert.Equal(uint32(07751), st.Mode&0o7777)
}

func testNormalizeFusePathRelativePathLimit(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	suite.libfuse.mountPath = strings.Repeat("m", 128)

	component := strings.Repeat("a", 200)
	name := strings.Repeat(component+"/", 19) + component

	normalized, errno := normalizeFusePath("/" + name)
	suite.assert.Equal(name, normalized)
	suite.assert.Equal(0, errno)
}

func testNormalizeFusePathTooLongComponent(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	name := strings.Repeat("a", maxNameSize+1)
	normalized, errno := normalizeFusePath("/" + name)

	suite.assert.Equal(name, normalized)
	suite.assert.Equal(-fuse.ENAMETOOLONG, errno)
}

func testReadMissingHandle(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	buf := make([]byte, 8)
	err := cfuseFS.Read("/path", buf, 0, 999)
	suite.assert.Equal(-fuse.EBADF, err)
}

func testReadCachedHandle(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	file, err := os.CreateTemp("", "cfuse-read-*")
	suite.assert.NoError(err)
	defer os.Remove(file.Name())
	defer file.Close()

	_, err = file.Write([]byte("hello"))
	suite.assert.NoError(err)

	handle := handlemap.NewHandle("file")
	handle.Flags.Set(handlemap.HandleFlagCached)
	handle.SetFileObject(file)
	fh := handlemap.Add(handle)
	defer handlemap.Delete(handle.ID)

	buf := make([]byte, 5)
	read := cfuseFS.Read("/file", buf, 0, uint64(fh))
	suite.assert.Equal(5, read)
	suite.assert.Equal("hello", string(buf))
}

func testReadCachedHandleEOF(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	file, err := os.CreateTemp("", "cfuse-read-eof-*")
	suite.assert.NoError(err)
	defer os.Remove(file.Name())
	defer file.Close()

	_, err = file.Write([]byte("hi"))
	suite.assert.NoError(err)

	handle := handlemap.NewHandle("file")
	handle.Flags.Set(handlemap.HandleFlagCached)
	handle.SetFileObject(file)
	fh := handlemap.Add(handle)
	defer handlemap.Delete(handle.ID)

	buf := make([]byte, 5)
	read := cfuseFS.Read("/file", buf, 0, uint64(fh))
	suite.assert.Equal(2, read)
	suite.assert.Equal("hi", string(buf[:read]))
}

func testReadFromComponent(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	handle := handlemap.NewHandle("file")
	fh := handlemap.Add(handle)
	defer handlemap.Delete(handle.ID)

	buf := make([]byte, 4)
	suite.mock.EXPECT().
		ReadInBuffer(gomock.AssignableToTypeOf(&internal.ReadInBufferOptions{})).
		DoAndReturn(func(opt *internal.ReadInBufferOptions) (int, error) {
			suite.assert.Equal(handle.ID, opt.Handle.ID)
			suite.assert.Equal("file", opt.Handle.Path)
			suite.assert.Equal(int64(7), opt.Offset)
			suite.assert.Len(opt.Data, 4)
			copy(opt.Data, []byte("ping"))
			return 4, nil
		})
	read := cfuseFS.Read("/file", buf, 7, uint64(fh))
	suite.assert.Equal(4, read)
	suite.assert.Equal("ping", string(buf))
}

func testReadAccessDenied(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	handle := handlemap.NewHandle("file")
	fh := handlemap.Add(handle)
	defer handlemap.Delete(handle.ID)

	buf := make([]byte, 4)
	suite.mock.EXPECT().
		ReadInBuffer(gomock.AssignableToTypeOf(&internal.ReadInBufferOptions{})).
		DoAndReturn(func(opt *internal.ReadInBufferOptions) (int, error) {
			suite.assert.Equal(handle.ID, opt.Handle.ID)
			suite.assert.Equal(int64(3), opt.Offset)
			suite.assert.Len(opt.Data, 4)
			return 0, os.ErrPermission
		})
	read := cfuseFS.Read("/file", buf, 3, uint64(fh))
	suite.assert.Equal(-fuse.EACCES, read)
}

func testReadError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	handle := handlemap.NewHandle("file")
	fh := handlemap.Add(handle)
	defer handlemap.Delete(handle.ID)

	buf := make([]byte, 4)
	suite.mock.EXPECT().
		ReadInBuffer(gomock.AssignableToTypeOf(&internal.ReadInBufferOptions{})).
		DoAndReturn(func(opt *internal.ReadInBufferOptions) (int, error) {
			suite.assert.Equal(handle.ID, opt.Handle.ID)
			suite.assert.Equal(int64(5), opt.Offset)
			suite.assert.Len(opt.Data, 4)
			return 0, errors.New("boom")
		})
	read := cfuseFS.Read("/file", buf, 5, uint64(fh))
	suite.assert.Equal(-fuse.EIO, read)
}

func testWriteMissingHandle(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	buf := []byte("data")
	err := cfuseFS.Write("/path", buf, 0, 999)
	suite.assert.Equal(-fuse.EBADF, err)
}

func testWriteSuccess(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	handle := handlemap.NewHandle("file")
	fh := handlemap.Add(handle)
	defer handlemap.Delete(handle.ID)

	buf := []byte("data")
	suite.mock.EXPECT().
		WriteFile(gomock.AssignableToTypeOf(&internal.WriteFileOptions{})).
		DoAndReturn(func(opt *internal.WriteFileOptions) (int, error) {
			suite.assert.Equal(handle.ID, opt.Handle.ID)
			suite.assert.Equal("file", opt.Handle.Path)
			suite.assert.Equal(int64(9), opt.Offset)
			suite.assert.Equal("data", string(opt.Data))
			return len(opt.Data), nil
		})
	written := cfuseFS.Write("/file", buf, 9, uint64(fh))
	suite.assert.Equal(len(buf), written)
}

func testWriteError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	handle := handlemap.NewHandle("file")
	fh := handlemap.Add(handle)
	defer handlemap.Delete(handle.ID)

	buf := []byte("data")
	suite.mock.EXPECT().
		WriteFile(gomock.AssignableToTypeOf(&internal.WriteFileOptions{})).
		DoAndReturn(func(opt *internal.WriteFileOptions) (int, error) {
			suite.assert.Equal(handle.ID, opt.Handle.ID)
			suite.assert.Equal(int64(11), opt.Offset)
			suite.assert.Equal("data", string(opt.Data))
			return 0, errors.New("boom")
		})
	written := cfuseFS.Write("/file", buf, 11, uint64(fh))
	suite.assert.Equal(-fuse.EIO, written)
}

func testWriteAccessDenied(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	handle := handlemap.NewHandle("file")
	fh := handlemap.Add(handle)
	defer handlemap.Delete(handle.ID)

	buf := []byte("data")
	suite.mock.EXPECT().
		WriteFile(gomock.AssignableToTypeOf(&internal.WriteFileOptions{})).
		DoAndReturn(func(opt *internal.WriteFileOptions) (int, error) {
			suite.assert.Equal(handle.ID, opt.Handle.ID)
			suite.assert.Equal(int64(13), opt.Offset)
			suite.assert.Equal("data", string(opt.Data))
			return 0, os.ErrPermission
		})
	written := cfuseFS.Write("/file", buf, 13, uint64(fh))
	suite.assert.Equal(-fuse.EACCES, written)
}

func testFlushNotDirty(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	handle := handlemap.NewHandle("file")
	fh := handlemap.Add(handle)
	defer handlemap.Delete(handle.ID)

	result := cfuseFS.Flush("/file", uint64(fh))
	suite.assert.Equal(0, result)
}

func testFlushErrors(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	handleNotExist := handlemap.NewHandle("file")
	handleNotExist.Flags.Set(handlemap.HandleFlagDirty)
	fhNotExist := handlemap.Add(handleNotExist)
	defer handlemap.Delete(handleNotExist.ID)

	handleAccessDenied := handlemap.NewHandle("file")
	handleAccessDenied.Flags.Set(handlemap.HandleFlagDirty)
	fhAccessDenied := handlemap.Add(handleAccessDenied)
	defer handlemap.Delete(handleAccessDenied.ID)

	handleError := handlemap.NewHandle("file")
	handleError.Flags.Set(handlemap.HandleFlagDirty)
	fhError := handlemap.Add(handleError)
	defer handlemap.Delete(handleError.ID)

	suite.mock.EXPECT().FlushFile(gomock.Any()).Return(syscall.ENOENT)
	suite.assert.Equal(-fuse.ENOENT, cfuseFS.Flush("/file", uint64(fhNotExist)))

	suite.mock.EXPECT().FlushFile(gomock.Any()).Return(syscall.EACCES)
	suite.assert.Equal(-fuse.EACCES, cfuseFS.Flush("/file", uint64(fhAccessDenied)))

	suite.mock.EXPECT().FlushFile(gomock.Any()).Return(errors.New("boom"))
	suite.assert.Equal(-fuse.EIO, cfuseFS.Flush("/file", uint64(fhError)))
}

func testFlushMissingHandle(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	err := cfuseFS.Flush("/file", 999)
	suite.assert.Equal(-fuse.EBADF, err)
}

func testReleaseMissingHandle(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	err := cfuseFS.Release("/file", 999)
	suite.assert.Equal(-fuse.EBADF, err)
}

func testReleaseError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	handle := handlemap.NewHandle("file")
	fh := handlemap.Add(handle)
	defer handlemap.Delete(handle.ID)

	suite.mock.EXPECT().ReleaseFile(gomock.Any()).Return(syscall.ENOENT)
	err := cfuseFS.Release("/file", uint64(fh))
	suite.assert.Equal(-fuse.ENOENT, err)
}

func testReleaseErrorAccess(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	handle := handlemap.NewHandle("file")
	fh := handlemap.Add(handle)
	defer handlemap.Delete(handle.ID)

	suite.mock.EXPECT().ReleaseFile(gomock.Any()).Return(syscall.EACCES)
	err := cfuseFS.Release("/file", uint64(fh))
	suite.assert.Equal(-fuse.EACCES, err)
}

func testUnsupportedOps(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	suite.assert.Equal(-fuse.ENOSYS, cfuseFS.Access("/path", 0))
	ret, data := cfuseFS.Getxattr("/path", "x")
	suite.assert.Equal(-fuse.ENOSYS, ret)
	suite.assert.Nil(data)
	suite.assert.Equal(-fuse.ENOSYS, cfuseFS.Link("/a", "/b"))
	suite.assert.Equal(
		-fuse.ENOSYS,
		cfuseFS.Listxattr("/path", func(name string) bool { return true }),
	)
	suite.assert.Equal(-fuse.ENOSYS, cfuseFS.Mknod("/path", 0, 0))
	suite.assert.Equal(-fuse.ENOSYS, cfuseFS.Removexattr("/path", "x"))
	suite.assert.Equal(-fuse.ENOSYS, cfuseFS.Setxattr("/path", "x", []byte("v"), 0))
}

// TODO: ReadDir test

func testReaddirMissingHandle(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	fill := func(name string, stat *fuse.Stat_t, ofst int64) bool {
		return true
	}

	err := cfuseFS.Readdir("/missing", fill, 0, 999)
	suite.assert.Equal(-fuse.EBADF, err)
}

func testReaddirMissingCache(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	fill := func(name string, stat *fuse.Stat_t, ofst int64) bool {
		return true
	}

	handle := handlemap.NewHandle("dir/")
	fh := handlemap.Add(handle)
	defer handlemap.Delete(handle.ID)

	err := cfuseFS.Readdir("/dir", fill, 0, uint64(fh))
	suite.assert.Equal(-fuse.EIO, err)
}

func testReaddirEmptyPageToken(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	handle := handlemap.NewHandle("dir/")
	cacheInfo := &dirChildCache{
		sIndex:   0,
		eIndex:   0,
		token:    "",
		length:   0,
		children: make([]*internal.ObjAttr, 0),
	}
	cacheDots(cacheInfo, "dir/")
	cacheInfo.token = "next"
	cacheInfo.lastPage = false
	handle.SetValue("cache", cacheInfo)
	fh := handlemap.Add(handle)
	defer handlemap.Delete(handle.ID)

	suite.mock.EXPECT().
		StreamDir(internal.StreamDirOptions{Name: "dir/", Token: "next"}).
		Return([]*internal.ObjAttr{}, "next", nil)

	fillCalled := false
	fill := func(name string, stat *fuse.Stat_t, ofst int64) bool {
		fillCalled = true
		return true
	}

	err := cfuseFS.Readdir("/dir", fill, 2, uint64(fh))
	suite.assert.Equal(0, err)
	suite.assert.False(fillCalled)
}

func testReaddirPermissionError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	fill := func(name string, stat *fuse.Stat_t, ofst int64) bool { return true }

	handle := handlemap.NewHandle("dir/")
	cacheInfo := &dirChildCache{
		sIndex:   0,
		eIndex:   2,
		token:    "next",
		length:   2,
		children: make([]*internal.ObjAttr, 0),
		lastPage: false,
	}
	cacheDots(cacheInfo, "dir/")
	cacheInfo.token = "next"
	cacheInfo.lastPage = false
	handle.SetValue("cache", cacheInfo)
	fh := handlemap.Add(handle)
	defer handlemap.Delete(handle.ID)

	suite.mock.EXPECT().
		StreamDir(internal.StreamDirOptions{Name: "dir/", Token: "next"}).
		Return(nil, "", os.ErrPermission)

	err := cfuseFS.Readdir("/dir", fill, 2, uint64(fh))
	suite.assert.Equal(-fuse.EACCES, err)
}

func testCreateFuseOptionsFlags(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	host := fuse.NewFileSystemHost(&CgofuseFS{})
	fuseFS.directIO = false

	umask := uint32(0022)
	options := createFuseOptions(host, true, true, true, false, 128, umask)
	suite.assert.Contains(options, "allow_other")
	suite.assert.Contains(options, "allow_root")
	suite.assert.Contains(options, "ro")
	expectedUmask := fmt.Sprintf("umask=%04o", umask)
	suite.assert.Contains(options, expectedUmask)
	suite.assert.Contains(options, "kernel_cache")
}

func testCreateFuseOptionsDirectIO(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	host := fuse.NewFileSystemHost(&CgofuseFS{})
	fuseFS.directIO = true

	options := createFuseOptions(host, false, false, false, false, 128, 0)
	if strings.Contains(options, "max_readahead=") {
		suite.assert.Contains(options, "direct_io")
	} else {
		suite.assert.NotContains(options, "direct_io")
	}
	suite.assert.NotContains(options, "kernel_cache")
}

func testPopulateDirChildCacheReplaceCache(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	handle := handlemap.NewHandle("dir/")
	cacheInfo := &dirChildCache{
		sIndex:   0,
		eIndex:   0,
		token:    "",
		length:   common.MaxDirListCount - 1,
		children: make([]*internal.ObjAttr, 0),
		lastPage: false,
	}

	returnedAttrs := []*internal.ObjAttr{
		{Name: "a", Flags: internal.NewFileBitMap()},
		{Name: "b", Flags: internal.NewFileBitMap()},
	}
	suite.mock.EXPECT().
		StreamDir(internal.StreamDirOptions{Name: "dir/", Token: ""}).
		Return(returnedAttrs, "", nil)

	errorCode := populateDirChildCache(handle, cacheInfo, 10)
	suite.assert.Equal(0, errorCode)
	suite.assert.Equal(uint64(10), cacheInfo.sIndex)
	suite.assert.Equal(uint64(12), cacheInfo.eIndex)
	suite.assert.Equal(uint64(2), cacheInfo.length)
	suite.assert.True(cacheInfo.lastPage)
}

func testPopulateDirChildCacheLastPage(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	handle := handlemap.NewHandle("dir/")
	cacheInfo := &dirChildCache{
		sIndex:   0,
		eIndex:   0,
		token:    "",
		length:   0,
		children: make([]*internal.ObjAttr, 0),
		lastPage: true,
	}

	suite.mock.EXPECT().StreamDir(gomock.Any()).Times(0)

	errorCode := populateDirChildCache(handle, cacheInfo, 0)
	suite.assert.Equal(0, errorCode)
}

func testPopulateDirChildCacheNotFound(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	handle := handlemap.NewHandle("dir/")
	cacheInfo := &dirChildCache{
		sIndex:   0,
		eIndex:   0,
		token:    "",
		length:   0,
		children: make([]*internal.ObjAttr, 0),
		lastPage: false,
	}

	suite.mock.EXPECT().
		StreamDir(internal.StreamDirOptions{Name: "dir/", Token: ""}).
		Return(nil, "", os.ErrNotExist)

	errorCode := populateDirChildCache(handle, cacheInfo, 0)
	suite.assert.Equal(-fuse.ENOENT, errorCode)
}

func testPopulateDirChildCacheAppend(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	handle := handlemap.NewHandle("dir/")
	cacheInfo := &dirChildCache{
		sIndex:   0,
		eIndex:   0,
		token:    "",
		length:   0,
		children: make([]*internal.ObjAttr, 0),
		lastPage: false,
	}

	returnedAttrs := []*internal.ObjAttr{{Name: "a", Flags: internal.NewFileBitMap()}}
	suite.mock.EXPECT().
		StreamDir(internal.StreamDirOptions{Name: "dir/", Token: ""}).
		Return(returnedAttrs, "next", nil)

	errorCode := populateDirChildCache(handle, cacheInfo, 0)
	suite.assert.Equal(0, errorCode)
	suite.assert.Equal(uint64(1), cacheInfo.eIndex)
	suite.assert.Equal(uint64(1), cacheInfo.length)
	suite.assert.False(cacheInfo.lastPage)
}

func testRmDir(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	isDirEmptyOptions := internal.IsDirEmptyOptions{Name: name}
	suite.mock.EXPECT().IsDirEmpty(isDirEmptyOptions).Return(true)
	deleteDirOptions := internal.DeleteDirOptions{Name: name}
	suite.mock.EXPECT().DeleteDir(deleteDirOptions).Return(nil)

	err := cfuseFS.Rmdir(path)
	suite.assert.Equal(0, err)
}

func testRmDirNotEmpty(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	isDirEmptyOptions := internal.IsDirEmptyOptions{Name: name}
	suite.mock.EXPECT().IsDirEmpty(isDirEmptyOptions).Return(false)

	err := cfuseFS.Rmdir(path)
	suite.assert.Equal(-fuse.ENOTEMPTY, err)
}

func testRmDirError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	isDirEmptyOptions := internal.IsDirEmptyOptions{Name: name}
	suite.mock.EXPECT().IsDirEmpty(isDirEmptyOptions).Return(true)
	deleteDirOptions := internal.DeleteDirOptions{Name: name}
	suite.mock.EXPECT().DeleteDir(deleteDirOptions).Return(errors.New("failed to delete directory"))

	err := cfuseFS.Rmdir(path)
	suite.assert.Equal(-fuse.EIO, err)
}

func testRmDirNotExists(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	isDirEmptyOptions := internal.IsDirEmptyOptions{Name: name}
	suite.mock.EXPECT().IsDirEmpty(isDirEmptyOptions).Return(true)
	deleteDirOptions := internal.DeleteDirOptions{Name: name}
	suite.mock.EXPECT().DeleteDir(deleteDirOptions).Return(os.ErrNotExist)

	err := cfuseFS.Rmdir(path)
	suite.assert.Equal(-fuse.ENOENT, err)
}

func testRmDirPermission(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	isDirEmptyOptions := internal.IsDirEmptyOptions{Name: name}
	suite.mock.EXPECT().IsDirEmpty(isDirEmptyOptions).Return(true)
	deleteDirOptions := internal.DeleteDirOptions{Name: name}
	suite.mock.EXPECT().DeleteDir(deleteDirOptions).Return(os.ErrPermission)

	err := cfuseFS.Rmdir(path)
	suite.assert.Equal(-fuse.EACCES, err)
}

func testRmDirRaceNotEmpty(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	isDirEmptyOptions := internal.IsDirEmptyOptions{Name: name}
	suite.mock.EXPECT().IsDirEmpty(isDirEmptyOptions).Return(true)
	deleteDirOptions := internal.DeleteDirOptions{Name: name}
	suite.mock.EXPECT().DeleteDir(deleteDirOptions).Return(syscall.ENOTEMPTY)

	err := cfuseFS.Rmdir(path)
	suite.assert.Equal(-fuse.ENOTEMPTY, err)
}

func testCreate(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(0775)
	options := internal.CreateFileOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().CreateFile(options).Return(&handlemap.Handle{}, nil)

	err, fh := cfuseFS.Create(path, 0, uint32(mode))
	suite.assert.Equal(0, err)

	option := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().GetAttr(option).Return(&internal.ObjAttr{}, nil)
	stbuf := &fuse.Stat_t{}
	err = cfuseFS.Getattr(path, stbuf, fh)
	suite.assert.Equal(0, err)
	suite.assert.Equal(int64(0), stbuf.Mtim.Nsec)
	suite.assert.NotEqual(int64(0), stbuf.Mtim.Sec)
}

func testCreateError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(0775)
	options := internal.CreateFileOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().
		CreateFile(options).
		Return(&handlemap.Handle{}, errors.New("failed to create file"))

	err, _ := cfuseFS.Create(path, 0, uint32(mode))
	suite.assert.Equal(-fuse.EIO, err)
}

func testCreateErrorExists(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(0775)
	options := internal.CreateFileOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().CreateFile(options).Return(&handlemap.Handle{}, os.ErrExist)

	err, _ := cfuseFS.Create(path, 0, uint32(mode))
	suite.assert.Equal(-fuse.EEXIST, err)
}

func testCreateErrorPermission(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(0775)
	options := internal.CreateFileOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().CreateFile(options).Return(&handlemap.Handle{}, os.ErrPermission)

	err, _ := cfuseFS.Create(path, 0, uint32(mode))
	suite.assert.Equal(-fuse.EACCES, err)
}

func testRenameFileFastPathSuccess(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	src := "src"
	dst := "dst"
	srcAttr := &internal.ObjAttr{Flags: internal.NewFileBitMap()}

	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: src}).Return(srcAttr, nil)
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: dst}).Times(0)
	suite.mock.EXPECT().RenameFile(gomock.AssignableToTypeOf(internal.RenameFileOptions{})).
		DoAndReturn(func(opts internal.RenameFileOptions) error {
			suite.assert.Equal(src, opts.Src)
			suite.assert.Equal(dst, opts.Dst)
			suite.assert.Equal(srcAttr, opts.SrcAttr)
			suite.assert.Nil(opts.DstAttr)
			return nil
		})

	err := cfuseFS.Rename("/"+src, "/"+dst)
	suite.assert.Equal(0, err)
}

func testRenameFileFastPathDstDirOnError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	src := "src"
	dst := "dst"
	srcAttr := &internal.ObjAttr{Flags: internal.NewFileBitMap()}
	dstAttr := &internal.ObjAttr{Flags: internal.NewDirBitMap()}

	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: src}).Return(srcAttr, nil)
	suite.mock.EXPECT().RenameFile(gomock.Any()).Return(errors.New("rename failed"))
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: dst}).Return(dstAttr, nil)

	err := cfuseFS.Rename("/"+src, "/"+dst)
	suite.assert.Equal(-fuse.EISDIR, err)
}

func testRenameFileFastPathError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	src := "src"
	dst := "dst"
	srcAttr := &internal.ObjAttr{Flags: internal.NewFileBitMap()}
	dstAttr := &internal.ObjAttr{Flags: internal.NewFileBitMap()}

	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: src}).Return(srcAttr, nil)
	suite.mock.EXPECT().RenameFile(gomock.Any()).Return(errors.New("rename failed"))
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: dst}).Return(dstAttr, nil)

	err := cfuseFS.Rename("/"+src, "/"+dst)
	suite.assert.Equal(-fuse.EIO, err)
}

func testRenameDirNotEmpty(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	src := "src"
	dst := "dst"
	srcAttr := &internal.ObjAttr{Flags: internal.NewDirBitMap()}
	dstAttr := &internal.ObjAttr{Flags: internal.NewDirBitMap()}

	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: src}).Return(srcAttr, nil)
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: dst}).Return(dstAttr, nil)
	suite.mock.EXPECT().IsDirEmpty(internal.IsDirEmptyOptions{Name: dst}).Return(false)

	err := cfuseFS.Rename("/"+src, "/"+dst)
	suite.assert.Equal(-fuse.ENOTEMPTY, err)
}

func testRenameDirDstNotDir(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	src := "src"
	dst := "dst"
	srcAttr := &internal.ObjAttr{Flags: internal.NewDirBitMap()}
	dstAttr := &internal.ObjAttr{Flags: internal.NewFileBitMap()}

	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: src}).Return(srcAttr, nil)
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: dst}).Return(dstAttr, nil)

	err := cfuseFS.Rename("/"+src, "/"+dst)
	suite.assert.Equal(-fuse.ENOTDIR, err)
}

func testRenameDirPermission(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	src := "src"
	dst := "dst"
	srcAttr := &internal.ObjAttr{Flags: internal.NewDirBitMap()}

	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: src}).Return(srcAttr, nil)
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: dst}).Return(nil, syscall.ENOENT)
	suite.mock.EXPECT().
		RenameDir(internal.RenameDirOptions{Src: src, Dst: dst}).
		Return(os.ErrPermission)

	err := cfuseFS.Rename("/"+src, "/"+dst)
	suite.assert.Equal(-fuse.EACCES, err)
}

func testRenameDirDstGetAttrPermission(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	src := "src"
	dst := "dst"
	srcAttr := &internal.ObjAttr{Flags: internal.NewDirBitMap()}

	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: src}).Return(srcAttr, nil)
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: dst}).Return(nil, syscall.EPERM)
	suite.mock.EXPECT().RenameDir(gomock.Any()).Times(0)

	err := cfuseFS.Rename("/"+src, "/"+dst)
	suite.assert.Equal(-fuse.EACCES, err)
}

func testRenameSrcGetAttrPermission(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	src := "src"
	dst := "dst"

	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: src}).Return(nil, os.ErrPermission)
	suite.mock.EXPECT().RenameFile(gomock.Any()).Times(0)
	suite.mock.EXPECT().RenameDir(gomock.Any()).Times(0)

	err := cfuseFS.Rename("/"+src, "/"+dst)
	suite.assert.Equal(-fuse.EACCES, err)
}

func testRenameSrcGetAttrError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	src := "src"
	dst := "dst"

	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: src}).Return(nil, errors.New("boom"))
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: dst}).Times(0)
	suite.mock.EXPECT().RenameDir(gomock.Any()).Times(0)
	suite.mock.EXPECT().RenameFile(gomock.Any()).Times(0)

	err := cfuseFS.Rename("/"+src, "/"+dst)
	suite.assert.Equal(-fuse.EIO, err)
}

func testOpen(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(fuseFS.filePermission)
	flags := fuse.O_RDWR & 0xffffffff
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err, _ := cfuseFS.Open(path, flags)
	suite.assert.Equal(0, err)
}

// fuse2 does not have writeback caching, so append flag is passed unchanged
func testOpenAppendFlagDefault(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	name := "path"
	path := "/" + name
	mode := fs.FileMode(fuseFS.filePermission)
	flags := fuse.O_RDWR | fuse.O_APPEND&0xffffffff
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err, _ := cfuseFS.Open(path, flags)
	suite.assert.Equal(0, err)

	flags = fuse.O_WRONLY | fuse.O_APPEND&0xffffffff
	options = internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err, _ = cfuseFS.Open(path, flags)
	suite.assert.Equal(0, err)
}

func testOpenAppendFlagDisableWritebackCache(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "libfuse:\n  disable-writeback-cache: true\n"
	suite.setupTestHelper(
		config,
	) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.True(suite.libfuse.disableWritebackCache)

	name := "path"
	path := "/" + name
	mode := fs.FileMode(fuseFS.filePermission)
	flags := fuse.O_RDWR | fuse.O_APPEND&0xffffffff
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err, _ := cfuseFS.Open(path, flags)
	suite.assert.Equal(0, err)

	flags = fuse.O_WRONLY | fuse.O_APPEND&0xffffffff
	options = internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err, _ = cfuseFS.Open(path, flags)
	suite.assert.Equal(0, err)
}

func testOpenAppendFlagIgnoreAppendFlag(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "libfuse:\n  ignore-open-flags: true\n"
	suite.setupTestHelper(
		config,
	) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.True(suite.libfuse.ignoreOpenFlags)

	name := "path"
	path := "/" + name
	mode := fs.FileMode(fuseFS.filePermission)
	flags := fuse.O_RDWR | fuse.O_APPEND&0xffffffff
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err, _ := cfuseFS.Open(path, flags)
	suite.assert.Equal(0, err)

	flags = fuse.O_WRONLY | fuse.O_APPEND&0xffffffff
	options = internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err, _ = cfuseFS.Open(path, flags)
	suite.assert.Equal(0, err)

	flags = fuse.O_WRONLY & 0xffffffff
	options = internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err, _ = cfuseFS.Open(path, flags)
	suite.assert.Equal(0, err)
}

func testOpenNotExists(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(fuseFS.filePermission)
	flags := fuse.O_RDWR & 0xffffffff
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, syscall.ENOENT)

	err, _ := cfuseFS.Open(path, flags)
	suite.assert.Equal(-fuse.ENOENT, err)
}

func testOpenError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(fuseFS.filePermission)
	flags := fuse.O_RDWR & 0xffffffff
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().
		OpenFile(options).
		Return(&handlemap.Handle{}, errors.New("failed to open a file"))

	err, _ := cfuseFS.Open(path, flags)
	suite.assert.Equal(-fuse.EIO, err)
}

func testOpenPermissionError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(fuseFS.filePermission)
	flags := fuse.O_RDWR & 0xffffffff
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, os.ErrPermission)

	err, _ := cfuseFS.Open(path, flags)
	suite.assert.Equal(-fuse.EACCES, err)
}

func testTruncate(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	size := int64(1024)
	options := internal.TruncateFileOptions{Name: name, OldSize: -1, NewSize: size}
	suite.mock.EXPECT().TruncateFile(options).Return(nil)

	err := cfuseFS.Truncate(path, size, 0)
	suite.assert.Equal(0, err)
}

func testTruncateError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	size := int64(1024)
	options := internal.TruncateFileOptions{Name: name, OldSize: -1, NewSize: size}
	suite.mock.EXPECT().TruncateFile(options).Return(errors.New("failed to truncate file"))

	err := cfuseFS.Truncate(path, size, 0)
	suite.assert.Equal(-fuse.EIO, err)
}

func testTruncatePermission(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	size := int64(1024)
	options := internal.TruncateFileOptions{Name: name, OldSize: -1, NewSize: size}
	suite.mock.EXPECT().TruncateFile(options).Return(os.ErrPermission)

	err := cfuseFS.Truncate(path, size, 0)
	suite.assert.Equal(-fuse.EACCES, err)
}

func testFTruncate(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	size := int64(1024)

	handle := handlemap.NewHandle(name)
	fh := handlemap.Add(handle)

	options := internal.TruncateFileOptions{Handle: handle, Name: name, OldSize: -1, NewSize: size}
	suite.mock.EXPECT().TruncateFile(options).Return(nil)

	err := cfuseFS.Truncate(path, size, uint64(fh))
	suite.assert.Equal(0, err)
}

func testFTruncateError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	size := int64(1024)

	handle := handlemap.NewHandle(name)
	fh := handlemap.Add(handle)

	options := internal.TruncateFileOptions{Handle: handle, Name: name, OldSize: -1, NewSize: size}
	suite.mock.EXPECT().TruncateFile(options).Return(errors.New("failed to truncate file"))

	err := cfuseFS.Truncate(path, size, uint64(fh))
	suite.assert.Equal(-fuse.EIO, err)
}

func testUnlink(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	options := internal.DeleteFileOptions{Name: name}
	suite.mock.EXPECT().DeleteFile(options).Return(nil)

	err := cfuseFS.Unlink(path)
	suite.assert.Equal(0, err)
}

func testUnlinkNotExists(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	options := internal.DeleteFileOptions{Name: name}
	suite.mock.EXPECT().DeleteFile(options).Return(syscall.ENOENT)

	err := cfuseFS.Unlink(path)
	suite.assert.Equal(-fuse.ENOENT, err)
}

func testUnlinkPermission(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	options := internal.DeleteFileOptions{Name: name}
	suite.mock.EXPECT().DeleteFile(options).Return(os.ErrPermission)

	err := cfuseFS.Unlink(path)
	suite.assert.Equal(-fuse.EACCES, err)
}

func testUnlinkError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	options := internal.DeleteFileOptions{Name: name}
	suite.mock.EXPECT().DeleteFile(options).Return(errors.New("failed to delete file"))

	err := cfuseFS.Unlink(path)
	suite.assert.Equal(-fuse.EIO, err)
}

// Rename

func testSymlink(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	target := "target"
	path := "/" + name
	t := target
	options := internal.CreateLinkOptions{Name: name, Target: target}
	suite.mock.EXPECT().CreateLink(options).Return(nil)

	err := cfuseFS.Symlink(t, path)
	suite.assert.Equal(0, err)
}

func testSymlinkError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	target := "target"
	path := "/" + name
	t := target
	options := internal.CreateLinkOptions{Name: name, Target: target}
	suite.mock.EXPECT().CreateLink(options).Return(errors.New("failed to create link"))

	err := cfuseFS.Symlink(t, path)
	suite.assert.Equal(-fuse.EIO, err)
}

func testSymlinkPermission(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	target := "target"
	path := "/" + name
	options := internal.CreateLinkOptions{Name: name, Target: target}
	suite.mock.EXPECT().CreateLink(options).Return(os.ErrPermission)

	err := cfuseFS.Symlink(target, path)
	suite.assert.Equal(-fuse.EACCES, err)
}

func testReadLink(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	linkSize := int64(16)
	options := internal.ReadLinkOptions{Name: name, Size: linkSize}
	getAttrOpt := internal.GetAttrOptions{Name: name}
	gomock.InOrder(
		suite.mock.EXPECT().GetAttr(getAttrOpt).Return(&internal.ObjAttr{Size: linkSize}, nil),
		suite.mock.EXPECT().ReadLink(options).Return("target", nil),
	)

	err, target := cfuseFS.Readlink(path)
	suite.assert.Equal(0, err)
	suite.assert.Equal("target", target)
}

func testReadLinkNotExists(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	linkSize := int64(8)
	options := internal.ReadLinkOptions{Name: name, Size: linkSize}
	getAttrOpt := internal.GetAttrOptions{Name: name}
	gomock.InOrder(
		suite.mock.EXPECT().GetAttr(getAttrOpt).Return(&internal.ObjAttr{Size: linkSize}, nil),
		suite.mock.EXPECT().ReadLink(options).Return("", syscall.ENOENT),
	)

	err, target := cfuseFS.Readlink(path)
	suite.assert.Equal(-fuse.ENOENT, err)
	suite.assert.Empty(target)
}

func testReadLinkError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	linkSize := int64(32)
	options := internal.ReadLinkOptions{Name: name, Size: linkSize}
	suite.mock.EXPECT().ReadLink(options).Return("", errors.New("failed to read link"))
	getAttrOpt := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().GetAttr(getAttrOpt).Return(&internal.ObjAttr{Size: linkSize}, nil)

	err, target := cfuseFS.Readlink(path)
	suite.assert.Equal(-fuse.EIO, err)
	suite.assert.Empty(target)
}

func testReadLinkPermission(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	linkSize := int64(64)
	options := internal.ReadLinkOptions{Name: name, Size: linkSize}
	suite.mock.EXPECT().ReadLink(options).Return("", os.ErrPermission)
	getAttrOpt := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().GetAttr(getAttrOpt).Return(&internal.ObjAttr{Size: linkSize}, nil)

	err, target := cfuseFS.Readlink(path)
	suite.assert.Equal(-fuse.EACCES, err)
	suite.assert.Empty(target)
}

func testFsync(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(fuseFS.filePermission)
	flags := fuse.O_RDWR & 0xffffffff
	handle := &handlemap.Handle{}
	openOptions := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(openOptions).Return(handle, nil)
	_, fh := cfuseFS.Open(path, flags)
	suite.assert.NotZero(fh)

	// Need to convert the ID to the correct filehandle for mocking
	handle.ID = (handlemap.HandleID)(fh)

	options := internal.SyncFileOptions{Handle: handle}
	suite.mock.EXPECT().SyncFile(options).Return(nil)

	err := cfuseFS.Fsync(path, false, fh)
	suite.assert.Equal(0, err)
}

func testFsyncHandleError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	fh := uint64(0)

	err := cfuseFS.Fsync(path, false, fh)
	suite.assert.Equal(-fuse.EIO, err)
}

func testFsyncError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(fuseFS.filePermission)
	flags := fuse.O_RDWR & 0xffffffff
	handle := &handlemap.Handle{}

	openOptions := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(openOptions).Return(handle, nil)
	_, fh := cfuseFS.Open(path, flags)
	suite.assert.NotZero(fh)

	// Need to convert the ID to the correct filehandle for mocking
	handle.ID = (handlemap.HandleID)(fh)

	options := internal.SyncFileOptions{Handle: handle}
	suite.mock.EXPECT().SyncFile(options).Return(errors.New("failed to sync file"))

	err := cfuseFS.Fsync(path, false, fh)
	suite.assert.Equal(-fuse.EIO, err)
}

func testFsyncPermission(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(fuseFS.filePermission)
	flags := fuse.O_RDWR & 0xffffffff
	handle := &handlemap.Handle{}

	openOptions := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(openOptions).Return(handle, nil)
	_, fh := cfuseFS.Open(path, flags)
	suite.assert.NotZero(fh)

	// Need to convert the ID to the correct filehandle for mocking
	handle.ID = (handlemap.HandleID)(fh)

	options := internal.SyncFileOptions{Handle: handle}
	suite.mock.EXPECT().SyncFile(options).Return(os.ErrPermission)

	err := cfuseFS.Fsync(path, false, fh)
	suite.assert.Equal(-fuse.EACCES, err)
}

func testFsyncDir(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	options := internal.SyncDirOptions{Name: name}
	suite.mock.EXPECT().SyncDir(options).Return(nil)

	err := cfuseFS.Fsyncdir(path, false, 0)
	suite.assert.Equal(0, err)
}

func testFsyncDirError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	options := internal.SyncDirOptions{Name: name}
	suite.mock.EXPECT().SyncDir(options).Return(errors.New("failed to sync dir"))

	err := cfuseFS.Fsyncdir(path, false, 0)
	suite.assert.Equal(-fuse.EIO, err)
}

func testFsyncDirPermission(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	options := internal.SyncDirOptions{Name: name}
	suite.mock.EXPECT().SyncDir(options).Return(os.ErrPermission)

	err := cfuseFS.Fsyncdir(path, false, 0)
	suite.assert.Equal(-fuse.EACCES, err)
}

func testChmod(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(0775)
	options := internal.ChmodOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().Chmod(options).Return(nil)

	err := cfuseFS.Chmod(path, 0775)
	suite.assert.Equal(0, err)
}

func testChmodNotExists(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(0775)
	options := internal.ChmodOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().Chmod(options).Return(syscall.ENOENT)

	err := cfuseFS.Chmod(path, 0775)
	suite.assert.Equal(-fuse.ENOENT, err)
}

func testChmodError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(0775)
	options := internal.ChmodOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().Chmod(options).Return(errors.New("failed to chmod"))

	err := cfuseFS.Chmod(path, 0775)
	suite.assert.Equal(-fuse.EIO, err)
}

func testChown(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	group := uint32(5)
	owner := uint32(4)

	err := cfuseFS.Chown(path, owner, group)
	suite.assert.Equal(0, err)
}

func testUtimens(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name

	err := cfuseFS.Utimens(path, nil)
	suite.assert.Equal(0, err)
}
