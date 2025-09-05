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

package attr_cache

import (
	"context"
	"fmt"
	"os"
	"path"
	"slices"
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
	cacheTimeout   uint32
	cacheOnList    bool
	enableSymlinks bool
	cacheDirs      bool
	maxFiles       int
	cache          *cacheTreeMap
	cacheLock      sync.RWMutex
	cleanupDone    chan bool
	cleanupCtx     context.Context
	cleanupStop    context.CancelFunc
}

// Structure defining your config parameters
type AttrCacheOptions struct {
	Timeout        uint32 `config:"timeout-sec"      yaml:"timeout-sec,omitempty"`
	NoCacheOnList  bool   `config:"no-cache-on-list" yaml:"no-cache-on-list,omitempty"`
	EnableSymlinks bool   `config:"enable-symlinks"  yaml:"enable-symlinks,omitempty"`
	NoCacheDirs    bool   `config:"no-cache-dirs"    yaml:"no-cache-dirs,omitempty"`
	// hidden option for backward compatibility
	NoSymlinks bool `config:"no-symlinks"      yaml:"no-symlinks,omitempty"`

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
	ac.cache = newCacheTreeMap()

	// Start background cleanup goroutine
	ac.cleanupCtx, ac.cleanupStop = context.WithCancel(ctx)
	ac.cleanupDone = make(chan bool)
	go ac.backgroundCleanup()

	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (ac *AttrCache) Stop() error {
	log.Trace("AttrCache::Stop : Stopping component %s", ac.Name())

	// Stop the background cleanup goroutine
	if ac.cleanupStop != nil {
		ac.cleanupStop()
		<-ac.cleanupDone // Wait for cleanup goroutine to finish
	}

	return nil
}

// GenConfig : Generate the default config for the component
func (ac *AttrCache) GenConfig() string {
	log.Info("AttrCache::Configure : config generation started")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n%s:", ac.Name()))
	sb.WriteString(fmt.Sprintf("\n  timeout-sec: %v", defaultAttrCacheTimeout))

	return sb.String()
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

	if config.IsSet(compName+".no-symlinks") && !config.IsSet(compName+".enable-symlinks") {
		ac.enableSymlinks = !conf.NoSymlinks
	} else {
		ac.enableSymlinks = conf.EnableSymlinks
	}

	ac.cacheDirs = !conf.NoCacheDirs

	log.Crit(
		"AttrCache::Configure : cache-timeout %d, enable-symlinks %t, cache-on-list %t, max-files %d",
		ac.cacheTimeout,
		ac.enableSymlinks,
		ac.cacheOnList,
		ac.maxFiles,
	)

	return nil
}

// OnConfigChange : If component has registered, on config file change this method is called
func (ac *AttrCache) OnConfigChange() {
	log.Trace("AttrCache::OnConfigChange : %s", ac.Name())
	_ = ac.Configure(true)
}

// Helper Methods

// Record that the directory and all its contents have been deleted from the file system
func (ac *AttrCache) deleteDirectory(path string, deletedAt time.Time) error {
	// get the entry to be marked deleted
	item, found := ac.cache.get(path)
	// handle errors and unexpected behavior
	dirExists := found && item.exists()
	if !dirExists {
		if ac.cacheDirs {
			// when cacheDirs is true, deleting a non-existent directory should return ENOENT
			log.Err("AttrCache::deleteDirectory : %s does not exist", path)
			return syscall.ENOENT
		} else {
			// when cacheDirs is false, attr_cache is not responsible for returning ENOENT
			// just log a warning for this unexpected behavior
			log.Warn("AttrCache::deleteDirectory : %s directory does not exist", path)
			// if not already done, record the fact that the directory has been deleted
			if !found {
				log.Info("AttrCache::deleteDirectory : %s recording directory as deleted", path)
				ac.cache.insert(insertOptions{
					attr:     internal.CreateObjAttrDir(path),
					exists:   false,
					cachedAt: deletedAt,
				})
			}
			return nil
		}
	}

	// record that the entry and all its children have been deleted
	item.markDeleted(deletedAt)
	if ac.cacheDirs {
		// update whether cloud storage has any record of the parent directory's existence
		ac.updateAncestorsInCloud(getParentDir(path), deletedAt)
	}

	return nil
}

// does the cache show this path as existing?
func (ac *AttrCache) pathExistsInCache(path string) bool {
	item, found := ac.cache.get(path)
	if !found {
		return false
	}
	return item.exists()
}

// returns the parent directory (without a trailing slash)
func getParentDir(childPath string) string {
	parentDir := path.Dir(internal.TruncateDirName(childPath))
	if parentDir == "." {
		parentDir = ""
	}
	return parentDir
}

// mark the directory and all its contents invalid
// only use when cacheDirs=false
func (ac *AttrCache) invalidateDirectory(path string) {
	item, found := ac.cache.get(path)
	if !found || !item.valid() {
		log.Warn("AttrCache::invalidateDirectory : %s entry not found or already invalid", path)
		return
	}

	// only invalidate directories when cacheDirs is false
	if ac.cacheDirs {
		// invalidating anything when cacheDirs=true is risky
		// TODO: should we do nothing here?
		// let's compromise: recursively invalidate only file items
		for _, childItem := range item.children {
			if !childItem.attr.IsDir() {
				childItem.invalidate()
			} else {
				ac.invalidateDirectory(childItem.attr.Path)
			}
		}
	} else {
		// invalidate the whole directory, recursively
		item.invalidate()
		return
	}

}

// move an item to a new location, and return the destination item
func (ac *AttrCache) moveCachedItem(
	srcItem *attrCacheItem,
	srcDir string,
	dstDir string,
	movedAt time.Time,
) *attrCacheItem {
	// don't move deleted items
	if !srcItem.exists() {
		return nil
	}
	// generate the destination name
	dstPath := strings.Replace(srcItem.attr.Path, srcDir, dstDir, 1)
	// create the destination attr
	var dstAttr *internal.ObjAttr
	if srcItem.attr.IsDir() {
		dstAttr = internal.CreateObjAttrDir(dstPath)
	} else {
		dstAttr = internal.CreateObjAttr(dstPath, srcItem.attr.Size, srcItem.attr.Mtime)
	}
	// add the destination item to the cache
	dstItem := ac.cache.insert(insertOptions{
		attr:     dstAttr,
		exists:   true,
		cachedAt: srcItem.cachedAt,
	})
	// copy the inCloud flag
	dstItem.attr.Mode = srcItem.attr.Mode
	dstItem.markInCloud(srcItem.isInCloud())
	// recurse over any children
	for _, srcChildItm := range srcItem.children {
		ac.moveCachedItem(srcChildItm, srcDir, dstDir, movedAt)
	}
	// mark the source item deleted
	srcItem.markDeleted(movedAt)
	// return the destination item
	return dstItem
}

// record that cloud storage has records of this directory and all its ancestors existing
func (ac *AttrCache) markAncestorsInCloud(dirPath string, time time.Time) {
	if len(dirPath) != 0 {
		// get or create directory cache item
		dirCacheItem, found := ac.cache.get(dirPath)
		if !found || !dirCacheItem.exists() {
			log.Warn("AttrCache::markAncestorsInCloud : Adding parent directory %s", dirPath)
			dirObjAttr := internal.CreateObjAttrDir(dirPath)
			dirCacheItem = ac.cache.insert(insertOptions{
				attr:     dirObjAttr,
				exists:   true,
				cachedAt: time,
			})
		} else if dirCacheItem.isInCloud() {
			// flag is already updated - no need to continue
			return
		}
		// update flag
		dirCacheItem.markInCloud(true)
		// recurse
		ac.markAncestorsInCloud(getParentDir(dirPath), time)
	}
}

// backgroundCleanup: runs in a separate goroutine to periodically clean up expired entries
func (ac *AttrCache) backgroundCleanup() {
	defer close(ac.cleanupDone)

	// Ensure minimum interval to prevent panic with NewTicker.
	// Note: `cacheTimeout` is immutable post-start and should not be modified during runtime.
	interval := time.Duration(ac.cacheTimeout) * time.Second
	if interval <= 0 {
		interval = time.Second // Use 1 second as minimum interval
	}

	// Create ticker based on cache timeout interval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ac.cleanupCtx.Done():
			log.Trace("AttrCache::backgroundCleanup : Stopping background cleanup")
			return
		case <-ticker.C:
			ac.cleanupExpiredEntries()
		}
	}
}

