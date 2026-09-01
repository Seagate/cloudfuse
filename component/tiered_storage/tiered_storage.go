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

package tiered_storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/handlemap"
)

/*
   TieredStorage treats local storage as the authoritative tier and cloud
   storage as an overflow tier behind it.

   - Files created through the mount live only on local disk. They move to the
     cloud when the LRU evicts them, and the local copy is removed at that point.
   - Objects that are already in the cloud are cached locally on open and the
     local copy is dropped on last close, after any changes are uploaded. Data is
     therefore resident in both tiers for as short a time as possible.

   Lock ordering. Acquire in this order and release in reverse:

     1. the object's file lock (c.fileLocks) - at most one, except rename, which
        takes source and destination in lexical order
     2. lruQueue.evictMu
     3. lruQueue.mu

   Never take a file lock while holding lruQueue.mu. The eviction path obeys this
   by reading handle counts (which are atomic) during nomination and by using
   TryLock in its workers, so it never blocks on user I/O.
*/

// Common structure for Component
type TieredStorage struct {
	internal.BaseComponent

	fileMap sync.Map // uses object name (common.JoinUnixFilepath)

	policy *lruQueue

	//use LockMap instead of mutex to allow parallel access to different files
	fileLocks *common.LockMap // uses object name (common.JoinUnixFilepath)
	tmpPath   string          // uses os.Separator (filepath.Join)

	cacheSize    *cacheSizeTracker
	maxCacheSize float64
}

// FileNode tracks file state. Its atomic fields can be accessed concurrently.
// Name changes can only be done while holding flock.
type FileNode struct {
	name        string
	size        atomic.Int64
	cloudBacked atomic.Bool
	isDirty     atomic.Bool
}

// Structure defining your config parameters
type TieredStorageOptions struct {
	// e.g. var1 uint32 `config:"var1"`
	TmpPath   string  `config:"path"        yaml:"path,omitempty"`
	MaxSizeMB float64 `config:"max-size-mb" yaml:"max-size-mb,omitempty"`
}

const (
	compName = "tiered_storage"
	// TODO: make thresholds configurable
	defaultHighThreshold      = 0.8
	defaultLowThreshold       = 0.6
	defaultParallelism        = 8
	defaultMaxEviction        = 5000
	capacityPollInterval      = time.Second
	reconcileCapacityInterval = 5 * time.Minute
	partialDownloadSuffix     = ".cloudfuse-partial"
)

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &TieredStorage{}

func (c *TieredStorage) Name() string {
	return compName
}

func (c *TieredStorage) SetName(name string) {
	c.BaseComponent.SetName(name)
}

func (c *TieredStorage) SetNextComponent(nc internal.Component) {
	c.BaseComponent.SetNextComponent(nc)
}

// Start : Pipeline calls this method to start the component functionality
//
//	this shall not block the call otherwise pipeline will not start
func (c *TieredStorage) Start(ctx context.Context) error {
	log.Trace("TieredStorage::Start : Starting component %s", c.Name())

	// A crash can leave partial downloads behind. They are not valid object
	// data, so remove them before anything can list, open or upload them.
	c.removePartialDownloads()
	snapshot, err := c.readSnapshot()
	if err != nil {
		log.Warn("TieredStorage::Start : ignoring invalid state snapshot [%v]", err)
	}

	// Seed the usage counter from what is actually on disk. This is the one
	// place where measuring the whole directory is worth its cost.
	c.cacheSize.Refresh()
	if err := c.recoverLocalState(snapshot); err != nil {
		return fmt.Errorf("TieredStorage: failed to recover local state: %w", err)
	}

	if c.policy != nil {
		if err := c.policy.StartPolicy(); err != nil {
			log.Err("TieredStorage::Start : failed to start LRU policy [%v]", err)
			return err
		}
	}

	return nil
}

// Stop : Stop the component functionality and kill all threads started
//
// The local cache is deliberately left in place: for this component it holds
// the only copy of any data that has not been evicted yet.
func (c *TieredStorage) Stop() error {
	log.Trace("TieredStorage::Stop : Stopping component %s", c.Name())

	if c.policy != nil {
		if err := c.policy.StopPolicy(); err != nil {
			return err
		}
	}
	return c.writeSnapshot()
}

