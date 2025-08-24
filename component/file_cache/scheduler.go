package file_cache

import (
	"context"
	"time"

	"github.com/Seagate/cloudfuse/common/log"
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
	fc.scheduleUploads(cronScheduler, fc.schedule)
	cronScheduler.Start()

	log.Info("FileCache::SetupScheduler : Scheduler started successfully")
	return nil
}

func isValidCronExpression(expr string) bool {
	parser := cron.NewParser(
		cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)
	_, err := parser.Parse(expr)
	return err == nil
}

func (fc *FileCache) scheduleUploads(c *cron.Cron, sched WeeklySchedule) {
	// define callbacks to activate and disable uploads
	startFunc := func() {
		log.Info("FileCache::SetupScheduler : Starting scheduled upload window")
		fc.closeWindowCh = make(chan struct{})
	}
	endFunc := func() {
		log.Info("FileCache::SetupScheduler : Upload window ended")
		close(fc.closeWindowCh)
	}
	// start up the schedules
	for _, config := range sched {
		windowName := config.Name
		duration, err := time.ParseDuration(config.Duration)
		if err != nil {
			log.Info("[%s] Invalid duration '%s': %v\n", windowName, config.Duration, err)
			continue
		}
		var initialWindowEndTime time.Time

		cronEntryId, err := c.AddFunc(config.CronExpr, func() {
			// Start a new window and track it
			fc.activeWindowsMutex.Lock()
			isFirstWindow := fc.activeWindows == 0
			fc.activeWindows++
			windowCount := fc.activeWindows
			fc.activeWindowsMutex.Unlock()

			// activate uploads
			if isFirstWindow {
				// open the window
				startFunc()
			}

			log.Info("schedule [%s] starting (active windows=%d)", windowName, windowCount)
			fc.serviceScheduledOps()

			// When should the window close?
			remainingDuration := duration
			currentTime := time.Now()
			if initialWindowEndTime.After(currentTime) {
				remainingDuration = initialWindowEndTime.Sub(currentTime)
			}
			// Create a context to end the window
			window, cancel := context.WithTimeout(context.Background(), remainingDuration)
			defer cancel()

			for {
				select {
				case <-fc.stopAsyncUpload:
					log.Info("Shutting down upload scheduler")
					return
				case <-window.Done():
					// Window has completed, update active window count
					fc.activeWindowsMutex.Lock()
					fc.activeWindows--
					isLastWindow := fc.activeWindows == 0
					windowCount := fc.activeWindows
					fc.activeWindowsMutex.Unlock()

					log.Info("[%s] Upload window ended at %s (remaining windows: %d)\n",
						windowName, time.Now().Format(time.Kitchen), windowCount)

					// Only close resources when the last window ends
					if isLastWindow {
						endFunc()
					}
					return
				case <-fc.uploadNotifyCh:
					log.Debug("[%s] File change detected, processing pending uploads at %s\n",
						windowName, time.Now().Format(time.Kitchen))
					fc.serviceScheduledOps()
				}
			}
		})
		if err != nil {
			log.Err("[%s] Failed to schedule cron job with expression '%s': %v\n",
				windowName, config.CronExpr, err)
			continue
		}

		// check if this schedule should already be active
		// did this schedule have a start time within the last duration?
		schedule := c.Entry(cronEntryId)
		currentTime := time.Now()
		currentWindowStartTime := schedule.Schedule.Next(currentTime.Add(-duration))
		if currentTime.After(currentWindowStartTime) {
			initialWindowEndTime = currentWindowStartTime.Add(duration)
			go schedule.Job.Run()
		}
	}
}

func (fc *FileCache) markFileForUpload(path string) {
	fc.scheduleOps.Store(path, struct{}{})
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

func (fc *FileCache) serviceScheduledOps() {
	log.Info("FileCache::serviceScheduledOps : Servicing scheduled uploads")

	// Process pending operations
	numFilesProcessed := 0
	fc.scheduleOps.Range(func(key, value interface{}) bool {
		numFilesProcessed++
		select {
		case <-fc.stopAsyncUpload:
			log.Info("FileCache::serviceScheduledOps : Upload processing interrupted")
			return false
		case <-fc.closeWindowCh:
			return false
		default:
			path := key.(string)
			err := fc.uploadPendingFile(path)
			if err != nil {
				log.Err(
					"FileCache::serviceScheduledOps : %s upload failed: %v",
					path,
					err,
				)
			}
		}
		return true
	})

	log.Info(
		"FileCache::serviceScheduledOps : Completed upload cycle, processed %d files",
		numFilesProcessed,
	)
}
