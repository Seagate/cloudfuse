package libfuse

/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

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

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"syscall"
	"time"

	"lyvecloudfuse/common"
	"lyvecloudfuse/common/log"
	"lyvecloudfuse/internal"
	"lyvecloudfuse/internal/handlemap"
	"lyvecloudfuse/internal/stats_manager"

	"github.com/winfsp/cgofuse/fuse"
)

type cgofuseFS struct {
	fuse.FileSystemBase
	uid uint32
	gid uint32
}

// Note: libfuse prepends "/" to the path.
// TODO: Not sure if this is needed for cgofuse, will need to check
// trimFusePath trims the first character from the path provided by libfuse
func trimFusePath(path string) string {
	if path == "" {
		return ""
	}

	if path[0] == '/' {
		return path[1:]
	}
	return path
}

func (cf *Libfuse) fillStat(attr *internal.ObjAttr, stbuf *fuse.Stat_t) {
	stbuf.Uid = cf.ownerUID
	stbuf.Gid = cf.ownerGID
	stbuf.Nlink = 1
	stbuf.Size = attr.Size

	// Populate mode
	// Backing storage implementation has support for mode.
	if !attr.IsModeDefault() {
		stbuf.Mode = uint32(attr.Mode) & 0xffffffff
	} else {
		if attr.IsDir() {
			stbuf.Mode = uint32(cf.dirPermission) & 0xffffffff
		} else {
			stbuf.Mode = uint32(cf.filePermission) & 0xffffffff
		}
	}

	if attr.IsDir() {
		stbuf.Nlink = 2
		stbuf.Size = 4096
		stbuf.Mode |= fuse.S_IFDIR
	} else if attr.IsSymlink() {
		stbuf.Mode |= fuse.S_IFLNK
	} else {
		stbuf.Mode |= fuse.S_IFREG
	}

	stbuf.Atim.Sec = attr.Atime.Unix()
	stbuf.Atim.Nsec = attr.Atime.UnixNano()

	stbuf.Ctim.Sec = attr.Ctime.Unix()
	stbuf.Ctim.Nsec = attr.Ctime.UnixNano()

	stbuf.Mtim.Sec = attr.Mtime.Unix()
	stbuf.Mtim.Nsec = attr.Mtime.UnixNano()
}

func (lf *Libfuse) initFuse() error {
	log.Trace("Libfuse::initFuse : Initializing FUSE")

	cf := NewcgofuseFS()
	cf.uid = lf.ownerUID
	cf.gid = lf.ownerGID

	lf.host = fuse.NewFileSystemHost(cf)

	opts := []string{}
	opts = append(opts, "-o", fmt.Sprintf("uid=%d", lf.ownerUID))
	opts = append(opts, "-o", fmt.Sprintf("gid=%d", lf.ownerGID))
	opts = append(opts, "-o", fmt.Sprintf("attr_timeout=%d", lf.attributeExpiration))
	opts = append(opts, "-o", fmt.Sprintf("entry_timeout=%d", lf.entryExpiration))
	opts = append(opts, "-o", fmt.Sprintf("negative_timeout=%d", lf.negativeTimeout))

	// While reading a file let kernel do readahed for better perf
	opts = append(opts, "-o", fmt.Sprintf("max_readahead=%d", 4*1024*1024))

	// Max background thread on the fuse layer for high parallelism
	opts = append(opts, "-o", fmt.Sprintf("max_background=%d", 128))

	if lf.allowOther {
		opts = append(opts, "-o", "allow_other")
	}
	if lf.readOnly {
		opts = append(opts, "-o", "ro")
	}
	if lf.nonEmptyMount {
		opts = append(opts, "-o", "nonempty")
	}
	if lf.traceEnable {
		opts = append(opts, "-d")
	}

	ret := lf.host.Mount(lf.mountPath, opts)
	if !ret {
		log.Err("Libfuse::initFuse : failed to mount fuse")
		return errors.New("failed to mount fuse")
	}

	return nil
}

func (lf *Libfuse) destroyFuse() error {
	log.Trace("Libfuse::destroyFuse : Destroying FUSE")
	lf.host.Unmount()
	return nil
}

func NewcgofuseFS() *cgofuseFS {
	cf := &cgofuseFS{}
	return cf
}

