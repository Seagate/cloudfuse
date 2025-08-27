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

package libfuse

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/handlemap"
	"github.com/Seagate/cloudfuse/internal/stats_manager"

	"github.com/winfsp/cgofuse/fuse"
)

/* --- IMPORTANT NOTE ---
In below code lot of places we are doing this sort of conversions:
	- handle, exists := handlemap.Load(handlemap.HandleID(fh))
or we are doing:
	- handle.ID = (handlemap.HandleID)(fh)

In cloudfuse we maintain handles as an object stored in a handlemap. Cgofuse gives us handles as integer
values so we need to do type conversions to convert those values to our Handle ID values that cloudfuse
uses so we convert the integer into a handle object.
*/

// CgofuseFS defines the file system with functions that interface with FUSE.
type CgofuseFS struct {
	// Implement the interface from cgofuse
	fuse.FileSystemBase

	// user identifier on linux
	uid uint32

	// group identifier on linux
	gid uint32
}

const windowsDefaultSDDL = "D:P(A;;FA;;;WD)" // Enables everyone on system to have access to mount

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

// initFuse passes the launch options for fuse and starts the mount.
// Here are the options for FUSE.
// LINK: https://man7.org/linux/man-pages/man8/mount.fuse3.8.html
func (lf *Libfuse) initFuse() error {
	log.Trace("Libfuse::initFuse : Initializing FUSE")

	cf := NewcgofuseFS()
	cf.uid = lf.ownerUID
	cf.gid = lf.ownerGID

	lf.host = fuse.NewFileSystemHost(cf)
	// prevent Windows from calling GetAttr redundantly
	lf.host.SetCapReaddirPlus(true)

	options := fmt.Sprintf("uid=%d,gid=%d,entry_timeout=%d,attr_timeout=%d,negative_timeout=%d",
		lf.ownerUID,
		lf.ownerGID,
		lf.entryExpiration,
		lf.attributeExpiration,
		lf.negativeTimeout)

	// With WinFSP this will present all files as owned by the Authenticated Users group
	if runtime.GOOS == "windows" {
		// if uid & gid were not specified, pass -1 for both (which will cause WinFSP to look up the current user)
		uid := int64(-1)
		gid := int64(-1)
		if lf.ownerUID != 0 {
			uid = int64(lf.ownerUID)
		}
		if lf.ownerGID != 0 {
			gid = int64(lf.ownerGID)
		}
		options = fmt.Sprintf("uid=%d,gid=%d,entry_timeout=%d,attr_timeout=%d,negative_timeout=%d",
			uid,
			gid,
			lf.entryExpiration,
			lf.attributeExpiration,
			lf.negativeTimeout)

		// Using SSDL file security option: https://github.com/rclone/rclone/issues/4717
		windowsSDDL := windowsDefaultSDDL
		if lf.windowsSDDL != "" {
			windowsSDDL = lf.windowsSDDL
		}
		options += ",FileSecurity=" + windowsSDDL
	}

	fuse_options := createFuseOptions(
		lf.host,
		lf.allowOther,
		lf.allowRoot,
		lf.readOnly,
		lf.nonEmptyMount,
		lf.maxFuseThreads,
		lf.umask,
	)
	options += fuse_options

	// Setup options as a slice
	opts := []string{"-o", options}

	// Runs as network file share on Windows only when mounting to drive letter.
	if runtime.GOOS == "windows" && lf.networkShare && common.IsDriveLetter(lf.mountPath) {
		var nameStorage string

		serverName, err := os.Hostname()
		if err != nil {
			log.Err(
				"Libfuse::initFuse : failed to mount fuse. unable to determine server host name.",
			)
			return errors.New("failed to mount fuse. unable to determine server host name")
		}
		// Borrow bucket-name string from attribute cache
		if config.IsSet("s3storage.bucket-name") {
			err := config.UnmarshalKey("s3storage.bucket-name", &nameStorage)
			if err != nil {
				nameStorage = "s3"
				log.Err("initFuse : Failed to unmarshal s3storage.bucket-name")
			}
		} else if config.IsSet("azstorage.container") {
			err := config.UnmarshalKey("azstorage.container", &nameStorage)
			if err != nil {
				nameStorage = "azure"
				log.Err("initFuse : Failed to unmarshal s3storage.bucket-name")
			}
		}

		volumePrefix := fmt.Sprintf("--VolumePrefix=\\%s\\%s", serverName, nameStorage)
		opts = append(opts, volumePrefix)
	}

	// Enabling trace is done by using -d rather than setting an option in fuse
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

func (lf *Libfuse) fillStat(attr *internal.ObjAttr, stbuf *fuse.Stat_t) {
	stbuf.Uid = lf.ownerUID
	stbuf.Gid = lf.ownerGID
	stbuf.Nlink = 1
	stbuf.Size = attr.Size

	// Populate mode
	// Backing storage implementation has support for mode.
	if !attr.IsModeDefault() {
		stbuf.Mode = uint32(attr.Mode) & 0xffffffff
	} else {
		if attr.IsDir() {
			stbuf.Mode = uint32(lf.dirPermission) & 0xffffffff
		} else {
			stbuf.Mode = uint32(lf.filePermission) & 0xffffffff
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

	stbuf.Atim = fuse.NewTimespec(attr.Atime)
	stbuf.Atim.Nsec = 0
	stbuf.Ctim = fuse.NewTimespec(attr.Ctime)
	stbuf.Ctim.Nsec = 0
	stbuf.Mtim = fuse.NewTimespec(attr.Mtime)
	stbuf.Mtim.Nsec = 0
	stbuf.Birthtim = fuse.NewTimespec(attr.Mtime)
	stbuf.Birthtim.Nsec = 0
}

// NewcgofuseFS creates a new empty fuse filesystem.
func NewcgofuseFS() *CgofuseFS {
	cf := &CgofuseFS{}
	return cf
}

// Init notifies the parent process once the mount is successful.
func (cf *CgofuseFS) Init() {
	log.Trace("Libfuse::Init : Initializing FUSE")

	log.Info("Libfuse::Init : Notifying parent for successful mount")
	if err := common.NotifyMountToParent(); err != nil {
		log.Err("Libfuse::initFuse : Failed to notify parent, error: [%v]", err)
	}
}

// Destroy currently does nothing.
func (cf *CgofuseFS) Destroy() {
	log.Trace("Libfuse::Destroy : Destroy")
}

// Getattr retrieves the file attributes at the path and fills them in stat.
func (cf *CgofuseFS) Getattr(path string, stat *fuse.Stat_t, fh uint64) int {
	// TODO: Currently not using filehandle
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)

	// Don't log these by default, as it noticeably affects performance
	// log.Trace("Libfuse::Getattr : %s", name)

	// Return the default configuration for the root
	if name == "" {
		stat.Mode = fuse.S_IFDIR | 0777
		stat.Uid = cf.uid
		stat.Gid = cf.gid
		stat.Nlink = 2
		stat.Size = 4096
		stat.Mtim = fuse.NewTimespec(time.Now())
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
		switch err {
		case syscall.ENOENT:
			return -fuse.ENOENT
		case syscall.EACCES:
			return -fuse.EACCES
		default:
			return -fuse.EIO
		}
	}

	// Populate stat
	fuseFS.fillStat(attr, stat)
	return 0
}

// Statfs sets file system statistics. It returns 0 if successful.
func (cf *CgofuseFS) Statfs(path string, stat *fuse.Statfs_t) int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_statfs : %s", name)

	attr, populated, err := fuseFS.NextComponent().StatFs()
	if err != nil {
		log.Err("Libfuse::Statfs: Failed to get stats %s [%s]", name, err.Error())
		return -fuse.EIO
	}

	// if populated then we need to overwrite root attributes
	if populated {
		stat.Bsize = uint64(attr.Bsize)
		stat.Frsize = uint64(attr.Frsize)
		// cloud storage always sets free and avail to zero
		statsFromCloudStorage := attr.Bfree == 0 && attr.Bavail == 0
		// calculate blocks used from attr
		blocksUnavailable := attr.Blocks - attr.Bavail
		blocksUsed := attr.Blocks - attr.Bfree
		// we only use displayCapacity to complement used size from cloud storage
		if statsFromCloudStorage {
			displayCapacityBlocks := fuseFS.displayCapacityMb * common.MbToBytes / uint64(
				attr.Bsize,
			)
			// if used > displayCapacity, then report used and show that we are out of space
			stat.Blocks = max(displayCapacityBlocks, blocksUnavailable)
		} else {
			stat.Blocks = attr.Blocks
		}
		// adjust avail and free to make sure we display used space correctly
		stat.Bavail = stat.Blocks - blocksUnavailable
		stat.Bfree = stat.Blocks - blocksUsed
		stat.Files = attr.Files
		stat.Ffree = attr.Ffree
		stat.Namemax = attr.Namemax
	} else {
		stat.Bsize = blockSize
		stat.Frsize = blockSize
		displayCapacityBlocks := fuseFS.displayCapacityMb * common.MbToBytes / blockSize
		stat.Blocks = displayCapacityBlocks
		stat.Bavail = displayCapacityBlocks
		stat.Bfree = displayCapacityBlocks
		stat.Files = 1e9
		stat.Ffree = 1e9
		stat.Namemax = maxNameSize
	}

	return 0
}

// Mkdir creates a new directory at the path with the given mode.
func (cf *CgofuseFS) Mkdir(path string, mode uint32) int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Mkdir : %s", name)

	// Check if the directory already exists. On Windows we need to make this call explicitly
	if runtime.GOOS == "windows" {
		_, err := fuseFS.NextComponent().GetAttr(internal.GetAttrOptions{Name: name})
		// If the the error is nil then a file or directory with this name exists
		if err == nil || errors.Is(err, fs.ErrExist) {
			return -fuse.EEXIST
		}
	}

	err := fuseFS.NextComponent().
		CreateDir(internal.CreateDirOptions{Name: name, Mode: fs.FileMode(mode)})
	if err != nil {
		log.Err("Libfuse::Mkdir : Failed to create %s [%s]", name, err.Error())
		if os.IsPermission(err) {
			return -fuse.EACCES
		} else if os.IsExist(err) {
			return -fuse.EEXIST
		} else {
			return -fuse.EIO
		}
	}

	libfuseStatsCollector.PushEvents(createDir, name, map[string]interface{}{md: fs.FileMode(mode)})
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, createDir, (int64)(1))

	return 0
}

