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
// This marks items deleted instead of invalidating them.
// That way if a request came in for a deleted item, we can respond from the cache.
func (ac *AttrCache) deleteDirectory(path string, time time.Time) {
	// Delete all descendants of the path, then delete the path
	// For example, filesystem: a/, a/b, a/c, aa/, ab.
	// When we delete directory a, we only want to delete a/, a/b, and a/c.
	// If we do not conditionally extend a, we would accidentally delete aa/ and ab

	//get attrCacheItem
	toBeDeleted, err := ac.cacheMap.get(path)

	// delete the path itself and children.
	if err != nil {
		log.Err("AttrCache::deleteDirectory : directory %s not found in cache", path)
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
	// For example, filesystem: a/, a/b, a/c, aa/, ab.
	// When we delete directory a, we only want to delete a/, a/b, and a/c.
	// If we do not conditionally extend a, we would accidentally delete aa/ and ab

	//get attrCacheItem
	toBeDeleted, err := ac.cacheMap.get(path)

	// delete the path itself and children.
	if err != nil {
		log.Err("could not find the cache map item due to the following error: ", err)
		return syscall.ENOENT
	} else {
		toBeDeleted.markDeleted(time)
	}

	// check if the directory to be deleted exists
	if toBeDeleted.children == nil && !ac.pathExistsInCache(path) {
		log.Err("AttrCache::deleteCachedDirectory : directory %s does not exist in attr cache.", path)
		return syscall.ENOENT
	}

	// If this leaves the parent or any ancestor directory empty, record that.
	// Although this involves an unnecessary second traversal through the cache,
	// because of the code complexity, I think it's worth the readability gained.
	ac.updateAncestorsInCloud(getParentDir(path), time)
	return nil
}

// pathExistsInCache: check if path is in cache, is valid, and not marked deleted
func (ac *AttrCache) pathExistsInCache(path string) bool {
	value, err := ac.cacheMap.get(internal.TruncateDirName(path))
	if err != nil {
		log.Err("could not find the attr cached item due to the following error: ", err)
		return false
	}
	return (value.valid() && value.exists())
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
	// For example, filesystem: a/, a/b, a/c, aa/, ab.
	// When we invalidate directory a, we only want to invalidate a/, a/b, and a/c.
	// If we do not conditionally extend a, we would accidentally invalidate aa/ and ab

	toBeInvalid, err := ac.cacheMap.get(path)
	if err != nil {
		log.Err("could not invalidate cached attr item due to the following error: ", err)
	} else {
		// don't invalidate directories when cacheDirs is true
		if !ac.cacheDirs && toBeInvalid.attr.IsDir() {
			toBeInvalid.invalidate()
		} else if ac.cacheDirs && !toBeInvalid.attr.IsDir() {
			toBeInvalid.invalidate()
		} else if ac.cacheDirs && toBeInvalid.attr.IsDir() {
			if toBeInvalid.children != nil {
				for _, childItem := range toBeInvalid.children {
					ac.invalidateDirectory(childItem.attr.Path)
				}
			}
		}
	}
}

// renameCachedDirectory: Renames a cached directory and all its contents when ac.cacheDirs is true.
func (ac *AttrCache) renameCachedDirectory(srcDir string, dstDir string, time time.Time) error {

	// First, check if the destination directory already exists
	if ac.pathExistsInCache(dstDir) {
		return os.ErrExist
	}

	// Rename all descendants of srcDir, then rename the srcDir itself
	// For example, filesystem: a/, a/b, a/c, aa/, ab.
	// When we rename directory a, we only want to rename a/, a/b, and a/c.
	// If we do not conditionally extend a, we would accidentally delete aa/ and ab

	// remember whether we actually found any contents
	srcItem, err := ac.cacheMap.get(srcDir)
	if err != nil {
		log.Err("could not get the source cached item for renaming directory due to the following error: ", err)
		return err
	} else {
		srcDir = internal.TruncateDirName(srcDir)
		dstDir = internal.TruncateDirName(dstDir)
		ac.renameCachedDirectoryHelper(srcItem, srcDir, dstDir, time)
	}

	// if there were no cached entries to move, does this directory even exist?
	if srcItem.children == nil && !ac.pathExistsInCache(srcDir) {
		log.Err("AttrCache::renameCachedDirectory : Source directory %s does not exist.", srcDir)
		return syscall.ENOENT
	}

	return nil
}

func (ac *AttrCache) renameCachedDirectoryHelper(itemSrc *attrCacheItem, srcDir string, dstDir string, time time.Time) {
	movedObjects := false
	var dstKey string
	if itemSrc.children != nil {
		for _, childSrc := range itemSrc.children {
			dstKey = strings.Replace(childSrc.attr.Path, srcDir, dstDir, 2)

			// track whether the destination is gaining objects
			movedObjects = movedObjects || (childSrc.isInCloud() && childSrc.exists() && childSrc.valid())

			// to keep the directory cache coherent,
			// any renamed directories need a new cache entry
			// TODO: revisit above comments. scanning sub tree?
			var dstDirCacheItem *attrCacheItem
			var err error
			if childSrc.attr.IsDir() && childSrc.valid() && childSrc.exists() {
				// add the destination directory to our cache
				dstDirAttr := internal.CreateObjAttrDir(dstKey)
				ac.cacheMap.insert(dstDirAttr, true, time)
				dstDirCacheItem, err = ac.cacheMap.get(dstDirAttr.Path)
				if err != nil {
					log.Err("could not find the destination attr cached item to rename directory due to the following error: ", err)
					return
				}
				dstDirCacheItem.markInCloud(childSrc.isInCloud())

			} else {
				// invalidate files so attributes get refreshed from the backend
				if dstDirCacheItem != nil {
					dstDirCacheItem.invalidate()
				}
			}

			if childSrc.children != nil {
				for _, srcGrdChild := range childSrc.children {
					ac.renameCachedDirectoryHelper(srcGrdChild, srcDir, dstDir, time)
				}
			}

			// TODO: maybe encapulate the following if else statement into a wrapper?
			if movedObjects {
				ac.markAncestorsInCloud(dstKey, time)
			} else {
				// add the destination directory to our cache
				// am I adding a child to the already existing dstItem? is it a duplicate item as a child?
				dstDir = internal.TruncateDirName(dstDir)
				dstItem, err := ac.cacheMap.get(dstDir)
				if err != nil {
					log.Err("could not find the attr cached item: ", err)
				} else { //TODO: peer review that this is correct
					dstDirAttr := internal.CreateObjAttrDir(dstDir)
					dstItem.insert(dstDirAttr, true, time)
					dstDirAttrCacheItem, err := ac.cacheMap.get(dstDirAttr.Path)
					if err != nil {
						log.Err("could not find the attr cached item: ", err)
					} else {
						dstDirAttrCacheItem.markInCloud(false)
					}

				}

				// delete the source directory from our cache
				itemSrc.markDeleted(time)

				// If this leaves the parent or ancestor directories empty, record that.
				// Although this involves an unnecessary second traversal through the cache,
				// because of the code complexity, I think it's worth the readability gained.
				ac.updateAncestorsInCloud(getParentDir(srcDir), time)
			}

			// either way, mark the old cache entry deleted
			childSrc.markDeleted(time)
		}
		itemSrc.markDeleted(time)
	} else {
		dstKey = strings.Replace(itemSrc.attr.Path, srcDir, dstDir, 2)

		// track whether the destination is gaining objects
		movedObjects = movedObjects || (itemSrc.isInCloud() && itemSrc.exists() && itemSrc.valid())

		// to keep the directory cache coherent,
		// any renamed directories need a new cache entry
		// TODO: files or objects (non dirs) in the sub tree are not getting inserted into cacheMap tree. is this a bug?
		var dstDirCacheItem *attrCacheItem
		var err error
		if itemSrc.attr.IsDir() && itemSrc.valid() && itemSrc.exists() {
			// add the destination directory to our cache
			dstDirAttr := internal.CreateObjAttrDir(dstKey)
			ac.cacheMap.insert(dstDirAttr, true, time)
			dstDirCacheItem, err = ac.cacheMap.get(dstDirAttr.Path)
			if err != nil {
				log.Err("could not find the destination attr cached item to rename directory due to the following error: ", err)
			} else {
				dstDirCacheItem.markInCloud(itemSrc.isInCloud())
			}
		} else {
			// invalidate files so attributes get refreshed from the backend
			if dstDirCacheItem != nil {
				dstDirCacheItem.invalidate()
			}
		}

		// either way, mark the old cache entry deleted
		itemSrc.markDeleted(time)

		if movedObjects {
			ac.markAncestorsInCloud(dstKey, time)
		} else {
			// add the destination directory to our cache
			// am I adding a child to the already existing dstItem? is it a duplicate item as a child?
			dstDir = internal.TruncateDirName(dstDir)
			dstItem, err := ac.cacheMap.get(dstDir)
			if err != nil {
				log.Err("could not find the attr cached item: ", err)
			} else { //TODO: peer review that this is correct
				dstDirAttr := internal.CreateObjAttrDir(dstDir)
				dstItem.insert(dstDirAttr, true, time)
				dstDirAttrCacheItem, err := ac.cacheMap.get(dstDirAttr.Path)
				if err != nil {
					log.Err("could not find the attr cached item: ", err)
				} else {
					dstDirAttrCacheItem.markInCloud(false)
				}

			}

			// delete the source directory from our cache
			itemSrc.markDeleted(time)

			// If this leaves the parent or ancestor directories empty, record that.
			// Although this involves an unnecessary second traversal through the cache,
			// because of the code complexity, I think it's worth the readability gained.
			ac.updateAncestorsInCloud(getParentDir(srcDir), time)
		}

	}

}

func (ac *AttrCache) markAncestorsInCloud(dirPath string, time time.Time) {
	dirPath = internal.TruncateDirName(dirPath)
	if len(dirPath) != 0 {
		dirCacheItem, err := ac.cacheMap.get(dirPath)

		// this insert is wiping children from this parent.the child path needs to come in here to resolve that
		if !(err == nil && dirCacheItem.valid() && dirCacheItem.exists()) { //TODO: do more specific error check for attrCacheItem not existing
			dirObjAttr := internal.CreateObjAttrDir(dirPath)
			ac.cacheMap.insert(dirObjAttr, true, time)
		}
		dirCacheItem, err = ac.cacheMap.get(dirPath)
		if err != nil { //TODO: do more specific error check for attrCacheItem not existing
			log.Err("could not get the attribute item from cache due to the following error: ", err)
		} else {
			dirCacheItem.markInCloud(true)
		}

		// recurse
		ac.markAncestorsInCloud(getParentDir(dirPath), time)
	}
}

// ------------------------- Methods implemented by this component -------------------------------------------
// CreateDir: Mark the directory invalid
func (ac *AttrCache) CreateDir(options internal.CreateDirOptions) error {
	log.Trace("AttrCache::CreateDir : %s", options.Name)
	err := ac.NextComponent().CreateDir(options)

	if err == nil {
		ac.cacheLock.Lock()
		defer ac.cacheLock.Unlock()
		if ac.cacheDirs {
			// check if directory already exists
			newDirPath := internal.TruncateDirName(options.Name)
			if ac.pathExistsInCache(newDirPath) {
				return os.ErrExist
			}
			newDirAttr := internal.CreateObjAttrDir(newDirPath)
			ac.cacheMap.insert(newDirAttr, true, time.Now())
			newDirAttrCacheItem, err := ac.cacheMap.get(newDirPath)
			if err != nil {
				log.Err("could not find the attr cached item: ", err)
			} else {
				newDirAttrCacheItem.markInCloud(false)
			}

		} else {
			dirAttrCacheItem, err := ac.cacheMap.get(internal.TruncateDirName(options.Name))
			if err != nil {
				log.Err("could not find the attr cached item: ", err)
			} else {
				dirAttrCacheItem.invalidate()
			}

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
			ac.cacheLock.RLock()
			defer ac.cacheLock.RUnlock()
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

	nonCloudItem, err := ac.cacheMap.get(listPath)

	if err != nil {
		log.Err("could not find the attr cached item: ", err)
	} else {
		if nonCloudItem.valid() && nonCloudItem.exists() && nonCloudItem.attr.IsDir() && !nonCloudItem.isInCloud() {
			ac.cacheLock.RLock()
			if nonCloudItem.children != nil {
				for _, item := range nonCloudItem.children {
					if !item.attr.IsDir() {
						pathList = append(pathList, item.attr)
						numAdded++
					}
				}
			}
			ac.cacheLock.RUnlock()
		}
	}

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

			// TODO: call the insert in cacheMap.go to guild out a nested map tree.

			// TODO: will this cause a bug when cacheDirs is enabled?
			// TODO: this will require a tree traversal / scan to get cachedItems count

			ac.cacheMap.insert(attr, true, currTime)

		}

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

	cachedContentItem, err := ac.cacheMap.get(prefix)
	if err != nil {
		log.Err("can't find cache map item due to following error: ", err)
	} else { //TODO: this isn't looking at each child or not going through the sub tree. is that an issue?
		if cachedContentItem.children != nil && cachedContentItem.valid() && cachedContentItem.exists() {
			return true
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
		if ac.cacheDirs {
			// renameCachedDirectory may cache the parent directory
			// so lock the cache for writing
			ac.cacheLock.Lock()
			defer ac.cacheLock.Unlock()
			err = ac.renameCachedDirectory(options.Src, options.Dst, currentTime)
		} else {
			ac.cacheLock.RLock()
			defer ac.cacheLock.RUnlock()
			ac.deleteDirectory(options.Src, currentTime)
			// TLDR: Dst is guaranteed to be non-existent or empty.
			// Note: We do not need to invalidate children of Dst due to the logic in our FUSE connector, see comments there,
			// but it is always safer to double check than not.
			ac.invalidateDirectory(options.Dst)
		}
	}

	return err
}

// CreateFile: Mark the file invalid
func (ac *AttrCache) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	log.Trace("AttrCache::CreateFile : %s", options.Name)
	h, err := ac.NextComponent().CreateFile(options)

	if err == nil {
		// TODO: the cache locks are used incorrectly here
		// They routinely lock the cache for reading, but then write to it
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		if ac.cacheDirs {
			// record that the parent directory tree contains at least one object
			ac.markAncestorsInCloud(getParentDir(options.Name), time.Now())
		}
		// TODO: we assume that the OS will call GetAttr after this.
		// 		if it doesn't, will invalidating this entry cause problems?
		toBeInvalid, err := ac.cacheMap.get(options.Name)
		if err != nil {
			log.Err("cannot find the attr cache item due to the following error: ", err)
		} else {
			toBeInvalid.invalidate()
		}
	}

	return h, err
}

// DeleteFile : Mark the file deleted
func (ac *AttrCache) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("AttrCache::DeleteFile : %s", options.Name)

	err := ac.NextComponent().DeleteFile(options)
	if err == nil {
		deletionTime := time.Now()
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		toBeDeleted, err := ac.cacheMap.get(options.Name)
		if err != nil {
			log.Err("cannot find the attr cache item due to the following error: ", err)
		} else {
			toBeDeleted.markDeleted(deletionTime)
		}
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

	ancestorPath := internal.TruncateDirName(dirPath)
	for ancestorPath != "" {

		ancestorCacheItem, err := ac.cacheMap.get(ancestorPath)
		if err != nil { //TODO: do more specific error check for attrCacheItem not existing
			ancestorObjAttr := internal.CreateObjAttrDir(ancestorPath)
			ac.cacheMap.insert(ancestorObjAttr, true, time)
		}

		var anyChildrenInCloud bool

		if ancestorCacheItem.children != nil {
			for _, item := range ancestorCacheItem.children {
				if item.exists() && item.valid() && item.isInCloud() {
					anyChildrenInCloud = item.isInCloud()
					if anyChildrenInCloud {
						break
					}
				}
			}
		}

		if anyChildrenInCloud && !ancestorCacheItem.isInCloud() {
			ancestorCacheItem.markInCloud(anyChildrenInCloud)
		} else if !anyChildrenInCloud && ancestorCacheItem.isInCloud() {
			ancestorCacheItem.markInCloud(anyChildrenInCloud)
		} else {
			break
		}

		// move on to the next ancestor
		ancestorPath = getParentDir(ancestorPath)
	}

}

// RenameFile : Mark the source file deleted. Invalidate the destination file.
func (ac *AttrCache) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("AttrCache::RenameFile : %s -> %s", options.Src, options.Dst)

	err := ac.NextComponent().RenameFile(options)
	if err == nil {
		renameTime := time.Now()
		if ac.cacheDirs {
			ac.cacheLock.Lock()
			ac.updateAncestorsInCloud(getParentDir(options.Src), renameTime)
			// mark the destination parent directory tree as containing objects
			ac.markAncestorsInCloud(getParentDir(options.Dst), renameTime)
			ac.cacheLock.Unlock()
		}
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		// TODO: Can we just copy over the attributes from the source to the destination so we don't have to invalidate?

		toBeDeleted, err := ac.cacheMap.get(options.Src)
		if err != nil {
			log.Err("could not find attr cache item due to following error: ", err)
		} else {
			toBeDeleted.markDeleted(renameTime)
		}

		toBeInvalid, err := ac.cacheMap.get(options.Dst)
		if err != nil {
			log.Err("could not find attr cache item due to following error: ", err)
		} else {
			toBeInvalid.invalidate()
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
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		// TODO: Could we just update the size and mod time of the file here? Or can other attributes change here?

		var toBeInvalid *attrCacheItem
		var err error
		if attr == nil {
			toBeInvalid, err = ac.cacheMap.get(options.Handle.Path)
		} else {
			toBeInvalid, err = ac.cacheMap.get(attr.Path) //attr is nil here for TestWriteFileDoesNotExist()
		}

		if err != nil {
			log.Err("could not find attribute item in cache to invalidate due to the following error: ", err)
		} else {
			toBeInvalid.invalidate()
		}
	}
	return size, err
}

// TruncateFile : Update the file with its truncated size
func (ac *AttrCache) TruncateFile(options internal.TruncateFileOptions) error {
	log.Trace("AttrCache::TruncateFile : %s", options.Name)

	err := ac.NextComponent().TruncateFile(options)
	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()

		// no need to truncate the name of the file
		value, err := ac.cacheMap.get(options.Name)
		if err != nil {
			log.Err("could not find attribute item in cache to truncate file due to the following error: ", err)
		} else {
			if value.valid() && value.exists() {
				value.setSize(options.Size)
			}
		}
	}
	return err
}

// CopyFromFile : Mark the file invalid
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
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		if ac.cacheDirs {
			// This call needs to be treated like it's creating a new file
			// Mark ancestors as existing in cloud storage now
			ac.markAncestorsInCloud(getParentDir(options.Name), time.Now())
		}
		// TODO: Could we just update the size and mod time of the file here? Or can other attributes change here?
		// TODO: we're RLocking the cache but we need to also lock this attr item because another thread could be reading this attr item

		toBeInvalid, err := ac.cacheMap.get(options.Name) //empty for TestCopyFromFileDoesNotExist()
		if err != nil {
			log.Err("The attribute item could not be invalidated in the cache due to the following error: ", err)
		} else {
			toBeInvalid.invalidate()
		}
	}
	return err
}

