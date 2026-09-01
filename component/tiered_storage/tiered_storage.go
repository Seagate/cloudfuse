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

// TieredStorage keeps new data local and uses cloud storage as overflow.
// Cloud objects are cached only while open.
//
// Lock order: file lock, evictMu, then lruQueue.mu. Rename takes file locks in
// lexical order. Never acquire a file lock while holding lruQueue.mu.
type TieredStorage struct {
	internal.BaseComponent

	fileMap sync.Map // uses object name (common.JoinUnixFilepath)

	policy *lruQueue

	fileLocks *common.LockMap // uses object name (common.JoinUnixFilepath)
	tmpPath   string          // uses os.Separator (filepath.Join)

	cacheSize    *cacheSizeTracker
	maxCacheSize float64
}

// FileNode fields are atomic except name, which requires the file lock.
type FileNode struct {
	name        string
	size        atomic.Int64
	cloudBacked atomic.Bool
	isDirty     atomic.Bool
	mode        atomic.Uint32
	modeDirty   atomic.Bool
	owner       atomic.Int64
	group       atomic.Int64
	ownerDirty  atomic.Bool
}

type TieredStorageOptions struct {
	TmpPath         string  `config:"path"              yaml:"path,omitempty"`
	MaxSizeMB       float64 `config:"max-size-mb"       yaml:"max-size-mb,omitempty"`
	HighThreshold   uint32  `config:"high-threshold"    yaml:"high-threshold,omitempty"`
	LowThreshold    uint32  `config:"low-threshold"     yaml:"low-threshold,omitempty"`
	MaxEviction     uint32  `config:"max-eviction"      yaml:"max-eviction,omitempty"`
	Parallelism     uint32  `config:"parallelism"       yaml:"parallelism,omitempty"`
	PollIntervalSec uint32  `config:"poll-interval-sec" yaml:"poll-interval-sec,omitempty"`
}

const (
	compName                  = "tiered_storage"
	defaultHighThreshold      = 80
	defaultLowThreshold       = 60
	defaultParallelism        = 8
	defaultMaxEviction        = 5000
	capacityPollInterval      = time.Second
	reconcileCapacityInterval = 5 * time.Minute
	partialDownloadSuffix     = ".cloudfuse-partial"
)

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

