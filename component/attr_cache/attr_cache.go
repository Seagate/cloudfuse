/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.

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

package attr_cache

import (
	"context"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/handlemap"
)

// By default attr cache is valid for 120 seconds
const defaultAttrCacheTimeout uint32 = (120)

// Common structure for AttrCache Component
type AttrCache struct {
	internal.BaseComponent
	cacheTimeout uint32
	cacheOnList  bool
	noSymlinks   bool
	cacheDirs    bool
	maxFiles     int
	cacheMap     *attrCacheItem
	cacheLock    sync.RWMutex
}

// Structure defining your config parameters
type AttrCacheOptions struct {
	Timeout       uint32 `config:"timeout-sec" yaml:"timeout-sec,omitempty"`
	NoCacheOnList bool   `config:"no-cache-on-list" yaml:"no-cache-on-list,omitempty"`
	NoSymlinks    bool   `config:"no-symlinks" yaml:"no-symlinks,omitempty"`
	NoCacheDirs   bool   `config:"no-cache-dirs" yaml:"no-cache-dirs,omitempty"`

	//maximum file attributes overall to be cached
	MaxFiles int `config:"max-files" yaml:"max-files,omitempty"`

	// support v1
	CacheOnList bool `config:"cache-on-list"`
}

const compName = "attr_cache"

// caching only first 5 mil files by default
// caching more means increased memory usage of the process
const defaultMaxFiles = 5000000 // 5 million max files overall to be cached

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &AttrCache{}

func (ac *AttrCache) Name() string {
	return compName
}

func (ac *AttrCache) SetName(name string) {
	ac.BaseComponent.SetName(name)
}

func (ac *AttrCache) SetNextComponent(nc internal.Component) {
	ac.BaseComponent.SetNextComponent(nc)
}

func (ac *AttrCache) Priority() internal.ComponentPriority {
	return internal.EComponentPriority.LevelTwo()
}