// The init function in blobfuse checks for different fuse connections which I beleive is
// already done at a lower level in cgofuse. We also need to set a readahead size but that
// is done when mounting like rclone did https://github.com/rclone/rclone/blob/555def2da7f425225b9f8657593733b5d71f901e/cmd/cmount/mount.go#L169
// We also need to set uid and gid which is also done when mounting.
//
// However, we won't mount in the init like blobfuse does since cgofuse has a separate mount
// function we need to call.
func (cf *cgofuseFS) Init() {
	log.Trace("Libfuse::Init : Initializing FUSE")
}

// Destory does nothing in blobfuse, so same here.
func (cf *cgofuseFS) Destroy() {
	log.Trace("Libfuse::Destroy : Destroy")
}

func (cf *cgofuseFS) Mkdir(path string, mode uint32) int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_mkdir : %s", name)

	// blobfuse uses a bitwise and trick to make sure mode is a uint32, we don't need that here
	err := fuseFS.NextComponent().CreateDir(internal.CreateDirOptions{Name: name, Mode: fs.FileMode(mode)})
	if err != nil {
		log.Err("Libfuse::libfuse_mkdir : Failed to create %s [%s]", name, err.Error())
		return -fuse.EIO
	}

	libfuseStatsCollector.PushEvents(createDir, name, map[string]interface{}{md: fs.FileMode(mode)})
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, createDir, (int64)(1))

	return 0
}

func (cf *cgofuseFS) Statfs(path string, stat *fuse.Statfs_t) int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_statfs : %s", name)

	attr, populated, err := fuseFS.NextComponent().StatFs()
	if err != nil {
		log.Err("Libfuse::libfuse_statfs: Failed to get stats %s [%s]", name, err.Error())
		return -fuse.EIO
	}

	// if populated then we need to overwrite root attributes
	if populated {
		stat.Bsize = uint64(attr.Bsize)
		stat.Frsize = uint64(attr.Frsize)
		stat.Blocks = attr.Blocks
		stat.Bavail = attr.Bavail
		stat.Bfree = attr.Bfree
		stat.Files = attr.Files
		stat.Ffree = attr.Ffree
		stat.Flag = uint64(attr.Flags)
		return 0
	}

	// TODO: Need to look into handling case where this is empty directory
	// given by just  "/"
	// blobfuse does this via the commented out code below

	// errno := 0
	// res := os.statvfs("/")
	// res = os.statvfs("/", stat)
	// if res == -1 {
	// 	return -errno
	// }

	return 0
}

func (cf *cgofuseFS) Opendir(path string) (int, uint64) {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	if name != "" {
		name = name + "/"
	}

	log.Trace("Libfuse::Opendir : %s", name)

	handle := handlemap.NewHandle(name)

	// For each handle created using opendir we create
	// this structure here to hold current block of children to serve readdir
	handle.SetValue("cache", &dirChildCache{
		sIndex:   0,
		eIndex:   0,
		token:    "",
		length:   0,
		children: make([]*internal.ObjAttr, 0),
	})

	fh := handlemap.Add(handle)

	// This needs to return a uint64 representing the filehandle
	// We have to do a casting here to make the Go compiler happy but
	// handle.ID should already be a uint64
	return 0, uint64(fh)
}

func (cf *cgofuseFS) Releasedir(path string, fh uint64) int {
	// Get the filehandle
	handle, exists := handlemap.Load(handlemap.HandleID(fh))
	if !exists {
		log.Trace("Libfuse::Releasedir : Failed to release %s, handle: %d", path, fh)
		return -fuse.EBADF
	}

	log.Trace("Libfuse::Releasedir : %s, handle: %d", handle.Path, handle.ID)

	handle.Cleanup()
	handlemap.Delete(handle.ID)
	return 0
}

