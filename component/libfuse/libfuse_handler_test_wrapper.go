//go:build linux && !fuse2

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

package libfuse

// #cgo CFLAGS: -DFUSE_USE_VERSION=39 -D_FILE_OFFSET_BITS=64
// #cgo LDFLAGS: -lfuse3 -ldl
// #include "libfuse_wrapper.h"
import "C"
import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"syscall"
	"unsafe"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/handlemap"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type libfuseTestSuite struct {
	suite.Suite
	assert   *assert.Assertions
	libfuse  *Libfuse
	mockCtrl *gomock.Controller
	mock     *internal.MockComponent
}

type fileHandle struct {
	fd  uint64
	obj uint64
}

var emptyConfig = ""
var defaultSize = int64(0)
var defaultMode = 0777

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
		panic("Unable to set silent logger as default.")
	}
	suite.setupTestHelper(emptyConfig)
}

func (suite *libfuseTestSuite) setupTestHelper(config string) {
	suite.assert = assert.New(suite.T())

	suite.mockCtrl = gomock.NewController(suite.T())
	suite.mock = internal.NewMockComponent(suite.mockCtrl)
	suite.libfuse = newTestLibfuse(suite.mock, config)
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
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(0775)
	options := internal.CreateDirOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().CreateDir(options).Return(nil)

	err := libfuse_mkdir(path, 0775)
	suite.assert.Equal(C.int(0), err)
}

func testMkDirError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(0775)
	options := internal.CreateDirOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().CreateDir(options).Return(errors.New("failed to create directory"))

	err := libfuse_mkdir(path, 0775)
	suite.assert.Equal(C.int(-C.EIO), err)
}

// testMkDirErrorAttrExist only runs on Windows to test the case that the directory already exists
// Sine fuse3 doesn't work on windows, do nothing
func testMkDirErrorAttrExist(suite *libfuseTestSuite) {
}

// TODO: ReadDir test

func testRmDir(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	isDirEmptyOptions := internal.IsDirEmptyOptions{Name: name}
	suite.mock.EXPECT().IsDirEmpty(isDirEmptyOptions).Return(true)
	deleteDirOptions := internal.DeleteDirOptions{Name: name}
	suite.mock.EXPECT().DeleteDir(deleteDirOptions).Return(nil)

	err := libfuse_rmdir(path)
	suite.assert.Equal(C.int(0), err)
}

func testRmDirNotEmpty(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	isDirEmptyOptions := internal.IsDirEmptyOptions{Name: name}
	suite.mock.EXPECT().IsDirEmpty(isDirEmptyOptions).Return(false)

	err := libfuse_rmdir(path)
	suite.assert.Equal(C.int(-C.ENOTEMPTY), err)
}

func testRmDirError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	isDirEmptyOptions := internal.IsDirEmptyOptions{Name: name}
	suite.mock.EXPECT().IsDirEmpty(isDirEmptyOptions).Return(true)
	deleteDirOptions := internal.DeleteDirOptions{Name: name}
	suite.mock.EXPECT().DeleteDir(deleteDirOptions).Return(errors.New("failed to delete directory"))

	err := libfuse_rmdir(path)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testCreate(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(0775)
	info := &C.fuse_file_info_t{}
	options := internal.CreateFileOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().CreateFile(options).Return(&handlemap.Handle{}, nil)

	err := libfuse_create(path, 0775, info)
	suite.assert.Equal(C.int(0), err)
	option := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().GetAttr(option).Return(&internal.ObjAttr{}, nil)
	stbuf := &C.stat_t{}
	err = libfuse_getattr(path, stbuf, &C.fuse_file_info_t{})
	suite.assert.Equal(C.int(0), err)
	suite.assert.Equal(stbuf.st_mtim.tv_nsec, C.long(0))
	suite.assert.NotEqual(stbuf.st_mtim.tv_sec, C.long(0))
}

func testCreateError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(0775)
	info := &C.fuse_file_info_t{}
	options := internal.CreateFileOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().CreateFile(options).Return(&handlemap.Handle{}, errors.New("failed to create file"))

	err := libfuse_create(path, 0775, info)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testOpen(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(fuseFS.filePermission)
	flags := C.O_RDWR & 0xffffffff
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err := libfuse_open(path, info)
	suite.assert.Equal(C.int(0), err)
}

func testOpenSyncDirectFlag(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(fuseFS.filePermission)
	flags := C.O_RDWR & 0xffffffff
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR | C.O_SYNC | C.__O_DIRECT
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err := libfuse_open(path, info)
	suite.assert.Equal(C.int(0), err)
	suite.assert.Equal(C.int(0), info.flags&C.O_SYNC)
	suite.assert.Equal(C.int(0), info.flags&C.__O_DIRECT)
}

// WriteBack caching and ignore-open-flags enabled by default
func testOpenAppendFlagDefault(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	suite.libfuse.ignoreOpenFlags = false

	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR | C.O_APPEND

	err := libfuse_open(path, info)
	suite.assert.Equal(C.int(-C.EINVAL), err)

	info.flags = C.O_WRONLY | C.O_APPEND

	err = libfuse_open(path, info)
	suite.assert.Equal(C.int(-C.EINVAL), err)
}