// Start : Pipeline calls this method to start the component functionality
//
//	this shall not block the call otherwise pipeline will not start
func (ac *AttrCache) Start(ctx context.Context) error {
	log.Trace("AttrCache::Start : Starting component %s", ac.Name())

	// AttrCache : start code goes here
	rootAttr := internal.CreateObjAttrDir("")
	ac.cacheMap = newAttrCacheItem(rootAttr, true, time.Now())

	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (ac *AttrCache) Stop() error {
	log.Trace("AttrCache::Stop : Stopping component %s", ac.Name())

	return nil
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (ac *AttrCache) Configure(_ bool) error {
	log.Trace("AttrCache::Configure : %s", ac.Name())

	// >> If you do not need any config parameters remove below code and return nil
	conf := AttrCacheOptions{}
	err := config.UnmarshalKey(ac.Name(), &conf)
	if err != nil {
		log.Err("AttrCache::Configure : config error [invalid config attributes]")
		return fmt.Errorf("config error in %s [%s]", ac.Name(), err.Error())
	}

	if config.IsSet(compName + ".timeout-sec") {
		ac.cacheTimeout = conf.Timeout
	} else {
		ac.cacheTimeout = defaultAttrCacheTimeout
	}

	if config.IsSet(compName + ".cache-on-list") {
		ac.cacheOnList = conf.CacheOnList
	} else {
		ac.cacheOnList = !conf.NoCacheOnList
	}

	if config.IsSet(compName + ".max-files") {
		ac.maxFiles = conf.MaxFiles
	} else {
		ac.maxFiles = defaultMaxFiles
	}

	ac.noSymlinks = conf.NoSymlinks
	ac.cacheDirs = !conf.NoCacheDirs

	log.Info("AttrCache::Configure : cache-timeout %d, symlink %t, cache-on-list %t",
		ac.cacheTimeout, ac.noSymlinks, ac.cacheOnList)

	return nil
}

// OnConfigChange : If component has registered, on config file change this method is called
func (ac *AttrCache) OnConfigChange() {
	log.Trace("AttrCache::OnConfigChange : %s", ac.Name())
	_ = ac.Configure(true)
}

// Helper Methods
// deleteDirectory: Marks a directory and all its contents deleted.
// This should only be called when ac.cacheDirs is false.
// This marks items deleted instead of invalidating them.
// That way if a request came in for a deleted item, we can respond from the cache.
func (ac *AttrCache) deleteDirectory(path string, time time.Time) {
	//get attrCacheItem
	toBeDeleted, getErr := ac.cacheMap.get(path)
	if getErr != nil || !toBeDeleted.exists() {
		log.Warn("AttrCache::deleteDirectory : %s directory does not exist", path)
		return
	}
	toBeDeleted.markDeleted(time)
}

// deleteCachedDirectory: marks a directory and all its contents deleted
// while keeping directory cache coherent.
// This should only be called when ac.cacheDirs is true.
// This marks items deleted instead of invalidating their entries.
// That way if a request comes in for a deleted item, it's still a cache hit.
func (ac *AttrCache) deleteCachedDirectory(path string, time time.Time) error {

	// Delete all descendants of the path, then delete the path

	//get attrCacheItem
	toBeDeleted, getErr := ac.cacheMap.get(path)
	if getErr != nil || !toBeDeleted.exists() {
		log.Err("AttrCache::deleteCachedDirectory : %s not found", path)
		return syscall.ENOENT
	}
	toBeDeleted.markDeleted(time)

	ac.updateAncestorsInCloud(getParentDir(path), time)
	return nil
}

// pathExistsInCache: check if path is in cache, is valid, and not marked deleted
func (ac *AttrCache) pathExistsInCache(path string) bool {
	value, getErr := ac.cacheMap.get(path)
	if getErr != nil {
		return false
	}
	return value.exists()
}

func getParentDir(childPath string) string {
	parentDir := path.Dir(internal.TruncateDirName(childPath))
	if parentDir == "." {
		parentDir = ""
	}
	return parentDir
}

// invalidateDirectory: Marks a directory and all its contents invalid
// Do not use this with ac.cacheDirs set
func (ac *AttrCache) invalidateDirectory(path string) {
	// Invalidate all descendants of the path, then invalidate the path

	toBeInvalid, getErr := ac.cacheMap.get(path)
	if getErr != nil || !toBeInvalid.valid() {
		log.Warn("AttrCache::invalidateDirectory : %s entry not found or already invalid", path)
		return
	}
	// only invalidate directories when cacheDirs is false
	if !ac.cacheDirs || !toBeInvalid.attr.IsDir() {
		toBeInvalid.invalidate()
		return
	}

	// recurse
	for _, childItem := range toBeInvalid.children {
		ac.invalidateDirectory(childItem.attr.Path)
	}
}

// moveAttrItem: used to move a subtree within cacheMap to a new location of the cacheMap tree.
// input: attrCacheItem to be moved, source and destination path, move timestamp
func (ac *AttrCache) moveAttrCachedItem(srcItem *attrCacheItem, srcDir string, dstDir string, time time.Time) *attrCacheItem {

	// take the source name and change it to the destination name
	dstPath := strings.Replace(srcItem.attr.Path, srcDir, dstDir, 1)

	// create an attribute using the destination name
	var dstAttr *internal.ObjAttr
	if srcItem.attr.IsDir() {
		dstAttr = internal.CreateObjAttrDir(dstPath)
	} else {
		dstAttr = internal.CreateObjAttr(dstPath, srcItem.attr.Size, srcItem.attr.Mtime)
	}

	// insert the attribute from previous step into the cacheMap
	dstItem := ac.cacheMap.insert(dstAttr, srcItem.exists(), srcItem.cachedAt)

	// mark whether the item is in the cloud
	dstItem.markInCloud(srcItem.isInCloud())

	// recurse over children
	for _, srcChildItm := range srcItem.children {
		ac.moveAttrCachedItem(srcChildItm, srcDir, dstDir, time)
	}
	srcItem.markDeleted(time)
	return dstItem
}

func (ac *AttrCache) markAncestorsInCloud(dirPath string, time time.Time) {
	if len(dirPath) != 0 {
		dirCacheItem, getErr := ac.cacheMap.get(dirPath)
		if getErr != nil || !dirCacheItem.exists() {
			log.Warn("AttrCache::markAncestorsInCloud : Adding parent directory %s", dirPath)
			dirObjAttr := internal.CreateObjAttrDir(dirPath)
			dirCacheItem = ac.cacheMap.insert(dirObjAttr, true, time)
		}

		dirCacheItem.markInCloud(true)
		// recurse
		ac.markAncestorsInCloud(getParentDir(dirPath), time)
	}
}

// ------------------------- Methods implemented by this component -------------------------------------------
// CreateDir: Mark the directory invalid, or
// insert the dir item into cache when cacheDirs is true.
func (ac *AttrCache) CreateDir(options internal.CreateDirOptions) error {
	log.Trace("AttrCache::CreateDir : %s", options.Name)
	err := ac.NextComponent().CreateDir(options)
	if err == nil {
		ac.cacheLock.Lock()
		defer ac.cacheLock.Unlock()
		// does the directory already exist?
		oldDirAttrCacheItem, getErr := ac.cacheMap.get(options.Name)
		directoryAlreadyExists := getErr == nil && oldDirAttrCacheItem.exists()
		// if the attribute cache tracks directory existence
		// then prevent redundant directory creation
		if ac.cacheDirs && directoryAlreadyExists {
			return os.ErrExist
		}
		// invalidate existing directory entry (this is redundant but readable)
		if getErr == nil {
			oldDirAttrCacheItem.invalidate()
		}
		// add (or replace) the directory entry
		newDirAttr := internal.CreateObjAttrDir(options.Name)
		newDirAttrCacheItem := ac.cacheMap.insert(newDirAttr, true, time.Now())
		// update flags for tracking directory existence
		if ac.cacheDirs {
			newDirAttrCacheItem.markInCloud(false)
		}
	}
	return err
}

// DeleteDir: Mark the directory deleted and mark all it's children deleted
func (ac *AttrCache) DeleteDir(options internal.DeleteDirOptions) error {
	log.Trace("AttrCache::DeleteDir : %s", options.Name)

	deletionTime := time.Now()
	err := ac.NextComponent().DeleteDir(options)

	if err == nil {
		if ac.cacheDirs {
			// deleteCachedDirectory may add the parent directory to the cache
			// so we must lock the cache for writing
			ac.cacheLock.Lock()
			defer ac.cacheLock.Unlock()
			err = ac.deleteCachedDirectory(options.Name, deletionTime)
		} else {
			ac.cacheLock.Lock()
			defer ac.cacheLock.Unlock()
			ac.deleteDirectory(options.Name, deletionTime)
		}
	}

	return err
}

// ReadDir : Optionally cache attributes of paths returned by next component
// If cacheDirs is true, then directory cache results are merged into the results from the next component
func (ac *AttrCache) ReadDir(options internal.ReadDirOptions) (pathList []*internal.ObjAttr, err error) {
	log.Trace("AttrCache::ReadDir : %s", options.Name)

	pathList, err = ac.NextComponent().ReadDir(options)
	if err == nil {
		ac.cacheAttributes(pathList)
		if ac.cacheDirs {
			// remember that this directory is in cloud storage
			if len(pathList) > 0 {
				ac.markAncestorsInCloud(options.Name, time.Now())
			}
			// merge directory cache into the results
			var numAdded int // prevent shadowing pathList in following line
			pathList, numAdded = ac.addDirsNotInCloudToListing(options.Name, pathList)
			log.Trace("AttrCache::ReadDir : %s +%d from cache = %d",
				options.Name, numAdded, len(pathList))
		}
	}
	return pathList, err
}

// merge results from our cache into pathMap
func (ac *AttrCache) addDirsNotInCloudToListing(listPath string, pathList []*internal.ObjAttr) ([]*internal.ObjAttr, int) {
	numAdded := 0

	dir, getErr := ac.cacheMap.get(listPath)
	if getErr != nil || !dir.exists() {
		log.Err("AttrCache:: addDirsNotInCloudToListing : %s does not exist in cache", listPath)
		return pathList, 0
	}

	ac.cacheLock.RLock()
	for _, child := range dir.children {
		if child.exists() && !child.isInCloud() {
			pathList = append(pathList, child.attr)
			numAdded++
		}
	}
	ac.cacheLock.RUnlock()

	// values should be returned in ascending order by key
	// sort the list before returning it
	sort.Slice(pathList, func(i, j int) bool {
		return pathList[i].Path < pathList[j].Path
	})

	return pathList, numAdded
}

// StreamDir : Optionally cache attributes of paths returned by next component
func (ac *AttrCache) StreamDir(options internal.StreamDirOptions) ([]*internal.ObjAttr, string, error) {
	log.Trace("AttrCache::StreamDir : %s", options.Name)

	pathList, token, err := ac.NextComponent().StreamDir(options)
	if err == nil {
		// strip symlink attributes
		if ac.noSymlinks {
			for _, attr := range pathList {
				if attr.IsSymlink() {
					attr.Flags.Clear(internal.PropFlagSymlink)
				}
			}
		}
		// TODO: will limiting the number of items cached cause bugs when cacheDirs is enabled?
		ac.cacheAttributes(pathList)

		// merge missing directory cache into the last page of results
		if ac.cacheDirs && token == "" {
			var numAdded int // prevent shadowing pathList in following line
			pathList, numAdded = ac.addDirsNotInCloudToListing(options.Name, pathList)
			log.Trace("AttrCache::StreamDir : %s +%d from cache = %d",
				options.Name, numAdded, len(pathList))
		}
	}

	return pathList, token, err
}

// cacheAttributes : On dir listing cache the attributes for all files
// this will lock and release the mutex for writing
func (ac *AttrCache) cacheAttributes(pathList []*internal.ObjAttr) {
	// Check whether or not we are supposed to cache on list
	if ac.cacheOnList && len(pathList) > 0 {
		// Putting this inside loop is heavy as for each item we will do a kernel call to get current time
		// If there are millions of blobs then cost of this is very high.
		currTime := time.Now()

		ac.cacheLock.Lock()
		defer ac.cacheLock.Unlock()
		for _, attr := range pathList {
			ac.cacheMap.insert(attr, true, currTime)
		}
		// pathList was returned by the cloud storage component when listing a directory
		// so that directory is clearly in the cloud
		ac.markAncestorsInCloud(getParentDir(pathList[0].Path), currTime)
	}
}

// IsDirEmpty: Whether or not the directory is empty
func (ac *AttrCache) IsDirEmpty(options internal.IsDirEmptyOptions) bool {
	log.Trace("AttrCache::IsDirEmpty : %s", options.Name)

	// This function only has a use if we're caching directories
	if !ac.cacheDirs {
		log.Debug("AttrCache::IsDirEmpty : %s Dir cache is disabled. Checking with container", options.Name)
		return ac.NextComponent().IsDirEmpty(options)
	}
	// Is the directory in our cache?
	ac.cacheLock.RLock()
	pathInCache := ac.pathExistsInCache(options.Name)
	ac.cacheLock.RUnlock()
	// If the directory does not exist in the attribute cache then let the next component answer
	if !pathInCache {
		log.Debug("AttrCache::IsDirEmpty : %s not found in attr_cache. Checking with container", options.Name)
		return ac.NextComponent().IsDirEmpty(options)
	}
	log.Debug("AttrCache::IsDirEmpty : %s found in attr_cache", options.Name)
	// Check if the cached directory is empty or not
	if ac.anyContentsInCache(options.Name) {
		log.Debug("AttrCache::IsDirEmpty : %s has a subpath in attr_cache", options.Name)
		return false
	}
	// Dir is in cache but no contents are, so check with container
	log.Debug("AttrCache::IsDirEmpty : %s children not found in cache. Checking with container", options.Name)
	return ac.NextComponent().IsDirEmpty(options)
}

func (ac *AttrCache) anyContentsInCache(prefix string) bool {
	ac.cacheLock.RLock()
	defer ac.cacheLock.RUnlock()

	directory, getErr := ac.cacheMap.get(prefix)
	if getErr == nil && directory.exists() {
		for _, chldItem := range directory.children {
			if chldItem.exists() {
				return true
			}
		}
	}
	return false
}

// RenameDir : Mark the source directory and all its contents deleted.
// Invalidate the destination since we may have overwritten it.
func (ac *AttrCache) RenameDir(options internal.RenameDirOptions) error {
	log.Trace("AttrCache::RenameDir : %s -> %s", options.Src, options.Dst)

	currentTime := time.Now()
	err := ac.NextComponent().RenameDir(options)

	if err == nil {
		ac.cacheLock.Lock()
		defer ac.cacheLock.Unlock()

		// TLDR: Dst is guaranteed to be non-existent or empty.
		// Note: We do not need to invalidate children of Dst due to the logic in our FUSE connector, see comments there,
		// but it is always safer to double check than not.
		ac.invalidateDirectory(options.Dst)

		// if attr_cache is tracking directories, validate this rename
		if ac.cacheDirs {
			// First, check if the destination directory already exists
			if ac.pathExistsInCache(options.Dst) {
				return os.ErrExist
			}
		}

		// get the source directory
		srcItem, getErr := ac.cacheMap.get(options.Src)
		if getErr != nil || !srcItem.exists() {
			log.Err("AttrCache::RenameDir : %s source not found", options.Src)
			if ac.cacheDirs {
				return syscall.ENOENT
			}
		} else {
			// move everything over
			srcDir := internal.TruncateDirName(options.Src)
			dstDir := internal.TruncateDirName(options.Dst)
			ac.moveAttrCachedItem(srcItem, srcDir, dstDir, currentTime)
		}
	}

	return err
}

// CreateFile: Cache a new entry for the file
func (ac *AttrCache) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	log.Trace("AttrCache::CreateFile : %s", options.Name)
	h, err := ac.NextComponent().CreateFile(options)

	if err == nil {
		currentTime := time.Now()
		// TODO: the cache locks are used incorrectly here
		// They routinely lock the cache for reading, but then write to it
		ac.cacheLock.Lock()
		defer ac.cacheLock.Unlock()
		if ac.cacheDirs {
			// record that the parent directory tree contains at least one object
			ac.markAncestorsInCloud(getParentDir(options.Name), currentTime)
		}
		// add new entry
		newFileAttr := internal.CreateObjAttr(options.Name, 0, currentTime)
		newFileEntry := ac.cacheMap.insert(newFileAttr, true, currentTime)
		newFileEntry.setMode(options.Mode)
	}

	return h, err
}

// DeleteFile : Mark the file deleted
func (ac *AttrCache) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("AttrCache::DeleteFile : %s", options.Name)

	err := ac.NextComponent().DeleteFile(options)
	if err == nil {
		deletionTime := time.Now()
		ac.cacheLock.Lock()
		defer ac.cacheLock.Unlock()
		toBeDeleted, getErr := ac.cacheMap.get(options.Name)
		if getErr != nil || !toBeDeleted.valid() {
			log.Warn("AttrCache::DeleteFile : %s no valid entry found. Adding entry...", options.Name)
			// add deleted file entry
			attr := internal.CreateObjAttr(options.Name, 0, deletionTime)
			toBeDeleted = ac.cacheMap.insert(attr, true, deletionTime)
		}
		toBeDeleted.markDeleted(deletionTime)
		if ac.cacheDirs {
			ac.updateAncestorsInCloud(getParentDir(options.Name), deletionTime)
		}
	}

	return err
}