func (cf *cgofuseFS) Readdir(path string, fill func(name string, stat *fuse.Stat_t, ofst int64) bool,
	ofst int64, fh uint64) int {
	handle, exists := handlemap.Load(handlemap.HandleID(fh))
	if !exists {
		log.Trace("Libfuse::Readdir : Failed to read %s, handle: %d", path, fh)
		return -fuse.EBADF
	}

	val, found := handle.GetValue("cache")
	if !found {
		return fuse.EIO
	}

	off_64 := uint64(ofst)
	cacheInfo := val.(*dirChildCache)
	if off_64 == 0 ||
		(off_64 >= cacheInfo.eIndex && cacheInfo.token != "") {
		attrs, token, err := fuseFS.NextComponent().StreamDir(internal.StreamDirOptions{
			Name:   handle.Path,
			Offset: off_64,
			Token:  cacheInfo.token,
			Count:  common.MaxDirListCount,
		})

		if err != nil {
			log.Err("Libfuse::Readdir : Path %s, handle: %d, offset %d. Error in retrieval", handle.Path, handle.ID, off_64)
			if os.IsNotExist(err) {
				return fuse.ENOENT
			} else {
				return fuse.EIO
			}
		}

		if off_64 == 0 {
			attrs = append([]*internal.ObjAttr{{Flags: fuseFS.lsFlags, Name: "."}, {Flags: fuseFS.lsFlags, Name: ".."}}, attrs...)
		}

		cacheInfo.sIndex = off_64
		cacheInfo.eIndex = off_64 + uint64(len(attrs))
		cacheInfo.length = uint64(len(attrs))
		cacheInfo.token = token
		cacheInfo.children = cacheInfo.children[:0]
		cacheInfo.children = attrs
	}

	if off_64 >= cacheInfo.eIndex {
		// If offset is still beyond the end index limit then we are done iterating
		return 0
	}

	stbuf := fuse.Stat_t{}

	// Populate the stat by calling filler
	for segmentIdx := off_64 - cacheInfo.sIndex; segmentIdx < cacheInfo.length; segmentIdx++ {
		fuseFS.fillStat(cacheInfo.children[segmentIdx], &stbuf)

		name := cacheInfo.children[segmentIdx].Name
		fill(name, &stbuf, ofst)
	}

	return 0
}

// TODO: Currently not using filehandle
func (cf *cgofuseFS) Getattr(path string, stat *fuse.Stat_t, fh uint64) int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Getattr : %s", name)

	// Return the default configuration for the root
	if name == "" {
		stat.Mode = fuse.S_IFDIR | 0777
		stat.Uid = cf.uid
		stat.Gid = cf.gid
		stat.Nlink = 2
		stat.Size = 4096
		stat.Mtim.Sec = time.Now().Unix()
		stat.Mtim.Nsec = time.Now().UnixNano()
		stat.Atim = stat.Mtim
		stat.Ctim = stat.Mtim
		return 0
	}

	// TODO: How does this work if we trim the path?
	// Check if the file is meant to be ignored
	if ignore, found := ignoreFiles[name]; found && ignore {
		return -fuse.ENOENT
	}

	// Get attributes
	attr, err := fuseFS.NextComponent().GetAttr(internal.GetAttrOptions{Name: name})
	if err != nil {
		log.Err("Libfuse::Getattr : Failed to get attributes of %s [%s]", name, err.Error())
		return -fuse.ENOENT
	}

	// Populate stat
	fuseFS.fillStat(attr, stat)
	return 0
}

func (cf *cgofuseFS) Rmdir(path string) int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Rmdir : %s", name)

	empty := fuseFS.NextComponent().IsDirEmpty(internal.IsDirEmptyOptions{Name: name})
	if !empty {
		return -fuse.ENOTEMPTY
	}

	err := fuseFS.NextComponent().DeleteDir(internal.DeleteDirOptions{Name: name})
	if err != nil {
		log.Err("Libfuse::Rmdir : Failed to delete %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -fuse.ENOENT
		} else {
			return -fuse.EIO
		}
	}

	libfuseStatsCollector.PushEvents(deleteDir, name, nil)
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, deleteDir, (int64)(1))

	return 0
}