// Opendir opens the directory at the path.
func (cf *CgofuseFS) Opendir(path string) (int, uint64) {
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
	log.Debug("Libfuse::Opendir : %s fh=%d", name, fh)

	// This needs to return a uint64 representing the filehandle
	// We have to do a casting here to make the Go compiler happy but
	// handle.ID should already be a uint64
	return 0, uint64(fh)
}

// Releasedir opens the handle for the directory at the path.
func (cf *CgofuseFS) Releasedir(path string, fh uint64) int {
	// Get the filehandle
	handle, exists := handlemap.LoadAndDelete(handlemap.HandleID(fh))
	if !exists {
		log.Trace("Libfuse::Releasedir : Failed to release %s, handle: %d", path, fh)
		return -fuse.EBADF
	}

	log.Trace("Libfuse::Releasedir : %s, handle: %d", handle.Path, handle.ID)

	handle.Cleanup()
	return 0
}

// Readdir reads a directory at the path.
func (cf *CgofuseFS) Readdir(
	path string,
	fill func(name string, stat *fuse.Stat_t, ofst int64) bool,
	ofst int64,
	fh uint64,
) int {
	// Readdir is called with a file handle, which was created when the OS called Opendir
	// Fetch our data for that file handle
	handle, exists := handlemap.Load(handlemap.HandleID(fh))
	if !exists {
		log.Trace("Libfuse::Readdir : Failed to read %s, handle: %d", path, fh)
		return -fuse.EBADF
	}
	// Get the directory listing cache (cacheInfo) from the file handle
	handle.RLock()
	val, found := handle.GetValue("cache")
	handle.RUnlock()
	if !found {
		return -fuse.EIO
	}
	cacheInfo := val.(*dirChildCache)

	// figure out what we need to provide to the OS
	offset := uint64(ofst)
	// is this a brand new request (not the continuation of a previous one)?
	newRequest := offset == 0
	if newRequest {
		// cache the first two entries ('.' & '..')
		cacheDots(cacheInfo)
	}
	startOffset := offset
	// fetch and serve directory contents back to the OS in a loop until their buffer is full
	for {
		// is the next offset we need already cached in our cacheInfo structure?
		offsetCached := startOffset >= cacheInfo.sIndex && startOffset < cacheInfo.eIndex
		fetchDataFromPipeline := !offsetCached
		if fetchDataFromPipeline {
			// populate cache from pipeline
			errorCode := populateDirChildCache(handle, cacheInfo, startOffset)
			if errorCode != 0 {
				log.Err(
					"Libfuse::Readdir : Path %s, handle: %d, offset %d. Error in retrieval",
					handle.Path,
					handle.ID,
					offset,
				)
				return errorCode
			}
		}
		// we can't get the requested data (validToken is probably false)
		if startOffset >= cacheInfo.eIndex {
			log.Warn("Libfuse::Readdir : %s offset=%d but last cached offset is %d (token=%s)",
				path, startOffset, cacheInfo.eIndex, cacheInfo.token)
			// If offset is still beyond the end index limit then we are done iterating
			return 0
		}
		// serve entries from cache
		nextOffset, done := serveCachedEntries(cacheInfo, startOffset, fill)
		log.Debug("Libfuse::Readdir : %s, offset: %d, handle: %d - returned entries %d-%d",
			path, offset, fh, startOffset, nextOffset-1)
		// break when the OS is done with this Readdir call
		if done {
			break
		}
		// update offset for iteration
		startOffset = nextOffset
	}
	return 0
}

