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

package file_cache

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/handlemap"
	"github.com/Seagate/cloudfuse/internal/stats_manager"
)

// Common structure for Component
type FileCache struct {
	internal.BaseComponent

	tmpPath   string          // uses os.Separator (filepath.Join)
	fileLocks *common.LockMap // uses object name (common.JoinUnixFilepath)
	policy    cachePolicy

	createEmptyFile bool
	allowNonEmpty   bool
	cacheTimeout    float64
	cleanupOnStart  bool
	policyTrace     bool
	missedChmodList sync.Map // uses object name (common.JoinUnixFilepath)
	mountPath       string   // uses os.Separator (filepath.Join)
	scheduleOps     sync.Map // uses object name (common.JoinUnixFilepath)
	allowOther      bool
	offloadIO       bool
	syncToFlush     bool
	syncToDelete    bool
	maxCacheSize    float64

	defaultPermission os.FileMode

	refreshSec        uint32
	hardLimit         bool
	diskHighWaterMark float64

	lazyWrite    bool
	fileCloseOpt sync.WaitGroup

	stopAsyncUpload    chan struct{}
	schedule           WeeklySchedule
	uploadNotifyCh     chan struct{}
	alwaysOn           bool
	activeWindows      int
	activeWindowsMutex *sync.Mutex
}

// Structure defining your config parameters
type FileCacheOptions struct {
	// e.g. var1 uint32 `config:"var1"`
	TmpPath string `config:"path"   yaml:"path,omitempty"`
	Policy  string `config:"policy" yaml:"policy,omitempty"`

	Timeout     uint32 `config:"timeout-sec"  yaml:"timeout-sec,omitempty"`
	MaxEviction uint32 `config:"max-eviction" yaml:"max-eviction,omitempty"`

	MaxSizeMB     float64 `config:"max-size-mb"    yaml:"max-size-mb,omitempty"`
	HighThreshold uint32  `config:"high-threshold" yaml:"high-threshold,omitempty"`
	LowThreshold  uint32  `config:"low-threshold"  yaml:"low-threshold,omitempty"`

	CreateEmptyFile bool `config:"create-empty-file"    yaml:"create-empty-file,omitempty"`
	AllowNonEmpty   bool `config:"allow-non-empty-temp" yaml:"allow-non-empty-temp,omitempty"`
	CleanupOnStart  bool `config:"cleanup-on-start"     yaml:"cleanup-on-start,omitempty"`

	EnablePolicyTrace bool `config:"policy-trace" yaml:"policy-trace,omitempty"`
	OffloadIO         bool `config:"offload-io"   yaml:"offload-io,omitempty"`

	SyncToFlush bool `config:"sync-to-flush"                 yaml:"sync-to-flush"`
	SyncNoOp    bool `config:"ignore-sync"                   yaml:"ignore-sync,omitempty"`

	RefreshSec uint32 `config:"refresh-sec" yaml:"refresh-sec,omitempty"`
	HardLimit  bool   `config:"hard-limit"  yaml:"hard-limit,omitempty"`
}

type openFileOptions struct {
	flags int
	fMode fs.FileMode
}

const (
	compName                = "file_cache"
	defaultMaxEviction      = 5000
	defaultMaxThreshold     = 80
	defaultMinThreshold     = 60
	defaultFileCacheTimeout = 216000
	minimumFileCacheTimeout = 1
	defaultCacheUpdateCount = 100
	MB                      = 1024 * 1024
)

/*
	In file cache, all calls to Open or OpenFile are done by the implementation in common,
	rather than by calling os.Open or os.OpenFile. This is due to an issue on Windows, where
	the implementation in os is not correct.

	If we are on Windows, we need to use our custom OpenFile or Open function which allows a file
	in the file cache to be deleted and renamed when open, which our codebase relies on.
	See the following issue to see why we need to do this ourselves
	https://github.com/golang/go/issues/32088
*/

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &FileCache{}

var fileCacheStatsCollector *stats_manager.StatsCollector

func (fc *FileCache) Name() string {
	return compName
}

func (fc *FileCache) SetName(name string) {
	fc.BaseComponent.SetName(name)
}

func (fc *FileCache) SetNextComponent(nc internal.Component) {
	fc.BaseComponent.SetNextComponent(nc)
}

func (fc *FileCache) Priority() internal.ComponentPriority {
	return internal.EComponentPriority.LevelMid()
}