func (cf *cgofuseFS) Create(path string, flags int, mode uint32) (int, uint64) {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_create : %s", name)

	handle, err := fuseFS.NextComponent().CreateFile(internal.CreateFileOptions{Name: name, Mode: fs.FileMode(mode)})
	if err != nil {
		log.Err("Libfuse::libfuse_create : Failed to create %s [%s]", name, err.Error())
		if os.IsExist(err) {
			return -fuse.EEXIST, 0
		} else {
			return -fuse.EIO, 0
		}
	}

	fh := handlemap.Add(handle)
	// Don't think we need this
	// ret_val := C.allocate_native_file_object(C.ulong(handle.UnixFD), C.ulong(uintptr(unsafe.Pointer(handle))), 0)
	// if !handle.Cached() {
	// 	ret_val.fd = 0
	// }
	log.Trace("Libfuse::libfuse_create : %s, handle %d", name, fh)
	//fi.fh = C.ulong(uintptr(unsafe.Pointer(ret_val)))

	libfuseStatsCollector.PushEvents(createFile, name, map[string]interface{}{md: fs.FileMode(mode)})

	// increment open file handles count
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, openHandles, (int64)(1))

	return 0, uint64(fh)
}

func (cf *cgofuseFS) Open(path string, flags int) (int, uint64) {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse:: Open : %s", name)
	// TODO: Should this sit behind a user option? What if we change something to support these in the future?
	// Mask out SYNC and DIRECT flags since write operation will fail
	//
	// Don't think we need to handle this as these flags aren't available in cgofuse
	// if fi.flags&fuse.O_SYNC != 0 || fi.flags&fuse.__O_DIRECT != 0 {
	// 	log.Err("Libfuse::libfuse_open : Reset flags for open %s, fi.flags %X", name, fi.flags)
	// 	// Blobfuse2 does not support the SYNC or DIRECT flag. If a user application passes this flag on to blobfuse2
	// 	// and we open the file with this flag, subsequent write operations wlil fail with "Invalid argument" error.
	// 	// Mask them out here in the open call so that write works.
	// 	// Oracle RMAN is one such application that sends these flags during backup
	// 	fi.flags = fi.flags &^ C.O_SYNC
	// 	fi.flags = fi.flags &^ C.__O_DIRECT
	// }

	handle, err := fuseFS.NextComponent().OpenFile(
		internal.OpenFileOptions{
			Name:  name,
			Flags: flags,
			Mode:  fs.FileMode(fuseFS.filePermission),
		})

	if err != nil {
		log.Err("Libfuse::libfuse_open : Failed to open %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -fuse.ENOENT, 0
		} else {
			return -fuse.EIO, 0
		}
	}

	fh := handlemap.Add(handle)
	// Don't think we need this
	// ret_val := C.allocate_native_file_object(C.ulong(handle.UnixFD), C.ulong(uintptr(unsafe.Pointer(handle))), C.ulong(handle.Size))
	// if !handle.Cached() {
	// 	ret_val.fd = 0
	// }
	log.Trace("Libfuse::libfuse_open : %s, handle %d", name, fh)
	// fi.fh = C.ulong(uintptr(unsafe.Pointer(ret_val)))

	// increment open file handles count
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, openHandles, (int64)(1))

	return 0, uint64(fh)
}

func (cf *cgofuseFS) Read(path string, buff []byte, ofst int64, fh uint64) int {
	// Get the filehandle
	handle, exists := handlemap.Load(handlemap.HandleID(fh))
	if !exists {
		log.Trace("Libfuse::Read : error getting handle for path %s, handle: %d", path, fh)
		return -fuse.EBADF
	}

	offset := uint64(ofst)

	var err error
	var bytesRead int

	if handle.Cached() {
		bytesRead, err = syscall.Pread(handle.FD(), buff, int64(offset))
		//bytesRead, err = handle.FObj.ReadAt(buff, int64(offset))
	} else {
		bytesRead, err = fuseFS.NextComponent().ReadInBuffer(
			internal.ReadInBufferOptions{
				Handle: handle,
				Offset: int64(offset),
				Data:   buff,
			})
	}

	if err == io.EOF {
		err = nil
	}
	if err != nil {
		log.Err("Libfuse::Read : error reading file %s, handle: %d [%s]", handle.Path, handle.ID, err.Error())
		return -fuse.EIO
	}

	return bytesRead
}

func (cf *cgofuseFS) Write(path string, buff []byte, ofst int64, fh uint64) int {
	// Get the filehandle
	handle, exists := handlemap.Load(handlemap.HandleID(fh))
	if !exists {
		log.Trace("Libfuse::Write : error getting handle for path %s, handle: %d", path, fh)
		return -fuse.EBADF
	}

	offset := uint64(ofst)
	bytesWritten, err := fuseFS.NextComponent().WriteFile(
		internal.WriteFileOptions{
			Handle:   handle,
			Offset:   int64(offset),
			Data:     buff,
			Metadata: nil,
		})

	if err != nil {
		log.Err("Libfuse::Write : error writing file %s, handle: %d [%s]", handle.Path, handle.ID, err.Error())
		return -fuse.EIO
	}

	return bytesWritten
}

