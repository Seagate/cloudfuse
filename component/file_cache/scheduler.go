package file_cache

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/handlemap"
	"github.com/robfig/cron/v3"
)

type UploadWindow struct {
	Name     string `yaml:"name"`
	CronExpr string `yaml:"cron"`
	Duration string `yaml:"duration"`
}

type Config struct {
	Schedule WeeklySchedule `yaml:"schedule"`
}

type WeeklySchedule []UploadWindow

func (fc *FileCache) SetupScheduler() error {
	if len(fc.schedule) == 0 {
		log.Info(
			"FileCache::SetupScheduler : Empty schedule configuration, defaulting to always-on mode",
		)
		fc.alwaysOn = true
		return nil
	}

	// Setup the cron scheduler
	cronScheduler := cron.New(cron.WithSeconds())

	startFunc := func() {
		log.Info("FileCache::SetupScheduler : Starting scheduled upload window")
	}

	endFunc := func() {
		log.Info("FileCache::SetupScheduler : Upload window ended")
	}
	fc.scheduleUploads(cronScheduler, fc.schedule, startFunc, endFunc)

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
	for _, config := range sched {
		windowName := config.Name

		durationParsed, err := time.ParseDuration(config.Duration)
		if err != nil {
			log.Info("[%s] Invalid duration '%s': %v\n", windowName, config.Duration, err)
			continue
		}

		c.AddFunc(config.CronExpr, func() {
			startFunc()
			log.Info("[%s] Starting upload at %s\n", windowName, time.Now().Format(time.Kitchen))

			fc.servicePendingOps()

			// Create a context with timeout for the duration of the window
			window, cancel := context.WithTimeout(context.Background(), durationParsed)
			defer cancel()

			// Set up the notification channel for upload window
			fc.uploadNotifyCh = make(chan struct{}, 100)
			defer func() {
				close(fc.uploadNotifyCh)
				fc.uploadNotifyCh = nil
			}()

			for {
				select {
				case <-window.Done():
					endFunc()
					log.Info(
						"[%s] Upload window ended at %s\n",
						windowName,
						time.Now().Format(time.Kitchen),
					)
					return
				case <-fc.uploadNotifyCh:
					log.Debug(
						"[%s] File change detected, processing pending uploads at %s\n",
						windowName,
						time.Now().Format(time.Kitchen),
					)
					fc.servicePendingOps()
				}
			}
		})
	}
}

func (fc *FileCache) markFileForUpload(path string) {
	fc.scheduleOps.Store(path, struct{}{})
	if fc.uploadNotifyCh != nil {
		select {
		case fc.uploadNotifyCh <- struct{}{}:
			// Successfully notified
			log.Info(
				"FileCache::markFileForUpload : Notified upload window about new file: %s",
				path,
			)
		default:
			// Channel buffer is full, which means notifications are already pending
			// No need to block here as uploads will be processed soon
			log.Info(
				"FileCache::markFileForUpload : Upload window notification channel full, skipping notify for: %s",
				path,
			)
		}
	}
}

func (fc *FileCache) servicePendingOps() {
	log.Info("FileCache::servicePendingOps : Servicing pending uploads")

	// Process pending operations
	fc.scheduleOps.Range(func(key, value interface{}) bool {
		select {
		case <-fc.stopAsyncUpload:
			log.Info("FileCache::servicePendingOps : Upload processing interrupted")
			return false
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
		return true
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
	// Successfully uploaded, removing from scheduleOps
	log.Info("FileCache::uploadPendingFile : File uploaded: %s", name)
	fc.scheduleOps.Delete(name)

	return nil
}

func (fc *FileCache) notInCloud(name string) bool {
	notInCloud, _ := fc.checkCloud(name)
	return notInCloud
}

func (fc *FileCache) checkCloud(name string) (notInCloud bool, getAttrErr error) {
	_, getAttrErr = fc.NextComponent().GetAttr(internal.GetAttrOptions{Name: name})
	notInCloud = errors.Is(getAttrErr, os.ErrNotExist)
	return notInCloud, getAttrErr
}