func testOpenAppendFlagDisableWritebackCache(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "libfuse:\n  disable-writeback-cache: true\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.True(suite.libfuse.disableWritebackCache)

	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(fuseFS.filePermission)
	flags := C.O_RDWR | C.O_APPEND&0xffffffff
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR | C.O_APPEND
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err := libfuse_open(path, info)
	suite.assert.Equal(C.int(0), err)

	flags = C.O_WRONLY | C.O_APPEND&0xffffffff
	info = &C.fuse_file_info_t{}
	info.flags = C.O_WRONLY | C.O_APPEND
	options = internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err = libfuse_open(path, info)
	suite.assert.Equal(C.int(0), err)
}

func testOpenAppendFlagIgnoreAppendFlag(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "libfuse:\n  ignore-open-flags: true\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.True(suite.libfuse.ignoreOpenFlags)

	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(fuseFS.filePermission)
	flags := C.O_RDWR & 0xffffffff
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR | C.O_APPEND
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err := libfuse_open(path, info)
	suite.assert.Equal(C.int(0), err)
	suite.assert.Equal(C.int(0), info.flags&C.O_APPEND)

	flags = C.O_RDWR & 0xffffffff
	info = &C.fuse_file_info_t{}
	info.flags = C.O_WRONLY | C.O_APPEND
	options = internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err = libfuse_open(path, info)
	suite.assert.Equal(C.int(0), err)
	suite.assert.Equal(C.int(0), info.flags&C.O_APPEND)

	flags = C.O_RDWR & 0xffffffff
	info = &C.fuse_file_info_t{}
	info.flags = C.O_WRONLY
	options = internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err = libfuse_open(path, info)
	suite.assert.Equal(C.int(0), err)
}

func testOpenNotExists(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(fuseFS.filePermission)
	flags := C.O_RDWR & 0xffffffff
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, syscall.ENOENT)

	err := libfuse_open(path, info)
	suite.assert.Equal(C.int(-C.ENOENT), err)
}

func testOpenError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(fuseFS.filePermission)
	flags := C.O_RDWR & 0xffffffff
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, errors.New("failed to open a file"))

	err := libfuse_open(path, info)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testTruncate(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	size := int64(1024)
	options := internal.TruncateFileOptions{Name: name, Size: size}
	suite.mock.EXPECT().TruncateFile(options).Return(nil)

	err := libfuse_truncate(path, C.long(size), nil)
	suite.assert.Equal(C.int(0), err)
}

func testTruncateError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	size := int64(1024)
	options := internal.TruncateFileOptions{Name: name, Size: size}
	suite.mock.EXPECT().TruncateFile(options).Return(errors.New("failed to truncate file"))

	err := libfuse_truncate(path, C.long(size), nil)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testUnlink(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	options := internal.DeleteFileOptions{Name: name}
	suite.mock.EXPECT().DeleteFile(options).Return(nil)

	err := libfuse_unlink(path)
	suite.assert.Equal(C.int(0), err)
}

func testUnlinkNotExists(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	options := internal.DeleteFileOptions{Name: name}
	suite.mock.EXPECT().DeleteFile(options).Return(syscall.ENOENT)

	err := libfuse_unlink(path)
	suite.assert.Equal(C.int(-C.ENOENT), err)
}

func testUnlinkError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	options := internal.DeleteFileOptions{Name: name}
	suite.mock.EXPECT().DeleteFile(options).Return(errors.New("failed to delete file"))

	err := libfuse_unlink(path)
	suite.assert.Equal(C.int(-C.EIO), err)
}

// Rename

func testSymlink(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	target := "target"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	t := C.CString(target)
	defer C.free(unsafe.Pointer(t))
	options := internal.CreateLinkOptions{Name: name, Target: target}
	suite.mock.EXPECT().CreateLink(options).Return(nil)

	err := libfuse_symlink(t, path)
	suite.assert.Equal(C.int(0), err)
}

func testSymlinkError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	target := "target"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	t := C.CString(target)
	defer C.free(unsafe.Pointer(t))
	options := internal.CreateLinkOptions{Name: name, Target: target}
	suite.mock.EXPECT().CreateLink(options).Return(errors.New("failed to create link"))

	err := libfuse_symlink(t, path)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testReadLink(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	options := internal.ReadLinkOptions{Name: name}
	suite.mock.EXPECT().ReadLink(options).Return("target", nil)

	// https://stackoverflow.com/questions/41953619/how-to-initialise-empty-c-cstring-in-cgo
	buf := C.CString("")
	err := libfuse_readlink(path, buf, 7)
	suite.assert.Equal(C.int(0), err)
	suite.assert.Equal("target", C.GoString(buf))
}

func testReadLinkNotExists(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	options := internal.ReadLinkOptions{Name: name}
	suite.mock.EXPECT().ReadLink(options).Return("", syscall.ENOENT)

	buf := C.CString("")
	err := libfuse_readlink(path, buf, 7)
	suite.assert.Equal(C.int(-C.ENOENT), err)
	suite.assert.NotEqual("target", C.GoString(buf))
}