// removePartialDownloads deletes interrupted downloads left by a previous run.
func (c *TieredStorage) removePartialDownloads() {
	err := filepath.WalkDir(c.tmpPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil //nolint:nilerr // an unreadable entry is not worth aborting the sweep
		}
		if filepath.Ext(path) != partialDownloadSuffix {
			return nil
		}
		log.Info("TieredStorage::removePartialDownloads : removing %s", path)
		if rmErr := os.Remove(path); rmErr != nil {
			log.Warn(
				"TieredStorage::removePartialDownloads : %s remove failed [%v]",
				path,
				rmErr,
			)
		}
		return nil
	})
	if err != nil {
		log.Warn("TieredStorage::removePartialDownloads : %s walk failed [%v]", c.tmpPath, err)
	}
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (c *TieredStorage) Configure(_ bool) error {
	log.Trace("TieredStorage::Configure : %s", c.Name())

	conf := TieredStorageOptions{}
	err := config.UnmarshalKey(c.Name(), &conf)
	if err != nil {
		log.Err("TieredStorage::Configure : config error [invalid config attributes]")
		return fmt.Errorf("TieredStorage: config error [invalid config attributes]")
	}

	c.tmpPath = filepath.Clean(common.ExpandPath(conf.TmpPath))
	if c.tmpPath == "" || c.tmpPath == "." {
		return fmt.Errorf("TieredStorage: path not set in config")
	}
	err = os.MkdirAll(c.tmpPath, 0755)

	if err != nil {
		log.Err("TieredStorage::Configure : failed to create tmp path %s [%v]", c.tmpPath, err)
		return fmt.Errorf("TieredStorage: failed to create tmp path: %w", err)
	}

	c.maxCacheSize = conf.MaxSizeMB * common.MbToBytes
	c.cacheSize = newCacheSizeTracker(c.tmpPath, reconcileCapacityInterval)

	c.policy = &lruQueue{
		cachePath:        c.tmpPath,
		maxCacheSize:     c.maxCacheSize,
		fileLocks:        c.fileLocks,
		size:             c.cacheSize,
		threshold:        defaultHighThreshold,
		targetRatio:      defaultLowThreshold,
		numWorkers:       defaultParallelism,
		maxEviction:      defaultMaxEviction,
		pollInterval:     capacityPollInterval,
		uploadandCleanFn: c.uploadandCleanFile,
	}

	return nil
}

// OnConfigChange : If component has registered, on config file change this method is called
func (c *TieredStorage) OnConfigChange() {
}

// localPath resolves an object name beneath the tiered storage root.
func (c *TieredStorage) localPath(name string) (string, error) {
	name = filepath.FromSlash(common.NormalizeObjectName(name))
	if filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return "", syscall.EINVAL
	}

	path := filepath.Join(c.tmpPath, name)
	rel, err := filepath.Rel(c.tmpPath, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", syscall.EINVAL
	}
	return path, nil
}