// Start : Pipeline calls this method to start the component functionality
//
//	this shall not block the call otherwise pipeline will not start
func (fc *FileCache) Start(ctx context.Context) error {
	log.Trace("Starting component : %s", fc.Name())

	if fc.cleanupOnStart {
		err := common.TempCacheCleanup(fc.tmpPath)
		if err != nil {
			return fmt.Errorf("error in %s error [fail to cleanup temp cache]", fc.Name())
		}
	}

	if fc.policy == nil {
		return fmt.Errorf("config error in %s error [cache policy missing]", fc.Name())
	}

	err := fc.policy.StartPolicy()
	if err != nil {
		return fmt.Errorf("config error in %s error [fail to start policy]", fc.Name())
	}

	// create stats collector for file cache
	fileCacheStatsCollector = stats_manager.NewStatsCollector(fc.Name())
	log.Debug("Starting file cache stats collector")

	err = fc.SetupScheduler()
	if err != nil {
		log.Warn("FileCache::Start : Failed to setup scheduler [%s]", err.Error())
	}

	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (fc *FileCache) Stop() error {
	log.Trace("Stopping component : %s", fc.Name())

	// Wait for all async upload to complete if any
	if fc.lazyWrite {
		log.Info("FileCache::Stop : Waiting for async close to complete")
		fc.fileCloseOpt.Wait()
	}

	_ = fc.policy.ShutdownPolicy()
	if !fc.allowNonEmpty {
		_ = common.TempCacheCleanup(fc.tmpPath)
	}

	fileCacheStatsCollector.Destroy()

	return nil
}

// GenConfig : Generate default config for the component
func (fc *FileCache) GenConfig() string {
	log.Info("FileCache::Configure : config generation started")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n%s:", fc.Name()))

	tmpPath := ""
	_ = config.UnmarshalKey("tmp-path", &tmpPath)

	directIO := false
	_ = config.UnmarshalKey("direct-io", &directIO)

	timeout := defaultFileCacheTimeout
	if directIO {
		timeout = 0
	}

	sb.WriteString(fmt.Sprintf("\n  path: %v", common.ExpandPath(tmpPath)))
	sb.WriteString(fmt.Sprintf("\n  timeout-sec: %v", timeout))

	return sb.String()
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (fc *FileCache) Configure(_ bool) error {
	log.Trace("FileCache::Configure : %s", fc.Name())

	conf := FileCacheOptions{}
	conf.SyncToFlush = true
	err := config.UnmarshalKey(compName, &conf)
	if err != nil {
		log.Err("FileCache: config error [invalid config attributes]")
		return fmt.Errorf("config error in %s [%s]", fc.Name(), err.Error())
	}

	fc.createEmptyFile = conf.CreateEmptyFile
	if config.IsSet(compName + ".timeout-sec") {
		fc.cacheTimeout = max(float64(conf.Timeout), minimumFileCacheTimeout)
	} else {
		fc.cacheTimeout = float64(defaultFileCacheTimeout)
	}

	directIO := false
	_ = config.UnmarshalKey("direct-io", &directIO)

	if directIO {
		fc.cacheTimeout = 0
		log.Crit("FileCache::Configure : Direct IO mode enabled, cache timeout is set to 0")
	}

	fc.allowNonEmpty = conf.AllowNonEmpty
	fc.cleanupOnStart = conf.CleanupOnStart
	fc.policyTrace = conf.EnablePolicyTrace
	fc.offloadIO = conf.OffloadIO
	fc.syncToFlush = conf.SyncToFlush
	fc.syncToDelete = !conf.SyncNoOp
	fc.refreshSec = conf.RefreshSec
	fc.hardLimit = conf.HardLimit

	err = config.UnmarshalKey("lazy-write", &fc.lazyWrite)
	if err != nil {
		log.Err("FileCache: config error [unable to obtain lazy-write]")
		return fmt.Errorf("config error in %s [%s]", fc.Name(), err.Error())
	}

	fc.tmpPath = filepath.Clean(common.ExpandPath(conf.TmpPath))
	if fc.tmpPath == "" || fc.tmpPath == "." {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Err("FileCache: Failed to get user home directory [%s]", err.Error())
		}
		log.Warn(
			"FileCache: tmp-path not set in config file, defaulting to $HOME/.cloudfuse/file_cache",
		)
		fc.tmpPath = filepath.Join(homeDir, ".cloudfuse", "file_cache")
	}

	err = config.UnmarshalKey("mount-path", &fc.mountPath)
	if err != nil {
		log.Err("FileCache: config error [unable to obtain Mount Path]")
		return fmt.Errorf("config error in %s [%s]", fc.Name(), err.Error())
	}
	if filepath.Clean(fc.mountPath) == filepath.Clean(fc.tmpPath) {
		log.Err("FileCache: config error [tmp-path is same as mount path]")
		return fmt.Errorf("config error in %s error [tmp-path is same as mount path]", fc.Name())
	}

	// Extract values from 'conf' and store them as you wish here
	_, err = os.Stat(fc.tmpPath)
	if os.IsNotExist(err) {
		log.Err("FileCache: config error [tmp-path does not exist. attempting to create tmp-path.]")
		err := os.MkdirAll(fc.tmpPath, os.FileMode(0755))
		if err != nil {
			log.Err("FileCache: config error creating directory after clean [%s]", err.Error())
			return fmt.Errorf("config error in %s [%s]", fc.Name(), err.Error())
		}
	}

	avail, err := fc.getAvailableSize()
	if err != nil {
		log.Err(
			"FileCache::Configure : config error %s [%s]. Assigning a default value of 4GB or if any value is assigned to .disk-size-mb in config.",
			fc.Name(),
			err.Error(),
		)
		fc.maxCacheSize = 4192
	} else {
		fc.maxCacheSize = 0.8 * float64(avail) / (MB)
	}

	if config.IsSet(compName+".max-size-mb") && conf.MaxSizeMB != 0 {
		fc.maxCacheSize = conf.MaxSizeMB
	}

	if !isLocalDirEmpty(fc.tmpPath) && !fc.allowNonEmpty {
		log.Err("FileCache: config error %s directory is not empty", fc.tmpPath)
		return fmt.Errorf("config error in %s [%s]", fc.Name(), "temp directory not empty")
	}

	err = config.UnmarshalKey("allow-other", &fc.allowOther)
	if err != nil {
		log.Err("FileCache::Configure : config error [unable to obtain allow-other]")
		return fmt.Errorf("config error in %s [%s]", fc.Name(), err.Error())
	}

	if fc.allowOther {
		fc.defaultPermission = common.DefaultAllowOtherPermissionBits
	} else {
		fc.defaultPermission = common.DefaultFilePermissionBits
	}

	cacheConfig := fc.GetPolicyConfig(conf)
	fc.policy = NewLRUPolicy(cacheConfig)
	if fc.policy == nil {
		log.Err("FileCache::Configure : failed to create cache eviction policy")
		return fmt.Errorf("config error in %s [%s]", fc.Name(), "failed to create cache policy")
	}

	if config.IsSet(compName + ".sync-to-flush") {
		log.Warn("Sync will upload current contents of file.")
	}

	fc.diskHighWaterMark = 0
	if conf.HardLimit && conf.MaxSizeMB != 0 {
		fc.diskHighWaterMark = (((conf.MaxSizeMB * MB) * float64(cacheConfig.highThreshold)) / 100)
	}

	if config.IsSet(compName + ".schedule") {
		var rawSchedule []map[string]interface{}
		err := config.UnmarshalKey(compName+".schedule", &rawSchedule)
		if err != nil {
			log.Err(
				"FileCache::Configure : Failed to parse schedule configuration [%s]",
				err.Error(),
			)
		} else {
			// Convert raw schedule to WeeklySchedule
			fc.schedule = make(WeeklySchedule, 0, len(rawSchedule))
			for _, rawWindow := range rawSchedule {
				window := UploadWindow{}
				if name, ok := rawWindow["name"].(string); ok {
					window.Name = name
				}
				if cronStr, ok := rawWindow["cron"].(string); ok {
					window.CronExpr = cronStr
				}
				if durStr, ok := rawWindow["duration"].(string); ok {
					window.Duration = durStr
				}
				if !isValidCronExpression(window.CronExpr) {
					log.Err("FileCache::Configure : Invalid cron expression '%s' for schedule window '%s', skipping",
						window.CronExpr, window.Name)
					continue
				}

				// Validate duration
				_, err := time.ParseDuration(window.Duration)
				if err != nil {
					log.Err("FileCache::Configure : Invalid duration '%s' for schedule window '%s': %v, skipping",
						window.Duration, window.Name, err)
					continue
				}

				fc.schedule = append(fc.schedule, window)
				log.Info("FileCache::Configure : Parsed schedule %s: cron=%s, duration=%s",
					window.Name, window.CronExpr, window.Duration)
			}
		}
	}

	log.Crit(
		"FileCache::Configure : create-empty %t, cache-timeout %d, tmp-path %s, max-size-mb %d, high-mark %d, low-mark %d, refresh-sec %v, max-eviction %v, hard-limit %v, policy %s, allow-non-empty-temp %t, cleanup-on-start %t, policy-trace %t, offload-io %t, sync-to-flush %t, ignore-sync %t, defaultPermission %v, diskHighWaterMark %v, maxCacheSize %v, mountPath %v, schedule-entries %d",
		fc.createEmptyFile,
		int(fc.cacheTimeout),
		fc.tmpPath,
		int(cacheConfig.maxSizeMB),
		int(cacheConfig.highThreshold),
		int(cacheConfig.lowThreshold),
		fc.refreshSec,
		cacheConfig.maxEviction,
		fc.hardLimit,
		conf.Policy,
		fc.allowNonEmpty,
		fc.cleanupOnStart,
		fc.policyTrace,
		fc.offloadIO,
		fc.syncToFlush,
		fc.syncToDelete,
		fc.defaultPermission,
		fc.diskHighWaterMark,
		fc.maxCacheSize,
		fc.mountPath,
		len(fc.schedule),
	)

	return nil
}

// OnConfigChange : If component has registered, on config file change this method is called
func (fc *FileCache) OnConfigChange() {
	log.Trace("FileCache::OnConfigChange : %s", fc.Name())

	conf := FileCacheOptions{}
	conf.SyncToFlush = true
	err := config.UnmarshalKey(compName, &conf)
	if err != nil {
		log.Err("FileCache: config error [invalid config attributes]")
	}

	fc.createEmptyFile = conf.CreateEmptyFile
	fc.cacheTimeout = max(float64(conf.Timeout), minimumFileCacheTimeout)
	fc.policyTrace = conf.EnablePolicyTrace
	fc.offloadIO = conf.OffloadIO
	fc.maxCacheSize = conf.MaxSizeMB
	fc.syncToFlush = conf.SyncToFlush
	fc.syncToDelete = !conf.SyncNoOp
	_ = fc.policy.UpdateConfig(fc.GetPolicyConfig(conf))
}

func (fc *FileCache) GetPolicyConfig(conf FileCacheOptions) cachePolicyConfig {
	// A user provided value of 0 doesn't make sense for MaxEviction, HighThreshold or LowThreshold.
	if conf.MaxEviction == 0 {
		conf.MaxEviction = defaultMaxEviction
	}
	if conf.HighThreshold == 0 {
		conf.HighThreshold = defaultMaxThreshold
	}
	if conf.LowThreshold == 0 {
		conf.LowThreshold = defaultMinThreshold
	}

	cacheConfig := cachePolicyConfig{
		tmpPath:       fc.tmpPath,
		maxEviction:   conf.MaxEviction,
		highThreshold: float64(conf.HighThreshold),
		lowThreshold:  float64(conf.LowThreshold),
		cacheTimeout:  uint32(fc.cacheTimeout),
		maxSizeMB:     conf.MaxSizeMB,
		fileLocks:     fc.fileLocks,
		policyTrace:   conf.EnablePolicyTrace,
	}

	return cacheConfig
}

func (fc *FileCache) StatFs() (*common.Statfs_t, bool, error) {

	statfs, populated, err := fc.NextComponent().StatFs()
	if populated {
		return statfs, populated, err
	}

	log.Trace("FileCache::StatFs")

	// cache_size = f_blocks * f_frsize/1024
	// cache_size - used = f_frsize * f_bavail/1024
	// cache_size - used = vfs.f_bfree * vfs.f_frsize / 1024
	// if cache size is set to 0 then we have the root mount usage
	maxCacheSize := fc.maxCacheSize * MB
	if maxCacheSize == 0 {
		log.Err("FileCache::StatFs : Not responding to StatFs because max cache size is zero")
		return nil, false, nil
	}
	usage, _ := common.GetUsage(fc.tmpPath)
	available := maxCacheSize - usage*MB

	// how much space is available on the underlying file system?
	availableOnCacheFS, err := fc.getAvailableSize()
	if err != nil {
		log.Err(
			"FileCache::StatFs : Not responding to StatFs because getAvailableSize failed. Here's why: %v",
			err,
		)
		return nil, false, err
	}

	const blockSize = 4096

	stat := common.Statfs_t{
		Blocks:  uint64(maxCacheSize) / uint64(blockSize),
		Bavail:  uint64(max(0, available)) / uint64(blockSize),
		Bfree:   availableOnCacheFS / uint64(blockSize),
		Bsize:   blockSize,
		Ffree:   1e9,
		Files:   1e9,
		Frsize:  blockSize,
		Namemax: 255,
	}

	log.Debug(
		"FileCache::StatFs : responding with free=%d avail=%d blocks=%d (bsize=%d)",
		stat.Bfree,
		stat.Bavail,
		stat.Blocks,
		stat.Bsize,
	)
	return &stat, true, nil
}

// isLocalDirEmpty: Whether or not the local directory is empty.
func isLocalDirEmpty(path string) bool {
	f, _ := common.Open(path)
	defer f.Close()

	_, err := f.Readdirnames(1)
	return err == io.EOF
}

// Note: The primary purpose of the file cache is to keep track of files that are opened by the user.
// So we do not need to support some APIs like Create Directory since the file cache will manage
// creating local directories as needed.

// DeleteDir: Recursively invalidate the directory and its children
func (fc *FileCache) DeleteDir(options internal.DeleteDirOptions) error {
	log.Trace("FileCache::DeleteDir : %s", options.Name)

	// The libfuse component only calls DeleteDir on empty directories, so this directory must be empty
	err := fc.NextComponent().DeleteDir(options)
	if err != nil {
		log.Err("FileCache::DeleteDir : %s failed", options.Name)
		// There is a chance that meta file for directory was not created in which case
		// rest api delete will fail while we still need to cleanup the local cache for the same
	} else {
		fc.policy.CachePurge(filepath.Join(fc.tmpPath, options.Name))
	}

	return err
}

// StreamDir : Add local files to the list retrieved from storage container
func (fc *FileCache) StreamDir(
	options internal.StreamDirOptions,
) ([]*internal.ObjAttr, string, error) {
	// For stream directory, there are three different child path situations we have to potentially handle.
	// 1. Path in storage but not in local cache
	// 2. Path not in storage but in local cache (this could happen if we recently created the file [and are currently writing to it]) (also supports immutable containers)
	// 3. Path in storage and in local cache (this could result in dirty properties on the service if we recently wrote to the file)

	// To cover case 1, grab all entries from storage
	attrs, token, err := fc.NextComponent().StreamDir(options)
	if err != nil {
		return attrs, token, err
	}

	// Get files from local cache
	localPath := filepath.Join(fc.tmpPath, options.Name)
	dirents, err := os.ReadDir(localPath)
	if err != nil {
		return attrs, token, nil
	}

	i := 0 // Index for cloud
	j := 0 // Index for local cache

	// Iterate through attributes from cloud and local cache, adding the elements in order alphabetically
	for i < len(attrs) && j < len(dirents) {
		attr := attrs[i]
		dirent := dirents[j]

		if attr.Name < dirent.Name() {
			i++
		} else if attr.Name > dirent.Name() {
			j++
		} else {
			// Case 3: Item is in both local cache and cloud
			if !attr.IsDir() {
				flock := fc.fileLocks.Get(attr.Path)
				flock.Lock()
				// use os.Stat instead of entry.Info() to be sure we get good info (with flock locked)
				info, err := os.Stat(filepath.Join(localPath, dirent.Name())) // Grab local cache attributes
				flock.Unlock()
				if err == nil {
					// attr is a pointer returned by NextComponent
					// modifying attr could corrupt cached directory listings
					// to update properties, we need to make a deep copy first
					newAttr := *attr
					newAttr.Mtime = info.ModTime()
					newAttr.Size = info.Size()
					attrs[i] = &newAttr
				}
			}
			i++
			j++
		}
	}

	// Case 2: file is only in local cache
	if token == "" {
		for _, entry := range dirents {
			entryPath := common.JoinUnixFilepath(options.Name, entry.Name())
			if !entry.IsDir() {
				// This is an overhead for streamdir for now
				// As list is paginated we have no way to know whether this particular item exists both in local cache
				// and container or not. So we rely on getAttr to tell if entry was cached then it exists in cloud storage too
				// If entry does not exists on storage then only return a local item here.
				_, err := fc.NextComponent().GetAttr(internal.GetAttrOptions{Name: entryPath})
				if err != nil && (err == syscall.ENOENT || os.IsNotExist(err)) {
					// get the lock on the file, to allow any pending operation to complete
					flock := fc.fileLocks.Get(entryPath)
					flock.Lock()
					// use os.Stat instead of entry.Info() to be sure we get good info (with flock locked)
					info, err := os.Stat(
						filepath.Join(localPath, entry.Name()),
					) // Grab local cache attributes
					flock.Unlock()
					// If local file is not locked then only use its attributes otherwise rely on container attributes
					if err == nil {
						// Case 2 (file only in local cache) so create a new attributes and add them to the storage attributes
						log.Debug("FileCache::StreamDir : serving %s from local cache", entryPath)
						attr := newObjAttr(entryPath, info)
						attrs = append(attrs, attr)
					}
				}
			}
		}
	}

	return attrs, token, err
}

// IsDirEmpty: Whether or not the directory is empty
func (fc *FileCache) IsDirEmpty(options internal.IsDirEmptyOptions) bool {
	log.Trace("FileCache::IsDirEmpty : %s", options.Name)

	// If the directory does not exist locally then call the next component
	localPath := filepath.Join(fc.tmpPath, options.Name)
	f, err := common.Open(localPath)
	if err == nil {
		log.Debug("FileCache::IsDirEmpty : %s found in local cache", options.Name)

		// Check local cache directory is empty or not
		path, err := f.Readdirnames(1)

		// If the local directory has a path in it, it is likely due to !createEmptyFile.
		if err == nil && !fc.createEmptyFile && len(path) > 0 {
			log.Debug(
				"FileCache::IsDirEmpty : %s had a subpath in the local cache (%s)",
				options.Name,
				path[0],
			)
			return false
		}

		// If there are files in local cache then don't allow deletion of directory
		if err != io.EOF {
			// Local directory is not empty fail the call
			log.Debug("FileCache::IsDirEmpty : %s was not empty in local cache", options.Name)
			return false
		}
	} else if os.IsNotExist(err) {
		// Not found in local cache so check with container
		log.Debug("FileCache::IsDirEmpty : %s not found in local cache", options.Name)
	} else {
		// Unknown error, check with container
		log.Err("FileCache::IsDirEmpty : %s failed while checking local cache [%s]", options.Name, err.Error())
	}

	log.Debug("FileCache::IsDirEmpty : %s checking with container", options.Name)
	return fc.NextComponent().IsDirEmpty(options)
}

// DeleteEmptyDirs: delete empty directories in local cache, return error if directory is not empty
func (fc *FileCache) DeleteEmptyDirs(options internal.DeleteDirOptions) (bool, error) {
	localPath := options.Name
	if !strings.Contains(options.Name, fc.tmpPath) {
		localPath = filepath.Join(fc.tmpPath, options.Name)
	}

	log.Trace("FileCache::DeleteEmptyDirs : %s", localPath)

	entries, err := os.ReadDir(localPath)
	if err != nil {
		log.Debug(
			"FileCache::DeleteEmptyDirs : Unable to read directory %s [%s]",
			localPath,
			err.Error(),
		)
		return false, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			val, err := fc.DeleteEmptyDirs(internal.DeleteDirOptions{
				Name: filepath.Join(localPath, entry.Name()),
			})
			if err != nil {
				log.Err(
					"FileCache::DeleteEmptyDirs : Unable to delete directory %s [%s]",
					localPath,
					err.Error(),
				)
				return val, err
			}
		} else {
			log.Err("FileCache::DeleteEmptyDirs : Directory %s is not empty, contains file %s", localPath, entry.Name())
			return false, fmt.Errorf("unable to delete directory %s, contains file %s", localPath, entry.Name())
		}
	}

	if !strings.EqualFold(fc.tmpPath, localPath) {
		err = os.Remove(localPath)
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

// RenameDir: Recursively move the source directory
func (fc *FileCache) RenameDir(options internal.RenameDirOptions) error {
	log.Trace("FileCache::RenameDir : src=%s, dst=%s", options.Src, options.Dst)

	// get a list of source objects form both cloud and cache
	// cloud
	var cloudObjects []string
	cloudObjects, err := fc.listCloudObjects(options.Src)
	if err != nil {
		log.Err(
			"FileCache::RenameDir : %s listCloudObjects failed. Here's why: %v",
			options.Src,
			err,
		)
		return err
	}
	// cache
	var localObjects []string
	localObjects, err = fc.listCachedObjects(options.Src)
	if err != nil {
		log.Err(
			"FileCache::RenameDir : %s listCachedObjects failed. Here's why: %v",
			options.Src,
			err,
		)
		return err
	}
	// combine the lists
	objectNames := combineLists(cloudObjects, localObjects)

	// add object destinations, and sort the result
	for _, srcName := range objectNames {
		dstName := strings.Replace(srcName, options.Src, options.Dst, 1)
		objectNames = append(objectNames, dstName)
	}
	sort.Strings(objectNames)

	// acquire a file lock on each entry (and defer unlock)
	var flocks []*common.LockMapItem
	for _, objectName := range objectNames {
		flock := fc.fileLocks.Get(objectName)
		flocks = append(flocks, flock)
		flock.Lock()
	}
	defer unlockAll(flocks)

	// rename the directory in the cloud
	err = fc.NextComponent().RenameDir(options)
	if err != nil {
		log.Err("FileCache::RenameDir : error %s [%s]", options.Src, err.Error())
		return err
	}

	// move the files in local storage
	localSrcPath := filepath.Join(fc.tmpPath, options.Src)
	localDstPath := filepath.Join(fc.tmpPath, options.Dst)
	// WalkDir goes through the tree in lexical order so 'dir' always comes before 'dir/file'
	var directoriesToPurge []string
	_ = filepath.WalkDir(localSrcPath, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d != nil {
			newPath := strings.Replace(path, localSrcPath, localDstPath, 1)
			if !d.IsDir() {
				log.Debug("FileCache::RenameDir : Renaming local file %s -> %s", path, newPath)
				// get object names and locks
				srcName := fc.getObjectName(path)
				dstName := fc.getObjectName(newPath)
				sflock := fc.fileLocks.Get(srcName)
				dflock := fc.fileLocks.Get(dstName)
				_ = fc.renameLocalFile(srcName, dstName, sflock, dflock, false)
			} else {
				log.Debug("FileCache::RenameDir : Creating local destination directory %s", newPath)
				// create the new directory
				mkdirErr := os.MkdirAll(newPath, fc.defaultPermission)
				if mkdirErr != nil {
					// log any error but do nothing about it
					log.Warn("FileCache::RenameDir : Failed to created directory %s. Here's why: %v", newPath, mkdirErr)
				}
				// remember to delete the src directory later (after its contents are deleted)
				directoriesToPurge = append(directoriesToPurge, path)
			}
		} else {
			// stat(localPath) failed. err is the one returned by stat
			// documentation: https://pkg.go.dev/io/fs#WalkDirFunc
			if os.IsNotExist(err) {
				// none of the files that were moved actually exist in local storage
				log.Info("FileCache::RenameDir : %s does not exist in local cache.", options.Src)
			} else if err != nil {
				log.Warn("FileCache::RenameDir : %s stat err [%v].", options.Src, err)
			}
		}
		return nil
	})

	// clean up leftover source directories in reverse order
	for i := len(directoriesToPurge) - 1; i >= 0; i-- {
		log.Debug("FileCache::RenameDir : Removing local directory %s", directoriesToPurge[i])
		fc.policy.CachePurge(directoriesToPurge[i])
	}

	// update any lazy open handles (which are not in the local listing)
	for _, srcName := range cloudObjects {
		dstName := strings.Replace(srcName, options.Src, options.Dst, 1)
		// get locks
		sflock := fc.fileLocks.Get(srcName)
		dflock := fc.fileLocks.Get(dstName)
		// update any remaining open handles
		fc.renameOpenHandles(srcName, dstName, sflock, dflock)
	}

	return nil
}

// recursively list all objects in the container at the given prefix / directory
func (fc *FileCache) listCloudObjects(prefix string) (objectNames []string, err error) {
	var done bool
	var token string
	for !done {
		var attrSlice []*internal.ObjAttr
		attrSlice, token, err = fc.NextComponent().
			StreamDir(internal.StreamDirOptions{Name: prefix, Token: token})
		if err != nil {
			return
		}
		for i := len(attrSlice) - 1; i >= 0; i-- {
			attr := attrSlice[i]
			if !attr.IsDir() {
				objectNames = append(objectNames, attr.Path)
			} else {
				// recurse!
				var subdirObjectNames []string
				subdirObjectNames, err = fc.listCloudObjects(attr.Path)
				if err != nil {
					return
				}
				objectNames = append(objectNames, subdirObjectNames...)
			}
		}
		done = token == ""
	}
	sort.Strings(objectNames)
	return
}

// recursively list all files in the directory
func (fc *FileCache) listCachedObjects(directory string) (objectNames []string, err error) {
	localDirPath := filepath.Join(fc.tmpPath, directory)
	walkDirErr := filepath.WalkDir(localDirPath, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d != nil {
			if !d.IsDir() {
				objectName := fc.getObjectName(path)
				objectNames = append(objectNames, objectName)
			}
		} else {
			// stat(localPath) failed. err is the one returned by stat
			// documentation: https://pkg.go.dev/io/fs#WalkDirFunc
			if os.IsNotExist(err) {
				// none of the files that were moved actually exist in local storage
				log.Info("FileCache::listObjects : %s does not exist in local cache.", directory)
			} else if err != nil {
				log.Warn("FileCache::listObjects : %s stat err [%v].", directory, err)
			}
		}
		return nil
	})
	if walkDirErr != nil && !os.IsNotExist(walkDirErr) {
		err = walkDirErr
	}
	sort.Strings(objectNames)
	return
}