func (c *TieredStorage) Start(ctx context.Context) error {
	log.Trace("TieredStorage::Start : Starting component %s", c.Name())

	snapshot, err := c.readSnapshot()
	if err != nil {
		log.Warn("TieredStorage::Start : ignoring invalid state snapshot [%v]", err)
	}

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

// Stop leaves local files in place for the next mount.
func (c *TieredStorage) Stop() error {
	log.Trace("TieredStorage::Stop : Stopping component %s", c.Name())

	if c.policy != nil {
		if err := c.policy.StopPolicy(); err != nil {
			return err
		}
	}
	return c.writeSnapshot()
}

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
	if conf.MaxSizeMB <= 0 {
		return fmt.Errorf("TieredStorage: max-size-mb must be greater than 0")
	}

	if conf.HighThreshold == 0 {
		conf.HighThreshold = defaultHighThreshold
	}
	if conf.LowThreshold == 0 {
		conf.LowThreshold = defaultLowThreshold
	}
	if conf.LowThreshold >= conf.HighThreshold || conf.HighThreshold > 100 {
		return fmt.Errorf(
			"TieredStorage: thresholds must satisfy 0 < low-threshold < high-threshold <= 100",
		)
	}
	if conf.MaxEviction == 0 {
		conf.MaxEviction = defaultMaxEviction
	}
	if conf.Parallelism == 0 {
		conf.Parallelism = defaultParallelism
	}
	pollInterval := time.Duration(conf.PollIntervalSec) * time.Second
	if pollInterval == 0 {
		pollInterval = capacityPollInterval
	}

	c.maxCacheSize = conf.MaxSizeMB * common.MbToBytes
	c.cacheSize = newCacheSizeTracker(c.tmpPath, reconcileCapacityInterval)

	c.policy = &lruQueue{
		cachePath:        c.tmpPath,
		maxCacheSize:     c.maxCacheSize,
		fileLocks:        c.fileLocks,
		size:             c.cacheSize,
		threshold:        float64(conf.HighThreshold) / 100,
		targetRatio:      float64(conf.LowThreshold) / 100,
		numWorkers:       int(conf.Parallelism),
		maxEviction:      conf.MaxEviction,
		pollInterval:     pollInterval,
		uploadandCleanFn: c.uploadandCleanFile,
	}

	return nil
}

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
		name := entry.Name()
		if strings.HasSuffix(name, partialDownloadSuffix) || options.Name == "" &&
			(name == tieredStorageSnapshotPath || name == tieredStorageSnapshotPath+".tmp") {
			continue
		}
		entryPath := common.JoinUnixFilepath(options.Name, name)
		info, err := entry.Info()
		if err != nil {
			return nil, "", err
		}
		localAttrs[name] = newTieredStorageObjAttr(entryPath, info)
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
	srcPath, err := c.localPath(options.Src)
	if err != nil {
		return err
	}
	dstPath, err := c.localPath(options.Dst)
	if err != nil {
		return err
	}
	_, localErr := os.Stat(srcPath)
	localExists := localErr == nil
	if localErr != nil && !errors.Is(localErr, os.ErrNotExist) {
		return localErr
	}

	prefix := common.JoinUnixFilepath(options.Src) + "/"
	var srcNames []string
	c.fileMap.Range(func(key, _ any) bool {
		name := key.(string)
		if strings.HasPrefix(name, prefix) {
			srcNames = append(srcNames, name)
		}
		return true
	})

	type renameEntry struct {
		src string
		dst string
	}
	entries := make([]renameEntry, 0, len(srcNames))
	lockNames := make([]string, 0, len(srcNames)*2)
	for _, src := range srcNames {
		dst := common.JoinUnixFilepath(options.Dst, strings.TrimPrefix(src, prefix))
		entries = append(entries, renameEntry{src: src, dst: dst})
		lockNames = append(lockNames, src, dst)
	}
	slices.Sort(lockNames)
	lockNames = slices.Compact(lockNames)
	locks := make([]*common.LockMapItem, 0, len(lockNames))
	for _, name := range lockNames {
		flock := c.fileLocks.Get(name)
		flock.Lock()
		locks = append(locks, flock)
	}
	defer func() {
		for _, flock := range slices.Backward(locks) {
			flock.Unlock()
		}
	}()

	cloudErr := c.NextComponent().RenameDir(options)
	if cloudErr != nil && !(localExists && errors.Is(cloudErr, os.ErrNotExist)) {
		return cloudErr
	}
	if !localExists {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}
	if err := os.Rename(srcPath, dstPath); err != nil {
		return err
	}

	for _, entry := range entries {
		value, found := c.fileMap.LoadAndDelete(entry.src)
		if !found {
			continue
		}
		node := value.(*FileNode)
		node.name = entry.dst
		c.fileMap.Store(entry.dst, node)

		_, queued := c.policy.nodeMap.Load(entry.src)
		c.policy.Dequeue(entry.src)
		if queued {
			c.policy.Enqueue(entry.dst)
		}
		c.renameOpenHandles(
			entry.src,
			entry.dst,
			c.fileLocks.Get(entry.src),
			c.fileLocks.Get(entry.dst),
		)
	}
	return nil
}

// File operations
func (c *TieredStorage) createFileUnlocked(
	options internal.CreateFileOptions,
) (*handlemap.Handle, error) {
	localPath, err := c.localPath(options.Name)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(filepath.Dir(localPath), 0755)

	if err != nil {
		return nil, err
	}

	localFile, err := common.OpenFile(
		localPath,
		os.O_CREATE|os.O_EXCL|os.O_RDWR,
		c.cacheFileMode(options.Mode),
	)
	if err != nil {
		return nil, err
	}

	node := &FileNode{name: options.Name}
	node.isDirty.Store(true)
	c.fileMap.Store(options.Name, node)

	handle := handlemap.NewHandle(options.Name)
	handle.SetFileObject(localFile)

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

	val, exists := c.fileMap.Load(options.Name)
	if !exists {
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

	// Cancel eviction before deleting state to prevent a stale upload.
	c.policy.Dequeue(options.Name)
	c.fileMap.Delete(options.Name)

	return c.purgeLocal(options.Name)
}

func (c *TieredStorage) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
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

	handle := handlemap.NewHandle(options.Name)
	handle.SetFileObject(localFile)
	if options.Flags&os.O_APPEND != 0 {
		handle.Flags.Set(handlemap.HandleOpenedAppend)
	}
	if options.Flags&os.O_TRUNC != 0 {
		if value, found := c.fileMap.Load(options.Name); found {
			node := value.(*FileNode)
			c.cacheSize.Add(-node.size.Swap(0))
			node.isDirty.Store(true)
		}
		c.setHandleDirty(handle)
	}

	flock.Inc()

	return handle, nil
}

func (c *TieredStorage) cacheFileMode(mode os.FileMode) os.FileMode {
	if mode == 0 || runtime.GOOS == "windows" {
		return common.DefaultFilePermissionBits
	}
	return mode
}

// downloadCopyFromCloud publishes the local path only after a complete download.
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

	return nil
}

// replaceFile handles Windows rename-over-existing behavior.
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