// Directory operations
func (c *TieredStorage) CreateDir(options internal.CreateDirOptions) error {
	if _, err := c.GetAttr(internal.GetAttrOptions{Name: options.Name}); err == nil {
		return syscall.EEXIST
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	localPath, err := c.localPath(options.Name)
	if err != nil {
		return err
	}
	mode := options.Mode
	if mode == 0 || runtime.GOOS == "windows" {
		mode = common.DefaultDirectoryPermissionBits
	}
	if err := os.Mkdir(localPath, mode); err != nil {
		return err
	}
	if err := c.NextComponent().CreateDir(options); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return nil
}

func (c *TieredStorage) DeleteDir(options internal.DeleteDirOptions) error {
	localPath, err := c.localPath(options.Name)
	if err != nil {
		return err
	}
	cloudErr := c.NextComponent().DeleteDir(options)
	if cloudErr != nil && !errors.Is(cloudErr, os.ErrNotExist) {
		return cloudErr
	}
	localErr := os.Remove(localPath)
	if localErr != nil && !errors.Is(localErr, os.ErrNotExist) {
		return localErr
	}
	if errors.Is(localErr, os.ErrNotExist) && errors.Is(cloudErr, os.ErrNotExist) {
		return syscall.ENOENT
	}
	return nil
}

func (c *TieredStorage) IsDirEmpty(options internal.IsDirEmptyOptions) bool {
	localPath, err := c.localPath(options.Name)
	if err != nil {
		return false
	}
	entries, localErr := os.ReadDir(localPath)
	if localErr == nil && len(entries) > 0 {
		return false
	}
	if localErr != nil && !errors.Is(localErr, os.ErrNotExist) {
		return false
	}
	return c.NextComponent().IsDirEmpty(options)
}

func (c *TieredStorage) OpenDir(options internal.OpenDirOptions) error {
	localPath, err := c.localPath(options.Name)
	if err != nil {
		return err
	}
	if info, err := os.Stat(localPath); err == nil && info.IsDir() {
		return nil
	}
	return c.NextComponent().OpenDir(options)
}

func (c *TieredStorage) StreamDir(
	options internal.StreamDirOptions,
) ([]*internal.ObjAttr, string, error) {
	localPath, pathErr := c.localPath(options.Name)
	if pathErr != nil {
		return nil, "", pathErr
	}

	attrs, token, cloudErr := c.NextComponent().StreamDir(options)
	entries, localErr := os.ReadDir(localPath)
	localExists := localErr == nil
	if localErr != nil && !errors.Is(localErr, os.ErrNotExist) {
		return nil, "", localErr
	}
	if cloudErr != nil && !(localExists && errors.Is(cloudErr, os.ErrNotExist)) {
		return attrs, token, cloudErr
	}
	if cloudErr != nil {
		attrs = nil
		token = ""
	}

	localAttrs := make(map[string]*internal.ObjAttr, len(entries))
	for _, entry := range entries {
		if c.internalCacheEntry(options.Name, entry.Name()) {
			continue
		}
		entryPath := common.JoinUnixFilepath(options.Name, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, "", err
		}
		localAttrs[entry.Name()] = newTieredStorageObjAttr(entryPath, info)
	}

	listed := make(map[string]struct{}, len(attrs))
	for index, attr := range attrs {
		listed[attr.Name] = struct{}{}
		local, found := localAttrs[attr.Name]
		if !found || attr.IsDir() {
			continue
		}
		merged := *attr
		merged.Size = local.Size
		merged.Mtime = local.Mtime
		attrs[index] = &merged
	}

	if token == "" {
		for name, attr := range localAttrs {
			if _, found := listed[name]; !found {
				attrs = append(attrs, attr)
			}
		}
	}
	slices.SortFunc(attrs, func(a, b *internal.ObjAttr) int {
		return strings.Compare(a.Path, b.Path)
	})
	return attrs, token, nil
}

func (c *TieredStorage) CloseDir(options internal.CloseDirOptions) error {
	localPath, err := c.localPath(options.Name)
	if err != nil {
		return err
	}
	if info, err := os.Stat(localPath); err == nil && info.IsDir() {
		return nil
	}
	return c.NextComponent().CloseDir(options)
}

func (c *TieredStorage) RenameDir(options internal.RenameDirOptions) error {
	return nil
}

// File operations
func (c *TieredStorage) createFileUnlocked(
	options internal.CreateFileOptions,
) (*handlemap.Handle, error) {
	// A new file holds no data yet, so there is nothing to make room for.
	// WriteFile reserves space as the file grows.

	//Create the file in the local cache, we will ignore the create empty and cloud stuff for now
	localPath, err := c.localPath(options.Name)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(filepath.Dir(localPath), 0755)

	if err != nil {
		return nil, err
	}

	//Open local file
	localFile, err := common.OpenFile(
		localPath,
		os.O_CREATE|os.O_EXCL|os.O_RDWR,
		c.cacheFileMode(options.Mode),
	)
	if err != nil {
		return nil, err
	}

	//Add file node to file map with cloudBacked as false
	node := &FileNode{name: options.Name}
	node.isDirty.Store(true)
	c.fileMap.Store(options.Name, node)

	//create handle
	handle := handlemap.NewHandle(options.Name)
	handle.SetFileObject(localFile)

	//Mark as dirty because the cloud doesn't know about it
	c.setHandleDirty(handle)

	return handle, nil

}

func (c *TieredStorage) CreateFile(
	options internal.CreateFileOptions,
) (*handlemap.Handle, error) {
	flock := c.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	localPath, err := c.localPath(options.Name)
	if err != nil {
		return nil, err
	}
	_, err = c.getAttrUnlocked(
		internal.GetAttrOptions{Name: options.Name},
		localPath,
	)
	if err == nil {
		return nil, syscall.EEXIST
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	handle, err := c.createFileUnlocked(options)
	if err != nil {
		return nil, err
	}
	flock.Inc()
	return handle, nil
}

func (c *TieredStorage) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("TieredStorage::DeleteFile : name=%s", options.Name)

	flock := c.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	// Read the state only after taking the lock: an eviction worker may have
	// been uploading this object right up until we acquired it.
	val, exists := c.fileMap.Load(options.Name)
	if !exists {
		// cloud only
		err := c.NextComponent().DeleteFile(options)
		if errors.Is(err, os.ErrNotExist) {
			return syscall.ENOENT
		}
		return err
	}

	node := val.(*FileNode)
	if node.cloudBacked.Load() {
		if err := c.NextComponent().DeleteFile(options); err != nil {
			return err
		}
	}

	// Cancel any eviction of this object before dropping our own state, so a
	// worker cannot resurrect it in the cloud after the user deleted it.
	c.policy.Dequeue(options.Name)
	c.fileMap.Delete(options.Name)

	return c.purgeLocal(options.Name)
}

// OpenFile: Makes the file available in the local cache for further file operations.
func (c *TieredStorage) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	// get the file lock, so only one open call can proceed for a file, other calls will wait here until lock is released
	flock := c.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	localPath, err := c.localPath(options.Name)
	if err != nil {
		return nil, err
	}
	attrs, attrErr := c.getAttrUnlocked(internal.GetAttrOptions{Name: options.Name}, localPath)
	if attrErr != nil && !errors.Is(attrErr, os.ErrNotExist) {
		return nil, attrErr
	}
	exists := attrErr == nil
	if options.Flags&os.O_CREATE != 0 && options.Flags&os.O_EXCL != 0 && exists {
		return nil, syscall.EEXIST
	}
	if !exists {
		if options.Flags&os.O_CREATE == 0 {
			return nil, syscall.ENOENT
		}
		handle, err := c.createFileUnlocked(
			internal.CreateFileOptions{Name: options.Name, Mode: options.Mode},
		)
		if err != nil {
			return nil, err
		}
		flock.Inc()
		return handle, nil
	}

	_, tracked := c.fileMap.Load(options.Name)
	if !tracked {
		info, statErr := os.Stat(localPath)
		if statErr == nil {
			log.Warn(
				"TieredStorage::OpenFile : Warning file exists locally on disk but not in tiered storage cache: %s",
				options.Name,
			)
			node := &FileNode{name: options.Name}
			node.size.Store(info.Size())
			node.isDirty.Store(true)
			c.fileMap.Store(options.Name, node)
		} else {
			localCopyNode := &FileNode{name: options.Name}
			localCopyNode.size.Store(attrs.Size)
			localCopyNode.cloudBacked.Store(true)

			if options.Flags&os.O_TRUNC != 0 {
				if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
					return nil, err
				}
				file, err := common.OpenFile(
					localPath,
					os.O_CREATE|os.O_EXCL|os.O_RDWR,
					c.cacheFileMode(options.Mode),
				)
				if err != nil {
					return nil, err
				}
				if err := file.Close(); err != nil {
					return nil, err
				}
				localCopyNode.size.Store(0)
				localCopyNode.isDirty.Store(true)
			} else {
				if err := c.reserveSpace(attrs.Size); err != nil {
					return nil, err
				}
				if err := c.downloadCopyFromCloud(options); err != nil {
					return nil, err
				}
			}
			c.fileMap.Store(options.Name, localCopyNode)
		}
	}

	openFlags := options.Flags &^ (os.O_CREATE | os.O_EXCL)
	localFile, err := common.OpenFile(
		localPath,
		openFlags,
		c.cacheFileMode(options.Mode),
	)
	if err != nil {
		return nil, err
	}

	// Create handle and attach file object to it
	handle := handlemap.NewHandle(options.Name)
	handle.SetFileObject(localFile)
	if options.Flags&os.O_APPEND != 0 {
		handle.Flags.Set(handlemap.HandleOpenedAppend)
	}
	if options.Flags&os.O_TRUNC != 0 {
		if value, found := c.fileMap.Load(options.Name); found {
			node := value.(*FileNode)
			c.recordSize(node, 0)
			node.isDirty.Store(true)
		}
		c.setHandleDirty(handle)
	}

	//increase handle count
	flock.Inc()

	return handle, nil
}