type fillFunc = func(name string, stat *fuse.Stat_t, ofst int64) bool

// add the first two entries in any directory listing ('.' and '..') to the cache
// this replaces any existing cache
func cacheDots(cacheInfo *dirChildCache) {
	dotAttrs := []*internal.ObjAttr{
		{Flags: fuseFS.lsFlags, Name: "."},
		{Flags: fuseFS.lsFlags, Name: ".."},
	}
	cacheInfo.sIndex = 0
	cacheInfo.eIndex = 2
	cacheInfo.length = 2
	cacheInfo.token = ""
	cacheInfo.children = cacheInfo.children[:0]
	cacheInfo.children = dotAttrs
	cacheInfo.lastPage = false
}

// Fill the directory list cache with data from the next component
func populateDirChildCache(
	handle *handlemap.Handle,
	cacheInfo *dirChildCache,
	offset uint64,
) (errorCode int) {
	// don't get more entries if there are no more
	if cacheInfo.lastPage {
		return
	}
	// get entries from the pipeline
	returnedAttrs, token, err := fuseFS.NextComponent().StreamDir(internal.StreamDirOptions{
		Name:  handle.Path,
		Token: cacheInfo.token,
	})
	if err != nil {
		if os.IsNotExist(err) {
			return -fuse.ENOENT
		} else if os.IsPermission(err) {
			return -fuse.EACCES
		}
		return -fuse.EIO
	}
	// compile results and update cache
	// let the cache grow to MaxDirListCount
	replaceCache := cacheInfo.length+uint64(len(returnedAttrs)) > common.MaxDirListCount
	if replaceCache {
		cacheInfo.sIndex = offset
		cacheInfo.eIndex = offset
		cacheInfo.children = cacheInfo.children[:0]
		cacheInfo.length = 0
	}
	cacheInfo.eIndex += uint64(len(returnedAttrs))
	cacheInfo.children = append(cacheInfo.children, returnedAttrs...)
	cacheInfo.length += uint64(len(returnedAttrs))
	cacheInfo.token = token
	cacheInfo.lastPage = token == ""

	return 0
}

