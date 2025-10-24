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
   SOFTWARE
*/

package size_tracker

import (
	"context"
	"fmt"

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
	bucketUsage         uint64
	statSizeOffset      uint64
}

type SizeTrackerOptions struct {
	JournalName           string `config:"journal-name"             yaml:"journal-name,omitempty"`
	TotalBucketCapacityMb uint64 `config:"bucket-capacity-fallback" yaml:"bucket-capacity-fallback,omitempty"`
}

const compName = "size_tracker"
const blockSize = int64(4096)
const defaultJournalName = "mount_size.dat"
const evictionThreshold = 0.9
const minDisplayCapacity = 1 * common.TbToBytes
const minServerFraction = 0.05

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
	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (st *SizeTracker) Stop() error {
	log.Trace("SizeTracker::Stop : Stopping component %s", st.Name())
	_ = st.mountSize.CloseFile()
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

	if conf.TotalBucketCapacityMb != 0 {
		// bucket capacity must be at least 10% of largest mounted device to be recognized by Nx
		if conf.TotalBucketCapacityMb*common.MbToBytes > minDisplayCapacity {
			st.totalBucketCapacity = conf.TotalBucketCapacityMb * common.MbToBytes
		} else {
			log.Warn(
				"SizeTracker::Configure : Bucket capacity set to %dMB. Defaulting to minimum (%dMB)",
				conf.TotalBucketCapacityMb,
				minDisplayCapacity/common.MbToBytes,
			)
			st.totalBucketCapacity = minDisplayCapacity
		}
		// set display capacity
		st.displayCapacity = st.totalBucketCapacity
		if config.IsSet("libfuse.display-capacity-mb") {
			var confDisplayCapacityMb uint64
			err = config.UnmarshalKey("libfuse.display-capacity-mb", &confDisplayCapacityMb)
			if err == nil {
				if confDisplayCapacityMb*common.MbToBytes > minDisplayCapacity {
					st.displayCapacity = confDisplayCapacityMb * common.MbToBytes
				} else {
					log.Warn(
						"SizeTracker::Configure : Display capacity set to %dMB. Defaulting to minimum (%dMB)",
						confDisplayCapacityMb,
						minDisplayCapacity/common.MbToBytes,
					)
					st.displayCapacity = minDisplayCapacity
				}
			} else {
				log.Err("SizeTracker::Configure : Invalid display capacity")
				confDisplayCapacityMb = 0
			}
		}
	}

	journalName := defaultJournalName
	if config.IsSet(compName + ".journal-name") {
		journalName = conf.JournalName
	} else {
		s3conf := s3storage.Options{}
		if err := config.UnmarshalKey("s3storage", &s3conf); err == nil {
			sanitizedName := common.SanitizeName(s3conf.BucketName + "-" + s3conf.PrefixPath)
			if sanitizedName != "" {
				journalName = sanitizedName + ".dat"
			}
		} else {
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
}

func (st *SizeTracker) RenameDir(options internal.RenameDirOptions) error {
	// Rename dir should not allow renaming files into a directory that already exists so we should not
	// need to update the size here.
	return st.NextComponent().RenameDir(options)
}

// File operations
func (st *SizeTracker) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	attr, getAttrErr := st.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Name})

	handle, err := st.NextComponent().CreateFile(options)

	// File already exists but create succeeded so remove old file size
	if err == nil && getAttrErr == nil {
		st.mountSize.Subtract(uint64(attr.Size))
	}

	return handle, err
}

func (st *SizeTracker) DeleteFile(options internal.DeleteFileOptions) error {
	attr, getAttrErr := st.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Name})

	err := st.NextComponent().DeleteFile(options)

	// If the file is a symlink then it has no size so don't change the size
	if err == nil && getAttrErr == nil && !attr.IsSymlink() {
		st.mountSize.Subtract(uint64(attr.Size))
	}

	return err
}

func (st *SizeTracker) RenameFile(options internal.RenameFileOptions) error {
	dstAttr, dstErr := st.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Dst})

	err := st.NextComponent().RenameFile(options)

	// If dst already exists and rename succeeds, remove overwritten dst size
	if dstErr == nil && err == nil {
		st.mountSize.Subtract(uint64(dstAttr.Size))
	}

	return err
}

func (st *SizeTracker) WriteFile(options internal.WriteFileOptions) (int, error) {
	var oldSize int64
	attr, getAttrErr1 := st.NextComponent().
		GetAttr(internal.GetAttrOptions{Name: options.Handle.Path})
	if getAttrErr1 == nil {
		oldSize = attr.Size
	} else {
		log.Err("SizeTracker::WriteFile : Unable to get attr for file %s. Current tracked size is invalid. Error: : %v", options.Handle.Path, getAttrErr1)
	}

	bytesWritten, err := st.NextComponent().WriteFile(options)
	if err != nil {
		return bytesWritten, err
	}
	newSize := max(oldSize, options.Offset+int64(len(options.Data)))

	diff := newSize - oldSize

	// File already exists and WriteFile succeeded subtract difference in file size
	if diff < 0 {
		// diff is negative, so change it back to positive before converting to a uint64
		st.mountSize.Subtract(uint64(-diff))
	} else {
		st.mountSize.Add(uint64(diff))
	}

	return bytesWritten, nil
}