func (c *TieredStorage) cacheFileMode(mode os.FileMode) os.FileMode {
	if mode == 0 || runtime.GOOS == "windows" {
		return common.DefaultFilePermissionBits
	}
	return mode
}

// downloadCopyFromCloud caches a cloud object on local storage.
//
// The data lands in a temporary file and is renamed into place only once it is
// complete. A crash can therefore never leave a truncated file at the object's
// real path, which matters because local data is authoritative here: a partial
// download found at a real path would later be uploaded over the good cloud copy.
func (c *TieredStorage) downloadCopyFromCloud(options internal.OpenFileOptions) error {
	localPath, err := c.localPath(options.Name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(localPath), 0755)
	if err != nil {
		return err
	}

	partPath := fmt.Sprintf("%s.%d%s", localPath, os.Getpid(), partialDownloadSuffix)
	partFile, err := common.OpenFile(
		partPath,
		os.O_CREATE|os.O_TRUNC|os.O_RDWR,
		c.cacheFileMode(options.Mode),
	)
	if err != nil {
		return err
	}

	//Download
	err = c.NextComponent().CopyToFile(internal.CopyToFileOptions{
		Name:   options.Name,
		Offset: 0,
		Count:  0,
		File:   partFile,
	})
	if err == nil {
		err = partFile.Sync()
	}
	if closeErr := partFile.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		log.Err("TieredStorage::downloadCopyFromCloud : %s download failed [%v]", options.Name, err)
		_ = os.Remove(partPath)
		return err
	}

	if err := replaceFile(partPath, localPath); err != nil {
		log.Err("TieredStorage::downloadCopyFromCloud : %s rename failed [%v]", options.Name, err)
		_ = os.Remove(partPath)
		return err
	}

	if info, statErr := os.Stat(localPath); statErr == nil {
		c.cacheSize.Add(info.Size())
	}

	//some sort of mode handling
	return nil
}