// reserveSpace evicts files as needed. Workers never block on file locks.
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
	f := options.Handle.GetFileObject()
	if f == nil {
		return 0, syscall.EBADF
	}

	var node *FileNode
	if val, ok := c.fileMap.Load(options.Handle.Path); ok {
		node = val.(*FileNode)
	}

	appending := options.Handle.Flags.IsSet(handlemap.HandleOpenedAppend)
	growth := int64(len(options.Data))
	if !appending && node != nil {
		growth = max(options.Offset+int64(len(options.Data))-node.size.Load(), 0)
	}
	if err := c.reserveSpace(growth); err != nil {
		return 0, err
	}

	var bytesWritten int
	var err error
	if appending {
		bytesWritten, err = f.Write(options.Data)
	} else {
		bytesWritten, err = f.WriteAt(options.Data, options.Offset)
	}

	if err == nil {
		c.setHandleDirty(options.Handle)
		if node != nil {
			node.isDirty.Store(true)
			if info, statErr := f.Stat(); statErr == nil {
				c.cacheSize.Add(info.Size() - node.size.Swap(info.Size()))
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
	log.Trace(
		"TieredStorage::FlushFile : handle=%d, path=%s",
		options.Handle.ID,
		options.Handle.Path,
	)

	if !options.Handle.Dirty() {
		return nil
	}
	f := options.Handle.GetFileObject()
	if f == nil {
		log.Err("TieredStorage::FlushFile : %s no file object in handle", options.Handle.Path)
		return syscall.EBADF
	}
	err := f.Sync()
	if err != nil {
		log.Err("TieredStorage::FlushFile : %s sync failed [%v]", options.Handle.Path, err)
		return syscall.EIO
	}

	return nil
}

func (c *TieredStorage) ReleaseFile(options internal.ReleaseFileOptions) error {
	flock := c.fileLocks.Get(options.Handle.Path)
	flock.Lock()
	defer flock.Unlock()

	flock.Dec()

	if f := options.Handle.GetFileObject(); f != nil {
		f.Close()
	}

	c.clearHandleDirty(options.Handle)
	options.Handle.Cleanup()

	handlemap.Delete(options.Handle.ID)

	if flock.Count() > 0 {
		return nil
	}

	val, ok := c.fileMap.Load(options.Handle.Path)
	if !ok {
		log.Debug(
			"TieredStorage::ReleaseFile : %s has no local data left",
			options.Handle.Path,
		)
		return nil
	}
	node := val.(*FileNode)

	if !node.cloudBacked.Load() {
		c.policy.Enqueue(options.Handle.Path)
		return nil
	}

	if node.isDirty.Load() {
		if err := c.uploadCachedFile(options.Handle.Path); err != nil {
			// Keep failed uploads local and eligible for eviction.
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

	c.fileMap.Delete(options.Handle.Path)
	return c.purgeLocal(options.Handle.Path)
}

func (c *TieredStorage) uploadCachedFile(name string) error {
	localPath, err := c.localPath(name)
	if err != nil {
		return err
	}

	f, err := common.Open(localPath)
	if err != nil {
		log.Err("TieredStorage::uploadFile : %s open failed [%v]", name, err)
		return err
	}
	defer f.Close()

	err = c.NextComponent().CopyFromFile(internal.CopyFromFileOptions{Name: name, File: f})
	if err != nil {
		log.Err("TieredStorage::uploadFile : %s upload failed [%v]", name, err)
		return err
	}

	value, found := c.fileMap.Load(name)
	if !found {
		return nil
	}
	node := value.(*FileNode)
	if node.modeDirty.Load() {
		err = c.NextComponent().Chmod(internal.ChmodOptions{
			Name: name,
			Mode: os.FileMode(node.mode.Load()),
		})
		if err != nil && !errors.Is(err, syscall.ENOTSUP) {
			return err
		}
		node.modeDirty.Store(false)
	}
	if node.ownerDirty.Load() {
		err = c.NextComponent().Chown(internal.ChownOptions{
			Name:  name,
			Owner: int(node.owner.Load()),
			Group: int(node.group.Load()),
		})
		if err != nil && !errors.Is(err, syscall.ENOTSUP) {
			return err
		}
		node.ownerDirty.Store(false)
	}
	return nil
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

	val, exists := c.fileMap.Load(options.Src)
	if exists {
		node := val.(*FileNode)
		if node.cloudBacked.Load() {
			err := c.NextComponent().RenameFile(options)
			if err != nil {
				return err
			}
		}
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

		// Dequeue also cancels an in-flight eviction.
		_, wasQueued := c.policy.nodeMap.Load(options.Src)
		c.policy.Dequeue(options.Src)
		if wasQueued {
			c.policy.Enqueue(options.Dst)
		}
		c.renameOpenHandles(options.Src, options.Dst, sflock, dflock)
	} else {
		err := c.NextComponent().RenameFile(options)
		if err != nil {
			return err
		}
	}
	return nil
}

// Both file locks must be held.
func (c *TieredStorage) renameOpenHandles(
	srcName, dstName string,
	sflock, dflock *common.LockMapItem,
) {
	if sflock.Count() > 0 {
		handlemap.GetHandles().Range(func(key, value any) bool {
			handle := value.(*handlemap.Handle)
			handle.Lock()
			if handle.Path == srcName {
				handle.Path = dstName
			}
			handle.Unlock()
			return true
		})
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

// Symlink operations
func (c *TieredStorage) CreateLink(options internal.CreateLinkOptions) error {
	return c.NextComponent().CreateLink(options)
}

func (c *TieredStorage) ReadLink(options internal.ReadLinkOptions) (string, error) {
	return c.NextComponent().ReadLink(options)
}

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
	flock := c.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	localPath, err := c.localPath(options.Name)
	if err != nil {
		return err
	}
	info, localErr := os.Stat(localPath)
	value, tracked := c.fileMap.Load(options.Name)
	cloudBacked := !tracked || value.(*FileNode).cloudBacked.Load() ||
		localErr == nil && info.IsDir()
	if cloudBacked {
		err = c.NextComponent().Chmod(options)
		if err != nil && !(localErr == nil && errors.Is(err, os.ErrNotExist)) {
			return err
		}
	}
	if localErr != nil {
		if cloudBacked {
			return nil
		}
		return localErr
	}
	if err := os.Chmod(localPath, options.Mode); err != nil {
		return err
	}
	if tracked && !value.(*FileNode).cloudBacked.Load() {
		node := value.(*FileNode)
		node.mode.Store(uint32(options.Mode))
		node.modeDirty.Store(true)
	}
	return nil
}

func (c *TieredStorage) Chown(options internal.ChownOptions) error {
	flock := c.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	localPath, err := c.localPath(options.Name)
	if err != nil {
		return err
	}
	info, localErr := os.Stat(localPath)
	value, tracked := c.fileMap.Load(options.Name)
	cloudBacked := !tracked || value.(*FileNode).cloudBacked.Load() ||
		localErr == nil && info.IsDir()
	if cloudBacked {
		err = c.NextComponent().Chown(options)
		if err != nil && !(localErr == nil && errors.Is(err, os.ErrNotExist)) {
			return err
		}
	}
	if localErr != nil {
		if cloudBacked {
			return nil
		}
		return localErr
	}
	if runtime.GOOS != "windows" {
		if err := os.Chown(localPath, options.Owner, options.Group); err != nil {
			return err
		}
	}
	if tracked && !value.(*FileNode).cloudBacked.Load() {
		node := value.(*FileNode)
		node.owner.Store(int64(options.Owner))
		node.group.Store(int64(options.Group))
		node.ownerDirty.Store(true)
	}
	return nil
}

func (c *TieredStorage) TruncateFile(options internal.TruncateFileOptions) error {
	if options.NewSize < 0 {
		return syscall.EINVAL
	}
	if options.Handle == nil {
		handle, err := c.OpenFile(internal.OpenFileOptions{
			Name:  options.Name,
			Flags: os.O_RDWR,
			Mode:  common.DefaultFilePermissionBits,
		})
		if err != nil {
			return err
		}
		options.Handle = handle
		if err := c.TruncateFile(options); err != nil {
			_ = c.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
			return err
		}
		return c.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	}

	name := options.Handle.Path
	flock := c.fileLocks.Get(name)
	flock.Lock()
	defer flock.Unlock()

	file := options.Handle.GetFileObject()
	if file == nil {
		return syscall.EBADF
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if err := c.reserveSpace(max(options.NewSize-info.Size(), 0)); err != nil {
		return err
	}
	if err := file.Truncate(options.NewSize); err != nil {
		return err
	}

	c.setHandleDirty(options.Handle)
	if value, found := c.fileMap.Load(name); found {
		node := value.(*FileNode)
		node.isDirty.Store(true)
		c.cacheSize.Add(options.NewSize - node.size.Swap(options.NewSize))
	}
	return nil
}

func (c *TieredStorage) FileUsed(name string) error {
	return nil
}

func (c *TieredStorage) StatFs() (*common.Statfs_t, bool, error) {
	const blockSize = 4096

	physicalFree, err := c.getAvailableSize()
	if err != nil {
		return nil, false, err
	}
	available := max(int64(c.maxCacheSize)-c.cacheSize.Used(), 0)
	stat := &common.Statfs_t{
		Blocks:  uint64(c.maxCacheSize) / blockSize,
		Bavail:  uint64(available) / blockSize,
		Bfree:   physicalFree / blockSize,
		Bsize:   blockSize,
		Frsize:  blockSize,
		Files:   1e9,
		Ffree:   1e9,
		Namemax: 255,
	}
	return stat, true, nil
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