func (st *SizeTracker) TruncateFile(options internal.TruncateFileOptions) error {
	var origSize int64
	attr, getAttrErr := st.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Name})
	if getAttrErr == nil {
		origSize = attr.Size
	}

	err := st.NextComponent().TruncateFile(options)
	newSize := options.Size - origSize

	// File already exists and truncate succeeded subtract difference in file size
	if err == nil && getAttrErr == nil && newSize < 0 {
		st.mountSize.Subtract(uint64(-newSize))
	} else if err == nil && getAttrErr == nil && newSize >= 0 {
		st.mountSize.Add(uint64(newSize))
	}

	return err
}

func (st *SizeTracker) CopyFromFile(options internal.CopyFromFileOptions) error {
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
	newSize := fileInfo.Size() - origSize

	// File already exists and CopyFromFile succeeded subtract difference in file size
	if newSize < 0 {
		st.mountSize.Subtract(uint64(-newSize))
	} else {
		st.mountSize.Add(uint64(newSize))
	}

	return nil
}

// Filesystem level operations
func (st *SizeTracker) StatFs() (*common.Statfs_t, bool, error) {
	log.Trace("SizeTracker::StatFs")

	blocks := st.mountSize.GetSize() / uint64(blockSize)

	if st.totalBucketCapacity != 0 {
		stat, ret, err := st.NextComponent().StatFs()

		if err == nil && ret {
			returnedBucketUsage := stat.Blocks * uint64(blockSize)
			// convert everything to float64
			bucketCapacity := float64(st.totalBucketCapacity)
			bucketUsage := float64(returnedBucketUsage)
			displayCapacity := float64(st.displayCapacity)
			serverUsage := float64(st.mountSize.GetSize())
			// Custom logic for use with Nx Plugin
			// the target is to fill the entire bucket to the eviction threshold
			bucketPercentFull := bucketUsage / bucketCapacity
			isBucketOverused := bucketPercentFull > evictionThreshold
			if isBucketOverused {
				// use a usage offset to control this server's storage use
				// update the offset whenever we get updated bucket usage
				isBucketUsageUpdated := returnedBucketUsage != st.bucketUsage
				if isBucketUsageUpdated {
					// record the bucket size to recognize the next update
					st.bucketUsage = returnedBucketUsage
					// bucket
					// the goal is to hold the bucket at 90% full
					targetBucketUsage := bucketCapacity * evictionThreshold
					// how much needs to be evicted from the bucket?
					targetBucketReduction := bucketUsage - targetBucketUsage

					// server
					// what fraction of the bucket is the server using?
					serverFraction := serverUsage / bucketUsage
					// so, if everyone needs to evict the same percent, how much does this server need to evict?
					targetServerReduction := targetBucketReduction * serverFraction
					// to get the server to evict that, we need to report being over 90% by that amount
					reportUsage := displayCapacity*evictionThreshold + targetServerReduction

					// don't starve latecomers
					// above, we chose to make everyone evict the same fraction of their data
					// but what if a new server joins, and has nothing to evict?
					// To handle this case, we check:
					// is this server being starved, while the bucket is not critically full?
					serverIsStarved := serverFraction < minServerFraction
					bucketHasRoom := bucketUsage < bucketCapacity*.97
					if serverIsStarved && bucketHasRoom {
						// artificially set the usage to 75% so the mount doesn't show up as reserved
						reportUsage = displayCapacity * 0.75
					}

					// to display the reported size, we use an offset
					st.statSizeOffset = uint64(max(0, reportUsage-serverUsage))
				}
				// use the size offset to communicate the right target size to the Nx application
				blocks += st.statSizeOffset / uint64(blockSize)
			}
		}
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

func (st *SizeTracker) CommitData(opt internal.CommitDataOptions) error {
	var origSize int64
	attr, err := st.NextComponent().GetAttr(internal.GetAttrOptions{Name: opt.Name})
	if err == nil {
		origSize = attr.Size
	} else {
		log.Err("SizeTracker::CommitData : Unable to get attr for file %s. Current tracked size is invalid. Error: : %v", opt.Name, err)
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
		log.Err("SizeTracker::CommitData : Unable to get attr for file %s. Current tracked size is invalid. Error: : %v", opt.Name, err)
	}

	diff := newSize - origSize

	// File already exists and CommitData succeeded subtract difference in file size
	if diff < 0 {
		st.mountSize.Subtract(uint64(-diff))
	} else {
		st.mountSize.Add(uint64(diff))
	}

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