func (cf *cgofuseFS) Flush(path string, fh uint64) int {
	// Get the filehandle
	handle, exists := handlemap.Load(handlemap.HandleID(fh))
	if !exists {
		log.Trace("Libfuse::Flush : error getting handle for path %s, handle: %d", path, fh)
		return -fuse.EBADF
	}

	log.Trace("Libfuse::Flush : %s, handle: %d", handle.Path, handle.ID)

	// If the file handle is not dirty, there is no need to flush
	if handle.Dirty() {
		handle.Flags.Set(handlemap.HandleFlagDirty)
	}

	if !handle.Dirty() {
		return 0
	}

	err := fuseFS.NextComponent().FlushFile(internal.FlushFileOptions{Handle: handle})
	if err != nil {
		log.Err("Libfuse::Flush : error flushing file %s, handle: %d [%s]", handle.Path, handle.ID, err.Error())
		return -fuse.EIO
	}

	return 0
}

func (cf *cgofuseFS) Truncate(path string, size int64, fh uint64) int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)

	log.Trace("Libfuse::Truncate : %s size %d", name, size)

	err := fuseFS.NextComponent().TruncateFile(internal.TruncateFileOptions{Name: name, Size: size})
	if err != nil {
		log.Err("Libfuse::Truncate : error truncating file %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -fuse.ENOENT
		}
		return -fuse.EIO
	}

	libfuseStatsCollector.PushEvents(truncateFile, name, map[string]interface{}{"size": size})
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, truncateFile, (int64)(1))

	return 0
}

func (cf *cgofuseFS) Release(path string, fh uint64) int {
	// Get the filehandle
	handle, exists := handlemap.Load(handlemap.HandleID(fh))
	if !exists {
		log.Trace("Libfuse::Release : error getting handle for path %s, handle: %d", path, fh)
		return -fuse.EBADF
	}
	log.Trace("Libfuse::Release : %s, handle: %d", handle.Path, handle.ID)

	// If the file handle is dirty then file-cache needs to flush this file
	if handle.Dirty() {
		handle.Flags.Set(handlemap.HandleFlagDirty)
	}

	err := fuseFS.NextComponent().CloseFile(internal.CloseFileOptions{Handle: handle})
	if err != nil {
		log.Err("Libfuse::Release : error closing file %s, handle: %d [%s]", handle.Path, handle.ID, err.Error())
		return -fuse.EIO
	}

	handlemap.Delete(handle.ID)

	// decrement open file handles count
	libfuseStatsCollector.UpdateStats(stats_manager.Decrement, openHandles, (int64)(1))

	return 0
}

func (cf *cgofuseFS) Unlink(path string) int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Unlink : %s", name)

	err := fuseFS.NextComponent().DeleteFile(internal.DeleteFileOptions{Name: name})
	if err != nil {
		log.Err("Libfuse::Unlink : error deleting file %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -fuse.ENOENT
		}
		return -fuse.EIO
	}

	libfuseStatsCollector.PushEvents(deleteFile, name, nil)
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, deleteFile, (int64)(1))

	return 0
}