func combineLists(listA, listB []string) []string {
	// since both lists are sorted, we can combine the two lists using a double-indexed for loop
	combinedList := listA
	i := 0 // Index for listA
	j := 0 // Index for listB
	// Iterate through both lists, adding entries from B that are missing from A
	for i < len(listA) && j < len(listB) {
		itemA := listA[i]
		itemB := listB[j]
		if itemA < itemB {
			i++
		} else if itemA > itemB {
			// we could insert here, but it's probably better to just sort later
			combinedList = append(combinedList, itemB)
			j++
		} else {
			i++
			j++
		}
	}
	// sort and return
	sort.Strings(combinedList)
	return combinedList
}

func (fc *FileCache) getObjectName(localPath string) string {
	relPath, err := filepath.Rel(fc.tmpPath, localPath)
	if err != nil {
		relPath = strings.TrimPrefix(localPath, fc.tmpPath+string(filepath.Separator))
		log.Warn(
			"FileCache::getObjectName : filepath.Rel failed on path %s [%v]. Using TrimPrefix: %s",
			localPath,
			err,
			relPath,
		)
	}
	return common.NormalizeObjectName(relPath)
}

func unlockAll(flocks []*common.LockMapItem) {
	for _, flock := range flocks {
		flock.Unlock()
	}
}