// call fill with cache entries from our cache of directory contents
func serveCachedEntries(
	cacheInfo *dirChildCache,
	startOffset uint64,
	fill fillFunc,
) (nextOffset uint64, done bool) {
	stbuf := fuse.Stat_t{}
	// Populate the stat by calling filler
	nextOffset = startOffset
	for cacheIndex := nextOffset - cacheInfo.sIndex; cacheIndex < cacheInfo.length && !done; cacheIndex++ {
		// prepare entry
		fuseFS.fillStat(cacheInfo.children[cacheIndex], &stbuf)
		name := cacheInfo.children[cacheIndex].Name
		// call fill with name, stat buffer, and the offset for the *next* entry
		nextOffset++
		done = !fill(name, &stbuf, int64(nextOffset))
	}
	// also quit when the directory has no more entries
	done = done || cacheInfo.lastPage

	return nextOffset, done
}

// Rmdir deletes a directory.
func (cf *CgofuseFS) Rmdir(path string) int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Rmdir : %s", name)

	empty := fuseFS.NextComponent().IsDirEmpty(internal.IsDirEmptyOptions{Name: name})
	if !empty {
		// delete empty directories from local cache directory
		val, err := fuseFS.NextComponent().DeleteEmptyDirs(internal.DeleteDirOptions{Name: name})
		if !val {
			// either file cache has failed or not present in the pipeline
			if err != nil {
				// if error is not nil, file cache has failed
				log.Err("Libfuse::libfuse_rmdir : Failed to delete %s [%s]", name, err.Error())
			}
			return -fuse.ENOTEMPTY
		} else {
			empty = fuseFS.NextComponent().IsDirEmpty(internal.IsDirEmptyOptions{Name: name})
			if !empty {
				return -fuse.ENOTEMPTY
			}
		}
	}

	err := fuseFS.NextComponent().DeleteDir(internal.DeleteDirOptions{Name: name})
	if err != nil {
		log.Err("Libfuse::Rmdir : Failed to delete %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -fuse.ENOENT
		}

		return -fuse.EIO
	}

	libfuseStatsCollector.PushEvents(deleteDir, name, nil)
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, deleteDir, (int64)(1))

	return 0
}