// Given the path to a directory, search its contents,
// and search the contents of all of its ancestors,
// to record which of them contain objects in their subtrees
func (ac *AttrCache) updateAncestorsInCloud(dirPath string, time time.Time) {
	for dirPath != "" {
		ancestorCacheItem, getErr := ac.cacheMap.get(dirPath)
		if getErr != nil || !ancestorCacheItem.exists() {
			log.Warn("AttrCache::updateAncestorsInCloud : Adding directory entry %s", dirPath)
			ancestorObjAttr := internal.CreateObjAttrDir(dirPath)
			ancestorCacheItem = ac.cacheMap.insert(ancestorObjAttr, true, time)
		}
		var anyChildrenInCloud bool

		for _, item := range ancestorCacheItem.children {
			if item.exists() && item.isInCloud() {
				anyChildrenInCloud = true
				break
			}
		}
		if ancestorCacheItem.isInCloud() != anyChildrenInCloud {
			ancestorCacheItem.markInCloud(anyChildrenInCloud)
		} else {
			//if we didn't change the parent, then no change is visible to the grandparent, etc.
			break
		}
		// move on to the next ancestor
		dirPath = getParentDir(dirPath)
	}
}

// RenameFile : Move item in cache
func (ac *AttrCache) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("AttrCache::RenameFile : %s -> %s", options.Src, options.Dst)

	err := ac.NextComponent().RenameFile(options)
	if err == nil {
		renameTime := time.Now()
		ac.cacheLock.Lock()
		defer ac.cacheLock.Unlock()

		//get the source item
		sourceItem, getErr := ac.cacheMap.get(options.Src)
		if getErr != nil || !sourceItem.exists() {
			log.Warn("AttrCache::RenameFile : Source %s does not exist in cache", options.Src)
			// don't return an error - cloud storage still succeeded
			return nil
		}

		// move source item to destination
		ac.moveAttrCachedItem(sourceItem, options.Src, options.Dst, renameTime)
		if ac.cacheDirs {
			ac.updateAncestorsInCloud(getParentDir(options.Src), renameTime)
			// mark the destination parent directory tree as containing objects
			ac.markAncestorsInCloud(getParentDir(options.Dst), renameTime)
		}
	}
	return err
}