// CreateFile: Create the file in local cache.
func (fc *FileCache) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	//defer exectime.StatTimeCurrentBlock("FileCache::CreateFile")()
	log.Trace("FileCache::CreateFile : name=%s, mode=%d", options.Name, options.Mode)

	flock := fc.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	// createEmptyFile was added to optionally support immutable containers. If customers do not care about immutability they can set this to true.
	if fc.createEmptyFile {
		newF, err := fc.NextComponent().CreateFile(options)
		if err != nil {
			log.Err("FileCache::CreateFile : Failed to create file %s", options.Name)
			return nil, err
		}
		newF.GetFileObject().Close()
	}

	// Create the file in local cache
	localPath := filepath.Join(fc.tmpPath, options.Name)
	fc.policy.CacheValid(localPath)

	err := os.MkdirAll(filepath.Dir(localPath), fc.defaultPermission)
	if err != nil {
		log.Err(
			"FileCache::CreateFile : unable to create local directory %s [%s]",
			options.Name,
			err.Error(),
		)
		return nil, err
	}

	// Open the file and grab a shared lock to prevent deletion by the cache policy.
	f, err := common.OpenFile(localPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, options.Mode)
	if err != nil {
		log.Err(
			"FileCache::CreateFile : error opening local file %s [%s]",
			options.Name,
			err.Error(),
		)
		return nil, err
	}
	// The user might change permissions WHILE creating the file therefore we need to account for that
	if options.Mode != common.DefaultFilePermissionBits {
		fc.missedChmodList.LoadOrStore(options.Name, true)
	}

	// Increment the handle count in this lock item as there is one handle open for this now
	flock.Inc()

	handle := handlemap.NewHandle(options.Name)
	handle.UnixFD = uint64(f.Fd())

	if !fc.offloadIO {
		handle.Flags.Set(handlemap.HandleFlagCached)
	}
	log.Info("FileCache::CreateFile : file=%s, fd=%d", options.Name, f.Fd())

	handle.SetFileObject(f)

	// If an empty file is created in cloud storage then there is no need to upload if FlushFile is called immediately after CreateFile.
	if !fc.createEmptyFile {
		handle.Flags.Set(handlemap.HandleFlagDirty)
	}

	// update state
	flock.LazyOpen = false

	return handle, nil
}

// Validate that storage 404 errors truly correspond to Does Not Exist.
// path: the storage path
// err: the storage error
// method: the caller method name
// recoverable: whether or not case 2 is recoverable on flush/close of the file
func (fc *FileCache) validateStorageError(
	path string,
	err error,
	method string,
	recoverable bool,
) error {
	// For methods that take in file name, the goal is to update the path in cloud storage and the local cache.
	// See comments in GetAttr for the different situations we can run into. This specifically handles case 2.
	if os.IsNotExist(err) {
		log.Debug("FileCache::%s : %s does not exist in cloud storage", method, path)
		if !fc.createEmptyFile {
			// Check if the file exists in the local cache
			// (policy might not think the file exists if the file is merely marked for eviction and not actually evicted yet)
			localPath := filepath.Join(fc.tmpPath, path)
			if _, err := os.Stat(localPath); os.IsNotExist(err) {
				// If the file is not in the local cache, then the file does not exist.
				log.Err("FileCache::%s : %s does not exist in local cache", method, path)
				return syscall.ENOENT
			} else {
				if !recoverable {
					log.Err("FileCache::%s : %s has not been closed/flushed yet, unable to recover this operation on close", method, path)
					return syscall.EIO
				} else {
					log.Info("FileCache::%s : %s has not been closed/flushed yet, we can recover this operation on close", method, path)
					return nil
				}
			}
		}
	}
	return err
}

