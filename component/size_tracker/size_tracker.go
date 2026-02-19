/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2026 Seagate Technology LLC and/or its Affiliates

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

package size_tracker

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/component/azstorage"
	"github.com/Seagate/cloudfuse/component/s3storage"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/handlemap"
)

// Common structure for Component
type SizeTracker struct {
	internal.BaseComponent
	mountSize           *MountSize
	totalBucketCapacity uint64
	displayCapacity     uint64
	serverCount         uint64
	evictionMode        EvictionMode
	bucketUsage         uint64
	statSizeOffset      uint64
	statfsRefresh       time.Duration
	lastStatfsUpdate    time.Time
	statfsMu            sync.Mutex
}

type EvictionMode int

const (
	Normal    EvictionMode = iota // 0
	Overuse                       // 1
	Emergency                     // 2
)

type SizeTrackerOptions struct {
	JournalName         string `config:"journal-name"             yaml:"journal-name,omitempty"`
	TotalBucketCapacity uint64 `config:"bucket-capacity-fallback" yaml:"bucket-capacity-fallback,omitempty"`
	StatfsRefreshSec    uint32 `config:"statfs-refresh-sec"       yaml:"statfs-refresh-sec,omitempty"`
}

const compName = "size_tracker"
const blockSize = int64(4096)
const defaultJournalName = "mount_size.dat"

// these usage thresholds determine when the eviction mode changes
const targetUtilization = 0.9                                          // 90%
const hysteresisMargin = 0.02                                          // 2%
const overuseThreshold = targetUtilization + hysteresisMargin          // 92%
const emergencyThreshold = overuseThreshold + .05                      // 97%
const bucketNormalizedThreshold = targetUtilization - hysteresisMargin // 88%

var _ internal.Component = &SizeTracker{}

func (st *SizeTracker) Name() string {
	return compName
}

func (st *SizeTracker) SetName(name string) {
	st.BaseComponent.SetName(name)
}

func (st *SizeTracker) SetNextComponent(nc internal.Component) {
	st.BaseComponent.SetNextComponent(nc)
}

// Start : Pipeline calls this method to start the component functionality
//
//	this shall not block the call otherwise pipeline will not start
func (st *SizeTracker) Start(ctx context.Context) error {
	log.Trace("SizeTracker::Start : Starting component %s", st.Name())
	st.mountSize.Start()
	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (st *SizeTracker) Stop() error {
	log.Trace("SizeTracker::Stop : Stopping component %s", st.Name())
	_ = st.mountSize.Stop()
	return nil
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (st *SizeTracker) Configure(_ bool) error {
	log.Trace("SizeTracker::Configure : %s", st.Name())

	// >> If you do not need any config parameters remove below code and return nil
	conf := SizeTrackerOptions{}
	err := config.UnmarshalKey(st.Name(), &conf)
	if err != nil {
		log.Err("SizeTracker::Configure : config error [invalid config attributes]")
		return fmt.Errorf("SizeTracker: config error [invalid config attributes]")
	}

	if conf.TotalBucketCapacity != 0 {
		// TODO: document these units
		st.totalBucketCapacity = conf.TotalBucketCapacity * common.MbToBytes
		// set display capacity
		st.displayCapacity = st.totalBucketCapacity
		if config.IsSet("libfuse.display-capacity-mb") {
			var confDisplayCapacityMb uint64
			err = config.UnmarshalKey("libfuse.display-capacity-mb", &confDisplayCapacityMb)
			if err == nil {
				st.displayCapacity = confDisplayCapacityMb * common.MbToBytes
			} else {
				log.Err("SizeTracker::Configure : Invalid display capacity")
			}
		}
		// calculate server count
		// round to the nearest whole number
		st.serverCount = (st.totalBucketCapacity + st.displayCapacity/2) / st.displayCapacity
	}

	if conf.StatfsRefreshSec > 0 {
		st.statfsRefresh = time.Duration(conf.StatfsRefreshSec) * time.Second
	}

	journalName := defaultJournalName
	if config.IsSet(compName + ".journal-name") {
		journalName = conf.JournalName
	} else {
		if config.IsSet("s3storage") {
			s3conf := s3storage.Options{}
			if err := config.UnmarshalKey("s3storage", &s3conf); err == nil {
				sanitizedName := common.SanitizeName(s3conf.BucketName + "-" + s3conf.PrefixPath)
				if sanitizedName != "" {
					journalName = sanitizedName + ".dat"
				}
			}
		} else if config.IsSet("azstorage") {
			azconf := azstorage.AzStorageOptions{}
			if err := config.UnmarshalKey("azstorage", &azconf); err == nil {
				sanitizedName := common.SanitizeName(azconf.Container + "-" + azconf.PrefixPath)
				if sanitizedName != "" {
					journalName = sanitizedName + ".dat"
				}
			}
		}
	}

	st.mountSize, err = CreateSizeJournal(journalName)
	return err
}

func (st *SizeTracker) Priority() internal.ComponentPriority {
	return internal.EComponentPriority.LevelOne()
}

// OnConfigChange : If component has registered, on config file change this method is called
func (st *SizeTracker) OnConfigChange() {
	log.Trace("SizeTracker::OnConfigChange : %s", st.Name())
}

func (st *SizeTracker) RenameDir(options internal.RenameDirOptions) error {
	// Rename dir should not allow renaming files into a directory that already exists so we should not
	// need to update the size here.
	return st.NextComponent().RenameDir(options)
}

// File operations
func (st *SizeTracker) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	log.Trace("SizeTracker::CreateFile : %s", options.Name)
	attr, getAttrErr := st.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Name})

	handle, err := st.NextComponent().CreateFile(options)

	// File already exists but create succeeded so remove old file size
	if err == nil && getAttrErr == nil {
		st.mountSize.Add(-attr.Size)
	}

	return handle, err
}