// WriteFile : Mark the file invalid
func (ac *AttrCache) WriteFile(options internal.WriteFileOptions) (int, error) {

	// GetAttr on cache hit will serve from cache, on cache miss will serve from next component.
	attr, err := ac.GetAttr(internal.GetAttrOptions{Name: options.Handle.Path, RetrieveMetadata: true})
	if err != nil {
		// Ignore not exists errors - this can happen if createEmptyFile is set to false
		if !(os.IsNotExist(err) || err == syscall.ENOENT) {
			return 0, err
		}
	}
	if attr != nil {
		options.Metadata = attr.Metadata
	}

	size, err := ac.NextComponent().WriteFile(options)

	if err == nil {
		modifyTime := time.Now()
		newSize := options.Offset + int64(len(options.Data))

		ac.cacheLock.Lock()
		defer ac.cacheLock.Unlock()

		modifiedEntry, getErr := ac.cacheMap.get(options.Handle.Path)
		if getErr != nil || !modifiedEntry.exists() {
			log.Warn("AttrCache::WriteFile : %s replacing missing cache entry", options.Handle.Path)
			// replace the missing entry
			modifiedAttr := internal.CreateObjAttr(options.Handle.Path, newSize, modifyTime)
			modifiedEntry = ac.cacheMap.insert(modifiedAttr, true, modifyTime)
		}
		modifiedEntry.setSize(newSize, modifyTime)
		modifiedEntry.attr.Metadata = options.Metadata
	}
	return size, err
}