// replaceFile moves src over dst. Windows will not replace a destination that
// another handle holds without share-delete, so fall back to removing it first.
func replaceFile(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	if rmErr := os.Remove(dst); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		return err
	}
	return os.Rename(src, dst)
}

// purgeLocal removes an object's local copy and updates the usage counter.
// The object's file lock must be held.
func (c *TieredStorage) purgeLocal(name string) error {
	localPath, err := c.localPath(name)
	if err != nil {
		return err
	}

	info, statErr := os.Stat(localPath)
	err = os.Remove(localPath)
	if err == nil && statErr == nil {
		c.cacheSize.Add(-info.Size())
	}
	return err
}

// recordSize updates what we believe a cached file's size to be and adjusts the
// usage counter by the difference.
func (c *TieredStorage) recordSize(node *FileNode, size int64) {
	if node == nil {
		return
	}
	c.cacheSize.Add(size - node.size.Swap(size))
}

// ensure there is enough available space for local storage to grow by the given amount.
// evict files if necessary. Return ENOSPC if there is no way to make room for the new data.
//
// Callers may hold a file lock. Eviction workers never block on file locks, so
// the object being written cannot deadlock against its own eviction.
func (c *TieredStorage) reserveSpace(numBytes int64) error {
	if c.maxCacheSize <= 0 || numBytes <= 0 {
		return nil
	}
	if float64(c.cacheSize.Used()+numBytes) <= c.maxCacheSize {
		return nil
	}
	if c.policy == nil || c.policy.EvictNow(numBytes) {
		return nil
	}

	log.Err(
		"TieredStorage::reserveSpace : cache is full and nothing can be evicted (need %d bytes, using %d of %d)",
		numBytes,
		c.cacheSize.Used(),
		int64(c.maxCacheSize),
	)
	return syscall.ENOSPC
}

func (c *TieredStorage) ReadInBuffer(options *internal.ReadInBufferOptions) (int, error) {
	f := options.Handle.GetFileObject()
	if f == nil {
		log.Err(
			"TieredStorage::ReadInBuffer : error [couldn't find fd in handle] %s",
			options.Handle.Path,
		)
		return 0, syscall.EBADF
	}

	n, err := f.ReadAt(options.Data, options.Offset)
	// ReadAt gives an error if it reads fewer bytes than the byte array. We discard that error.
	if n < len(options.Data) && err == io.EOF {
		return n, nil
	}
	return n, err
}