func (fc *FileCache) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("FileCache::DeleteFile : name=%s", options.Name)

	flock := fc.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	err := fc.NextComponent().DeleteFile(options)
	err = fc.validateStorageError(options.Name, err, "DeleteFile", true)
	if err != nil {
		log.Err("FileCache::DeleteFile : error  %s [%s]", options.Name, err.Error())
		return err
	}

	localPath := filepath.Join(fc.tmpPath, options.Name)
	fc.policy.CachePurge(localPath)

	// Delete from scheduleOps if it exists
	fc.scheduleOps.Delete(options.Name)

	// update file state
	flock.LazyOpen = false

	return nil
}

func openCompleted(handle *handlemap.Handle) bool {
	handle.Lock()
	defer handle.Unlock()
	_, found := handle.GetValue("openFileOptions")
	return !found
}

// flock must already be locked before calling this function
func (fc *FileCache) openFileInternal(handle *handlemap.Handle, flock *common.LockMapItem) error {
	log.Trace("FileCache::openFileInternal : name=%s", handle.Path)

	handle.Lock()
	defer handle.Unlock()

	//extract flags and mode out of the value from handle
	var flags int
	var fMode fs.FileMode
	val, found := handle.GetValue("openFileOptions")
	if !found {
		return nil
	}
	fileOptions := val.(openFileOptions)
	flags = fileOptions.flags
	fMode = fileOptions.fMode

	localPath := filepath.Join(fc.tmpPath, handle.Path)
	var f *os.File

	fc.policy.CacheValid(localPath)
	downloadRequired, fileExists, attr, err := fc.isDownloadRequired(localPath, handle.Path, flock)
	if err != nil && !os.IsNotExist(err) {
		log.Err(
			"FileCache::openFileInternal : Failed to check if download is required for %s [%s]",
			handle.Path,
			err.Error(),
		)
	}

	fileMode := fc.defaultPermission
	if downloadRequired {
		log.Debug("FileCache::openFileInternal : Need to download %s", handle.Path)

		fileSize := int64(0)
		if attr != nil {
			fileSize = int64(attr.Size)
		}

		if fileExists {
			log.Debug("FileCache::openFileInternal : Delete cached file %s", handle.Path)

			err := deleteFile(localPath)
			if err != nil && !os.IsNotExist(err) {
				log.Err("FileCache::openFileInternal : Failed to delete old file %s", handle.Path)
			}
		} else {
			// Create the file if if doesn't already exist.
			err := os.MkdirAll(filepath.Dir(localPath), fc.defaultPermission)
			if err != nil {
				log.Err("FileCache::openFileInternal : error creating directory structure for file %s [%s]", handle.Path, err.Error())
				return err
			}
		}

		// Open the file in write mode.
		f, err = common.OpenFile(localPath, os.O_CREATE|os.O_RDWR, fMode)
		if err != nil {
			log.Err(
				"FileCache::openFileInternal : error creating new file %s [%s]",
				handle.Path,
				err.Error(),
			)
			return err
		}

		if flags&os.O_TRUNC != 0 {
			fileSize = 0
		}

		if fileSize > 0 {
			// Download/Copy the file from storage to the local file.
			// We pass a count of 0 to get the entire object
			err = fc.NextComponent().CopyToFile(
				internal.CopyToFileOptions{
					Name:   handle.Path,
					Offset: 0,
					Count:  0,
					File:   f,
				})
			if err != nil {
				// File was created locally and now download has failed so we need to delete it back from local cache
				log.Err(
					"FileCache::openFileInternal : error downloading file from storage %s [%s]",
					handle.Path,
					err.Error(),
				)
				_ = f.Close()
				_ = os.Remove(localPath)
				return err
			}
		}

		// Update the last download time of this file
		flock.SetDownloadTime()
		// update file state
		flock.LazyOpen = false

		log.Debug("FileCache::openFileInternal : Download of %s is complete", handle.Path)
		f.Close()

		// After downloading the file, update the modified times and mode of the file.
		if attr != nil && !attr.IsModeDefault() {
			fileMode = attr.Mode
		}
	}

	// If user has selected some non default mode in config then every local file shall be created with that mode only
	err = os.Chmod(localPath, fileMode)
	if err != nil {
		log.Err(
			"FileCache::openFileInternal : Failed to change mode of file %s [%s]",
			handle.Path,
			err.Error(),
		)
	}
	// TODO: When chown is supported should we update that?

	if attr != nil {
		// chtimes shall be the last api otherwise calling chmod/chown will update the last change time
		err = os.Chtimes(localPath, attr.Atime, attr.Mtime)
		if err != nil {
			log.Err(
				"FileCache::openFileInternal : Failed to change times of file %s [%s]",
				handle.Path,
				err.Error(),
			)
		}
	}

	fileCacheStatsCollector.UpdateStats(stats_manager.Increment, dlFiles, (int64)(1))

	// Open the file and grab a shared lock to prevent deletion by the cache policy.
	f, err = common.OpenFile(localPath, flags, fMode)
	if err != nil {
		log.Err(
			"FileCache::openFileInternal : error opening cached file %s [%s]",
			handle.Path,
			err.Error(),
		)
		return err
	}

	if flags&os.O_TRUNC != 0 {
		handle.Flags.Set(handlemap.HandleFlagDirty)
	}

	inf, err := f.Stat()
	if err == nil {
		handle.Size = inf.Size()
	}

	handle.UnixFD = uint64(f.Fd())
	if !fc.offloadIO {
		handle.Flags.Set(handlemap.HandleFlagCached)
	}

	log.Info("FileCache::openFileInternal : file=%s, fd=%d", handle.Path, f.Fd())
	handle.SetFileObject(f)

	//set boolean in isDownloadNeeded value to signal that the file has been downloaded
	handle.RemoveValue("openFileOptions")
	// update file state
	flock.LazyOpen = false

	return nil
}

// OpenFile: Makes the file available in the local cache for further file operations.
func (fc *FileCache) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace(
		"FileCache::OpenFile : name=%s, flags=%d, mode=%s",
		options.Name,
		options.Flags,
		options.Mode,
	)

	// get the file lock
	flock := fc.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	localPath := filepath.Join(fc.tmpPath, options.Name)
	downloadRequired, _, cloudAttr, err := fc.isDownloadRequired(localPath, options.Name, flock)

	// return err in case of authorization permission mismatch
	if err != nil && err == syscall.EACCES {
		return nil, err
	}

	// check if we are running out of space
	if downloadRequired && cloudAttr != nil {
		fileSize := int64(cloudAttr.Size)
		if fc.diskHighWaterMark != 0 {
			currSize, err := common.GetUsage(fc.tmpPath)
			if err != nil {
				log.Err(
					"FileCache::OpenFile : error getting current usage of cache [%s]",
					err.Error(),
				)
			} else {
				if (currSize + float64(fileSize)) > fc.diskHighWaterMark {
					log.Err("FileCache::OpenFile : cache size limit reached [%f] failed to open %s", fc.maxCacheSize, options.Name)
					return nil, syscall.ENOSPC
				}
			}
		}
	}

	// create handle and record openFileOptions for later
	handle := handlemap.NewHandle(options.Name)
	handle.SetValue("openFileOptions", openFileOptions{flags: options.Flags, fMode: options.Mode})
	if options.Flags&os.O_APPEND != 0 {
		handle.Flags.Set(handlemap.HandleOpenedAppend)
	}

	// Increment the handle count
	flock.Inc()

	// will opening the file require downloading it?
	var openErr error
	if !downloadRequired {
		// use the local file to complete the open operation now
		// flock is already locked, as required by openFileInternal
		openErr = fc.openFileInternal(handle, flock)
	} else {
		// use a lazy open algorithm to avoid downloading unnecessarily (do nothing for now)
		// update file state
		flock.LazyOpen = true
	}

	return handle, openErr
}