// TruncateFile : Update the file with its truncated size
func (ac *AttrCache) TruncateFile(options internal.TruncateFileOptions) error {
	log.Trace("AttrCache::TruncateFile : %s", options.Name)

	err := ac.NextComponent().TruncateFile(options)
	if err == nil {
		modifyTime := time.Now()

		ac.cacheLock.Lock()
		defer ac.cacheLock.Unlock()

		truncatedItem, getErr := ac.cacheMap.get(options.Name)
		if getErr != nil || !truncatedItem.exists() {
			log.Warn("AttrCache::TruncateFile : %s replacing missing cache entry", options.Name)
			// replace the missing entry
			truncatedAttr := internal.CreateObjAttr(options.Name, options.Size, modifyTime)
			truncatedItem = ac.cacheMap.insert(truncatedAttr, true, modifyTime)
		}
		truncatedItem.setSize(options.Size, modifyTime)
	}
	return err
}

// CopyFromFile : Upload file and update cache entry
func (ac *AttrCache) CopyFromFile(options internal.CopyFromFileOptions) error {
	log.Trace("AttrCache::CopyFromFile : %s", options.Name)

	// GetAttr on cache hit will serve from cache, on cache miss will serve from next component.
	attr, err := ac.GetAttr(internal.GetAttrOptions{Name: options.Name, RetrieveMetadata: true})
	if err != nil {
		// Ignore not exists errors - this can happen if createEmptyFile is set to false
		if !(os.IsNotExist(err) || err == syscall.ENOENT) {
			return err
		}
	}
	if attr != nil {
		options.Metadata = attr.Metadata
	}

	err = ac.NextComponent().CopyFromFile(options)
	if err == nil {
		uploadTime := time.Now()
		ac.cacheLock.Lock()
		defer ac.cacheLock.Unlock()

		if ac.cacheDirs {
			// This call needs to be treated like it's creating a new file
			// Mark ancestors as existing in cloud storage now
			ac.markAncestorsInCloud(getParentDir(options.Name), uploadTime)
		}

		// use local file to update the attribute cache entry
		fileStat, statErr := options.File.Stat()
		if statErr != nil {
			// if we can't stat the local file, we know nothing
			log.Err("AttrCache::CopyFromFile : %s Can't stat local file: %s", options.Name, statErr)
			entry, getErr := ac.cacheMap.get(options.Name)
			if getErr == nil {
				entry.invalidate()
			}
		} else {
			// replace entry
			attr := internal.CreateObjAttr(options.Name, fileStat.Size(), fileStat.ModTime())
			entry := ac.cacheMap.insert(attr, true, uploadTime)
			entry.setMode(fileStat.Mode())
		}
	}
	return err
}