// Create creates a new file and opens it.
func (cf *CgofuseFS) Create(path string, flags int, mode uint32) (int, uint64) {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Create : %s", name)

	handle, err := fuseFS.NextComponent().
		CreateFile(internal.CreateFileOptions{Name: name, Mode: fs.FileMode(mode)})
	if err != nil {
		log.Err("Libfuse::Create : Failed to create %s [%s]", name, err.Error())
		if os.IsExist(err) {
			return -fuse.EEXIST, 0
		} else if os.IsPermission(err) {
			return -fuse.EACCES, 0
		}

		return -fuse.EIO, 0
	}

	fh := handlemap.Add(handle)
	// Don't think we need this
	// ret_val := C.allocate_native_file_object(C.ulong(handle.UnixFD), C.ulong(uintptr(unsafe.Pointer(handle))), 0)
	// if !handle.Cached() {
	// 	ret_val.fd = 0
	// }
	log.Trace("Libfuse::Create : %s, handle %d", name, fh)
	//fi.fh = C.ulong(uintptr(unsafe.Pointer(ret_val)))

	libfuseStatsCollector.PushEvents(
		createFile,
		name,
		map[string]interface{}{md: fs.FileMode(mode)},
	)

	// increment open file handles count
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, openHandles, (int64)(1))

	return 0, uint64(fh)
}

