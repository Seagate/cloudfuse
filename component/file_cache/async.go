/*
	Licensed under the MIT License <http://opensource.org/licenses/MIT>.

	Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates

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
	SOFTWARE.
*/

package file_cache

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/netresearch/go-cron"
)

type UploadWindow struct {
	name        string `yaml:"name"`
	cronExpr    string `yaml:"cron"`
	duration    time.Duration
	cronEntryID int
}

type Config struct {
	Schedule WeeklySchedule `yaml:"schedule"`
}

type WeeklySchedule []UploadWindow

type pendingFlags struct {
	isDir      bool
	isDeletion bool
}

func (fc *FileCache) configureScheduler() error {
	// load from config
	var rawSchedule []map[string]interface{}
	err := config.UnmarshalKey(compName+".schedule", &rawSchedule)
	if err != nil {
		return err
	}
	// initialize the scheduler
	fc.cronScheduler = cron.New(cron.WithSeconds())
	// create parser for cron expressions
	parser := cron.MustNewParser(
		cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)
	// Convert raw schedule to WeeklySchedule
	fc.schedule = make(WeeklySchedule, 0, len(rawSchedule))
	for _, rawWindow := range rawSchedule {
		window := UploadWindow{}
		if name, ok := rawWindow["name"].(string); ok {
			window.name = name
		}
		if cronExpr, ok := rawWindow["cron"].(string); ok {
			window.cronExpr = cronExpr
		}
		if duration, ok := rawWindow["duration"].(string); ok {
			window.duration, err = time.ParseDuration(duration)
			if err != nil {
				log.Err(
					"FileCache::Configure : %s invalid window duration %s (%v)",
					window.name,
					duration,
					err,
				)
				return err
			}
		}
		// Determine if we're joining a window that's already active by
		// finding the most recent scheduled start via Prev().
		now := time.Now()
		var initialWindowEndTime time.Time
		var jobOpts []cron.JobOption
		schedule, _ := parser.Parse(window.cronExpr)
		if sp, ok := schedule.(cron.ScheduleWithPrev); ok {
			prevStart := sp.Prev(now)
			if !prevStart.IsZero() && prevStart.Add(window.duration).After(now) {
				// We're inside an active window that started at prevStart.
				initialWindowEndTime = prevStart.Add(window.duration)
				// Run immediately to join the in-progress window with shortened duration.
				jobOpts = append(jobOpts, cron.WithRunImmediately())
				log.Info(
					"FileCache::scheduleUploads : [%s] joining active window (started %s, ends %s)",
					window.name,
					prevStart.Format(time.Kitchen),
					initialWindowEndTime.Format(time.Kitchen),
				)
			}
		}
		// add cron callback
		entryId, err := fc.cronScheduler.AddFunc(window.cronExpr, func() {
			// Is this a transition from inactive?
			windowCount := fc.activeWindows.Add(1)
			if windowCount == 1 {
				// transition to active - open the window
				close(fc.startScheduledUploads)
				log.Info(
					"FileCache::SchedulerCronFunc : %s - enabled scheduled uploads",
					window.name,
				)
			}
			log.Info(
				"FileCache::SchedulerCronFunc : %s (%s) started (numActive=%d)",
				window.name,
				window.cronExpr,
				windowCount,
			)
			// When should the window close?
			remainingDuration := window.duration
			currentTime := time.Now()
			if initialWindowEndTime.After(currentTime) {
				remainingDuration = initialWindowEndTime.Sub(currentTime)
			}
			// Create a context to end the window
			ctx, cancel := context.WithTimeout(context.Background(), remainingDuration)
			defer cancel()
			for {
				select {
				case <-fc.componentStopping:
					log.Info("FileCache::SchedulerCronFunc : %s - stopping cron job", window.name)
					return
				case <-ctx.Done():
					// Window has completed, update active window count
					windowCount = fc.activeWindows.Add(-1)
					log.Info(
						"FileCache::SchedulerCronFunc : %s (%s) ended (numActive=%d)",
						window.name,
						window.duration,
						windowCount,
					)
					// Only close resources when the last window ends
					if windowCount == 0 {
						fc.startScheduledUploads = make(chan struct{})
						log.Info(
							"FileCache::SchedulerCronFunc : %s window ended - deferring uploads",
							window.name,
						)
					}
					return
				}
			}
		}, jobOpts...)
		if err != nil {
			log.Err(
				"FileCache::Configure : Schedule %s invalid cron expression (%v)",
				window.name,
				err,
			)
			return err
		}
		// save window to fc.schedule
		window.cronEntryID = int(entryId)
		fc.schedule = append(fc.schedule, window)
		log.Info(
			"FileCache::Configure : Added schedule %s ('%s', %s)",
			window.name,
			window.cronExpr,
			window.duration,
		)
	}

	return nil
}

func (fc *FileCache) startScheduler() {
	// check if any schedules should already be active
	for _, window := range fc.schedule {
		entry := fc.cronScheduler.Entry(cron.EntryID(window.cronEntryID))
		// check if this entry should already be active
		// did this entry have a start time within the last duration?
		now := time.Now()
		var initialWindowEndTime time.Time
		for t := entry.Schedule.Next(now.Add(-window.duration)); now.After(t); t = entry.Schedule.Next(t) {
			initialWindowEndTime = t.Add(window.duration)
		}
		if !initialWindowEndTime.IsZero() {
			go entry.Job.Run()
		}
	}
	fc.cronScheduler.Start()
}