func (ac *AttrCache) SyncFile(options internal.SyncFileOptions) error {
	log.Trace("AttrCache::SyncFile : %s", options.Handle.Path)

	err := ac.NextComponent().SyncFile(options)
	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		toBeInvalid, err := ac.cacheMap.get(options.Handle.Path)
		if err != nil {
			log.Err("The attribute item could not be invalidated in the cache due to the following error: ", err)
		} else {
			toBeInvalid.invalidate()
		}
	}
	return err
}

func (ac *AttrCache) SyncDir(options internal.SyncDirOptions) error {
	log.Trace("AttrCache::SyncDir : %s", options.Name)

	err := ac.NextComponent().SyncDir(options)
	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		ac.invalidateDirectory(options.Name)
	}
	return err
}

// GetAttr : Try to serve the request from the attribute cache, otherwise cache attributes of the path returned by next component
func (ac *AttrCache) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	log.Trace("AttrCache::GetAttr : %s", options.Name)
	truncatedPath := internal.TruncateDirName(options.Name)

	ac.cacheLock.RLock()
	value, err := ac.cacheMap.get(truncatedPath)
	ac.cacheLock.RUnlock()

	if err == nil && value.valid() && time.Since(value.cachedAt).Seconds() < float64(ac.cacheTimeout) {
		// Try to serve the request from the attribute cache

		// Is the entry marked deleted?
		if value.isDeleted() {
			log.Debug("AttrCache::GetAttr : %s served from cache", options.Name)
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
		// Retrieved attributes so cache them
		// TODO: bug: when cacheDirs is true, the cache limit will cause some directories to be double-listed
		// TODO: shouldn't this be an LRU? This sure looks like the opposite...
		if pathAttr == nil {
			ac.cacheMap.insert(&internal.ObjAttr{Path: options.Name}, true, time.Now())
		} else {
			ac.cacheMap.insert(pathAttr, true, time.Now())
		}

		if ac.cacheDirs {
			ac.markAncestorsInCloud(getParentDir(options.Name), time.Now())
		}
	} else if err == syscall.ENOENT {
		// Path does not exist so cache a no-entry item
		// this insert should involve the key of the cacheMap item being the truncatedPath. Do we have insert some how know the truncated path to use as the key? or do we do a key literal insert right here? the latter is mesyier for cacheMap access scope.

		//ac.cacheMap.insert(&internal.ObjAttr{}, false, time.Now())
		ac.cacheMap.children = make(map[string]*attrCacheItem)
		ac.cacheMap.children[truncatedPath] = newAttrCacheItem(&internal.ObjAttr{}, false, time.Now())
	}

	return pathAttr, err
}