func (c *TieredStorage) WriteFile(options *internal.WriteFileOptions) (int, error) {
	//1.Get the file object
	f := options.Handle.GetFileObject()
	if f == nil {
		return 0, syscall.EBADF
	}

	var node *FileNode
	if val, ok := c.fileMap.Load(options.Handle.Path); ok {
		node = val.(*FileNode)
	}

	//2. Make room for however much bigger this write makes the file
	appending := options.Handle.Flags.IsSet(handlemap.HandleOpenedAppend)
	growth := int64(len(options.Data))
	if !appending && node != nil {
		growth = max(options.Offset+int64(len(options.Data))-node.size.Load(), 0)
	}
	if err := c.reserveSpace(growth); err != nil {
		return 0, err
	}

	//3. Decide where to write in file
	var bytesWritten int
	var err error
	if appending {
		//write to end of file, standard
		bytesWritten, err = f.Write(options.Data)
	} else {
		//write to specific offset, need to use WriteAt
		bytesWritten, err = f.WriteAt(options.Data, options.Offset)
	}

	//4. Mark file as dirty for release later
	if err == nil {
		c.setHandleDirty(options.Handle)
		if node != nil {
			node.isDirty.Store(true)
			// One fstat is the cheapest way to be right about the new size for
			// every kind of write - appending, sparse, or overwriting in place.
			if info, statErr := f.Stat(); statErr == nil {
				c.recordSize(node, info.Size())
			}
		}
	} else {
		log.Err(
			"TieredStorage::WriteFile : failed to write %s [%s]",
			options.Handle.Path,
			err.Error(),
		)
	}

	return bytesWritten, err
}

func (c *TieredStorage) SyncFile(options internal.SyncFileOptions) error {
	log.Trace(
		"TieredStorage::SyncFile : handle=%d, path=%s",
		options.Handle.ID,
		options.Handle.Path,
	)
	return c.FlushFile(internal.FlushFileOptions{Handle: options.Handle})
}

func (c *TieredStorage) FlushFile(options internal.FlushFileOptions) error {
	//Ok so we just need to flush locally, which means just write it to the disc
	log.Trace(
		"TieredStorage::FlushFile : handle=%d, path=%s",
		options.Handle.ID,
		options.Handle.Path,
	)

	//1. Only need to flush dirty files
	if !options.Handle.Dirty() {
		return nil
	}
	//2. Check if there is local file object form handle
	f := options.Handle.GetFileObject()
	if f == nil {
		log.Err("TieredStorage::FlushFile : %s no file object in handle", options.Handle.Path)
		return syscall.EBADF
	}
	//3. Sync to Disk
	err := f.Sync()
	if err != nil {
		log.Err("TieredStorage::FlushFile : %s sync failed [%v]", options.Handle.Path, err)
		return syscall.EIO
	}

	return nil
}

func (c *TieredStorage) ReleaseFile(options internal.ReleaseFileOptions) error {
	// get the file lock, so only one open call can proceed for a file, other calls will wait here until lock is released
	flock := c.fileLocks.Get(options.Handle.Path)
	flock.Lock()
	defer flock.Unlock()

	//Dec Handle Count First
	flock.Dec()

	//close file associated with handle
	if f := options.Handle.GetFileObject(); f != nil {
		f.Close()
	}

	//clean handle state
	c.clearHandleDirty(options.Handle)
	options.Handle.Cleanup()

	//remove from global handle map
	handlemap.Delete(options.Handle.ID)

	//Only the last handle decides what happens to the local copy
	if flock.Count() > 0 {
		return nil
	}

	val, ok := c.fileMap.Load(options.Handle.Path)
	if !ok {
		// the object was deleted or evicted while it was open - closing a
		// deleted file is normal, so this is not an error
		log.Debug(
			"TieredStorage::ReleaseFile : %s has no local data left",
			options.Handle.Path,
		)
		return nil
	}
	node := val.(*FileNode)

	if !node.cloudBacked.Load() {
		// Local-only data is authoritative. The LRU decides when it moves to
		// cloud storage, and the local copy stays until it does.
		c.policy.Enqueue(options.Handle.Path)
		return nil
	}

	if node.isDirty.Load() {
		if err := c.uploadCachedFile(options.Handle.Path); err != nil {
			// Keep the local copy: it is newer than the cloud object. Hand it
			// to the LRU so the upload is retried, rather than stranding the
			// only good copy of the data in the cache with nothing watching it.
			log.Err(
				"TieredStorage::ReleaseFile : upload failed for %s [%v]",
				options.Handle.Path,
				err,
			)
			c.policy.Enqueue(options.Handle.Path)
			return err
		}
		node.isDirty.Store(false)
	}

	// Cached cloud data is dropped as soon as the last handle closes, so an
	// object spends as little time as possible resident in both tiers.
	c.fileMap.Delete(options.Handle.Path)
	return c.purgeLocal(options.Handle.Path)
}