// flock must be locked
func (fc *FileCache) addPendingOp(name string, value pendingFlags) {
	log.Trace("FileCache::addPendingOp : %s", name)
	fc.pendingOps.Store(name, value)
	select {
	case fc.pendingOpAdded <- struct{}{}:
	default: // do not block
	}
}

// persistent background thread function
func (fc *FileCache) servicePendingOps() {
	for {
		select {
		case <-fc.componentStopping:
			log.Crit("FileCache::servicePendingOps : Stopping")
			// TODO: Persist pending ops
			return
		case <-fc.startScheduledUploads:
			// check if we're connected
			// exponential backoff is implemented inside CloudConnected(),
			//  so we're safe to call it naively every second like this
			if !fc.NextComponent().CloudConnected() {
				// we are offline, wait for a while before checking again
				select {
				case <-time.After(time.Second):
				case <-fc.componentStopping:
				}
				break
			}
			numFilesProcessed := 0
			// Iterate over pending ops
			fc.pendingOps.Range(func(key, value interface{}) bool {
				numFilesProcessed++
				select {
				case <-fc.componentStopping:
					return false
				case <-fc.startScheduledUploads:
					path := key.(string)
					value := value.(pendingFlags)
					err := fc.updateObject(path, value)
					if isOffline(err) {
						return false // connection lost - abort iteration
					}
					if err != nil {
						log.Err("FileCache::servicePendingOps : %s upload failed: %v", path, err)
					}
				default:
					return false // upload window ended
				}
				return true // Continue the iteration
			})
			log.Info(
				"FileCache::servicePendingOps : Completed upload cycle, processed %d files",
				numFilesProcessed,
			)
			if numFilesProcessed == 0 {
				// we're online but there's nothing to do
				// wait for a task to be added
				select {
				case <-fc.pendingOpAdded:
				case <-fc.componentStopping:
				}
			}
		}
	}
}

// synchronize pending operation with cloud storage
func (fc *FileCache) updateObject(name string, flags pendingFlags) error {
	log.Trace("FileCache::updateObject : %s", name)

	// lock the file
	flock := fc.fileLocks.Get(name)
	flock.Lock()
	defer flock.Unlock()

	// don't double upload
	_, stillPending := fc.pendingOps.Load(name)
	if !stillPending {
		return nil
	}

	// look up file (or folder!)
	localPath := filepath.Join(fc.tmpPath, name)
	info, localErr := os.Stat(localPath)
	localMissing := os.IsNotExist(localErr)
	// in case of inconsistency, local state takes precedence (except to prevent incorrect deletions)
	if !flags.isDeletion && localErr != nil {
		log.Err("FileCache::updateObject : %s stat failed. Here's why: %v", name, localErr)
		fc.pendingOps.Delete(name)
		return localErr
	}
	if flags.isDeletion && !localMissing {
		log.Err("FileCache::updateObject : %s exists. Ignoring deletion flag!", name)
	}
	if !localMissing && flags.isDir != info.IsDir() {
		log.Err("FileCache::updateObject : %s has wrong dir flag (%t)!", name, flags.isDir)
	}

	// update cloud
	op := "deletion"
	objType := "directory"
	var cloudErr error
	if localMissing {
		if flags.isDeletion && fc.notInCloud(name) {
			log.Info("FileCache::updateObject : %s skipping cloud deletion (not in cloud)", name)
			fc.pendingOps.Delete(name)
			return nil
		}
		if flags.isDir {
			// delete folder
			options := internal.DeleteDirOptions{Name: name}
			cloudErr = fc.NextComponent().DeleteDir(options)
		} else {
			// delete file
			objType = "file"
			options := internal.DeleteFileOptions{Name: name}
			cloudErr = fc.NextComponent().DeleteFile(options)
		}
	} else {
		op = "creation/update"
		if info.IsDir() {
			// upload folder
			options := internal.CreateDirOptions{Name: name, Mode: info.Mode()}
			cloudErr = fc.NextComponent().CreateDir(options)
		} else {
			// upload file
			objType = "file"
			cloudErr = fc.uploadFile(name)
		}
	}
	// handle errors
	if cloudErr != nil {
		log.Err("FileCache::updateObject : %s %s %s failed [%v]", name, objType, op, cloudErr)
		return cloudErr
	}

	// update state
	log.Info("FileCache::updateObject : %s sync successful", name)
	fc.pendingOps.Delete(name)

	return nil
}

// returns true if we *know* that this entity does not exist in cloud storage
// otherwise returns false (including ambiguous cases)
func (fc *FileCache) notInCloud(name string) bool {
	cloudStateKnown, existsInCloud, _ := fc.checkCloud(name)
	return cloudStateKnown && !existsInCloud
}

// and getAttrErr is the error returned from GetAttr
func (fc *FileCache) checkCloud(
	name string,
) (cloudStateKnown bool, inCloud bool, getAttrErr error) {
	_, getAttrErr = fc.NextComponent().GetAttr(internal.GetAttrOptions{Name: name})
	return cachedData(getAttrErr), !errors.Is(getAttrErr, os.ErrNotExist), getAttrErr
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
	return !(isOffline(err) && errors.Is(err, &common.NoCachedDataError{}))
}