// CreateLink : Mark the link and target invalid
func (ac *AttrCache) CreateLink(options internal.CreateLinkOptions) error {
	log.Trace("AttrCache::CreateLink : Create symlink %s -> %s", options.Name, options.Target)

	err := ac.NextComponent().CreateLink(options)

	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		toBeInvalid, err := ac.cacheMap.get(options.Name)
		if err != nil {
			log.Err("The attribute item could not be invalidated in the cache due to the following error: ", err)
		} else {
			toBeInvalid.invalidate()
			if ac.cacheDirs {
				ac.markAncestorsInCloud(getParentDir(options.Name), time.Now())
			}
		}
	}

	return err
}

// FlushFile : flush file
func (ac *AttrCache) FlushFile(options internal.FlushFileOptions) error {
	log.Trace("AttrCache::FlushFile : %s", options.Handle.Path)
	err := ac.NextComponent().FlushFile(options)
	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		toBeInvalid, err := ac.cacheMap.get(options.Handle.Path)
		if err != nil {
			log.Err("The attribute item could not be invalidated in the cache due to the following error: ", err)
		} else {
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
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()

		value, err := ac.cacheMap.get(internal.TruncateDirName(options.Name))
		if err != nil {
			log.Err("The attribute item could not be retrieved from the cache due to the following error: ", err)
		} else {
			if value.valid() && value.exists() {
				value.setMode(options.Mode)
			}
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
