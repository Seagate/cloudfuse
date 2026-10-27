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
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/netresearch/go-cron"
)

// bounds on the backoff between upload cycles that ended in failure
const (
	minRetryDelay = time.Second
	maxRetryDelay = 15 * time.Second
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
	var rawSchedule []map[string]any
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
			fc.scheduleLock.Lock()
			windowCount := fc.activeWindows.Add(1)
			if windowCount == 1 {
				// transition to active - open the window
				close(fc.startScheduledUploads)
				log.Info(
					"FileCache::SchedulerCronFunc : %s - enabled scheduled uploads",
					window.name,
				)
			}
			fc.scheduleLock.Unlock()
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
					fc.scheduleLock.Lock()
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
					fc.scheduleLock.Unlock()
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

// uploadWindow returns a channel that is closed while scheduled uploads are allowed
func (fc *FileCache) uploadWindow() <-chan struct{} {
	fc.scheduleLock.RLock()
	defer fc.scheduleLock.RUnlock()
	return fc.startScheduledUploads
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
	// grows while upload cycles keep failing, resets as soon as one succeeds
	var retryDelay time.Duration
	for {
		select {
		case <-fc.componentStopping:
			log.Crit("FileCache::servicePendingOps : Stopping")
			return
		case <-fc.uploadWindow():
			if retryDelay > 0 {
				select {
				case <-time.After(retryDelay):
				case <-fc.componentStopping:
					return
				}
			}
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
			numFilesProcessed, err := fc.runPendingOpCycle()
			log.Info(
				"FileCache::servicePendingOps : Completed upload cycle, processed %d files",
				numFilesProcessed,
			)
			if err != nil {
				retryDelay = min(max(2*retryDelay, minRetryDelay), maxRetryDelay)
				log.Warn(
					"FileCache::servicePendingOps : Upload cycle failed, retrying in %v",
					retryDelay,
				)
				break
			}
			retryDelay = 0
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

// runPendingOpCycle : sync pending ops with cloud storage until the upload window
// closes or an operation fails. Returns the number of ops attempted, and the error
// that ended the cycle, if any.
func (fc *FileCache) runPendingOpCycle() (numFilesProcessed int, err error) {
	fc.pendingOps.Range(func(key, value any) bool {
		select {
		case <-fc.componentStopping:
			return false
		case <-fc.uploadWindow():
			name := key.(string)
			numFilesProcessed++
			err = fc.updateObject(name, value.(pendingFlags))
			return err == nil
		default:
			return false // upload window ended
		}
	})

	return numFilesProcessed, err
}

// synchronize pending operation with cloud storage
// returns the cloud error *if and only if* the op stays pending
func (fc *FileCache) updateObject(name string, flags pendingFlags) error {
	log.Trace(
		"FileCache::updateObject : %s, isDir=%t, isDeletion=%t",
		name,
		flags.isDir,
		flags.isDeletion,
	)

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
		if localMissing {
			log.Err("FileCache::updateObject : %s not found. Pending operation dropped.", localPath)
			fc.pendingOps.Delete(name)
			// do not interrupt other pending operations
			return nil
		} else {
			// other errors (EIO, EACCESS, out of resources, etc.)
			log.Err("FileCache::updateObject : %s %v", name, localErr)
			return localErr
		}
	}
	if flags.isDeletion && !localMissing {
		log.Err("FileCache::updateObject : %s exists. Ignoring deletion flag!", name)
	}
	if !localMissing && flags.isDir != info.IsDir() {
		log.Err("FileCache::updateObject : %s has wrong dir flag (%t)!", name, flags.isDir)
	}

	// update cloud
	var op, objType string
	var cloudErr error
	if localMissing {
		op = "deletion"
		if flags.isDeletion && fc.notInCloud(name) {
			// nothing to do
			log.Info("FileCache::updateObject : %s skipping cloud deletion (not in cloud)", name)
		} else if flags.isDir {
			objType = "directory"
			options := internal.DeleteDirOptions{Name: name}
			cloudErr = fc.NextComponent().DeleteDir(options)
		} else {
			objType = "file"
			options := internal.DeleteFileOptions{Name: name}
			cloudErr = fc.NextComponent().DeleteFile(options)
		}
		// ignore ENOENT
		if isNotExist(cloudErr) {
			log.Debug("FileCache::updateObject : %s does not exist in cloud", name)
			cloudErr = nil
		}
	} else {
		op = "creation/update"
		if info.IsDir() {
			objType = "directory"
			options := internal.CreateDirOptions{Name: name, Mode: info.Mode()}
			cloudErr = fc.NextComponent().CreateDir(options)
		} else {
			objType = "file"
			cloudErr = fc.uploadFile(name)
		}
		// ignore EEXIST
		if errors.Is(cloudErr, os.ErrExist) {
			log.Debug("FileCache::updateObject : %s already exists in cloud", name)
			cloudErr = nil
		}
	}

	// drop any ops that will never succeed
	if errors.Is(cloudErr, syscall.ENAMETOOLONG) {
		// the key is too long for the backend, so retrying can never succeed
		log.Err("FileCache::updateObject : %s key too long, abandoning upload", name)
		fc.pendingOps.Delete(name)
		// do not interrupt other pending operations
		return nil
	}

	// any remaining errors should interrupt all pending operations and delay retries
	if cloudErr != nil {
		err := fmt.Errorf("%s %s %s failed [%w]", name, objType, op, cloudErr)
		log.Err("FileCache::updateObject : %v", err)
		return err
	}

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
	noCachedData := isOffline(err) && errors.Is(err, &common.NoCachedDataError{})
	return !noCachedData
}
