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
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/cache_size"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/handlemap"
)

// TieredStorage keeps new data local and uses cloud storage as overflow.
// Cloud objects are cached only while open.
//
// Lock order: evictMu, file locks (lexical order), then lruQueue.mu. Never
// acquire a file lock while holding lruQueue.mu.
type TieredStorage struct {
	internal.BaseComponent

	policy *lruQueue

	fileLocks *common.LockMap // uses object name (common.JoinUnixFilepath)
	tmpPath   string          // uses os.Separator (filepath.Join)

	diskUsage *cache_size.Tracker
	maxSize   float64
}

type TieredStorageOptions struct {
	TmpPath   string  `config:"path"        yaml:"path,omitempty"`
	MaxSizeMB float64 `config:"max-size-mb" yaml:"max-size-mb,omitempty"`

	HighThreshold uint32 `config:"high-threshold" yaml:"high-threshold,omitempty"`
	LowThreshold  uint32 `config:"low-threshold"  yaml:"low-threshold,omitempty"`
}

const (
	compName           = "tiered_storage"
	defaultMaxEviction = 5000

	defaultHighThreshold      = 90
	defaultLowThreshold       = 80
	numUploadWorkers          = 8
	capacityPollInterval      = time.Second
	reconcileCapacityInterval = 5 * time.Minute
	// cacheCopyKey marks a handle opened on a cache/ copy. It is not
	// handlemap.HandleFlagCached, which tells libfuse to read the file object
	// directly.
	cacheCopyKey = "tieredStorage.cacheCopy"
	// The tiered root holds exactly these two trees. A file is in at most one.
	localDirName = "local"
	cacheDirName = "cache"
	// The tiered root holds user data and is only accessed by this process (CWE-732).
	dirPerm os.FileMode = 0700
	// share of free disk space to use when max-size-mb is not set
	defaultMaxSizeRatio = 0.8
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

	if err := c.recoverLocalFiles(); err != nil {
		log.Err("TieredStorage::Start : %v", err)
		return err
	}

	if c.policy != nil {
		if err := c.policy.StartPolicy(); err != nil {
			log.Err("TieredStorage::Start : failed to start LRU policy [%v]", err)
			return err
		}
	}

	return nil
}

// recoverLocalFiles enqueues every file in local/ in the LRU, oldest
// modification time at the tail. Every such file is local-owned.
func (c *TieredStorage) recoverLocalFiles() error {
	if c.policy == nil {
		return nil
	}
	type localFile struct {
		name  string
		mtime time.Time
	}
	var files []localFile
	localRoot := filepath.Join(c.tmpPath, localDirName)
	err := filepath.WalkDir(localRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(localRoot, path)
		if err != nil {
			return err
		}
		files = append(files, localFile{name: filepath.ToSlash(rel), mtime: info.ModTime()})
		return nil
	})
	if err != nil {
		return fmt.Errorf("TieredStorage: failed to scan %s: %w", localRoot, err)
	}

	// Enqueue puts each file at the head, so enqueue oldest first.
	slices.SortFunc(files, func(a, b localFile) int {
		return a.mtime.Compare(b.mtime)
	})
	for _, file := range files {
		c.policy.Enqueue(file.name)
	}
	log.Info("TieredStorage::recoverLocalFiles : found %d local files", len(files))
	return nil
}

// Stop leaves local files in place for the next mount.
func (c *TieredStorage) Stop() error {
	log.Trace("TieredStorage::Stop : Stopping component %s", c.Name())

	if c.policy != nil {
		return c.policy.StopPolicy()
	}

	return nil
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

	if err := c.prepareRoot(); err != nil {
		log.Err("TieredStorage::Configure : %v", err)
		return err
	}
	c.diskUsage = newDiskUsageTracker(c.tmpPath, reconcileCapacityInterval)
	if err := c.diskUsage.Reconcile(); err != nil {
		return fmt.Errorf("TieredStorage: failed to measure %s: %w", c.tmpPath, err)
	}
	if conf.MaxSizeMB == 0 {
		free, _, err := common.GetAvailFree(c.tmpPath)
		if err != nil {
			return fmt.Errorf("TieredStorage: failed to get free space for %s: %w", c.tmpPath, err)
		}
		// local/ persists across mounts, so count it as available.
		conf.MaxSizeMB = math.Floor(
			defaultMaxSizeRatio * float64(free+uint64(c.diskUsage.Used())) / common.MbToBytes,
		)
		log.Info("TieredStorage::Configure : max-size-mb defaulted to %.0f", conf.MaxSizeMB)
	}
	if conf.MaxSizeMB <= 0 {
		return fmt.Errorf("TieredStorage: max-size-mb must be greater than 0")
	}
	c.maxSize = conf.MaxSizeMB * common.MbToBytes

	c.policy = newLRUQueue(lruQueueConfig{
		localRoot:        filepath.Join(c.tmpPath, localDirName),
		maxSize:          c.maxSize,
		fileLocks:        c.fileLocks,
		size:             c.diskUsage,
		threshold:        float64(conf.HighThreshold) / 100,
		targetRatio:      float64(conf.LowThreshold) / 100,
		numWorkers:       numUploadWorkers,
		maxEviction:      defaultMaxEviction,
		pollInterval:     capacityPollInterval,
		uploadandCleanFn: c.uploadandCleanFile,
	})

	return nil
}