func (st *SizeTracker) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("SizeTracker::DeleteFile : %s", options.Name)
	attr, getAttrErr := st.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Name})

	err := st.NextComponent().DeleteFile(options)

	if err == nil && getAttrErr == nil {
		st.mountSize.Add(-attr.Size)
	}

	return err
}

func (st *SizeTracker) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("SizeTracker::RenameFile : %s->%s", options.Src, options.Dst)
	dstAttr, dstErr := st.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Dst})

	err := st.NextComponent().RenameFile(options)

	// If dst already exists and rename succeeds, remove overwritten dst size
	if dstErr == nil && err == nil {
		st.mountSize.Add(-dstAttr.Size)
	}

	return err
}

func (st *SizeTracker) WriteFile(options *internal.WriteFileOptions) (int, error) {
	var oldSize int64
	attr, getAttrErr1 := st.NextComponent().
		GetAttr(internal.GetAttrOptions{Name: options.Handle.Path})
	if getAttrErr1 == nil {
		oldSize = attr.Size
	} else {
		log.Err(
			"SizeTracker::WriteFile : Unable to get attr for file %s. Current tracked size is invalid. Error: : %v",
			options.Handle.Path,
			getAttrErr1,
		)
	}

	bytesWritten, err := st.NextComponent().WriteFile(options)
	if err != nil {
		return bytesWritten, err
	}
	newSize := max(oldSize, options.Offset+int64(len(options.Data)))

	diff := newSize - oldSize

	// File already exists and WriteFile succeeded subtract difference in file size
	st.mountSize.Add(diff)

	return bytesWritten, nil
}

func (st *SizeTracker) TruncateFile(options internal.TruncateFileOptions) error {
	log.Trace("SizeTracker::TruncateFile : %s to %dB", options.Name, options.NewSize)
	var origSize int64
	attr, getAttrErr := st.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Name})
	if getAttrErr == nil {
		origSize = attr.Size
	} else if !os.IsNotExist(getAttrErr) {
		log.Err(
			"SizeTracker::TruncateFile : %s GetAttr failed. Here's why: %v",
			options.Name,
			getAttrErr,
		)
	}

	err := st.NextComponent().TruncateFile(options)
	if err != nil {
		return err
	}

	// subtract difference in file size
	st.mountSize.Add(options.NewSize - origSize)
	return nil
}

func (st *SizeTracker) CopyFromFile(options internal.CopyFromFileOptions) error {
	log.Trace("SizeTracker::CopyFromFile : %s", options.Name)
	var origSize int64
	attr, err := st.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Name})
	if err == nil {
		origSize = attr.Size
	}

	err = st.NextComponent().CopyFromFile(options)
	if err != nil {
		return err
	}
	fileInfo, err := options.File.Stat()
	if err != nil {
		return nil
	}
	st.mountSize.Add(fileInfo.Size() - origSize)
	return nil
}