// cleanupExpiredEntries: removes expired entries from the cache map
// This runs in a background goroutine to prevent memory leaks
func (ac *AttrCache) cleanupExpiredEntries() {
	// First pass: collect keys to delete under read lock to minimize write lock duration
	var keysToDelete []string

	ac.cacheLock.RLock()
	for path, item := range ac.cache.cacheMap {
		// Never delete the root entry
		if path == "" {
			continue
		}
		// Check if entry has exceeded the cache timeout
		if time.Since(item.cachedAt).Seconds() >= float64(ac.cacheTimeout) {
			keysToDelete = append(keysToDelete, path)
		}
	}
	ac.cacheLock.RUnlock()

	// Second pass: delete expired entries under write lock, re-checking expiration
	if len(keysToDelete) > 0 {
		ac.cacheLock.Lock()
		for _, path := range keysToDelete {
			// Never delete the root entry
			if path == "" {
				continue
			}
			// Re-check if entry still exists and is still expired
			if item, exists := ac.cache.cacheMap[path]; exists {
				if time.Since(item.cachedAt).Seconds() >= float64(ac.cacheTimeout) {
					delete(ac.cache.cacheMap, path)
				}
			}
		}
		ac.cacheLock.Unlock()
	}
}

// ------------------------- Methods implemented by this component -------------------------------------------
// CreateDir: Mark the directory invalid, or
// insert the dir item into cache when cacheDirs is true.
func (ac *AttrCache) CreateDir(options internal.CreateDirOptions) error {
	log.Trace("AttrCache::CreateDir : %s", options.Name)
	err := ac.NextComponent().CreateDir(options)
	if err == nil || err == syscall.EEXIST {
		ac.cacheLock.Lock()
		defer ac.cacheLock.Unlock()
		// does the directory already exist?
		oldDirAttrCacheItem, found := ac.cache.get(options.Name)
		directoryAlreadyExists := found && oldDirAttrCacheItem.exists()
		// if the attribute cache tracks directory existence
		// then prevent redundant directory creation
		if ac.cacheDirs && directoryAlreadyExists {
			return os.ErrExist
		}
		// invalidate existing directory entry (this is redundant but readable)
		if found {
			oldDirAttrCacheItem.invalidate()
		}
		// add (or replace) the directory entry
		newDirAttr := internal.CreateObjAttrDir(options.Name)
		newDirAttrCacheItem := ac.cache.insert(insertOptions{
			attr:     newDirAttr,
			exists:   true,
			cachedAt: time.Now(),
		})
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
		// deleteDirectory may add the parent directory to the cache
		// so we must lock the cache for writing
		ac.cacheLock.Lock()
		defer ac.cacheLock.Unlock()
		err = ac.deleteDirectory(options.Name, deletionTime)
	}

	return err
}