// prepareRoot lays out the tiered root and discards cache/ from any previous
// run. Entries other than local/, cache/, and OS-managed volume entries are
// refused rather than guessed at, since they may hold data from an older layout
// or a misconfigured path.
func (c *TieredStorage) prepareRoot() error {
	if err := os.MkdirAll(c.tmpPath, dirPerm); err != nil {
		return fmt.Errorf("TieredStorage: failed to create %s: %w", c.tmpPath, err)
	}
	entries, err := os.ReadDir(c.tmpPath)
	if err != nil {
		return fmt.Errorf("TieredStorage: failed to read %s: %w", c.tmpPath, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == localDirName || name == cacheDirName {
			continue
		}
		if isOSManagedEntry(entry) {
			log.Debug("TieredStorage::prepareRoot : ignoring OS-managed entry %q", name)
			continue
		}
		return fmt.Errorf(
			"TieredStorage: %s contains unexpected entry %q; it may only contain %s and %s (move existing files into %s)",
			c.tmpPath,
			name,
			localDirName,
			cacheDirName,
			filepath.Join(c.tmpPath, localDirName),
		)
	}
	if err := os.RemoveAll(filepath.Join(c.tmpPath, cacheDirName)); err != nil {
		return fmt.Errorf("TieredStorage: failed to clear %s: %w", cacheDirName, err)
	}
	for _, tree := range []string{localDirName, cacheDirName} {
		if err := os.MkdirAll(filepath.Join(c.tmpPath, tree), dirPerm); err != nil {
			return fmt.Errorf("TieredStorage: failed to create %s: %w", tree, err)
		}
	}
	return nil
}

// osManagedRootEntries are created by the OS or filesystem at the root of a
// volume, so they are expected when the tiered root is a volume root. The NTFS
// names also appear when an NTFS or exFAT volume is mounted on Linux.
var osManagedRootEntries = []string{"System Volume Information", "$RECYCLE.BIN", "lost+found"}

// isOSManagedEntry reports whether a tiered root entry belongs to the OS.
func isOSManagedEntry(entry fs.DirEntry) bool {
	for _, name := range osManagedRootEntries {
		if strings.EqualFold(entry.Name(), name) {
			return true
		}
	}
	info, err := entry.Info()
	return err == nil && common.IsProtectedSystemFile(info)
}

// newDiskUsageTracker tracks the local/ and cache/ trees under path. Only the
// trees the component owns are measured, so OS-managed entries that share the
// tiered path are not counted against max-size-mb.
func newDiskUsageTracker(path string, reconcileInterval time.Duration) *cache_size.Tracker {
	return cache_size.NewTracker(
		reconcileInterval,
		filepath.Join(path, localDirName),
		filepath.Join(path, cacheDirName),
	)
}

func (c *TieredStorage) OnConfigChange() {
}

// localPath resolves an object name beneath local/, which holds local-owned files.
func (c *TieredStorage) localPath(name string) string {
	return c.treePath(localDirName, name)
}

// cachePath resolves an object name beneath cache/, which holds disposable
// copies of open cloud-owned files.
func (c *TieredStorage) cachePath(name string) string {
	return c.treePath(cacheDirName, name)
}

// treePath resolves an object name beneath one of the tiered root's trees.
// An empty name resolves to the tree itself. A name that would escape the tree
// resolves to "", which every file operation rejects.
func (c *TieredStorage) treePath(tree, name string) string {
	osName := filepath.FromSlash(name)
	if osName != "" && !filepath.IsLocal(osName) {
		return ""
	}
	return filepath.Join(c.tmpPath, tree, osName)
}

// Directory operations
func (c *TieredStorage) CreateDir(options internal.CreateDirOptions) error {
	if _, err := c.GetAttr(internal.GetAttrOptions{Name: options.Name}); err == nil {
		return syscall.EEXIST
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	localPath := c.localPath(options.Name)
	mode := options.Mode
	if mode == 0 || runtime.GOOS == "windows" {
		mode = common.DefaultDirectoryPermissionBits
	}
	// The parent may exist only in the cloud.
	if err := os.MkdirAll(filepath.Dir(localPath), dirPerm); err != nil {
		return err
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
	localPath := c.localPath(options.Name)
	cachePath := c.cachePath(options.Name)
	if err := c.checkCloudReachable(options.Name); err != nil {
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
	// cache/ is not pruned, so it may hold this directory. Startup clears it
	// if this fails.
	_ = os.Remove(cachePath)

	if errors.Is(localErr, os.ErrNotExist) && errors.Is(cloudErr, os.ErrNotExist) {
		return syscall.ENOENT
	}
	return nil
}

func (c *TieredStorage) IsDirEmpty(options internal.IsDirEmptyOptions) bool {
	localPath := c.localPath(options.Name)
	entries, localErr := os.ReadDir(localPath)
	if localErr == nil && len(entries) > 0 {
		return false
	}
	if localErr != nil && !errors.Is(localErr, os.ErrNotExist) {
		return false
	}
	return c.NextComponent().IsDirEmpty(options)
}

func (c *TieredStorage) StreamDir(
	options internal.StreamDirOptions,
) ([]*internal.ObjAttr, string, error) {
	localPath := c.localPath(options.Name)

	entries, localErr := os.ReadDir(localPath)
	if localErr != nil && !errors.Is(localErr, os.ErrNotExist) {
		return nil, "", localErr
	}
	localExists := localErr == nil
	localNames := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		localNames[entry.Name()] = struct{}{}
	}

	// Local entries win, as in GetAttr. They are listed with the first page,
	// and their names are dropped from every cloud page. cache/ copies match
	// the cloud, so cache/ is never listed.
	var page []*internal.ObjAttr
	if options.Token == "" {
		for _, entry := range entries {
			// On Windows this is the directory data, whose size can lag for an
			// open file, as on NTFS itself.
			info, err := entry.Info()
			if err != nil {
				// Evicted since ReadDir, so the cloud lists it.
				delete(localNames, entry.Name())
				continue
			}
			entryName := common.JoinUnixFilepath(options.Name, entry.Name())
			page = append(page, internal.NewLocalObjAttr(entryName, info))
		}
	}

	for {
		attrs, token, err := c.NextComponent().StreamDir(options)
		if errors.Is(err, os.ErrNotExist) && localExists {
			// A directory that exists only locally may be unknown to the cloud.
			return page, "", nil
		}
		if err != nil {
			return nil, "", err
		}
		// attrs may be attr_cache's cached listing, so it is copied, not edited.
		for _, attr := range attrs {
			if _, isLocal := localNames[attr.Name]; !isLocal {
				page = append(page, attr)
			}
		}
		// libfuse takes an empty page as the end of the listing.
		if len(page) > 0 || token == "" {
			return page, token, nil
		}
		options.Token = token
	}
}

func (c *TieredStorage) RenameDir(options internal.RenameDirOptions) error {
	log.Trace("TieredStorage::RenameDir : src=%s, dst=%s", options.Src, options.Dst)

	// The FUSE layer keeps every other operation under src and dst out until
	// this returns (libfuse's path tree lock; WinFsp's rename lock), so no
	// name can appear under src after it is listed below.

	srcLocal := c.localPath(options.Src)
	srcCache := c.cachePath(options.Src)
	dstLocal := c.localPath(options.Dst)
	dstCache := c.cachePath(options.Dst)

	srcPrefix := common.JoinUnixFilepath(options.Src) + "/"
	names, err := c.filesUnder(options.Src)
	if err != nil {
		return err
	}
	dstName := func(src string) string {
		return common.JoinUnixFilepath(options.Dst, strings.TrimPrefix(src, srcPrefix))
	}
	lockNames := make([]string, 0, len(names)*2)
	for _, src := range names {
		lockNames = append(lockNames, src, dstName(src))
	}
	slices.Sort(lockNames)
	lockNames = slices.Compact(lockNames)
	for _, name := range lockNames {
		c.fileLocks.Get(name).Lock()
	}
	defer func() {
		for _, name := range slices.Backward(lockNames) {
			c.fileLocks.Get(name).Unlock()
		}
	}()

	if err := c.checkCloudReachable(options.Src); err != nil {
		return err
	}
	if _, err := os.Stat(dstLocal); err == nil {
		return syscall.EEXIST
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// cache/ is not pruned, so dst may be an empty leftover there.
	_ = os.Remove(dstCache)

	// Rename locally first: it can fail (on Windows, for open files), but is
	// cheap to undo. The cloud rename runs last.
	type move struct{ from, to string }
	var moved []move
	revert := func() {
		for _, m := range slices.Backward(moved) {
			if err := os.Rename(m.to, m.from); err != nil {
				log.Err(
					"TieredStorage::RenameDir : reverting %s -> %s failed; local and cloud have diverged [%v]",
					m.to,
					m.from,
					err,
				)
			}
		}
	}
	srcLocalExists := false
	for _, m := range []move{{srcLocal, dstLocal}, {srcCache, dstCache}} {
		if _, err := os.Stat(m.from); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			revert()
			return err
		}
		if m.from == srcLocal {
			srcLocalExists = true
		}
		err := os.MkdirAll(filepath.Dir(m.to), dirPerm)
		if err == nil {
			err = os.Rename(m.from, m.to)
		}
		if err != nil {
			revert()
			if runtime.GOOS == "windows" && errors.Is(err, os.ErrPermission) {
				// Windows refuses to rename a directory with open files in it.
				return syscall.EBUSY
			}
			return err
		}
		moved = append(moved, m)
	}

	// A directory that exists only locally may be unknown to the cloud.
	cloudErr := c.NextComponent().RenameDir(options)
	neverUploaded := srcLocalExists && errors.Is(cloudErr, os.ErrNotExist)
	if cloudErr != nil && !neverUploaded {
		log.Err(
			"TieredStorage::RenameDir : %s -> %s cloud rename failed [%v]",
			options.Src,
			options.Dst,
			cloudErr,
		)
		revert()
		return cloudErr
	}

	for _, src := range names {
		dst := dstName(src)
		c.policy.Rename(src, dst)
		sflock, dflock := c.fileLocks.Get(src), c.fileLocks.Get(dst)
		dflock.CloudStale, sflock.CloudStale = sflock.CloudStale, false
		c.renameOpenHandles(src, dst, c.fileLocks.Get(src), c.fileLocks.Get(dst))
	}
	return nil
}

// filesUnder lists the object names of files beneath a directory in local/ and
// cache/.
func (c *TieredStorage) filesUnder(dir string) ([]string, error) {
	var names []string
	for _, tree := range []string{localDirName, cacheDirName} {
		root := c.treePath(tree, dir)
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			if err != nil || !entry.Type().IsRegular() {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			names = append(names, common.JoinUnixFilepath(dir, filepath.ToSlash(rel)))
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return names, nil
}

// checkCloudReachable refuses, before anything changes, a namespace change
// (delete, rename) that might need the cloud while it is disconnected. This
// avoids holding locks through SDK retries and exposing mixed local/cloud
// state. Offline, only a plain not-exist from GetAttr lets the change go
// ahead. It is a hint: callers still handle cloud failures afterwards.
func (c *TieredStorage) checkCloudReachable(name string) error {
	if c.NextComponent().CloudConnected() {
		return nil
	}
	_, err := c.NextComponent().GetAttr(internal.GetAttrOptions{Name: name})
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	log.Err(
		"TieredStorage::checkCloudReachable : refusing to change %s while the cloud is disconnected [%v]",
		name,
		err,
	)
	cloudErr := common.NewCloudUnreachableError(errors.New("cloud storage is disconnected"))
	return &cloudErr
}

// File operations

// createEmptyLocal creates a new, empty local-owned file. The file lock must
// be held.
func (c *TieredStorage) createEmptyLocal(name string, mode os.FileMode) error {
	localPath := c.localPath(name)
	if err := os.MkdirAll(filepath.Dir(localPath), dirPerm); err != nil {
		return err
	}
	file, err := common.OpenFile(
		localPath,
		os.O_CREATE|os.O_EXCL|os.O_WRONLY,
		c.fileMode(mode),
	)
	if err != nil {
		return err
	}
	return file.Close()
}

// openHandle opens a file under the tiered root for an object. cached marks a
// copy in cache/, which must be promoted before it is changed. The file lock
// must be held.
func (c *TieredStorage) openHandle(
	name, path string,
	flags int,
	cached bool,
	flock *common.LockMapItem,
) (*handlemap.Handle, error) {
	file, err := common.OpenFile(path, flags&^(os.O_CREATE|os.O_EXCL), 0)
	if err != nil {
		return nil, err
	}
	handle := handlemap.NewHandle(name)
	handle.SetFileObject(file)
	if flags&os.O_APPEND != 0 {
		handle.Flags.Set(handlemap.HandleOpenedAppend)
	}
	if cached {
		// The handle is not published yet, so it needs no lock.
		handle.SetValue(cacheCopyKey, true)
	}
	flock.Inc()
	return handle, nil
}

// promote makes an open cloud-owned file local-owned before its first
// change: cache/x is renamed to local/x, marked cloudStale, and enqueued in the
// LRU. Open handles stay valid. It does nothing if cache/x is gone, because
// another handle promoted it or it was deleted. The file lock must be held.
func (c *TieredStorage) promote(name string) error {
	cachePath := c.cachePath(name)
	localPath := c.localPath(name)
	if _, err := os.Stat(cachePath); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if _, err := os.Stat(localPath); err == nil {
		// local/x is authoritative, so cache/x is a stray left by a failed
		// cleanup. Its handles refer to a replaced file; never let it win.
		if err := c.removeFile(cachePath); err != nil {
			log.Warn("TieredStorage::promote : failed to remove stray cached %s [%v]", name, err)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(localPath), dirPerm); err != nil {
		return err
	}
	if err := os.Rename(cachePath, localPath); err != nil {
		log.Err("TieredStorage::promote : %s rename to local failed [%v]", name, err)
		return err
	}
	c.fileLocks.Get(name).CloudStale = true
	c.policy.Enqueue(name)
	return nil
}

// promoteHandle promotes the file behind a cached handle, so the handle no
// longer refers to a copy in cache/. The handle's file lock must be held.
func (c *TieredStorage) promoteHandle(handle *handlemap.Handle) error {
	if !c.onCacheCopy(handle) {
		return nil
	}
	if err := c.promote(handle.Path); err != nil {
		return err
	}
	handle.Lock()
	handle.RemoveValue(cacheCopyKey)
	handle.Unlock()
	return nil
}

// onCacheCopy reports whether a handle was opened on a cache/ copy and has not
// been promoted since. It is a hint that lets writes skip promotion without
// the file lock; promote decides by location under the lock. It is per handle
// because a rename can change a handle's name while a write checks it, but
// never moves its file back into cache/, so a stale hint only costs one lock
// and stat.
func (c *TieredStorage) onCacheCopy(handle *handlemap.Handle) bool {
	handle.RLock()
	defer handle.RUnlock()
	_, found := handle.GetValue(cacheCopyKey)
	return found
}

func (c *TieredStorage) CreateFile(
	options internal.CreateFileOptions,
) (*handlemap.Handle, error) {
	flock := c.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	_, err := c.GetAttr(internal.GetAttrOptions{Name: options.Name})
	if err == nil {
		return nil, syscall.EEXIST
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	if err := c.createEmptyLocal(options.Name, options.Mode); err != nil {
		return nil, err
	}
	localPath := c.localPath(options.Name)
	handle, err := c.openHandle(options.Name, localPath, os.O_RDWR, false, flock)
	if err != nil {
		return nil, err
	}
	c.setHandleDirty(handle)
	return handle, nil
}

func (c *TieredStorage) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("TieredStorage::DeleteFile : name=%s", options.Name)

	flock := c.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	localPath := c.localPath(options.Name)
	cachePath := c.cachePath(options.Name)
	if err := c.checkCloudReachable(options.Name); err != nil {
		return err
	}

	// Delete in the cloud first, so a stale cloud copy cannot resurface once
	// the local copy is gone. A local-owned file may never have been uploaded.
	cloudErr := c.NextComponent().DeleteFile(options)
	if cloudErr != nil && !errors.Is(cloudErr, os.ErrNotExist) {
		return cloudErr
	}
	found := cloudErr == nil

	// Open handles stay valid after the file is removed.
	for _, path := range []string{localPath, cachePath} {
		err := c.removeFile(path)
		if err == nil {
			found = true
		} else if !errors.Is(err, os.ErrNotExist) {
			log.Err(
				"TieredStorage::DeleteFile : %s deleted in cloud, but not locally [%v]",
				options.Name,
				err,
			)
			return err
		}
	}
	c.policy.Dequeue(options.Name)
	flock.CloudStale = false

	if !found {
		return syscall.ENOENT
	}
	return nil
}

func (c *TieredStorage) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	flock := c.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	localPath := c.localPath(options.Name)
	cachePath := c.cachePath(options.Name)
	exclusive := options.Flags&os.O_CREATE != 0 && options.Flags&os.O_EXCL != 0
	truncate := options.Flags&os.O_TRUNC != 0

	path, cached, created := "", false, false
	var truncatedSize int64
	if info, err := os.Stat(localPath); err == nil {
		if exclusive {
			return nil, syscall.EEXIST
		}
		path = localPath
		truncatedSize = info.Size()
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	} else if info, err := os.Stat(cachePath); err == nil {
		if exclusive {
			return nil, syscall.EEXIST
		}
		// Another handle has this cloud-owned file open.
		path, cached = cachePath, true
		if truncate {
			if err := c.promote(options.Name); err != nil {
				return nil, err
			}
			path, cached = localPath, false
			truncatedSize = info.Size()
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	} else {
		attrs, err := c.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Name})
		switch {
		case errors.Is(err, os.ErrNotExist):
			if options.Flags&os.O_CREATE == 0 {
				return nil, syscall.ENOENT
			}
			if err := c.createEmptyLocal(options.Name, options.Mode); err != nil {
				return nil, err
			}
			path, created = localPath, true
		case err != nil:
			return nil, err
		case exclusive:
			return nil, syscall.EEXIST
		case truncate:
			// The old content is discarded, so there is nothing to download.
			if err := c.createEmptyLocal(options.Name, attrs.Mode.Perm()); err != nil {
				return nil, err
			}
			flock.CloudStale = true
			path = localPath
		default:
			if err := c.downloadToCache(options.Name, attrs.Mode.Perm()); err != nil {
				return nil, err
			}
			path, cached = cachePath, true
		}
	}

	handle, err := c.openHandle(options.Name, path, options.Flags, cached, flock)
	if err != nil {
		return nil, err
	}
	if truncate {
		c.diskUsage.Add(-truncatedSize)
	}
	if created || truncate {
		c.setHandleDirty(handle)
	}
	return handle, nil
}

func (c *TieredStorage) fileMode(mode os.FileMode) os.FileMode {
	if mode == 0 || runtime.GOOS == "windows" {
		return common.DefaultFilePermissionBits
	}
	return mode
}

// downloadToCache copies a cloud object into cache/. The object's file lock
// must be held, which keeps other openers from seeing a partial download. A
// failed download is removed; startup removes any left by a crash.
func (c *TieredStorage) downloadToCache(name string, mode os.FileMode) error {
	cachePath := c.cachePath(name)
	if err := os.MkdirAll(filepath.Dir(cachePath), dirPerm); err != nil {
		return err
	}
	// The mode matters if the file is later promoted to local/.
	file, err := common.OpenFile(
		cachePath,
		os.O_CREATE|os.O_EXCL|os.O_WRONLY,
		c.fileMode(mode),
	)
	if err != nil {
		return err
	}

	err = c.NextComponent().CopyToFile(internal.CopyToFileOptions{Name: name, File: file})
	var size int64
	if err == nil {
		var info os.FileInfo
		if info, err = file.Stat(); err == nil {
			size = info.Size()
		}
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		log.Err("TieredStorage::downloadToCache : %s download failed [%v]", name, err)
		_ = os.Remove(cachePath)
		return err
	}

	c.diskUsage.Add(size)
	return nil
}

// removeFile deletes a file under the tiered root and updates the usage
// counter.
func (c *TieredStorage) removeFile(path string) error {
	info, statErr := os.Stat(path)
	err := os.Remove(path)
	if err == nil && statErr == nil {
		c.diskUsage.Add(-info.Size())
	}
	return err
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
	if c.onCacheCopy(options.Handle) {
		flock := c.fileLocks.Get(options.Handle.Path)
		flock.Lock()
		err := c.promoteHandle(options.Handle)
		flock.Unlock()
		if err != nil {
			return 0, err
		}
	}

	// Concurrent writes that extend the same file can count the growth
	// twice. Reconcile corrects such drift.
	before, beforeErr := f.Stat()

	appending := options.Handle.Flags.IsSet(handlemap.HandleOpenedAppend)
	var bytesWritten int
	var err error
	if appending {
		bytesWritten, err = f.Write(options.Data)
	} else {
		bytesWritten, err = f.WriteAt(options.Data, options.Offset)
	}

	if bytesWritten > 0 {
		c.setHandleDirty(options.Handle)
		if after, statErr := f.Stat(); beforeErr == nil && statErr == nil {
			c.diskUsage.Add(after.Size() - before.Size())
		}
	}
	if err != nil {
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
	name := options.Handle.Path

	flock.Dec()

	var closeErr error
	if f := options.Handle.GetFileObject(); f != nil {
		closeErr = f.Close()
	}

	c.clearHandleDirty(options.Handle)
	options.Handle.Cleanup()

	handlemap.Delete(options.Handle.ID)

	if flock.Count() > 0 {
		return closeErr
	}

	cachePath := c.cachePath(name)
	localPath := c.localPath(name)
	if _, err := os.Stat(cachePath); err == nil {
		// The copy is clean, so it can be dropped. If that fails, the next
		// release or startup removes it.
		if err := c.removeFile(cachePath); err != nil {
			log.Warn("TieredStorage::ReleaseFile : failed to remove cached %s [%v]", name, err)
		}
	}
	if _, err := os.Stat(localPath); err == nil {
		c.policy.Enqueue(name)
		if flock.CloudStale {
			c.policy.EvictOnClose(name)
		}
	}
	return closeErr
}

// uploadLocal copies local/x to the cloud, with its mode and owner where the
// platform has them. The file lock must be held.
func (c *TieredStorage) uploadLocal(name string) error {
	localPath := c.localPath(name)
	f, err := common.Open(localPath)
	if err != nil {
		log.Err("TieredStorage::uploadLocal : %s open failed [%v]", name, err)
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}

	err = c.NextComponent().CopyFromFile(internal.CopyFromFileOptions{Name: name, File: f})
	if err != nil {
		log.Err("TieredStorage::uploadLocal : %s upload failed [%v]", name, err)
		return err
	}

	// Windows files carry no POSIX mode or owner, so applying them would
	// overwrite the cloud object's metadata with defaults.
	owner, group, ok := common.GetFileOwner(info)
	if !ok {
		return nil
	}
	err = c.NextComponent().Chmod(internal.ChmodOptions{Name: name, Mode: info.Mode().Perm()})
	if err != nil && !errors.Is(err, syscall.ENOTSUP) {
		log.Err("TieredStorage::uploadLocal : %s chmod failed [%v]", name, err)
		return err
	}
	err = c.NextComponent().Chown(internal.ChownOptions{Name: name, Owner: owner, Group: group})
	if err != nil && !errors.Is(err, syscall.ENOTSUP) {
		log.Err("TieredStorage::uploadLocal : %s chown failed [%v]", name, err)
		return err
	}
	return nil
}

// uploadandCleanFile evicts a closed local-owned file: it uploads local/x,
// then deletes it. It is the LRU's eviction callback, so it runs with the
// object's file lock held.
func (c *TieredStorage) uploadandCleanFile(name string) error {
	if err := c.uploadLocal(name); err != nil {
		return err
	}
	localPath := c.localPath(name)
	if err := c.removeFile(localPath); err != nil {
		log.Err("TieredStorage::uploadandCleanFile : %s remove failed [%v]", name, err)
		return err
	}
	c.fileLocks.Get(name).CloudStale = false
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

	srcLocal := c.localPath(options.Src)
	srcCache := c.cachePath(options.Src)
	dstLocal := c.localPath(options.Dst)
	dstCache := c.cachePath(options.Dst)
	if err := c.checkCloudReachable(options.Src); err != nil {
		return err
	}
	dstInfo, dstLocalErr := os.Stat(dstLocal)
	if dstLocalErr == nil && dstInfo.IsDir() {
		return syscall.EISDIR
	}

	// Rename whichever local copy src has, before the harder-to-undo cloud rename.
	_, srcLocalErr := os.Stat(srcLocal)
	_, srcCacheErr := os.Stat(srcCache)
	from, to := "", ""
	switch {
	case srcLocalErr == nil:
		log.Debug("TieredStorage::RenameFile : %s -> %s src is local", options.Src, options.Dst)
		from, to = srcLocal, dstLocal
	case !errors.Is(srcLocalErr, os.ErrNotExist):
		log.Err("TieredStorage::RenameFile : %s local stat failed [%v]", options.Src, srcLocalErr)
		return srcLocalErr
	case srcCacheErr == nil && dstLocalErr == nil:
		log.Debug("TieredStorage::RenameFile : %s -> %s cache -> local", options.Src, options.Dst)
		from, to = srcCache, dstLocal
	case srcCacheErr == nil:
		log.Debug("TieredStorage::RenameFile : %s -> %s cache -> cache", options.Src, options.Dst)
		from, to = srcCache, dstCache
	case !errors.Is(srcCacheErr, os.ErrNotExist):
		log.Err("TieredStorage::RenameFile : %s cache stat failed [%v]", options.Src, srcCacheErr)
		return srcCacheErr
	}
	if from != "" {
		log.Debug("TieredStorage::RenameFile : %s -> %s cloud only", options.Src, options.Dst)
		if err := os.MkdirAll(filepath.Dir(to), dirPerm); err != nil {
			return err
		}
		replaced, replacedErr := os.Stat(to)
		if err := os.Rename(from, to); err != nil {
			return err
		}
		// The rename overwrote dst's copy in this tree, even if later reverted.
		if replacedErr == nil {
			c.diskUsage.Add(-replaced.Size())
		}
	}

	cloudErr := c.NextComponent().RenameFile(options)
	neverUploaded := from == srcLocal && errors.Is(cloudErr, os.ErrNotExist)
	if neverUploaded {
		// don't leave a dangling object at dst in the cloud
		_ = c.NextComponent().DeleteFile(internal.DeleteFileOptions{Name: options.Dst})
	} else if cloudErr != nil && from != "" {
		// revert local rename if cloud rename failed
		if err := os.Rename(to, from); err != nil {
			// complain loudly if names diverge
			log.Err(
				"TieredStorage::RenameFile : %s -> %s failed in cloud, and revert %s -> %s failed; local and cloud have diverged! [%v]",
				options.Src,
				options.Dst,
				to,
				from,
				err,
			)
		}
		return cloudErr
	}

	// dst may have had a copy in the other tree. It was replaced, so drop it.
	for _, path := range []string{dstLocal, dstCache} {
		if path == to {
			continue
		}
		if err := c.removeFile(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Err("TieredStorage::RenameFile : failed to remove replaced %s [%v]", path, err)
		}
	}

	c.policy.Rename(options.Src, options.Dst)
	dflock.CloudStale = sflock.CloudStale && !neverUploaded
	sflock.CloudStale = false
	c.renameOpenHandles(options.Src, options.Dst, sflock, dflock)
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
			// TODO: do we have to lock before checking the name?
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

// GetAttr answers from local/ when the file is local-owned, without a cloud
// call. A cache/ copy matches the cloud object, so it is never consulted.
// It takes no lock: files move between trees by atomic rename.
func (c *TieredStorage) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	localPath := c.localPath(options.Name)
	info, err := os.Stat(localPath)
	if err == nil {
		return internal.NewLocalObjAttr(options.Name, info), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return c.NextComponent().GetAttr(options)
}

func (c *TieredStorage) Chmod(options internal.ChmodOptions) error {
	flock := c.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	return c.changeAttr(
		options.Name,
		func() error { return c.NextComponent().Chmod(options) },
		func(path string) error { return os.Chmod(path, options.Mode) },
	)
}

func (c *TieredStorage) Chown(options internal.ChownOptions) error {
	flock := c.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	return c.changeAttr(
		options.Name,
		func() error { return c.NextComponent().Chown(options) },
		func(path string) error {
			if runtime.GOOS == "windows" {
				return nil
			}
			return os.Chown(path, options.Owner, options.Group)
		},
	)
}

// changeAttr applies a metadata change to the authoritative copy:
//   - local/x: only the local file. Eviction uploads the change with the file.
//   - cache/x: the cloud object, then the copy, which may later be promoted.
//   - neither: only the cloud object.
//
// Directories exist in both places, so both are changed. The file lock must
// be held.
func (c *TieredStorage) changeAttr(
	name string,
	changeCloud func() error,
	changeLocal func(path string) error,
) error {
	localPath := c.localPath(name)
	cachePath := c.cachePath(name)

	info, err := os.Stat(localPath)
	switch {
	case err == nil && !info.IsDir():
		return changeLocal(localPath)
	case err == nil:
		if err := changeCloud(); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return changeLocal(localPath)
	case !errors.Is(err, os.ErrNotExist):
		return err
	}

	if err := changeCloud(); err != nil {
		return err
	}
	if _, err := os.Stat(cachePath); err == nil {
		if err := changeLocal(cachePath); err != nil {
			log.Warn(
				"TieredStorage::changeAttr : %s changed in cloud, but not in cache [%v]",
				name,
				err,
			)
		}
	}
	return nil
}

func (c *TieredStorage) TruncateFile(options internal.TruncateFileOptions) error {
	if options.NewSize < 0 {
		return syscall.EINVAL
	}
	if options.Handle == nil {
		flags := os.O_RDWR
		if options.NewSize == 0 {
			flags |= os.O_TRUNC
		}
		handle, err := c.OpenFile(internal.OpenFileOptions{
			Name:  options.Name,
			Flags: flags,
			Mode:  common.DefaultFilePermissionBits,
		})
		if err != nil {
			return err
		}
		if options.NewSize != 0 {
			options.Handle = handle
			if err := c.TruncateFile(options); err != nil {
				_ = c.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
				return err
			}
		}
		return c.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	}

	flock := c.fileLocks.Get(options.Handle.Path)
	flock.Lock()
	defer flock.Unlock()

	file := options.Handle.GetFileObject()
	if file == nil {
		return syscall.EBADF
	}
	if err := c.promoteHandle(options.Handle); err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if err := file.Truncate(options.NewSize); err != nil {
		return err
	}

	c.setHandleDirty(options.Handle)
	c.diskUsage.Add(options.NewSize - info.Size())
	return nil
}

// FileUsed refreshes a local-owned file's LRU position.
func (c *TieredStorage) FileUsed(name string) error {
	localPath := c.localPath(name)
	if _, err := os.Stat(localPath); err == nil {
		c.policy.Enqueue(name)
	}
	return nil
}

func (c *TieredStorage) StatFs() (*common.Statfs_t, bool, error) {
	const blockSize = 4096

	physicalFree, _, err := common.GetAvailFree(c.tmpPath)
	if err != nil {
		return nil, false, err
	}
	available := min(uint64(max(int64(c.maxSize)-c.diskUsage.Used(), 0)), physicalFree)
	stat := &common.Statfs_t{
		Blocks:  uint64(c.maxSize) / blockSize,
		Bavail:  available / blockSize,
		Bfree:   available / blockSize,
		Bsize:   blockSize,
		Frsize:  blockSize,
		Files:   1e9,
		Ffree:   1e9,
		Namemax: 255,
	}
	return stat, true, nil
}

func (c *TieredStorage) GetFileBlockOffsets(
	options internal.GetFileBlockOffsetsOptions,
) (*common.BlockOffsetList, error) {
	return nil, syscall.ENOTSUP
}

func (c *TieredStorage) GetCommittedBlockList(
	name string,
) (*internal.CommittedBlockList, error) {
	return nil, syscall.ENOTSUP
}

func (c *TieredStorage) StageData(options internal.StageDataOptions) error {
	return syscall.ENOTSUP
}

func (c *TieredStorage) CommitData(options internal.CommitDataOptions) error {
	return syscall.ENOTSUP
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
