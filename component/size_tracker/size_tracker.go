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
	"strings"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/component/azstorage"
	"github.com/Seagate/cloudfuse/component/s3storage"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/handlemap"
)

/* NOTES:
   - Component shall have a structure which inherits "internal.BaseComponent" to participate in pipeline
   - Component shall register a name and its constructor to participate in pipeline  (add by default by generator)
   - Order of calls : Constructor -> Configure -> Start ..... -> Stop
   - To read any new setting from config file follow the Configure method default comments
*/

// Common structure for Component
type SizeTracker struct {
	internal.BaseComponent
	mountSize           *MountSize
	displayCapacityMb   uint64
	totalBucketCapacity uint64
}

// Structure defining your config parameters
type SizeTrackerOptions struct {
	JournalName         string `config:"journal-name" yaml:"journal-name,omitempty"`
	TotalBucketCapacity uint64 `config:"bucket-capacity-fallback" yaml:"bucket-capacity-fallback,omitempty"`
}

const compName = "size_tracker"
const blockSize = int64(4096)
const defaultJournalName = "mount_size.dat"
const evictionThreshold = 0.95

// Verification to check satisfaction criteria with Component Interface
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

	st.totalBucketCapacity = conf.TotalBucketCapacity

	if st.totalBucketCapacity != 0 {
		// Borrow enable-symlinks flag from attribute cache
		if config.IsSet("libfuse.display-capacity-mb") {
			err := config.UnmarshalKey("libfuse.display-capacity-mb", &st.displayCapacityMb)
			if err != nil {
				st.totalBucketCapacity = 0
				log.Err("Configure : Failed to unmarshal libfuse.display-capacity-mb. Attempting to use" +
					" bucket capacity fallback without setting display capacity.")
			}
		}
	}

	journalName := defaultJournalName
	if config.IsSet(compName + ".journal-name") {
		journalName = conf.JournalName
	} else {
		s3conf := s3storage.Options{}
		if err := config.UnmarshalKey("s3storage", &s3conf); err == nil {
			sanitizedName := sanitizeFileName(s3conf.BucketName + "-" + s3conf.PrefixPath)
			if sanitizedName != "" {
				journalName = sanitizedName + ".dat"
			}
		} else {
			azconf := azstorage.AzStorageOptions{}
			if err := config.UnmarshalKey("azstorage", &azconf); err == nil {
				sanitizedName := sanitizeFileName(azconf.Container + "-" + azconf.PrefixPath)
				if sanitizedName != "" {
					journalName = sanitizedName + ".dat"
				}
			}
		}
	}

	st.mountSize, err = CreateSizeJournal(journalName)
	return err
}

func sanitizeFileName(filename string) string {
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(filename)
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
	attr, getAttrErr1 := st.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Handle.Path})
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

	var journalErr error
	// File already exists and WriteFile succeeded subtract difference in file size
	if diff < 0 {
		// diff is negative, so change it back to positive before converting to a uint64
		st.mountSize.Subtract(uint64(-diff))
	} else {
		st.mountSize.Add(uint64(diff))
	}
	if journalErr != nil {
		log.Err("SizeTracker::WriteFile : Unable to journal size. Error: %v", journalErr)
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

	var journalErr error
	// File already exists and truncate succeeded subtract difference in file size
	if err == nil && getAttrErr == nil && newSize < 0 {
		st.mountSize.Subtract(uint64(-newSize))
	} else if err == nil && getAttrErr == nil && newSize >= 0 {
		st.mountSize.Add(uint64(newSize))
	}
	if journalErr != nil {
		log.Err("SizeTracker::TruncateFile : Unable to journal size. Error: %v", journalErr)
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

	var journalErr error
	// File already exists and CopyFromFile succeeded subtract difference in file size
	if newSize < 0 {
		st.mountSize.Subtract(uint64(-newSize))
	} else {
		st.mountSize.Add(uint64(newSize))
	}
	if journalErr != nil {
		log.Err("SizeTracker::CopyFromFile : Unable to journal size. Error: %v", journalErr)
	}

	return nil
}

func (st *SizeTracker) FlushFile(options internal.FlushFileOptions) error {
	var origSize int64
	attr, getAttrErr1 := st.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Handle.Path})
	if getAttrErr1 == nil {
		origSize = attr.Size
	} else {
		log.Err("SizeTracker::FlushFile : Unable to get attr for file %s. Current tracked size is invalid. Error: : %v", options.Handle.Path, getAttrErr1)
	}

	err := st.NextComponent().FlushFile(options)
	if err != nil {
		return err
	}

	var newSize int64
	attr, getAttrErr2 := st.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Handle.Path})
	if getAttrErr2 == nil {
		newSize = attr.Size
	} else {
		log.Err("SizeTracker::FlushFile : Unable to get attr for file %s. Current tracked size is invalid. Error: : %v", options.Handle.Path, getAttrErr2)
	}

	if getAttrErr1 != nil || getAttrErr2 != nil {
		return nil
	}

	diff := newSize - origSize

	var journalErr error
	// File already exists and FlushFile succeeded subtract difference in file size
	if diff < 0 {
		st.mountSize.Subtract(uint64(-diff))
	} else {
		st.mountSize.Add(uint64(diff))
	}
	if journalErr != nil {
		log.Err("SizeTracker::FlushFile : Unable to journal size. Error: %v", journalErr)
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
			// Custom logic for use with Nx Plugin
			// If the user is over the capacity limit set by Nx, then we need to prevent them from
			// accidental overuse of their bucket. So we change our reporting to instead report
			// the used capacity of the bucket to enable the VMS to start eviction
			if float64(stat.Blocks*uint64(blockSize)) > evictionThreshold*float64(st.totalBucketCapacity) {
				log.Warn("SizeTracker::StatFs : changing from size_tracker size to S3 bucket size due to overuse of bucket")
				blocks = stat.Blocks
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

	log.Debug("SizeTracker::StatFs : responding with free=%d avail=%d blocks=%d (bsize=%d)", stat.Bfree, stat.Bavail, stat.Blocks, stat.Bsize)

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

	var journalErr error
	// File already exists and CommitData succeeded subtract difference in file size
	if diff < 0 {
		st.mountSize.Subtract(uint64(-diff))
	} else {
		st.mountSize.Add(uint64(diff))
	}
	if journalErr != nil {
		log.Err("SizeTracker::CommitData : Unable to journal size. Error: %v", journalErr)
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