// Open opens a file.
func (cf *CgofuseFS) Open(path string, flags int) (int, uint64) {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Open : %s", name)

	handle, err := fuseFS.NextComponent().OpenFile(
		internal.OpenFileOptions{
			Name:  name,
			Flags: flags,
			Mode:  fs.FileMode(fuseFS.filePermission),
		})

	if err != nil {
		log.Err("Libfuse::Open : Failed to open %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -fuse.ENOENT, 0
		} else if os.IsPermission(err) {
			return -fuse.EACCES, 0
		}

		return -fuse.EIO, 0
	}

	fh := handlemap.Add(handle)
	// Don't think we need this
	// ret_val := C.allocate_native_file_object(C.ulong(handle.UnixFD), C.ulong(uintptr(unsafe.Pointer(handle))), C.ulong(handle.Size))
	// if !handle.Cached() {
	// 	ret_val.fd = 0
	// }
	log.Trace("Libfuse::Open : %s, handle %d", name, fh)
	// fi.fh = C.ulong(uintptr(unsafe.Pointer(ret_val)))

	// increment open file handles count
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, openHandles, (int64)(1))

	return 0, uint64(fh)
}

// Read reads data from a file into the buffer with the given offset.
func (cf *CgofuseFS) Read(path string, buff []byte, ofst int64, fh uint64) int {
	//skipping the logging to avoid creating log noise and the performance costs from huge number of calls.
	//log.Debug("Libfuse::Read : reading path %s, handle: %d", path, fh)
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
		// Remove Pread as not supported on Windows
		//bytesRead, err = syscall.Pread(handle.FD(), buff, int64(offset))
		bytesRead, err = handle.FObj.ReadAt(buff, int64(offset))
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
		log.Err(
			"Libfuse::Read : error reading file %s, handle: %d [%s]",
			handle.Path,
			handle.ID,
			err.Error(),
		)
		return -fuse.EIO
	}

	return bytesRead
}

// Write writes data to a file from the buffer with the given offset.
func (cf *CgofuseFS) Write(path string, buff []byte, ofst int64, fh uint64) int {
	//skipping the logging to avoid creating log noise and the performance costs from huge number of calls
	//log.Debug("Libfuse::Write : Writing path %s, handle: %d", path, fh)
	// Get the filehandle
	handle, exists := handlemap.Load(handlemap.HandleID(fh))
	if !exists {
		log.Trace("Libfuse::Write : error getting handle for path %s, handle: %d", path, fh)
		return -fuse.EBADF
	}

	bytesWritten, err := fuseFS.NextComponent().WriteFile(
		internal.WriteFileOptions{
			Handle:   handle,
			Offset:   ofst,
			Data:     buff,
			Metadata: nil,
		})

	if err != nil {
		log.Err(
			"Libfuse::Write : error writing file %s, handle: %d [%s]",
			handle.Path,
			handle.ID,
			err.Error(),
		)
		return -fuse.EIO
	}

	return bytesWritten
}

// Flush flushes any cached file data.
func (cf *CgofuseFS) Flush(path string, fh uint64) int {
	// Get the filehandle
	handle, exists := handlemap.Load(handlemap.HandleID(fh))
	if !exists {
		log.Trace("Libfuse::Flush : error getting handle for path %s, handle: %d", path, fh)
		return -fuse.EBADF
	}

	log.Trace("Libfuse::Flush : %s, handle: %d", handle.Path, handle.ID)

	// If the file handle is not dirty, there is no need to flush
	if !handle.Dirty() {
		return 0
	}

	err := fuseFS.NextComponent().FlushFile(internal.FlushFileOptions{Handle: handle})
	if err != nil {
		log.Err(
			"Libfuse::Flush : error flushing file %s, handle: %d [%s]",
			handle.Path,
			handle.ID,
			err.Error(),
		)
		switch err {
		case syscall.ENOENT:
			return -fuse.ENOENT
		case syscall.EACCES:
			return -fuse.EACCES
		default:
			return -fuse.EIO
		}
	}

	return 0
}