func (c *TieredStorage) uploadCachedFile(name string) error {
	//get the local path
	localPath, err := c.localPath(name)
	if err != nil {
		return err
	}
	_, err = os.Stat(localPath)
	if err != nil {
		log.Err("TieredStorage::uploadFile : %s stat failed [%v]", name, err)
		return err
	}

	//open read-only handle/file for uploading
	f, openErr := common.Open(localPath)
	if openErr != nil {
		log.Err("TieredStorage::uploadFile : %s open failed [%v]", name, openErr)
		return openErr
	}
	defer f.Close()

	//upload
	uploadErr := c.NextComponent().CopyFromFile(internal.CopyFromFileOptions{Name: name, File: f})
	if uploadErr != nil {
		log.Err("TieredStorage::uploadFile : %s upload failed [%v]", name, uploadErr)
	}
	return uploadErr
}

// uploadandCleanFile moves an object to cloud storage and removes the local
// copy. It is the LRU's eviction callback, so it runs with the object's file
// lock held.
func (c *TieredStorage) uploadandCleanFile(name string) error {
	err := c.uploadCachedFile(name)
	if err != nil {
		return err
	}
	c.fileMap.Delete(name)
	err = c.purgeLocal(name)
	if err != nil {
		log.Err("TieredStorage::uploadandCleanFile : %s remove failed [%v]", name, err)
		return err
	}
	return nil
}

func (c *TieredStorage) RenameFile(options internal.RenameFileOptions) error {
	//Ok we are going to follow DeleteFile, we have to rename this File in the various states that its in
	//So First we lock in alphabetical order
	log.Trace("TieredStorage::RenameFile : src=%s, dst=%s", options.Src, options.Dst)

	sflock := c.fileLocks.Get(options.Src)
	dflock := c.fileLocks.Get(options.Dst)

	if options.Src < options.Dst {
		sflock.Lock()
		dflock.Lock()
	} else {
		dflock.Lock()
		sflock.Lock()
	}
	defer sflock.Unlock()
	defer dflock.Unlock()

	//Ok now we have to consider all the states
	//Rename File that is Local Only
	//sync map in both local and in the LRU, also need to rename the node in the queue
	//Check that it exists
	val, exists := c.fileMap.Load(options.Src)
	//Potential local or local + cloud state
	if exists {
		node := val.(*FileNode)
		// //Local and Cloud State
		if node.cloudBacked.Load() {
			//just rename from the cloud
			err := c.NextComponent().RenameFile(options)
			if err != nil {
				return err
			}
		}
		//Local only State, this will happen anyways if it exists local
		//Rename
		srcPath, err := c.localPath(options.Src)
		if err != nil {
			return err
		}
		dstPath, err := c.localPath(options.Dst)
		if err != nil {
			return err
		}
		err = os.Rename(srcPath, dstPath)
		if err != nil {
			return err
		}
		c.fileMap.Delete(options.Src)
		node.name = options.Dst
		c.fileMap.Store(options.Dst, node)

		// Move the queue entry across. The source is still in nodeMap even if a
		// worker is mid-eviction, and Dequeue cancels that eviction - which is
		// what stops a renamed object from falling out of the queue entirely.
		_, wasQueued := c.policy.nodeMap.Load(options.Src)
		c.policy.Dequeue(options.Src)
		if wasQueued {
			c.policy.Enqueue(options.Dst)
		}
		//Change the handle and the lock counts
		c.renameOpenHandles(options.Src, options.Dst, sflock, dflock)

		//Cloud only state
	} else {
		err := c.NextComponent().RenameFile(options)
		if err != nil {
			return err
		}
	}
	return nil
}