// SyncFile : Refresh attributes from storage
func (ac *AttrCache) SyncFile(options internal.SyncFileOptions) error {
	log.Trace("AttrCache::SyncFile : %s", options.Handle.Path)
	err := ac.NextComponent().SyncFile(options)
	if err == nil {
		ac.cacheLock.Lock()
		defer ac.cacheLock.Unlock()
		toBeInvalid, getErr := ac.cacheMap.get(options.Handle.Path)
		if getErr == nil {
			toBeInvalid.invalidate()
		}
	}
	return err
}

// SyncDir : Refresh attributes from storage
func (ac *AttrCache) SyncDir(options internal.SyncDirOptions) error {
	log.Trace("AttrCache::SyncDir : %s", options.Name)

	err := ac.NextComponent().SyncDir(options)
	if err == nil {
		ac.cacheLock.Lock()
		defer ac.cacheLock.Unlock()
		ac.invalidateDirectory(options.Name)
	}
	return err
}

// GetAttr : Try to serve the request from the attribute cache, otherwise cache attributes of the path returned by next component
func (ac *AttrCache) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	log.Trace("AttrCache::GetAttr : %s", options.Name)
	ac.cacheLock.RLock()
	value, getErr := ac.cacheMap.get(options.Name)
	ac.cacheLock.RUnlock()
	if getErr == nil && value.valid() && time.Since(value.cachedAt).Seconds() < float64(ac.cacheTimeout) {
		// Try to serve the request from the attribute cache
		// Is the entry marked deleted?
		if value.isDeleted() {
			log.Debug("AttrCache::GetAttr : %s (ENOENT) served from cache", options.Name)
			return &internal.ObjAttr{}, syscall.ENOENT
		}
		// IsMetadataRetrieved is false in the case of ADLS List since the API does not support metadata.
		// Once migration of ADLS list to blob endpoint is done (in future service versions), we can remove this.
		// options.RetrieveMetadata is set by CopyFromFile and WriteFile which need metadata to ensure it is preserved.
		if value.getAttr().IsMetadataRetrieved() || (ac.noSymlinks && !options.RetrieveMetadata) {
			// path exists and we have all the metadata required or we do not care about metadata
			log.Debug("AttrCache::GetAttr : %s served from cache", options.Name)
			return value.getAttr(), nil
		}
	}

	// Get the attributes from next component and cache them
	pathAttr, err := ac.NextComponent().GetAttr(options)

	ac.cacheLock.Lock()
	defer ac.cacheLock.Unlock()

	if err == nil {
		if ac.noSymlinks {
			// strip symlink attribute
			pathAttr.Flags.Clear(internal.PropFlagSymlink)
		}
		// Retrieved attributes so cache them
		ac.cacheMap.insert(pathAttr, true, time.Now())
		if ac.cacheDirs {
			ac.markAncestorsInCloud(getParentDir(options.Name), time.Now())
		}
	} else if err == syscall.ENOENT {
		// cache this entity not existing
		ac.cacheMap.insert(internal.CreateObjAttr(options.Name, 0, time.Now()), false, time.Now())
	}
	return pathAttr, err
}