// https://man7.org/linux/man-pages/man2/rename.2.html
// errors handled: EISDIR, ENOENT, ENOTDIR, ENOTEMPTY, EEXIST
// TODO: handle EACCESS, EINVAL?
func (cf *cgofuseFS) Rename(oldpath string, newpath string) int {
	srcPath := trimFusePath(oldpath)
	srcPath = common.NormalizeObjectName(srcPath)
	dstPath := trimFusePath(newpath)
	dstPath = common.NormalizeObjectName(dstPath)
	log.Trace("Libfuse::Rename : %s -> %s", srcPath, dstPath)
	// Note: When running other commands from the command line, a lot of them seemed to handle some cases like ENOENT themselves.
	// Rename did not, so we manually check here.

	// ENOENT. Not covered: a directory component in dst does not exist
	if srcPath == "" || dstPath == "" {
		log.Err("Libfuse::Rename : src: [%s] or dst: [%s] is an empty string", srcPath, dstPath)
		return -fuse.ENOENT
	}

	srcAttr, srcErr := fuseFS.NextComponent().GetAttr(internal.GetAttrOptions{Name: srcPath})
	if os.IsNotExist(srcErr) {
		log.Err("Libfuse::Rename : Failed to get attributes of %s [%s]", srcPath, srcErr.Error())
		return -fuse.ENOENT
	}
	dstAttr, dstErr := fuseFS.NextComponent().GetAttr(internal.GetAttrOptions{Name: dstPath})

	// EISDIR
	if (dstErr == nil || os.IsExist(dstErr)) && dstAttr.IsDir() && !srcAttr.IsDir() {
		log.Err("Libfuse::Rename : dst [%s] is an existing directory but src [%s] is not a directory", dstPath, srcPath)
		return -fuse.EISDIR
	}

	// ENOTDIR
	if (dstErr == nil || os.IsExist(dstErr)) && !dstAttr.IsDir() && srcAttr.IsDir() {
		log.Err("Libfuse::Rename : dst [%s] is an existing file but src [%s] is a directory", dstPath, srcPath)
		return -fuse.ENOTDIR
	}

	if srcAttr.IsDir() {
		// ENOTEMPTY
		if dstErr == nil || os.IsExist(dstErr) {
			empty := fuseFS.NextComponent().IsDirEmpty(internal.IsDirEmptyOptions{Name: dstPath})
			if !empty {
				return -fuse.ENOTEMPTY
			}
		}

		err := fuseFS.NextComponent().RenameDir(internal.RenameDirOptions{Src: srcPath, Dst: dstPath})
		if err != nil {
			log.Err("Libfuse::Rename : error renaming directory %s -> %s [%s]", srcPath, dstPath, err.Error())
			return -fuse.EIO
		}

		libfuseStatsCollector.PushEvents(renameDir, srcPath, map[string]interface{}{source: srcPath, dest: dstPath})
		libfuseStatsCollector.UpdateStats(stats_manager.Increment, renameDir, (int64)(1))

	} else {
		err := fuseFS.NextComponent().RenameFile(internal.RenameFileOptions{Src: srcPath, Dst: dstPath})
		if err != nil {
			log.Err("Libfuse::Rename : error renaming file %s -> %s [%s]", srcPath, dstPath, err.Error())
			return -fuse.EIO
		}

		libfuseStatsCollector.PushEvents(renameFile, srcPath, map[string]interface{}{source: srcPath, dest: dstPath})
		libfuseStatsCollector.UpdateStats(stats_manager.Increment, renameFile, (int64)(1))

	}

	return 0
}

// Symlink Operations

func (cf *cgofuseFS) Symlink(target string, newpath string) int {
	name := trimFusePath(newpath)
	name = common.NormalizeObjectName(name)
	targetPath := common.NormalizeObjectName(target)
	log.Trace("Libfuse::Symlink : Received for %s -> %s", name, targetPath)

	err := fuseFS.NextComponent().CreateLink(internal.CreateLinkOptions{Name: name, Target: targetPath})
	if err != nil {
		log.Err("Libfuse::Symlink : error linking file %s -> %s [%s]", name, targetPath, err.Error())
		return -fuse.EIO
	}

	libfuseStatsCollector.PushEvents(createLink, name, map[string]interface{}{trgt: targetPath})
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, createLink, (int64)(1))

	return 0
}

func (cf *cgofuseFS) Readlink(path string) (int, string) {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Readlink : Received for %s", name)

	targetPath, err := fuseFS.NextComponent().ReadLink(internal.ReadLinkOptions{Name: name})
	if err != nil {
		log.Err("Libfuse::Readlink : error reading link file %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -fuse.ENOENT, targetPath
		}
		return -fuse.EIO, targetPath
	}

	// Don't think we need when with using cgofuse
	// data := (*[1 << 30]byte)(unsafe.Pointer(buf))
	// copy(data, targetPath)
	// data[len(targetPath)] = 0

	libfuseStatsCollector.PushEvents(readLink, name, map[string]interface{}{trgt: targetPath})
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, readLink, (int64)(1))

	return 0, targetPath
}