// flock must already be locked before calling this function
func (fc *FileCache) isDownloadRequired(
	localPath string,
	objectPath string,
	flock *common.LockMapItem,
) (bool, bool, *internal.ObjAttr, error) {
	cached := false
	downloadRequired := false
	lmt := time.Time{}

	// check if the file exists locally
	finfo, statErr := os.Stat(localPath)
	if statErr == nil {
		// The file does not need to be downloaded as long as it is in the cache policy
		fileInPolicyCache := fc.policy.IsCached(localPath)
		if fileInPolicyCache {
			cached = true
		} else {
			log.Warn("FileCache::isDownloadRequired : %s exists but is not present in local cache policy", localPath)
		}
		// gather stat details
		lmt = finfo.ModTime()
	} else if os.IsNotExist(statErr) {
		// The file does not exist in the local cache so it needs to be downloaded
		log.Debug("FileCache::isDownloadRequired : %s not present in local cache", localPath)
	} else {
		// Catch all, the file needs to be downloaded
		log.Debug("FileCache::isDownloadRequired : error calling stat %s [%s]", localPath, statErr.Error())
	}

	// check if the file is due for a refresh from cloud storage
	refreshTimerExpired := fc.refreshSec != 0 &&
		time.Since(flock.DownloadTime()).Seconds() > float64(fc.refreshSec)

	// get cloud attributes
	cloudAttr, err := fc.NextComponent().GetAttr(internal.GetAttrOptions{Name: objectPath})
	if err != nil && !os.IsNotExist(err) {
		log.Err(
			"FileCache::isDownloadRequired : Failed to get attr of %s [%s]",
			objectPath,
			err.Error(),
		)
	}

	if !cached && cloudAttr != nil {
		downloadRequired = true
	}

	if cached && refreshTimerExpired && cloudAttr != nil {
		// File is not expired, but the user has configured a refresh timer, which has expired.
		// Does the cloud have a newer copy?
		cloudHasLatestData := cloudAttr.Mtime.After(lmt) || finfo.Size() != cloudAttr.Size
		// Is the local file open?
		fileIsOpen := flock.Count() > 0 && !flock.LazyOpen
		if cloudHasLatestData && !fileIsOpen {
			log.Info(
				"FileCache::isDownloadRequired : File is modified in container, so forcing redownload %s [A-%v : L-%v] [A-%v : L-%v]",
				objectPath,
				cloudAttr.Mtime,
				lmt,
				cloudAttr.Size,
				finfo.Size(),
			)
			downloadRequired = true
		} else {
			// log why we decided not to refresh
			if !cloudHasLatestData {
				log.Info("FileCache::isDownloadRequired : File in container is not latest, skip redownload %s [A-%v : L-%v]", objectPath, cloudAttr.Mtime, lmt)
			} else if fileIsOpen {
				log.Info("FileCache::isDownloadRequired : Need to re-download %s, but skipping as handle is already open", objectPath)
			}
			// As we have decided to continue using old file, we reset the timer to check again after refresh time interval
			flock.SetDownloadTime()
		}
	}

	return downloadRequired, cached, cloudAttr, err
}

// CloseFile: Flush the file and invalidate it from the cache.
func (fc *FileCache) CloseFile(options internal.CloseFileOptions) error {
	// Lock the file so that while close is in progress no one can open the file again
	flock := fc.fileLocks.Get(options.Handle.Path)
	flock.Lock()

	// Async close is called so schedule the upload and return here
	fc.fileCloseOpt.Add(1)

	if !fc.lazyWrite {
		// Sync close is called so wait till the upload completes
		return fc.closeFileInternal(options, flock)
	}

	go fc.closeFileInternal(options, flock) //nolint
	return nil
}

// flock must already be locked before calling this function
func (fc *FileCache) closeFileInternal(
	options internal.CloseFileOptions,
	flock *common.LockMapItem,
) error {
	log.Trace(
		"FileCache::closeFileInternal : name=%s, handle=%d",
		options.Handle.Path,
		options.Handle.ID,
	)

	// Lock is acquired by CloseFile, at end of this method we need to unlock
	// If its async call file shall be locked till the upload completes.
	defer flock.Unlock()
	defer fc.fileCloseOpt.Done()

	// if file has not been interactively read or written to by end user, then there is no cached file to close.
	_, noCachedHandle := options.Handle.GetValue("openFileOptions")

	if !noCachedHandle {
		// flock is already locked, as required by flushFileInternal
		err := fc.flushFileInternal(
			internal.FlushFileOptions{
				Handle:          options.Handle,
				CloseInProgress: true,
			},
		) //nolint
		if err != nil {
			log.Err("FileCache::closeFileInternal : failed to flush file %s", options.Handle.Path)
			return err
		}

		f := options.Handle.GetFileObject()
		if f == nil {
			log.Err(
				"FileCache::closeFileInternal : error [missing fd in handle object] %s",
				options.Handle.Path,
			)
			return syscall.EBADF
		}

		err = f.Close()
		if err != nil {
			log.Err(
				"FileCache::closeFileInternal : error closing file %s(%d) [%s]",
				options.Handle.Path,
				int(f.Fd()),
				err.Error(),
			)
			return err
		}
	}

	flock.Dec()

	// if this is the last lazy handle, clear the lazy flag
	if noCachedHandle && flock.Count() == 0 {
		flock.LazyOpen = false
	}

	// If it is an fsync op then purge the file
	if options.Handle.Fsynced() {
		log.Trace("FileCache::closeFileInternal : fsync/sync op, purging %s", options.Handle.Path)
		localPath := filepath.Join(fc.tmpPath, options.Handle.Path)
		fc.policy.CachePurge(localPath)
		return nil
	}

	return nil
}

// ReadInBuffer: Read the local file into a buffer
func (fc *FileCache) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	//defer exectime.StatTimeCurrentBlock("FileCache::ReadInBuffer")()
	// The file should already be in the cache since CreateFile/OpenFile was called before and a shared lock was acquired.
	// log.Debug("FileCache::ReadInBuffer : Reading %v bytes from %s", len(options.Data), options.Handle.Path)

	if !openCompleted(options.Handle) {
		flock := fc.fileLocks.Get(options.Handle.Path)
		// openFileInternal requires flock be locked before it's called
		flock.Lock()
		err := fc.openFileInternal(options.Handle, flock)
		flock.Unlock()
		if err != nil {
			return 0, fmt.Errorf("error downloading file %s [%s]", options.Handle.Path, err)
		}
	}

	f := options.Handle.GetFileObject()
	if f == nil {
		log.Err(
			"FileCache::ReadInBuffer : error [couldn't find fd in handle] %s",
			options.Handle.Path,
		)
		return 0, syscall.EBADF
	}

	// Read and write operations are very frequent so updating cache policy for every read is a costly operation
	// Update cache policy every 1K operations (includes both read and write) instead
	options.Handle.Lock()
	options.Handle.OptCnt++
	options.Handle.Unlock()
	if (options.Handle.OptCnt % defaultCacheUpdateCount) == 0 {
		localPath := filepath.Join(fc.tmpPath, options.Handle.Path)
		fc.policy.CacheValid(localPath)
	}

	// Removing Pread as it is not supported on Windows
	// return syscall.Pread(options.Handle.FD(), options.Data, options.Offset)
	n, err := f.ReadAt(options.Data, options.Offset)
	// ReadAt gives an error if it reads fewer bytes than the byte array. We discard that error.
	if n < len(options.Data) && err == io.EOF {
		return n, nil
	}
	return n, err
}

// WriteFile: Write to the local file
func (fc *FileCache) WriteFile(options internal.WriteFileOptions) (int, error) {
	//defer exectime.StatTimeCurrentBlock("FileCache::WriteFile")()
	// The file should already be in the cache since CreateFile/OpenFile was called before and a shared lock was acquired.
	//log.Debug("FileCache::WriteFile : Writing %v bytes from %s", len(options.Data), options.Handle.Path)

	if !openCompleted(options.Handle) {
		flock := fc.fileLocks.Get(options.Handle.Path)
		// openFileInternal requires flock be locked before it's called
		flock.Lock()
		err := fc.openFileInternal(options.Handle, flock)
		flock.Unlock()
		if err != nil {
			return 0, fmt.Errorf("error downloading file for %s [%s]", options.Handle.Path, err)
		}
	}

	var err error

	f := options.Handle.GetFileObject()
	if f == nil {
		log.Err("FileCache::WriteFile : error [couldn't find fd in handle] %s", options.Handle.Path)
		return 0, syscall.EBADF
	}

	if fc.diskHighWaterMark != 0 {
		currSize, err := common.GetUsage(fc.tmpPath)
		if err != nil {
			log.Err("FileCache::WriteFile : error getting current usage of cache [%s]", err.Error())
		} else {
			if (currSize + float64(len(options.Data))) > fc.diskHighWaterMark {
				log.Err("FileCache::WriteFile : cache size limit reached [%f] failed to open %s", fc.maxCacheSize, options.Handle.Path)
				return 0, syscall.ENOSPC
			}
		}
	}

	// Read and write operations are very frequent so updating cache policy for every read is a costly operation
	// Update cache policy every 1K operations (includes both read and write) instead
	options.Handle.Lock()
	options.Handle.OptCnt++
	options.Handle.Unlock()
	if (options.Handle.OptCnt % defaultCacheUpdateCount) == 0 {
		localPath := filepath.Join(fc.tmpPath, options.Handle.Path)
		fc.policy.CacheValid(localPath)
	}

	// Removing Pwrite as it is not supported on Windows
	// bytesWritten, err := syscall.Pwrite(options.Handle.FD(), options.Data, options.Offset)

	var bytesWritten int
	if options.Handle.Flags.IsSet(handlemap.HandleOpenedAppend) {
		bytesWritten, err = f.Write(options.Data)
	} else {
		bytesWritten, err = f.WriteAt(options.Data, options.Offset)
	}

	if err == nil {
		// Mark the handle dirty so the file is written back to storage on FlushFile.
		options.Handle.Flags.Set(handlemap.HandleFlagDirty)
	} else {
		log.Err("FileCache::WriteFile : failed to write %s [%s]", options.Handle.Path, err.Error())
	}

	return bytesWritten, err
}