// CreateLink : Mark the new link invalid
func (ac *AttrCache) CreateLink(options internal.CreateLinkOptions) error {
	log.Trace("AttrCache::CreateLink : Create symlink %s -> %s", options.Name, options.Target)

	err := ac.NextComponent().CreateLink(options)

	if err == nil {
		currentTime := time.Now()
		ac.cacheLock.Lock()
		defer ac.cacheLock.Unlock()
		linkAttr := internal.CreateObjAttr(options.Name, int64(len([]byte(options.Target))), currentTime)
		linkAttr.Flags.Set(internal.PropFlagSymlink)
		ac.cacheMap.insert(linkAttr, true, currentTime)
		if ac.cacheDirs {
			ac.markAncestorsInCloud(getParentDir(options.Name), currentTime)
		}
	}

	return err
}

// FlushFile : flush file
func (ac *AttrCache) FlushFile(options internal.FlushFileOptions) error {
	log.Trace("AttrCache::FlushFile : %s", options.Handle.Path)
	err := ac.NextComponent().FlushFile(options)
	if err == nil {
		ac.cacheLock.Lock()
		defer ac.cacheLock.Unlock()
		toBeInvalid, getErr := ac.cacheMap.get(options.Handle.Path)
		if getErr == nil {
			toBeInvalid.invalidate()
		}
	}
	return err
}