func (cf *cgofuseFS) Fsync(path string, datasync bool, fh uint64) int {
	if fh == 0 {
		return -fuse.EIO
	}

	// Get the filehandle
	handle, exists := handlemap.Load(handlemap.HandleID(fh))
	if !exists {
		log.Trace("Libfuse::Fsync : error getting handle for path %s, handle: %d", path, fh)
		return -fuse.EBADF
	}
	log.Trace("Libfuse::Fsync : %s, handle: %d", handle.Path, handle.ID)

	options := internal.SyncFileOptions{Handle: handle}
	// If the datasync parameter is non-zero, then only the user data should be flushed, not the metadata.
	// TODO : Should we support this?

	err := fuseFS.NextComponent().SyncFile(options)
	if err != nil {
		log.Err("Libfuse::Fsync : error syncing file %s [%s]", handle.Path, err.Error())
		return -fuse.EIO
	}

	libfuseStatsCollector.PushEvents(syncFile, handle.Path, nil)
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, syncFile, (int64)(1))

	return 0
}

func (cf *cgofuseFS) Fsyncdir(path string, datasync bool, fh uint64) int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Fsyncdir : %s", name)

	options := internal.SyncDirOptions{Name: name}
	// If the datasync parameter is non-zero, then only the user data should be flushed, not the metadata.
	// TODO : Should we support this?

	err := fuseFS.NextComponent().SyncDir(options)
	if err != nil {
		log.Err("Libfuse::Fsyncdir : error syncing dir %s [%s]", name, err.Error())
		return -fuse.EIO
	}

	libfuseStatsCollector.PushEvents(syncDir, name, nil)
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, syncDir, (int64)(1))

	return 0
}

func (cf *cgofuseFS) Chmod(path string, mode uint32) int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Chmod : %s", name)

	err := fuseFS.NextComponent().Chmod(
		internal.ChmodOptions{
			Name: name,
			Mode: fs.FileMode(mode),
		})
	if err != nil {
		log.Err("Libfuse::Chmod : error in chmod of %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -fuse.ENOENT
		}
		return -fuse.EIO
	}

	libfuseStatsCollector.PushEvents(chmod, name, map[string]interface{}{md: fs.FileMode(mode)})
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, chmod, (int64)(1))

	return 0
}

func (cf *cgofuseFS) Chown(path string, uid uint32, gid uint32) int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Chown : %s", name)
	// TODO: Implement
	return 0
}

func (cf *cgofuseFS) Utimens(path string, tmsp []fuse.Timespec) int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Utimens : %s", name)
	// TODO: is the conversion from [2]timespec to *timespec ok?
	// TODO: Implement
	// For now this returns 0 to allow touch to work correctly
	return 0
}

// Not implemented
func (cf *cgofuseFS) Access(path string, mask uint32) int {
	return -fuse.ENOSYS
}

// Not implemented
func (cf *cgofuseFS) Getxattr(path string, name string) (int, []byte) {
	return -fuse.ENOSYS, nil
}

// Not implemented
func (cf *cgofuseFS) Link(oldpath string, newpath string) int {
	return -fuse.ENOSYS
}

// Not implemented
func (cf *cgofuseFS) Listxattr(path string, fill func(name string) bool) int {
	return -fuse.ENOSYS
}

// Not implemented
func (cf *cgofuseFS) Mknod(path string, mode uint32, dev uint64) int {
	return -fuse.ENOSYS
}

// Not implemented
func (cf *cgofuseFS) Removexattr(path string, name string) int {
	return -fuse.ENOSYS
}

// Not implemented
func (cf *cgofuseFS) Setxattr(path string, name string, value []byte, flags int) int {
	return -fuse.ENOSYS
}

// blobfuse_cache_update refresh the file-cache policy for this file
// TODO: Figure out when to call this function since this was called with c code before
// func blobfuse_cache_update(path string) int {
// 	name := trimFusePath(path)
// 	name = common.NormalizeObjectName(name)
// 	go fuseFS.NextComponent().FileUsed(name) //nolint
// 	return 0
// }

// Verify that we follow the interface
var (
	_ fuse.FileSystemInterface = (*cgofuseFS)(nil)
)