// Truncate changes the size of the given file.
func (cf *CgofuseFS) Truncate(path string, size int64, fh uint64) int {
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

// Release closes an open file.
func (cf *CgofuseFS) Release(path string, fh uint64) int {
	// Get the filehandle
	handle, exists := handlemap.Load(handlemap.HandleID(fh))
	if !exists {
		log.Trace("Libfuse::Release : error getting handle for path %s, handle: %d", path, fh)
		return -fuse.EBADF
	}
	log.Trace("Libfuse::Release : %s, handle: %d", handle.Path, handle.ID)

	err := fuseFS.NextComponent().CloseFile(internal.CloseFileOptions{Handle: handle})
	if err != nil {
		log.Err(
			"Libfuse::Release : error closing file %s, handle: %d [%s]",
			handle.Path,
			handle.ID,
			err.Error(),
		)
		switch err {
		case syscall.ENOENT:
			return -fuse.ENOENT
		case syscall.EACCES:
			return -fuse.EACCES
		default:
			return -fuse.EIO
		}
	}

	handlemap.Delete(handle.ID)

	// decrement open file handles count
	libfuseStatsCollector.UpdateStats(stats_manager.Decrement, openHandles, (int64)(1))

	return 0
}

// Unlink deletes a file.
func (cf *CgofuseFS) Unlink(path string) int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Unlink : %s", name)

	err := fuseFS.NextComponent().DeleteFile(internal.DeleteFileOptions{Name: name})
	if err != nil {
		log.Err("Libfuse::Unlink : error deleting file %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -fuse.ENOENT
		} else if os.IsPermission(err) {
			return -fuse.EACCES
		}
		return -fuse.EIO

	}

	libfuseStatsCollector.PushEvents(deleteFile, name, nil)
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, deleteFile, (int64)(1))

	return 0
}

// Rename renames a file.
// https://man7.org/linux/man-pages/man2/rename.2.html
// errors handled: EISDIR, ENOENT, ENOTDIR, ENOTEMPTY, EEXIST
// TODO: handle EACCESS, EINVAL?
func (cf *CgofuseFS) Rename(oldpath string, newpath string) int {
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
	if dstErr == nil && dstAttr.IsDir() && !srcAttr.IsDir() {
		log.Err(
			"Libfuse::Rename : dst [%s] is an existing directory but src [%s] is not a directory",
			dstPath,
			srcPath,
		)
		return -fuse.EISDIR
	}

	// ENOTDIR
	if dstErr == nil && !dstAttr.IsDir() && srcAttr.IsDir() {
		log.Err(
			"Libfuse::Rename : dst [%s] is an existing file but src [%s] is a directory",
			dstPath,
			srcPath,
		)
		return -fuse.ENOTDIR
	}

	if srcAttr.IsDir() {
		// ENOTEMPTY
		if dstErr == nil {
			empty := fuseFS.NextComponent().IsDirEmpty(internal.IsDirEmptyOptions{Name: dstPath})
			if !empty {
				return -fuse.ENOTEMPTY
			}
		}

		err := fuseFS.NextComponent().
			RenameDir(internal.RenameDirOptions{Src: srcPath, Dst: dstPath})
		if err != nil {
			log.Err(
				"Libfuse::Rename : error renaming directory %s -> %s [%s]",
				srcPath,
				dstPath,
				err.Error(),
			)
			return -fuse.EIO
		}

		libfuseStatsCollector.PushEvents(
			renameDir,
			srcPath,
			map[string]interface{}{source: srcPath, dest: dstPath},
		)
		libfuseStatsCollector.UpdateStats(stats_manager.Increment, renameDir, (int64)(1))

	} else {
		err := fuseFS.NextComponent().RenameFile(internal.RenameFileOptions{
			Src:     srcPath,
			Dst:     dstPath,
			SrcAttr: srcAttr,
			DstAttr: dstAttr,
		})
		if err != nil {
			log.Err("Libfuse::Rename : error renaming file %s -> %s [%s]", srcPath, dstPath, err.Error())
			return -fuse.EIO
		}

		libfuseStatsCollector.PushEvents(renameFile, srcPath, map[string]interface{}{source: srcPath, dest: dstPath})
		libfuseStatsCollector.UpdateStats(stats_manager.Increment, renameFile, (int64)(1))

	}

	return 0
}