// Chmod : Update the file with its new permissions
func (ac *AttrCache) Chmod(options internal.ChmodOptions) error {
	log.Trace("AttrCache::Chmod : Change mode of file/directory %s", options.Name)

	err := ac.NextComponent().Chmod(options)

	if err == nil {
		ac.cacheLock.Lock()
		defer ac.cacheLock.Unlock()

		value, getErr := ac.cacheMap.get(options.Name)
		if getErr != nil {
			log.Err("AttrCache::Chmod : %v", getErr)
		} else if !value.exists() {
			log.Err("AttrCache::Chmod : invalidating deleted entry %s", options.Name)
			value.invalidate()
		} else {
			value.setMode(options.Mode)
		}
	}
	return err
}

// Chown : Update the file with its new owner and group (when datalake chown is implemented)
func (ac *AttrCache) Chown(options internal.ChownOptions) error {
	log.Trace("AttrCache::Chown : Change owner of file/directory %s", options.Name)

	err := ac.NextComponent().Chown(options)
	// TODO: Implement when datalake chown is supported.

	return err
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewAttrCacheComponent() internal.Component {
	comp := &AttrCache{}
	comp.SetName(compName)

	config.AddConfigChangeEventListener(comp)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewAttrCacheComponent)

	attrCacheTimeout := config.AddUint32Flag("attr-cache-timeout", defaultAttrCacheTimeout, "attribute cache timeout")
	config.BindPFlag(compName+".timeout-sec", attrCacheTimeout)

	noSymlinks := config.AddBoolFlag("no-symlinks", false, "whether or not symlinks should be supported")
	config.BindPFlag(compName+".no-symlinks", noSymlinks)
	noCacheDirs := config.AddBoolFlag("no-cache-dirs", false, "whether or not empty directories should be cached")
	config.BindPFlag(compName+".no-cache-dirs", noCacheDirs)

	cacheOnList := config.AddBoolFlag("cache-on-list", true, "Cache attributes on listing.")
	config.BindPFlag(compName+".cache-on-list", cacheOnList)
	cacheOnList.Hidden = true
}