// Filesystem level operations
func (st *SizeTracker) StatFs() (*common.Statfs_t, bool, error) {
	log.Trace("SizeTracker::StatFs")

	blocks := st.mountSize.GetSize() / uint64(blockSize)

	if st.totalBucketCapacity != 0 {
		now := time.Now()
		shouldRefresh := true
		var statSizeOffset uint64

		if st.statfsRefresh > 0 {
			st.statfsMu.Lock()
			if !st.lastStatfsUpdate.IsZero() && now.Sub(st.lastStatfsUpdate) < st.statfsRefresh {
				shouldRefresh = false
				statSizeOffset = st.statSizeOffset
			}
			st.statfsMu.Unlock()
		}

		if shouldRefresh {
			stat, ret, err := st.NextComponent().StatFs()
			if err != nil {
				return nil, true, err
			}

			if ret {
				// Custom logic for use with Nx Plugin
				// The Nx VMS evicts data until utilization is at 90% (of display capacity)
				// Use a size offset to show the Nx eviction threshold at our desired utilization
				// Only update the offset when bucket usage is updated
				returnedBucketUsage := stat.Blocks * uint64(blockSize)

				st.statfsMu.Lock()
				isBucketUsageUpdated := returnedBucketUsage != st.bucketUsage
				if isBucketUsageUpdated {
					// record the updated usage
					st.bucketUsage = returnedBucketUsage
					// convert everything to float64
					bucketCapacity := float64(st.totalBucketCapacity)
					bucketUsage := float64(returnedBucketUsage)
					displayCapacity := float64(st.displayCapacity)
					serverUsage := float64(st.mountSize.GetSize())
					serverCount := float64(st.serverCount)
					sizeOffset := float64(st.statSizeOffset)
					nxEvictionThreshold := targetUtilization * displayCapacity
					intendedCapacity := bucketCapacity / serverCount
					// Use a finite state machine. The evictionMode is the state.
					// calculate bucket usage and update eviction mode accordingly
					st.updateState(bucketUsage / bucketCapacity)
					switch st.evictionMode {
					case Normal:
						// the server count starts as the bucket capacity divided by the display capacity
						// if the server count has been incremented, offset the tracked size
						sizeOffset = nxEvictionThreshold - targetUtilization*intendedCapacity
					case Overuse:
						// drive to a utilization target below the bucketNormalizedThreshold
						normalizationTargetFactor := bucketNormalizedThreshold - hysteresisMargin
						sizeOffset = nxEvictionThreshold - normalizationTargetFactor*intendedCapacity
					case Emergency:
						// just report the whole bucket usage
						sizeOffset = bucketUsage - serverUsage
					}
					st.statSizeOffset = uint64(max(0, sizeOffset))
				}
				st.lastStatfsUpdate = now
				statSizeOffset = st.statSizeOffset
				st.statfsMu.Unlock()
			} else {
				st.statfsMu.Lock()
				statSizeOffset = st.statSizeOffset
				st.statfsMu.Unlock()
			}
		}

		// add the offset
		blocks += statSizeOffset / uint64(blockSize)
	}

	stat := common.Statfs_t{
		Blocks: blocks,
		// there is no set capacity limit in cloud storage
		// so we use zero for free and avail
		// this zero value is used in the libfuse component to recognize that cloud storage responded
		Bavail:  0,
		Bfree:   0,
		Bsize:   blockSize,
		Ffree:   1e9,
		Files:   1e9,
		Frsize:  blockSize,
		Namemax: 255,
	}

	log.Debug(
		"SizeTracker::StatFs : responding with free=%d avail=%d blocks=%d (bsize=%d)",
		stat.Bfree,
		stat.Bavail,
		stat.Blocks,
		stat.Bsize,
	)

	return &stat, true, nil
}

func (st *SizeTracker) updateState(bucketUsageFactor float64) {
	switch st.evictionMode {
	case Normal:
		if bucketUsageFactor > overuseThreshold {
			st.evictionMode = Overuse
		}
	case Overuse:
		if bucketUsageFactor < bucketNormalizedThreshold {
			st.evictionMode = Normal
		} else if bucketUsageFactor > emergencyThreshold {
			st.evictionMode = Emergency
			// severe overuse strongly suggests an incorrect server count
			st.serverCount++
		}
	case Emergency:
		if bucketUsageFactor < bucketNormalizedThreshold {
			st.evictionMode = Normal
		}
	}
}

func (st *SizeTracker) CommitData(opt internal.CommitDataOptions) error {
	log.Trace("SizeTracker::CopyFromFile : %s", opt.Name)
	var origSize int64
	attr, err := st.NextComponent().GetAttr(internal.GetAttrOptions{Name: opt.Name})
	if err == nil {
		origSize = attr.Size
	} else {
		log.Err(
			"SizeTracker::CommitData : Unable to get attr for file %s. Current tracked size is invalid. Error: : %v",
			opt.Name,
			err,
		)
	}

	err = st.NextComponent().CommitData(opt)
	if err != nil {
		return err
	}

	var newSize int64
	attr, err = st.NextComponent().GetAttr(internal.GetAttrOptions{Name: opt.Name})
	if err == nil {
		newSize = attr.Size
	} else {
		log.Err(
			"SizeTracker::CommitData : Unable to get attr for file %s. Current tracked size is invalid. Error: : %v",
			opt.Name,
			err,
		)
	}

	st.mountSize.Add(newSize - origSize)

	return nil
}

func (st *SizeTracker) CreateLink(options internal.CreateLinkOptions) error {
	log.Trace("SizeTracker::CreateLink : %s", options.Name)
	var origSize int64
	attr, err := st.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Name})
	if err == nil {
		origSize = attr.Size
	}

	err = st.NextComponent().CreateLink(options)
	if err != nil {
		return err
	}

	newSize := int64(len(options.Target))
	st.mountSize.Add(newSize - origSize)

	return nil
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewSizeTrackerComponent() internal.Component {
	comp := &SizeTracker{}
	comp.SetName(compName)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewSizeTrackerComponent)
}
