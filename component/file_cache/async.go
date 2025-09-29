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
	"github.com/Seagate/cloudfuse/internal/handlemap"
	"github.com/robfig/cron/v3"
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

func (fc *FileCache) configureScheduler() error {
	// load from config
	var rawSchedule []map[string]interface{}
	err := config.UnmarshalKey(compName+".schedule", &rawSchedule)
	if err != nil {
		return err
	}
	// initialize the scheduler
	fc.cronScheduler = cron.New(cron.WithSeconds())
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
		// load schedules into cron
		var initialWindowEndTime time.Time
		entryId, err := fc.cronScheduler.AddFunc(window.cronExpr, func() {
			// Is this a transition from inactive?
			windowCount := fc.activeWindows.Add(1)
			if windowCount == 1 {
				// transition to active - open the window
				fc.schedulerActiveCh = make(chan struct{})
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
						close(fc.schedulerActiveCh)
						log.Info(
							"FileCache::SchedulerCronFunc : %s - disabled scheduled uploads",
							window.name,
						)
					}
					return
				}
			}
		})
		if err != nil {
			log.Err(
				"FileCache::Configure : Schedule %s invalid cron expression (%v)",
				window.name,
				err,
			)
			return err
		}
		// calculate end time for windows that will start open
		now := time.Now()
		entry := fc.cronScheduler.Entry(entryId)
		for t := entry.Schedule.Next(now.Add(-window.duration)); now.After(t); t = entry.Schedule.Next(t) {
			initialWindowEndTime = t.Add(window.duration)
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

func (fc *FileCache) addPendingOp(name string, flock *common.LockMapItem) {
	log.Trace("FileCache::addPendingOp : %s", name)
	fc.pendingOps.Store(name, struct{}{})
	flock.SyncPending = true
	select {
	case fc.pendingOpAdded <- struct{}{}:
	default: // do not block
	}
}

func (fc *FileCache) servicePendingOps() {
	for {
		select {
		case <-fc.componentStopping:
			log.Crit("FileCache::servicePendingOps : Stopping")
			// TODO: Persist pending ops
			return
		case <-fc.schedulerActiveCh:
			// upload schedule is not active, wait before checking again
			select {
			case <-time.After(time.Second):
			case <-fc.componentStopping:
			}
		default:
			// check if we're connected
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
				case <-fc.schedulerActiveCh:
					return false // upload window ended
				default:
					path := key.(string)
					err := fc.uploadPendingFile(path)
					if isOffline(err) {
						return false // connection lost - abort iteration
					}
					if err != nil {
						log.Err("FileCache::servicePendingOps : %s upload failed: %v", path, err)
					}
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

func (fc *FileCache) uploadPendingFile(name string) error {
	log.Trace("FileCache::uploadPendingFile : %s", name)

	// lock the file
	flock := fc.fileLocks.Get(name)
	flock.Lock()
	defer flock.Unlock()

	// don't double upload
	if !flock.SyncPending {
		return nil
	}

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
		// this is a file
		// prepare a handle
		handle := handlemap.NewHandle(name)
		// open the cached file
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

		// upload the file
		err = fc.flushFileInternal(internal.FlushFileOptions{Handle: handle, AsyncUpload: true})
		f.Close()
		if err != nil {
			log.Err("FileCache::uploadPendingFile : %s Upload failed. Here's why: %v", name, err)
			return err
		}
	}
	// update state
	flock.SyncPending = false
	log.Info("FileCache::uploadPendingFile : File uploaded: %s", name)
	fc.pendingOps.Delete(name)

	return nil
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