func (fc *FileCache) SyncFile(options internal.SyncFileOptions) error {
	log.Trace("FileCache::SyncFile : handle=%d, path=%s", options.Handle.ID, options.Handle.Path)
	if fc.syncToFlush {
		err := fc.FlushFile(
			internal.FlushFileOptions{Handle: options.Handle, CloseInProgress: true},
		) //nolint
		if err != nil {
			log.Err("FileCache::SyncFile : failed to flush file %s", options.Handle.Path)
			return err
		}
	} else if fc.syncToDelete {
		err := fc.NextComponent().SyncFile(options)
		if err != nil {
			log.Err("FileCache::SyncFile : %s failed", options.Handle.Path)
			return err
		}

		options.Handle.Flags.Set(handlemap.HandleFlagFSynced)
	}

	return nil
}

// in SyncDir we're not going to clear the file cache for now
// on regular linux its fs responsibility
// func (fc *FileCache) SyncDir(options internal.SyncDirOptions) error {
// 	log.Trace("FileCache::SyncDir : %s", options.Name)

// 	err := fc.NextComponent().SyncDir(options)
// 	if err != nil {
// 		log.Err("FileCache::SyncDir : %s failed", options.Name)
// 		return err
// 	}
// 	// TODO: we can decide here if we want to flush all the files in the directory first or not. Currently I'm just invalidating files
// 	// within the dir
// 	go fc.invalidateDirectory(options.Name)
// 	return nil
// }

// FlushFile: Flush the local file to storage
func (fc *FileCache) FlushFile(options internal.FlushFileOptions) error {
	var flock *common.LockMapItem

	// if flush will upload the file, then acquire the file lock
	if options.Handle.Dirty() && (!fc.lazyWrite || options.CloseInProgress) {
		flock = fc.fileLocks.Get(options.Handle.Path)
		flock.Lock()
		defer flock.Unlock()
	}

	// flock is locked, as required by flushFileInternal
	return fc.flushFileInternal(options)
}

// file must be locked before calling this function
func (fc *FileCache) flushFileInternal(options internal.FlushFileOptions) error {
	//defer exectime.StatTimeCurrentBlock("FileCache::FlushFile")()
	log.Trace("FileCache::FlushFile : handle=%d, path=%s", options.Handle.ID, options.Handle.Path)

	// The file should already be in the cache since CreateFile/OpenFile was called before and a shared lock was acquired.
	localPath := filepath.Join(fc.tmpPath, options.Handle.Path)
	fc.policy.CacheValid(localPath)
	// if our handle is dirty then that means we wrote to the file

	if options.Handle.Dirty() {
		if fc.lazyWrite && !options.CloseInProgress {
			// As lazy-write is enable, upload will be scheduled when file is closed.
			log.Info(
				"FileCache::FlushFile : %s will be flushed when handle %d is closed",
				options.Handle.Path,
				options.Handle.ID,
			)
			return nil
		}

		f := options.Handle.GetFileObject()
		if f == nil {
			log.Err(
				"FileCache::FlushFile : error [couldn't find fd in handle] %s",
				options.Handle.Path,
			)
			return syscall.EBADF
		}

		// Flush all data to disk that has been buffered by the kernel.
		// We cannot close the incoming handle since the user called flush, note close and flush can be called on the same handle multiple times.
		// To ensure the data is flushed to disk before writing to storage, we duplicate the handle and close that handle.
		// f.fsync() is another option but dup+close does it quickly compared to sync
		// dupFd, err := syscall.Dup(int(f.Fd()))
		// if err != nil {
		// 	log.Err("FileCache::FlushFile : error [couldn't duplicate the fd] %s", options.Handle.Path)
		// 	return syscall.EIO
		// }
		// err = syscall.Close(dupFd)
		// if err != nil {
		// 	log.Err("FileCache::FlushFile : error [unable to close duplicate fd] %s", options.Handle.Path)
		// 	return syscall.EIO
		// }
		// Replace above with Sync since Dup is not supported on Windows
		err := f.Sync()
		if err != nil {
			log.Err("FileCache::FlushFile : error [unable to sync file] %s", options.Handle.Path)
			return syscall.EIO
		}

		// Write to storage
		// Create a new handle for the SDK to use to upload (read local file)
		// The local handle can still be used for read and write.
		var orgMode fs.FileMode
		modeChanged := false
		notInCloud := fc.notInCloud(
			options.Handle.Path,
		)
		// Figure out if we should upload immediately or append to pending OPS
		if options.ImmediateUpload || !notInCloud || fc.alwaysOn {
			uploadHandle, err := common.Open(localPath)
			if err != nil {
				if os.IsPermission(err) {
					info, _ := os.Stat(localPath)
					orgMode = info.Mode()
					newMode := orgMode | 0444
					err = os.Chmod(localPath, newMode)
					if err == nil {
						modeChanged = true
						uploadHandle, err = common.Open(localPath)
						log.Info(
							"FileCache::FlushFile : read mode added to file %s",
							options.Handle.Path,
						)
					}
				}

				if err != nil {
					log.Err(
						"FileCache::FlushFile : error [unable to open upload handle] %s [%s]",
						options.Handle.Path,
						err.Error(),
					)
					return err
				}
			}
			err = fc.NextComponent().CopyFromFile(
				internal.CopyFromFileOptions{
					Name: options.Handle.Path,
					File: uploadHandle,
				})

			uploadHandle.Close()
			if err == nil {
				// Clear dirty flag since file was successfully uploaded
				options.Handle.Flags.Clear(handlemap.HandleFlagDirty)
			}

			if err != nil {
				log.Err(
					"FileCache::FlushFile : %s upload failed [%s]",
					options.Handle.Path,
					err.Error(),
				)
				return err
			}

			if modeChanged {
				err1 := os.Chmod(localPath, orgMode)
				if err1 != nil {
					log.Err(
						"FileCache::FlushFile : Failed to remove read mode from file %s [%s]",
						options.Handle.Path,
						err1.Error(),
					)
				}
			}
		} else {
			//push to scheduleOps as default since we don't want to upload to the cloud
			log.Info(
				"FileCache::FlushFile : %s upload deferred (Scheduled for upload)",
				options.Handle.Path,
			)
			_, statErr := os.Stat(localPath)
			if statErr == nil {
				fc.markFileForUpload(options.Handle.Path)
				flock := fc.fileLocks.Get(options.Handle.Path)
				flock.SyncPending = true
			}
			options.Handle.Flags.Clear(handlemap.HandleFlagDirty)

		}

		// If chmod was done on the file before it was uploaded to container then setting up mode would have been missed
		// Such file names are added to this map and here post upload we try to set the mode correctly
		// Delete the entry from map so that any further flush do not try to update the mode again
		_, found := fc.missedChmodList.LoadAndDelete(options.Handle.Path)
		if found {
			// If file is found in map it means last chmod was missed on this

			// When chmod on container was missed, local file was updated with correct mode
			// Here take the mode from local cache and update the container accordingly
			localPath := filepath.Join(fc.tmpPath, options.Handle.Path)
			info, err := os.Stat(localPath)
			if err == nil {
				err = fc.chmodInternal(
					internal.ChmodOptions{Name: options.Handle.Path, Mode: info.Mode()},
				)
				if err != nil {
					// chmod was missed earlier for this file and doing it now also
					// resulted in error so ignore this one and proceed for flush handling
					log.Err(
						"FileCache::FlushFile : %s chmod failed [%s]",
						options.Handle.Path,
						err.Error(),
					)
				}
			}
		}
	}
	return nil
}

// GetAttr: Consolidate attributes from storage and local cache
func (fc *FileCache) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	// Don't log these by default, as it noticeably affects performance
	// log.Trace("FileCache::GetAttr : %s", options.Name)

	// For get attr, there are three different path situations we have to potentially handle.
	// 1. Path in cloud storage but not in local cache
	// 2. Path not in cloud storage but in local cache (this could happen if we recently created the file [and are currently writing to it]) (also supports immutable containers)
	// 3. Path in cloud storage and in local cache (this could result in dirty properties on the service if we recently wrote to the file)

	// If the file is being downloaded or deleted, the size and mod time will be incorrect
	// wait for download or deletion to complete before getting local file info
	flock := fc.fileLocks.Get(options.Name)
	// TODO: should we add RLock and RUnlock to the lock map for GetAttr?
	flock.Lock()
	defer flock.Unlock()

	// To cover case 1, get attributes from storage
	var exists bool
	attrs, err := fc.NextComponent().GetAttr(options)
	if err != nil {
		if err == syscall.ENOENT || os.IsNotExist(err) {
			log.Debug("FileCache::GetAttr : %s does not exist in cloud storage", options.Name)
			exists = false
		} else {
			log.Err("FileCache::GetAttr : Failed to get attr of %s [%s]", options.Name, err.Error())
			return nil, err
		}
	} else {
		exists = true
	}

	// To cover cases 2 and 3, grab the attributes from the local cache
	localPath := filepath.Join(fc.tmpPath, options.Name)
	info, err := os.Stat(localPath)
	// All directory operations are guaranteed to be synced with storage so they cannot be in a case 2 or 3 state.
	if err == nil && !info.IsDir() {
		if exists { // Case 3 (file in cloud storage and in local cache) so update the relevant attributes
			log.Debug("FileCache::GetAttr : updating %s from local cache", options.Name)
			// attrs is a pointer returned by NextComponent
			// modifying attrs could corrupt cached directory listings
			// to update properties, we need to make a deep copy first
			newAttr := *attrs
			newAttr.Mtime = info.ModTime()
			newAttr.Size = info.Size()
			attrs = &newAttr
		} else { // Case 2 (file only in local cache) so create a new attributes and add them to the storage attributes
			log.Debug("FileCache::GetAttr : serving %s attr from local cache", options.Name)
			exists = true
			attrs = newObjAttr(options.Name, info)
		}
	}

	if !exists {
		return nil, syscall.ENOENT
	}

	return attrs, nil
}