func testReadLinkError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	options := internal.ReadLinkOptions{Name: name}
	suite.mock.EXPECT().ReadLink(options).Return("", errors.New("failed to read link"))

	buf := C.CString("")
	err := libfuse_readlink(path, buf, 7)
	suite.assert.Equal(C.int(-C.EIO), err)
	suite.assert.NotEqual("target", C.GoString(buf))
}

func testFsync(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(fuseFS.filePermission)
	flags := C.O_RDWR & 0xffffffff
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR
	handle := &handlemap.Handle{}
	openOptions := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(openOptions).Return(handle, nil)
	libfuse_open(path, info)
	suite.assert.NotEqual(C.ulong(0), info.fh)

	// libfuse component will return back handle in form of an integer value
	// that needs to be converted back to a pointer to a handle object
	fobj := (*fileHandle)(unsafe.Pointer(uintptr(info.fh)))
	handle = (*handlemap.Handle)(unsafe.Pointer(uintptr(fobj.obj)))

	options := internal.SyncFileOptions{Handle: handle}
	suite.mock.EXPECT().SyncFile(options).Return(nil)

	err := libfuse_fsync(path, C.int(0), info)
	suite.assert.Equal(C.int(0), err)
}

func testFsyncHandleError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR

	err := libfuse_fsync(path, C.int(0), info)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testFsyncError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(fuseFS.filePermission)
	flags := C.O_RDWR & 0xffffffff
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR
	handle := &handlemap.Handle{}
	openOptions := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(openOptions).Return(handle, nil)
	libfuse_open(path, info)
	suite.assert.NotEqual(C.ulong(0), info.fh)

	// libfuse component will return back handle in form of an integer value
	// that needs to be converted back to a pointer to a handle object
	fobj := (*fileHandle)(unsafe.Pointer(uintptr(info.fh)))
	handle = (*handlemap.Handle)(unsafe.Pointer(uintptr(fobj.obj)))

	options := internal.SyncFileOptions{Handle: handle}
	suite.mock.EXPECT().SyncFile(options).Return(errors.New("failed to sync file"))

	err := libfuse_fsync(path, C.int(0), info)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testFsyncDir(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	options := internal.SyncDirOptions{Name: name}
	suite.mock.EXPECT().SyncDir(options).Return(nil)

	err := libfuse_fsyncdir(path, C.int(0), nil)
	suite.assert.Equal(C.int(0), err)
}

func testFsyncDirError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	options := internal.SyncDirOptions{Name: name}
	suite.mock.EXPECT().SyncDir(options).Return(errors.New("failed to sync dir"))

	err := libfuse_fsyncdir(path, C.int(0), nil)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testChmod(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(0775)
	options := internal.ChmodOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().Chmod(options).Return(nil)

	err := libfuse_chmod(path, 0775, nil)
	suite.assert.Equal(C.int(0), err)
}

func testChmodNotExists(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(0775)
	options := internal.ChmodOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().Chmod(options).Return(syscall.ENOENT)

	err := libfuse_chmod(path, 0775, nil)
	suite.assert.Equal(C.int(-C.ENOENT), err)
}

func testStatFs(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	path := C.CString("/")
	defer C.free(unsafe.Pointer(path))
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{Frsize: 1,
		Blocks: 2, Bavail: 3, Bfree: 4}, true, nil)
	buf := &C.statvfs_t{}
	libfuse_statfs(path, buf)

	suite.assert.Equal(1, int(buf.f_frsize))
	suite.assert.Equal(2, int(buf.f_blocks))
	suite.assert.Equal(3, int(buf.f_bavail))
	suite.assert.Equal(4, int(buf.f_bfree))
}

func testStatFsNotPopulated(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	path := C.CString("/")
	defer C.free(unsafe.Pointer(path))
	suite.mock.EXPECT().StatFs().Return(nil, false, nil)
	buf := &C.statvfs_t{}
	libfuse_statfs(path, buf)

	// By default these are all 0, so they should be populated by the system
	// and thus each larger than 0
	suite.assert.Positive(buf.f_frsize)
	suite.assert.Positive(buf.f_blocks)
	suite.assert.Positive(buf.f_bavail)
	suite.assert.Positive(buf.f_bfree)
	suite.assert.Positive(buf.f_bsize)
	suite.assert.Positive(buf.f_files)
	suite.assert.Positive(buf.f_ffree)
	suite.assert.Positive(buf.f_namemax)
}

func testStatFsError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	path := C.CString("/")
	suite.mock.EXPECT().StatFs().Return(nil, false, errors.New("Error"))
	buf := &C.statvfs_t{}
	err := libfuse_statfs(path, buf)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testChmodError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(0775)
	options := internal.ChmodOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().Chmod(options).Return(errors.New("failed to chmod"))

	err := libfuse_chmod(path, 0775, nil)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testChown(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	group := C.uint(5)
	owner := C.uint(4)

	err := libfuse_chown(path, owner, group, nil)
	suite.assert.Equal(C.int(0), err)
}

func testUtimens(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))

	err := libfuse_utimens(path, nil, nil)
	suite.assert.Equal(C.int(0), err)
}