// merge results from our cache into pathMap
func (ac *AttrCache) addDirsNotInCloudToListing(
	listPath string,
	pathList []*internal.ObjAttr,
) ([]*internal.ObjAttr, int) {
	numAdded := 0

	dir, found := ac.cache.get(listPath)
	if !found || !dir.exists() {
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

	return pathList, numAdded
}

// StreamDir : Optionally cache attributes of paths returned by next component
func (ac *AttrCache) StreamDir(
	options internal.StreamDirOptions,
) ([]*internal.ObjAttr, string, error) {
	log.Trace("AttrCache::StreamDir : %s, token=\"%s\"", options.Name, options.Token)

	// try to fetch listing from cache
	cachedPathList, cachedToken, err := ac.fetchCachedDirList(options.Name, options.Token)
	if err == nil {
		return cachedPathList, cachedToken, err
	}
	// listing cache is not complete, so call cloud storage
	pathList, nextToken, err := ac.NextComponent().StreamDir(options)
	if err == nil {
		log.Debug("AttrCache::StreamDir : %s got %d entries from cloud, token=\"%s\"",
			options.Name, len(pathList), nextToken)
		// cache returned list
		ac.cacheAttributes(pathList, options.Name)
		//
		if ac.cacheDirs {
			// remember that this directory is in cloud storage
			if len(pathList) > 0 {
				ac.cacheLock.Lock()
				ac.markAncestorsInCloud(options.Name, time.Now())
				ac.cacheLock.Unlock()
			}
			// merge missing directory cache into the last page of results
			if ac.cacheDirs && nextToken == "" {
				var numAdded int // prevent shadowing pathList in following line
				pathList, numAdded = ac.addDirsNotInCloudToListing(options.Name, pathList)
				log.Info("AttrCache::StreamDir : %s +%d from cache = %d",
					options.Name, numAdded, len(pathList))
			}
		}
	}
	// add cached items in
	if len(cachedPathList) > 0 {
		log.Info(
			"AttrCache::StreamDir : %s merging in %d list cache entries...",
			options.Name,
			len(cachedPathList),
		)
		pathList = append(pathList, cachedPathList...)
	}
	// values should be returned in ascending order by key, without duplicates
	// sort
	slices.SortFunc[[]*internal.ObjAttr, *internal.ObjAttr](
		pathList,
		func(a, b *internal.ObjAttr) int {
			return strings.Compare(a.Path, b.Path)
		},
	)
	// remove duplicates
	pathList = slices.CompactFunc[[]*internal.ObjAttr, *internal.ObjAttr](
		pathList,
		func(a, b *internal.ObjAttr) bool {
			return a.Path == b.Path
		},
	)
	ac.cacheListSegment(pathList, options.Name, options.Token, nextToken)
	log.Trace("AttrCache::StreamDir : %s returning %d entries", options.Name, len(pathList))
	return pathList, nextToken, err
}

// Return directory listing from cache
// Any request other than a request for the next page will return all children,
// and the token for the next page (if there is one).
// If page requests are repeated or backtrack, this may cause unexpected OS behavior.
func (ac *AttrCache) fetchCachedDirList(
	path string,
	token string,
) ([]*internal.ObjAttr, string, error) {
	var pathList []*internal.ObjAttr
	if !ac.cacheOnList {
		return pathList, "", fmt.Errorf("cache on list is disabled")
	}
	// start accessing the cache
	ac.cacheLock.RLock()
	defer ac.cacheLock.RUnlock()
	// get directory cache item
	listDirCache, found := ac.cache.get(path)
	if !found {
		log.Warn("AttrCache::fetchCachedDirList : %s directory not found in cache", path)
		return pathList, "", fmt.Errorf("%s directory not found in cache", path)
	}
	// is the requested data cached?
	if listDirCache.listCache == nil {
		listDirCache.listCache = make(map[string]listCacheSegment)
	}
	cachedListSegment, found := listDirCache.listCache[token]
	if !found {
		// the data for this token is not in the cache
		// don't provide cached data when new (uncached) data is being requested
		log.Info("AttrCache::fetchCachedDirList : %s listing segment %s not cached", path, token)
		return pathList, "", fmt.Errorf("%s directory listing segment %s not cached", path, token)
	}
	// check timeout
	if time.Since(cachedListSegment.cachedAt).Seconds() >= float64(ac.cacheTimeout) {
		log.Info("AttrCache::fetchCachedDirList : %s listing segment %s cache expired", path, token)
		// drop the invalid segment from the list cache
		delete(listDirCache.listCache, token)
		return pathList, "", fmt.Errorf(
			"%s directory listing segment %s cache expired",
			path,
			token,
		)
	}
	log.Trace("AttrCache::fetchCachedDirList : %s token=\"%s\"->\"%s\" serving %d items from cache",
		path, token, cachedListSegment.nextToken, len(cachedListSegment.entries))
	return cachedListSegment.entries, cachedListSegment.nextToken, nil
}

// cacheAttributes : On dir listing cache the attributes for all files
// this will lock and release the mutex for writing
func (ac *AttrCache) cacheAttributes(pathList []*internal.ObjAttr, listDirPath string) {
	// Check whether or not we are supposed to cache on list
	if !ac.cacheOnList {
		return
	}
	// Putting time.Now() inside a loop is heavy as for each item we will do a kernel call to get current time
	// If there are millions of blobs then cost of this is very high.
	currTime := time.Now()
	ac.cacheLock.Lock()
	defer ac.cacheLock.Unlock()
	// if a non-empty pathList was returned by the cloud storage component when listing a directory
	// then that directory is clearly in the cloud
	if len(pathList) > 0 {
		ac.markAncestorsInCloud(listDirPath, currTime)
	}
	for _, attr := range pathList {
		ac.cache.insert(insertOptions{
			attr:        attr,
			exists:      true,
			cachedAt:    currTime,
			fromDirList: true,
		})
	}
	log.Trace("AttrCache::cacheAttributes : %s cached %d items", listDirPath, len(pathList))
}

// cacheListSegment : On dir listing cache the listing
// this will lock and release the mutex for writing
func (ac *AttrCache) cacheListSegment(
	pathList []*internal.ObjAttr,
	listDirPath string,
	token, nextToken string,
) {
	// Check whether or not we are supposed to cache on list
	if !ac.cacheOnList {
		return
	}
	// Putting time.Now() inside a loop is heavy as for each item we will do a kernel call to get current time
	// If there are millions of blobs then cost of this is very high.
	currTime := time.Now()
	ac.cacheLock.Lock()
	defer ac.cacheLock.Unlock()
	// record when the directory was listed, an up to what token
	// this will allow us to serve directory listings from this cache
	listDirItem, found := ac.cache.get(listDirPath)
	if !found {
		log.Err("AttrCache::cacheListSegment : %s directory not found in cache", listDirPath)
		return
	}
	newListCacheSegment := listCacheSegment{
		entries:   pathList,
		nextToken: nextToken,
		cachedAt:  currTime,
	}
	if listDirItem.listCache == nil {
		listDirItem.listCache = make(map[string]listCacheSegment)
	}
	// add the new entry
	listDirItem.listCache[token] = newListCacheSegment
	// scan the listing cache and remove expired entries
	for k, v := range listDirItem.listCache {
		if currTime.Sub(v.cachedAt).Seconds() >= float64(ac.cacheTimeout) {
			delete(listDirItem.listCache, k)
		}
	}
	log.Trace("AttrCache::cacheListSegment : %s cached list entries \"%s\"-\"%s\" (%d items)",
		listDirPath, token, nextToken, len(pathList))
}

// IsDirEmpty: Whether or not the directory is empty
func (ac *AttrCache) IsDirEmpty(options internal.IsDirEmptyOptions) bool {
	log.Trace("AttrCache::IsDirEmpty : %s", options.Name)

	// This function only has a use if we're caching directories
	if !ac.cacheDirs {
		log.Debug(
			"AttrCache::IsDirEmpty : %s Dir cache is disabled. Checking with container",
			options.Name,
		)
		return ac.NextComponent().IsDirEmpty(options)
	}
	// Is the directory in our cache?
	ac.cacheLock.RLock()
	pathInCache := ac.pathExistsInCache(options.Name)
	ac.cacheLock.RUnlock()
	// If the directory does not exist in the attribute cache then let the next component answer
	if !pathInCache {
		log.Debug(
			"AttrCache::IsDirEmpty : %s not found in attr_cache. Checking with container",
			options.Name,
		)
		return ac.NextComponent().IsDirEmpty(options)
	}
	log.Debug("AttrCache::IsDirEmpty : %s found in attr_cache", options.Name)
	// Check if the cached directory is empty or not
	if ac.anyContentsInCache(options.Name) {
		log.Debug("AttrCache::IsDirEmpty : %s has a subpath in attr_cache", options.Name)
		return false
	}
	// Dir is in cache but no contents are, so check with container
	log.Debug(
		"AttrCache::IsDirEmpty : %s children not found in cache. Checking with container",
		options.Name,
	)
	return ac.NextComponent().IsDirEmpty(options)
}

func (ac *AttrCache) anyContentsInCache(prefix string) bool {
	ac.cacheLock.RLock()
	defer ac.cacheLock.RUnlock()

	directory, found := ac.cache.get(prefix)
	if found && directory.exists() {
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

		// check if destination already exists in cache
		if ac.cacheDirs {
			// if attr_cache is tracking directories, validate this rename
			// First, check if the destination directory already exists
			if ac.pathExistsInCache(options.Dst) {
				return os.ErrExist
			}
		} else {
			// TLDR: Dst is guaranteed to be non-existent or empty.
			// Note: We do not need to invalidate children of Dst due to the logic in our FUSE connector, see comments there,
			// but it is always safer to double check than not.
			ac.invalidateDirectory(options.Dst)
		}

		// get the source directory
		srcItem, found := ac.cache.get(options.Src)
		if !found || !srcItem.exists() {
			log.Err("AttrCache::RenameDir : %s source not found", options.Src)
			if ac.cacheDirs {
				return syscall.ENOENT
			}
		} else {
			// move everything over
			srcDir := internal.TruncateDirName(options.Src)
			dstDir := internal.TruncateDirName(options.Dst)
			ac.moveCachedItem(srcItem, srcDir, dstDir, currentTime)
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
		newFileEntry := ac.cache.insert(insertOptions{
			attr:     newFileAttr,
			exists:   true,
			cachedAt: currentTime,
		})
		newFileEntry.setMode(options.Mode)
	}

	return h, err
}

// OpenFile: Update cache with Open results from cloud
func (ac *AttrCache) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace("AttrCache::OpenFile : %s", options.Name)

	h, err := ac.NextComponent().OpenFile(options)
	// sometimes a file is deleted in the cloud concurrently
	// then this cache needs to be updated
	if err != nil && os.IsNotExist(err) {
		currentTime := time.Now()
		ac.cacheLock.Lock()
		defer ac.cacheLock.Unlock()
		cacheItem, found := ac.cache.get(options.Name)
		if found && cacheItem.exists() {
			cacheItem.markDeleted(currentTime)
		}
		if ac.cacheDirs {
			ac.updateAncestorsInCloud(getParentDir(options.Name), currentTime)
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
		ac.cacheLock.Lock()
		defer ac.cacheLock.Unlock()
		toBeDeleted, found := ac.cache.get(options.Name)
		if !found || !toBeDeleted.valid() {
			log.Warn(
				"AttrCache::DeleteFile : %s no valid entry found. Adding entry...",
				options.Name,
			)
			// add deleted file entry
			attr := internal.CreateObjAttr(options.Name, 0, deletionTime)
			toBeDeleted = ac.cache.insert(insertOptions{
				attr:     attr,
				exists:   true,
				cachedAt: deletionTime,
			})
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
		ancestorCacheItem, found := ac.cache.get(dirPath)
		if !found || !ancestorCacheItem.exists() {
			log.Warn("AttrCache::updateAncestorsInCloud : Adding directory entry %s", dirPath)
			ancestorObjAttr := internal.CreateObjAttrDir(dirPath)
			ancestorCacheItem = ac.cache.insert(insertOptions{
				attr:     ancestorObjAttr,
				exists:   true,
				cachedAt: time,
			})
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
		sourceItem, found := ac.cache.get(options.Src)
		if !found || !sourceItem.exists() {
			log.Warn("AttrCache::RenameFile : Source %s does not exist in cache", options.Src)
			// don't return an error - cloud storage still succeeded
			return nil
		}

		// move source item to destination
		ac.moveCachedItem(sourceItem, options.Src, options.Dst, renameTime)
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
	attr, err := ac.GetAttr(
		internal.GetAttrOptions{Name: options.Handle.Path, RetrieveMetadata: true},
	)
	if err != nil {
		// Ignore not exists errors - this can happen if createEmptyFile is set to false
		if !os.IsNotExist(err) && err != syscall.ENOENT {
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

		modifiedEntry, found := ac.cache.get(options.Handle.Path)
		if !found || !modifiedEntry.exists() {
			log.Warn("AttrCache::WriteFile : %s replacing missing cache entry", options.Handle.Path)
			// replace the missing entry
			modifiedAttr := internal.CreateObjAttr(options.Handle.Path, newSize, modifyTime)
			modifiedEntry = ac.cache.insert(insertOptions{
				attr:     modifiedAttr,
				exists:   true,
				cachedAt: modifyTime,
			})
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

		truncatedItem, found := ac.cache.get(options.Name)
		if !found || !truncatedItem.exists() {
			log.Warn("AttrCache::TruncateFile : %s replacing missing cache entry", options.Name)
			// replace the missing entry
			truncatedAttr := internal.CreateObjAttr(options.Name, options.Size, modifyTime)
			truncatedItem = ac.cache.insert(insertOptions{
				attr:     truncatedAttr,
				exists:   true,
				cachedAt: modifyTime,
			})
		}
		truncatedItem.setSize(options.Size, modifyTime)
	}
	return err
}

// Update attribute cache when CopyToFile returns that a file doesn't exist
func (ac *AttrCache) CopyToFile(options internal.CopyToFileOptions) error {
	log.Trace("AttrCache::CopyToFile : %s", options.Name)

	err := ac.NextComponent().CopyToFile(options)
	if err != nil {
		entry, found := ac.cache.get(options.Name)
		if found {
			entry.markDeleted(time.Now())
		}
		// todo: invalidating path here rather than updating with etag
		// due to some changes that are required in az storage comp which
		// were not necessarily required. Once they were done invalidation
		// of the attribute can be removed.
		value, found := ac.cache.get(internal.TruncateDirName(options.Name))
		if found {
			value.invalidate()
		}
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
		if !os.IsNotExist(err) && err != syscall.ENOENT {
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
			entry, found := ac.cache.get(options.Name)
			if found {
				entry.invalidate()
			}
		} else {
			// replace entry
			attr := internal.CreateObjAttr(options.Name, fileStat.Size(), fileStat.ModTime())
			entry := ac.cache.insert(insertOptions{
				attr:     attr,
				exists:   true,
				cachedAt: uploadTime,
			})
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
		toBeInvalid, found := ac.cache.get(options.Handle.Path)
		if found {
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
	// Don't log these by default, as it noticeably affects performance
	// log.Trace("AttrCache::GetAttr : %s", options.Name)

	ac.cacheLock.RLock()
	value, found := ac.cache.get(options.Name)
	ac.cacheLock.RUnlock()
	if found && value.valid() && time.Since(value.cachedAt).Seconds() < float64(ac.cacheTimeout) {
		// Try to serve the request from the attribute cache
		// Is the entry marked deleted?
		if !value.exists() {
			log.Debug("AttrCache::GetAttr : %s (ENOENT) served from cache", options.Name)
			return nil, syscall.ENOENT
		} else {
			return value.attr, nil
		}
	}

	// Get the attributes from next component and cache them
	pathAttr, err := ac.NextComponent().GetAttr(options)

	ac.cacheLock.Lock()
	defer ac.cacheLock.Unlock()

	switch err {
	case nil:
		// Retrieved attributes so cache them
		ac.cache.insert(insertOptions{
			attr:     pathAttr,
			exists:   true,
			cachedAt: time.Now(),
		})
		if ac.cacheDirs {
			ac.markAncestorsInCloud(getParentDir(options.Name), time.Now())
		}
	case syscall.ENOENT:
		// cache this entity not existing
		ac.cache.insert(insertOptions{
			attr:     internal.CreateObjAttr(options.Name, 0, time.Now()),
			exists:   false,
			cachedAt: time.Now(),
		})
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
		linkAttr := internal.CreateObjAttr(
			options.Name,
			int64(len([]byte(options.Target))),
			currentTime,
		)
		linkAttr.Flags.Set(internal.PropFlagSymlink)
		ac.cache.insert(insertOptions{
			attr:     linkAttr,
			exists:   true,
			cachedAt: currentTime,
		})
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
		toBeInvalid, found := ac.cache.get(options.Handle.Path)
		if found {
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

		value, found := ac.cache.get(options.Name)
		if !found {
			log.Err("AttrCache::Chmod : %s not found in cache", options.Name)
		} else if !value.exists() {
			log.Err("AttrCache::Chmod : %s is marked deleted", options.Name)
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

func (ac *AttrCache) CommitData(options internal.CommitDataOptions) error {
	log.Trace("AttrCache::CommitData : %s", options.Name)
	err := ac.NextComponent().CommitData(options)
	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()

		entry, found := ac.cache.get(options.Name)
		if found {
			entry.invalidate()
		}
	}
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
}