// flock must be locked for both files
func (c *TieredStorage) renameOpenHandles(
	srcName, dstName string,
	sflock, dflock *common.LockMapItem,
) {
	// update open handles
	if sflock.Count() > 0 {
		// update any open handles to the file with its new name
		handlemap.GetHandles().Range(func(key, value any) bool {
			handle := value.(*handlemap.Handle)
			handle.Lock()
			if handle.Path == srcName {
				handle.Path = dstName
			}
			handle.Unlock()
			return true
		})
		// copy the number of open handles to the new name
		for sflock.Count() > 0 {
			sflock.Dec()
			dflock.Inc()
		}
		for sflock.DirtyCount() > 0 {
			sflock.DecDirty()
			dflock.IncDirty()
		}
	}
}

func (c *TieredStorage) SyncDir(options internal.SyncDirOptions) error {
	return c.NextComponent().SyncDir(options)
}

func (c *TieredStorage) internalCacheEntry(directory, name string) bool {
	return directory == "" &&
		(name == tieredStorageSnapshotPath || name == tieredStorageSnapshotPath+".tmp") ||
		strings.HasSuffix(name, partialDownloadSuffix)
}

// Symlink operations
func (c *TieredStorage) CreateLink(options internal.CreateLinkOptions) error {
	return nil
}

func (c *TieredStorage) ReadLink(options internal.ReadLinkOptions) (string, error) {
	return "", nil
}

// Dirty Handle Operations
func (c *TieredStorage) setHandleDirty(handle *handlemap.Handle) {
	handle.Lock()
	alreadyDirty := handle.Dirty()
	if !alreadyDirty {
		handle.Flags.Set(handlemap.HandleFlagDirty)
	}
	handle.Unlock()
	if !alreadyDirty {
		c.fileLocks.Get(handle.Path).IncDirty()
	}
}

// setter
func (c *TieredStorage) clearHandleDirty(handle *handlemap.Handle) {
	handle.Lock()
	wasDirty := handle.Dirty()
	if wasDirty {
		handle.Flags.Clear(handlemap.HandleFlagDirty)
	}
	handle.Unlock()
	if wasDirty {
		c.fileLocks.Get(handle.Path).DecDirty()
	}
}

// Filesystem level operations
func (c *TieredStorage) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	localPath, err := c.localPath(options.Name)
	if err != nil {
		return nil, err
	}

	flock := c.fileLocks.Get(options.Name)
	flock.RLock()
	defer flock.RUnlock()
	return c.getAttrUnlocked(options, localPath)
}

// getAttrUnlocked merges local and cloud attributes. The object's file lock
// must already be held.
func (c *TieredStorage) getAttrUnlocked(
	options internal.GetAttrOptions,
	localPath string,
) (*internal.ObjAttr, error) {
	info, localErr := os.Stat(localPath)
	if localErr != nil && !errors.Is(localErr, os.ErrNotExist) {
		return nil, localErr
	}

	attrs, cloudErr := c.NextComponent().GetAttr(options)
	if localErr != nil {
		return attrs, cloudErr
	}

	localAttrs := newTieredStorageObjAttr(options.Name, info)
	if cloudErr != nil || attrs == nil {
		return localAttrs, nil
	}
	if info.IsDir() {
		return attrs, nil
	}

	merged := *attrs
	merged.Size = localAttrs.Size
	merged.Mtime = localAttrs.Mtime
	return &merged, nil
}

func (c *TieredStorage) Chmod(options internal.ChmodOptions) error {
	return nil
}

func (c *TieredStorage) Chown(options internal.ChownOptions) error {
	return nil
}

func (c *TieredStorage) TruncateFile(options internal.TruncateFileOptions) error {
	return nil
}

func (c *TieredStorage) FileUsed(name string) error {
	return nil
}

func (c *TieredStorage) StatFs() (*common.Statfs_t, bool, error) {
	return nil, false, nil
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewTieredStorageComponent() internal.Component {
	comp := &TieredStorage{
		fileLocks: common.NewLockMap(),
	}
	comp.SetName(compName)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewTieredStorageComponent)
}
