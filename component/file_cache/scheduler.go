package file_cache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/handlemap"
	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v2"
)

type UploadWindow struct {
	CronExpr string `yaml:"cron"`
	Duration string `yaml:"duration"`
	Repeat   bool   `yaml:"repeat"`
}

type Config struct {
	Schedule map[string]UploadWindow `yaml:"schedule"`
}

type WeeklySchedule map[string]UploadWindow

func LoadConfig(path string) (WeeklySchedule, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return cfg.Schedule, nil
}

func (fc *FileCache) SetupScheduler(configPath string) error {
	schedule, err := LoadConfig(configPath)
	if err != nil {
		log.Err(
			"FileCache::SetupScheduler : Failed to load schedule configuration [%s]",
			err.Error(),
		)
		return fmt.Errorf("failed to load scheduler config: %w", err)
	}
	if len(schedule) == 0 {
		log.Info("FileCache::SetupScheduler : No schedule configuration found")
		return nil
	}

	cronScheduler := cron.New(cron.WithSeconds())

	startFunc := func() {
		log.Info("FileCache::SetupScheduler : Starting scheduled upload window")
		fc.servicePendingOps()
	}

	endFunc := func() {
		log.Info("FileCache::SetupScheduler : Upload window ended")
	}

	fc.scheduleUploads(cronScheduler, schedule, startFunc, endFunc)

	// Start the scheduler
	cronScheduler.Start()

	log.Info("FileCache::SetupScheduler : Scheduler started successfully")
	return nil
}

func (fc *FileCache) scheduleUploads(
	c *cron.Cron,
	sched WeeklySchedule,
	startFunc func(),
	endFunc func(),
) {
	for day, config := range sched {
		currentDay := day
		uploadConfig := config

		durationParsed, err := time.ParseDuration(config.Duration)
		if err != nil {
			log.Info("[%s] Invalid duration '%s': %v\n", day, config.Duration, err)
			continue
		}

		var entryID cron.EntryID
		entryID, _ = c.AddFunc(config.CronExpr, func() {
			// Call the startFunc callback to notify upload window is starting
			startFunc() // Also servicing all current pending uploads first
			log.Info("[%s] Starting upload at %s\n", day, time.Now().Format(time.Kitchen))
			window, cancel := context.WithTimeout(context.Background(), durationParsed)
			ticker := time.NewTicker(1 * time.Minute)
			defer ticker.Stop()
			go func() {
				defer cancel()
				for {
					select {
					case <-window.Done():
						// Call the endFunc callback to notify upload window is ending
						endFunc()
						fmt.Printf(
							"[%s] Upload window ended at %s\n",
							day,
							time.Now().Format(time.Kitchen),
						)
						return
					case <-ticker.C:
						log.Debug(
							"[%s] Checking for pending uploads at %s\n",
							day,
							time.Now().Format(time.Kitchen),
						)
						fc.servicePendingOps()
					}
				}
			}()

			if !uploadConfig.Repeat {
				fmt.Printf("[%s] One-time schedule, removing\n", currentDay)
				c.Remove(entryID)
			}
		})
	}
}

func (fc *FileCache) servicePendingOps() {
	log.Info("FileCache::servicePendingOps : Servicing pending uploads")

	// check if we're connected (keep this safety check)
	if !fc.cloudConnected() {
		log.Info(
			"FileCache::servicePendingOps : Cloud storage not connected, skipping upload cycle",
		)
		return
	}

	// Process pending operations
	fc.offlineOps.Range(func(key, value interface{}) bool {
		select {
		case <-fc.stopAsyncUpload:
			log.Info("FileCache::servicePendingOps : Upload processing interrupted")
			return false // Stop the iteration
		default:
			path := key.(string)
			err := fc.uploadPendingFile(path)
			if err != nil {
				log.Err(
					"FileCache::servicePendingOps : %s upload failed: %v",
					path,
					err,
				)
			}
		}
		return true // Continue the iteration
	})

	log.Info("FileCache::servicePendingOps : Completed upload cycle, processed %d files")
}

func (fc *FileCache) uploadPendingFile(name string) error {
	log.Trace("FileCache::uploadPendingFile : %s", name)

	// lock the file
	flock := fc.fileLocks.Get(name)
	flock.Lock()
	defer flock.Unlock()

	// look up file (or folder!)
	localPath := filepath.Join(fc.tmpPath, name)
	info, err := os.Stat(localPath)
	if err != nil {
		log.Err("FileCache::uploadPendingFile : %s failed to stat file. Here's why: %v", name, err)
		return err
	}
	if info.IsDir() {
		// upload folder
		options := internal.CreateDirOptions{Name: name, Mode: info.Mode()}
		err = fc.NextComponent().CreateDir(options)
		if err != nil && !os.IsExist(err) {
			return err
		}
	} else {
		handle := handlemap.NewHandle(name)
		f, err := common.OpenFile(localPath, os.O_RDONLY, fc.defaultPermission)
		if err != nil {
			log.Err("FileCache::uploadPendingFile : %s failed to open file. Here's why: %v", name, err)
			return err
		}
		// write handle attributes
		inf, err := f.Stat()
		if err == nil {
			handle.Size = inf.Size()
		}
		handle.UnixFD = uint64(f.Fd())
		handle.SetFileObject(f)
		handle.Flags.Set(handlemap.HandleFlagDirty)

		err = fc.flushFileInternal(internal.FlushFileOptions{
			Handle:          handle,
			CloseInProgress: false,
			ImmediateUpload: true,
		})

		f.Close()

		if err != nil {
			log.Err("FileCache::uploadPendingFile : %s Upload failed. Cause: %v", name, err)
			return err
		}
	}
	// update state
	flock.SyncPending = false
	// Successfully uploaded, removing from offlineOps
	fmt.Println("File uploaded:", name)
	fc.offlineOps.Delete(name)

	return nil
}

// checks if we are offline by requesting state information from the cloud storage component
func (fc *FileCache) cloudConnected() bool {
	// TODO: create a new component API function to check this (SRGDEV-614), instead of using StatFs
	_, _, err := fc.NextComponent().StatFs()
	return !isOffline(err)
}

// this returns true when offline access is enabled, and it's safe to access this object offline
func (fc *FileCache) offlineOperationAllowed(name string) bool {
	return fc.offlineAccess && fc.notInCloud(name)
}

// returns true if we *know* that this entity does not exist in cloud storage
// otherwise returns false (including ambiguous cases)
func (fc *FileCache) notInCloud(name string) bool {
	notInCloud, _ := fc.checkCloud(name)
	return notInCloud
}

// notInCloud is true if we *know* that this entity does not exist in cloud storage
// and getAttrErr is the error returned from GetAttr
func (fc *FileCache) checkCloud(name string) (notInCloud bool, getAttrErr error) {
	_, getAttrErr = fc.NextComponent().GetAttr(internal.GetAttrOptions{Name: name})
	notInCloud = errors.Is(getAttrErr, os.ErrNotExist)
	return notInCloud, getAttrErr
}

// checks if the error returned from cloud storage means we're offline
func isOffline(err error) bool {
	return errors.Is(err, &common.CloudUnreachableError{})
}

// checks whether we have usable metadata, despite being offline
func offlineDataAvailable(err error) bool {
	return isOffline(err) && cachedData(err)
}

// checks whether we have usable metadata, despite being offline
func cachedData(err error) bool {
	return !errors.Is(err, &common.NoCachedDataError{}) || !isOffline(err)
}