// RenameFile: Invalidate the file in local cache.
func (fc *FileCache) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("FileCache::RenameFile : src=%s, dst=%s", options.Src, options.Dst)

	// acquire file locks
	sflock := fc.fileLocks.Get(options.Src)
	dflock := fc.fileLocks.Get(options.Dst)
	// always lock files in lexical order to prevent deadlock
	if options.Src < options.Dst {
		sflock.Lock()
		dflock.Lock()
	} else {
		dflock.Lock()
		sflock.Lock()
	}
	defer sflock.Unlock()
	defer dflock.Unlock()

	err := fc.NextComponent().RenameFile(options)
	localOnly := os.IsNotExist(err)
	err = fc.validateStorageError(options.Src, err, "RenameFile", true)
	if err != nil {
		log.Err("FileCache::RenameFile : %s failed to rename file [%s]", options.Src, err.Error())
		return err
	}

	return fc.renameLocalFile(options.Src, options.Dst, sflock, dflock, localOnly)
}

// source and destination files should already be locked before calling this function
func (fc *FileCache) renameLocalFile(
	srcName, dstName string,
	sflock, dflock *common.LockMapItem,
	localOnly bool,
) error {
	localSrcPath := filepath.Join(fc.tmpPath, srcName)
	localDstPath := filepath.Join(fc.tmpPath, dstName)

	err := os.Rename(localSrcPath, localDstPath)
	switch {
	case err == nil:
		log.Debug(
			"FileCache::renameLocalFile : %s -> %s Successfully renamed local file",
			localSrcPath,
			localDstPath,
		)
		fc.policy.CacheValid(localDstPath)

		// Transfer entry from scheduleOps if it exists
		if _, found := fc.scheduleOps.Load(srcName); found {
			fc.scheduleOps.Store(dstName, struct{}{})
			fc.scheduleOps.Delete(srcName)

			// Ensure SyncPending flag is set on destination
			dflock.SyncPending = true
		}
	case os.IsNotExist(err):
		if localOnly {
			// neither cloud nor file cache has this file, so return ENOENT
			log.Err("FileCache::renameLocalFile : %s source file not found", srcName)
			return syscall.ENOENT
		} else {
			// Case 1
			log.Info("FileCache::renameLocalFile : %s source file not cached", localSrcPath)
		}
	default:
		// unexpected error from os.Rename
		log.Err(
			"FileCache::renameLocalFile : os.Rename(%s -> %s) failed. Here's why: %v",
			localSrcPath,
			localDstPath,
			err,
		)
		// check if the file is open
		if sflock.Count() > 0 {
			log.Warn(
				"FileCache::renameLocalFile : open local file (%s) will be uploaded as %s on close.",
				localSrcPath,
				dstName,
			)
		}
	}

	// delete the source from our cache policy
	// this will also delete the source file from local storage (if rename failed)
	fc.policy.CachePurge(localSrcPath)

	// rename open handles
	fc.renameOpenHandles(srcName, dstName, sflock, dflock)

	return nil
}

// files should already be locked before calling this function
func (fc *FileCache) renameOpenHandles(
	srcName, dstName string,
	sflock, dflock *common.LockMapItem,
) {
	// update open handles
	if sflock.Count() > 0 {
		// update any open handles to the file with its new name
		handlemap.GetHandles().Range(func(key, value any) bool {
			handle := value.(*handlemap.Handle)
			if handle.Path == srcName {
				handle.Path = dstName
			}
			return true
		})
		// copy the number of open handles to the new name
		for sflock.Count() > 0 {
			sflock.Dec()
			dflock.Inc()
		}
	}
}

// TruncateFile: Update the file with its new size.
func (fc *FileCache) TruncateFile(options internal.TruncateFileOptions) error {
	log.Trace("FileCache::TruncateFile : name=%s, size=%d", options.Name, options.Size)

	if fc.diskHighWaterMark != 0 {
		currSize, err := common.GetUsage(fc.tmpPath)
		if err != nil {
			log.Err(
				"FileCache::TruncateFile : error getting current usage of cache [%s]",
				err.Error(),
			)
		} else {
			if (currSize + float64(options.Size)) > fc.diskHighWaterMark {
				log.Err("FileCache::TruncateFile : cache size limit reached [%f] failed to open %s", fc.maxCacheSize, options.Name)
				return syscall.ENOSPC
			}
		}
	}

	flock := fc.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	err := fc.NextComponent().TruncateFile(options)
	err = fc.validateStorageError(options.Name, err, "TruncateFile", true)
	if err != nil {
		log.Err("FileCache::TruncateFile : %s failed to truncate [%s]", options.Name, err.Error())
		return err
	}

	// Update the size of the file in the local cache
	localPath := filepath.Join(fc.tmpPath, options.Name)
	info, err := os.Stat(localPath)
	if err == nil || os.IsExist(err) {
		fc.policy.CacheValid(localPath)

		if info.Size() != options.Size {
			err = os.Truncate(localPath, options.Size)
			if err != nil {
				log.Err(
					"FileCache::TruncateFile : error truncating cached file %s [%s]",
					localPath,
					err.Error(),
				)
				return err
			}
		}
	}

	return nil
}

// Chmod : Update the file with its new permissions
func (fc *FileCache) Chmod(options internal.ChmodOptions) error {
	log.Trace("FileCache::Chmod : Change mode of path %s", options.Name)

	flock := fc.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	return fc.chmodInternal(options)
}

// file must be locked before calling this function
func (fc *FileCache) chmodInternal(options internal.ChmodOptions) error {
	log.Trace("FileCache::Chmod : Change mode of path %s", options.Name)

	// Update the file in cloud storage
	err := fc.NextComponent().Chmod(options)
	err = fc.validateStorageError(options.Name, err, "Chmod", false)
	if err != nil {
		if err != syscall.EIO {
			log.Err("FileCache::Chmod : %s failed to change mode [%s]", options.Name, err.Error())
			return err
		} else {
			fc.missedChmodList.LoadOrStore(options.Name, true)
		}
	}

	// Update the mode of the file in the local cache
	localPath := filepath.Join(fc.tmpPath, options.Name)
	info, err := os.Stat(localPath)
	if err == nil {
		fc.policy.CacheValid(localPath)

		if info.Mode() != options.Mode {
			err = os.Chmod(localPath, options.Mode)
			if err != nil {
				log.Err(
					"FileCache::Chmod : error changing mode on the cached path %s [%s]",
					localPath,
					err.Error(),
				)
				return err
			}
		}
	}

	return nil
}

// Chown : Update the file with its new owner and group
func (fc *FileCache) Chown(options internal.ChownOptions) error {
	log.Trace("FileCache::Chown : Change owner of path %s", options.Name)

	flock := fc.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	// Update the file in cloud storage
	err := fc.NextComponent().Chown(options)
	err = fc.validateStorageError(options.Name, err, "Chown", false)
	if err != nil {
		log.Err("FileCache::Chown : %s failed to change owner [%s]", options.Name, err.Error())
		return err
	}

	// Update the owner and group of the file in the local cache
	localPath := filepath.Join(fc.tmpPath, options.Name)
	_, err = os.Stat(localPath)
	if err == nil {
		fc.policy.CacheValid(localPath)

		if runtime.GOOS != "windows" {
			err = os.Chown(localPath, options.Owner, options.Group)
			if err != nil {
				log.Err(
					"FileCache::Chown : error changing owner on the cached path %s [%s]",
					localPath,
					err.Error(),
				)
				return err
			}
		}
	}

	return nil
}

func (fc *FileCache) FileUsed(name string) error {
	// Update the owner and group of the file in the local cache
	localPath := filepath.Join(fc.tmpPath, name)
	fc.policy.CacheValid(localPath)
	return nil
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewFileCacheComponent() internal.Component {
	comp := &FileCache{
		fileLocks:          common.NewLockMap(),
		activeWindowsMutex: &sync.Mutex{},
	}
	comp.SetName(compName)
	config.AddConfigChangeEventListener(comp)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewFileCacheComponent)
}