// Symlink creates a symbolic link
func (cf *CgofuseFS) Symlink(target string, newpath string) int {
	name := trimFusePath(newpath)
	name = common.NormalizeObjectName(name)
	targetPath := common.NormalizeObjectName(target)
	log.Trace("Libfuse::Symlink : Received for %s -> %s", name, targetPath)

	err := fuseFS.NextComponent().
		CreateLink(internal.CreateLinkOptions{Name: name, Target: targetPath})
	if err != nil {
		log.Err(
			"Libfuse::Symlink : error linking file %s -> %s [%s]",
			name,
			targetPath,
			err.Error(),
		)
		return -fuse.EIO
	}

	libfuseStatsCollector.PushEvents(createLink, name, map[string]interface{}{trgt: targetPath})
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, createLink, (int64)(1))

	return 0
}

// Readlink reads the target of a symbolic link.
func (cf *CgofuseFS) Readlink(path string) (int, string) {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Readlink : Received for %s", name)

	linkSize := int64(0)
	attr, err := fuseFS.NextComponent().GetAttr(internal.GetAttrOptions{Name: name})
	if err == nil && attr != nil {
		linkSize = attr.Size
	}

	targetPath, err := fuseFS.NextComponent().
		ReadLink(internal.ReadLinkOptions{Name: name, Size: linkSize})
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

// Fsync synchronizes the file.
func (cf *CgofuseFS) Fsync(path string, datasync bool, fh uint64) int {
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

// Fsyncdir synchronizes a directory.
func (cf *CgofuseFS) Fsyncdir(path string, datasync bool, fh uint64) int {
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

// Chmod changes permissions of a file.
func (cf *CgofuseFS) Chmod(path string, mode uint32) int {
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
		} else if os.IsPermission(err) {
			return -fuse.EACCES
		}
		return -fuse.EIO
	}

	libfuseStatsCollector.PushEvents(chmod, name, map[string]interface{}{md: fs.FileMode(mode)})
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, chmod, (int64)(1))

	return 0
}

// Chown changes the owner of a file.
func (cf *CgofuseFS) Chown(path string, uid uint32, gid uint32) int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Chown : %s", name)
	// TODO: Implement
	return 0
}

// Utimens changes the access and modification time of a file.
func (cf *CgofuseFS) Utimens(path string, tmsp []fuse.Timespec) int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Utimens : %s", name)
	// TODO: is the conversion from [2]timespec to *timespec ok?
	// TODO: Implement
	// For now this returns 0 to allow touch to work correctly
	return 0
}

// Access is not implemented.
func (cf *CgofuseFS) Access(path string, mask uint32) int {
	return -fuse.ENOSYS
}

// Getxattr  is not implemented.
func (cf *CgofuseFS) Getxattr(path string, name string) (int, []byte) {
	return -fuse.ENOSYS, nil
}

// Link is not implemented.
func (cf *CgofuseFS) Link(oldpath string, newpath string) int {
	return -fuse.ENOSYS
}

// Listxattr is not implemented.
func (cf *CgofuseFS) Listxattr(path string, fill func(name string) bool) int {
	return -fuse.ENOSYS
}

// Mknod is not implemented.
func (cf *CgofuseFS) Mknod(path string, mode uint32, dev uint64) int {
	return -fuse.ENOSYS
}

// Removexattr is not implemented.
func (cf *CgofuseFS) Removexattr(path string, name string) int {
	return -fuse.ENOSYS
}

// Setxattr  is not implemented.
func (cf *CgofuseFS) Setxattr(path string, name string, value []byte, flags int) int {
	return -fuse.ENOSYS
}

// cloudfuse_cache_update refresh the file-cache policy for this file
// TODO: Figure out when to call this function since this was called with c code before
// func cloudfuse_cache_update(path string) int {
// 	name := trimFusePath(path)
// 	name = common.NormalizeObjectName(name)
// 	go fuseFS.NextComponent().FileUsed(name) //nolint
// 	return 0
// }

// Verify that we follow the interface
var (
	_ fuse.FileSystemInterface = (*CgofuseFS)(nil)
)
