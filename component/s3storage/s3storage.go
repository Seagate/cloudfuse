/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

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

package s3storage

import (
	"context"
	"fmt"
	"sync/atomic"
	"syscall"
	"time"

	"lyvecloudfuse/common"
	"lyvecloudfuse/common/config"
	"lyvecloudfuse/common/log"
	"lyvecloudfuse/internal"
	"lyvecloudfuse/internal/handlemap"
	"lyvecloudfuse/internal/stats_manager"
)

// S3Storage Wrapper type around aws-sdk-go-v2/service/s3
type S3Storage struct {
	internal.BaseComponent
	storage  S3Connection
	stConfig Config
}

const compName = "s3storage"

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &S3Storage{}

var s3StatsCollector *stats_manager.StatsCollector

func (s3 *S3Storage) Name() string {
	return s3.BaseComponent.Name()
}

func (s3 *S3Storage) SetName(name string) {
	s3.BaseComponent.SetName(name)
}

func (s3 *S3Storage) SetNextComponent(c internal.Component) {
	s3.BaseComponent.SetNextComponent(c)
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
func (s3 *S3Storage) Configure(isParent bool) error {
	log.Trace("S3Storage::Configure : %s", s3.Name())

	conf := Options{}
	err := config.UnmarshalKey(s3.Name(), &conf)
	if err != nil {
		log.Err("S3Storage::Configure : config error [invalid config attributes]")
		return fmt.Errorf("config error in %s [%s]", s3.Name(), err.Error())
	}

	err = ParseAndValidateConfig(s3, conf)
	if err != nil {
		log.Err("S3Storage::Configure : Config validation failed [%s]", err.Error())
		return fmt.Errorf("config error in %s [%s]", s3.Name(), err.Error())
	}

	err = s3.configureAndTest(isParent)
	if err != nil {
		log.Err("S3Storage::Configure : Failed to validate storage account [%s]", err.Error())
		return err
	}

	return nil
}

func (s3 *S3Storage) Priority() internal.ComponentPriority {
	return internal.EComponentPriority.Consumer()
}

// OnConfigChange : When config file is changed, this will be called by pipeline. Refresh required config here
func (s3 *S3Storage) OnConfigChange() {
	log.Trace("S3Storage::OnConfigChange : %s", s3.Name())

	conf := Options{}
	err := config.UnmarshalKey(s3.Name(), &conf)
	if err != nil {
		log.Err("S3Storage::OnConfigChange : Config error [invalid config attributes]")
		return
	}

	// TODO: re-parse config here (ParseAndReadDynamicConfig)

	err = s3.storage.UpdateConfig(s3.stConfig)
	if err != nil {
		log.Err("S3Storage::OnConfigChange : failed to UpdateConfig", err.Error())
		return
	}
}

func (s3 *S3Storage) configureAndTest(isParent bool) error {
	s3.storage = NewConnection(s3.stConfig)
	return nil
}

// Start : Initialize the go-sdk pipeline here and test auth is working fine
func (s3 *S3Storage) Start(ctx context.Context) error {
	log.Trace("S3Storage::Start : Starting component %s", s3.Name())
	// create stats collector for s3storage
	s3StatsCollector = stats_manager.NewStatsCollector(s3.Name())
	log.Debug("Starting s3 stats collector")

	return nil
}

// Stop : Disconnect all running operations here
func (s3 *S3Storage) Stop() error {
	log.Trace("S3Storage::Stop : Stopping component %s", s3.Name())
	s3StatsCollector.Destroy()
	return nil
}

// ------------------------- Bucket listing -------------------------------------------
func (s3 *S3Storage) ListBuckets() ([]string, error) {
	return s3.storage.ListBuckets()
}

// ------------------------- Core Operations -------------------------------------------

// Directory operations
func (s3 *S3Storage) CreateDir(options internal.CreateDirOptions) error {
	log.Trace("S3Storage::CreateDir : %s", options.Name)

	err := s3.storage.CreateDirectory(internal.TruncateDirName(options.Name))

	if err == nil {
		s3StatsCollector.PushEvents(createDir, options.Name, map[string]interface{}{mode: options.Mode.String()})
		s3StatsCollector.UpdateStats(stats_manager.Increment, createDir, (int64)(1))
	}

	return err
}

func (s3 *S3Storage) DeleteDir(options internal.DeleteDirOptions) error {
	log.Trace("S3Storage::DeleteDir : %s", options.Name)

	err := s3.storage.DeleteDirectory(internal.TruncateDirName(options.Name))

	if err == nil {
		s3StatsCollector.PushEvents(deleteDir, options.Name, nil)
		s3StatsCollector.UpdateStats(stats_manager.Increment, deleteDir, (int64)(1))
	}

	return err
}

func formatListDirName(path string) string {
	// If we check the root directory, make sure we pass "" instead of "/"
	// If we aren't checking the root directory, then we want to extend the directory name so List returns all children and does not include the path itself.
	if path == "/" {
		path = ""
	} else if path != "" {
		path = internal.ExtendDirName(path)
	}
	return path
}

func (s3 *S3Storage) IsDirEmpty(options internal.IsDirEmptyOptions) bool {
	log.Trace("S3Storage::IsDirEmpty : %s", options.Name)
	list, _, err := s3.storage.List(formatListDirName(options.Name), nil, 1)
	if err != nil {
		log.Err("S3Storage::IsDirEmpty : error listing [%s]", err)
		return false
	}
	if len(list) == 0 {
		return true
	}
	return false
}

func (s3 *S3Storage) ReadDir(options internal.ReadDirOptions) ([]*internal.ObjAttr, error) {
	log.Trace("S3Storage::ReadDir : %s", options.Name)
	objectList := make([]*internal.ObjAttr, 0)

	path := formatListDirName(options.Name)
	var iteration int  // = 0
	var marker *string // = nil
	for {
		newList, newMarker, err := s3.storage.List(path, marker, common.MaxDirListCount)
		if err != nil {
			log.Err("S3Storage::ReadDir : Failed to read dir [%s]", err)
			return objectList, err
		}
		objectList = append(objectList, newList...)
		marker = newMarker
		iteration++

		log.Debug("S3Storage::ReadDir : So far retrieved %d objects in %d iterations", len(objectList), iteration)
		if newMarker == nil || *newMarker == "" {
			break
		}
	}

	return objectList, nil
}

func (s3 *S3Storage) StreamDir(options internal.StreamDirOptions) ([]*internal.ObjAttr, string, error) {
	log.Trace("S3Storage::StreamDir : Path %s, offset %d, count %d", options.Name, options.Offset, options.Count)

	path := formatListDirName(options.Name)

	newList, newMarker, err := s3.storage.List(path, &options.Token, options.Count)
	if err != nil {
		log.Err("S3Storage::StreamDir : Failed to read dir [%s]", err)
		return newList, "", err
	}

	log.Debug("S3Storage::StreamDir : Retrieved %d objects with %s marker for Path %s", len(newList), options.Token, path)

	if newMarker != nil && *newMarker != "" {
		log.Debug("S3Storage::StreamDir : next-marker %s for Path %s", *newMarker, path)
		if len(newList) == 0 {
			/* In some customer scenario we have seen that newList is empty but marker is not empty
			   which means backend has not returned any items this time but there are more left.
			   If we return back this empty list to fuse layer it will assume listing has completed
			   and will terminate the readdir call. As there are more items left on the server side we
			   need to retry getting a list here.
			*/
			log.Warn("S3Storage::StreamDir : next-marker %s but current list is empty. Need to retry listing", *newMarker)
			options.Token = *newMarker
			return s3.StreamDir(options)
		}
	}
	if newMarker == nil {
		blnkStr := ""
		newMarker = &blnkStr
	}

	// if path is empty, it means it is the root, relative to the mounted directory
	if len(path) == 0 {
		path = "/"
	}
	s3StatsCollector.PushEvents(streamDir, path, map[string]interface{}{count: len(newList)})

	// increment streamDir call count
	s3StatsCollector.UpdateStats(stats_manager.Increment, streamDir, (int64)(1))

	return newList, *newMarker, nil
}

func (s3 *S3Storage) RenameDir(options internal.RenameDirOptions) error {
	log.Trace("S3Storage::RenameDir : %s to %s", options.Src, options.Dst)
	options.Src = internal.TruncateDirName(options.Src)
	options.Dst = internal.TruncateDirName(options.Dst)

	err := s3.storage.RenameDirectory(options.Src, options.Dst)

	if err == nil {
		s3StatsCollector.PushEvents(renameDir, options.Src, map[string]interface{}{src: options.Src, dest: options.Dst})
		s3StatsCollector.UpdateStats(stats_manager.Increment, renameDir, (int64)(1))
	}
	return err
}

// File operations
func (s3 *S3Storage) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	log.Trace("S3Storage::CreateFile : %s", options.Name)

	// Create a handle object for the file being created
	// This handle will be added to handlemap by the first component in pipeline
	handle := handlemap.NewHandle(options.Name)
	if handle == nil {
		log.Err("S3Storage::CreateFile : Failed to create handle for %s", options.Name)
		return nil, syscall.EFAULT
	}

	err := s3.storage.CreateFile(options.Name, options.Mode)
	if err != nil {
		return nil, err
	}
	handle.Mtime = time.Now()

	s3StatsCollector.PushEvents(createFile, options.Name, map[string]interface{}{mode: options.Mode.String()})

	// increment open file handles count
	s3StatsCollector.UpdateStats(stats_manager.Increment, openHandles, (int64)(1))

	return handle, nil
}

func (s3 *S3Storage) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace("S3Storage::OpenFile : %s", options.Name)

	attr, err := s3.storage.GetAttr(options.Name)
	if err != nil {
		return nil, err
	}

	// Create a handle object for the file being opened
	// This handle will be added to handlemap by the first component in pipeline
	handle := handlemap.NewHandle(options.Name)
	if handle == nil {
		log.Err("S3Storage::OpenFile : Failed to create handle for %s", options.Name)
		return nil, syscall.EFAULT
	}
	handle.Size = int64(attr.Size)
	handle.Mtime = attr.Mtime

	// increment open file handles count
	s3StatsCollector.UpdateStats(stats_manager.Increment, openHandles, (int64)(1))

	return handle, nil
}

func (s3 *S3Storage) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("S3Storage::CloseFile : %s", options.Handle.Path)

	// decrement open file handles count
	s3StatsCollector.UpdateStats(stats_manager.Decrement, openHandles, (int64)(1))

	return nil
}

func (s3 *S3Storage) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("S3Storage::DeleteFile : %s", options.Name)

	err := s3.storage.DeleteFile(options.Name)

	if err == nil {
		s3StatsCollector.PushEvents(deleteFile, options.Name, nil)
		s3StatsCollector.UpdateStats(stats_manager.Increment, deleteFile, (int64)(1))
	}

	return err
}

func (s3 *S3Storage) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("S3Storage::RenameFile : %s to %s", options.Src, options.Dst)

	err := s3.storage.RenameFile(options.Src, options.Dst)

	if err == nil {
		s3StatsCollector.PushEvents(renameFile, options.Src, map[string]interface{}{src: options.Src, dest: options.Dst})
		s3StatsCollector.UpdateStats(stats_manager.Increment, renameFile, (int64)(1))
	}
	return err
}

// Read and return file data as a buffer.
func (s3 *S3Storage) ReadFile(options internal.ReadFileOptions) ([]byte, error) {
	//log.Trace("S3Storage::ReadFile : Read %s", h.Path)
	return s3.storage.ReadBuffer(options.Handle.Path, 0, 0)
}

// Read file data into the buffer given in options.Data.
func (s3 *S3Storage) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	//log.Trace("S3Storage::ReadInBuffer : Read %s from %d offset", h.Path, offset)

	if options.Offset > atomic.LoadInt64(&options.Handle.Size) {
		return 0, syscall.ERANGE
	}

	var dataLen int64 = int64(len(options.Data))
	if atomic.LoadInt64(&options.Handle.Size) < (options.Offset + int64(len(options.Data))) {
		dataLen = options.Handle.Size - options.Offset
	}

	if dataLen == 0 {
		return 0, nil
	}

	err := s3.storage.ReadInBuffer(options.Handle.Path, options.Offset, dataLen, options.Data)
	if err != nil {
		log.Err("S3Storage::ReadInBuffer : Failed to read %s [%s]", options.Handle.Path, err.Error())
	}

	length := int(dataLen)
	return length, err
}

func (s3 *S3Storage) WriteFile(options internal.WriteFileOptions) (int, error) {
	err := s3.storage.Write(options)
	return len(options.Data), err
}

func (s3 *S3Storage) GetFileBlockOffsets(options internal.GetFileBlockOffsetsOptions) (*common.BlockOffsetList, error) {
	return s3.storage.GetFileBlockOffsets(options.Name)

}

func (s3 *S3Storage) TruncateFile(options internal.TruncateFileOptions) error {
	log.Trace("S3Storage::TruncateFile : %s to %d bytes", options.Name, options.Size)
	err := s3.storage.TruncateFile(options.Name, options.Size)

	if err == nil {
		s3StatsCollector.PushEvents(truncateFile, options.Name, map[string]interface{}{size: options.Size})
		s3StatsCollector.UpdateStats(stats_manager.Increment, truncateFile, (int64)(1))
	}
	return err
}

func (s3 *S3Storage) CopyToFile(options internal.CopyToFileOptions) error {
	log.Trace("S3Storage::CopyToFile : Read file %s", options.Name)
	return s3.storage.ReadToFile(options.Name, options.Offset, options.Count, options.File)
}

func (s3 *S3Storage) CopyFromFile(options internal.CopyFromFileOptions) error {
	log.Trace("S3Storage::CopyFromFile : Upload file %s", options.Name)
	return s3.storage.WriteFromFile(options.Name, options.Metadata, options.File)
}

// Symlink operations
func (s3 *S3Storage) CreateLink(options internal.CreateLinkOptions) error {
	log.Trace("S3Storage::CreateLink : Create symlink %s -> %s", options.Name, options.Target)
	err := s3.storage.CreateLink(options.Name, options.Target)

	if err == nil {
		s3StatsCollector.PushEvents(createLink, options.Name, map[string]interface{}{target: options.Target})
		s3StatsCollector.UpdateStats(stats_manager.Increment, createLink, (int64)(1))
	}

	return err
}

func (s3 *S3Storage) ReadLink(options internal.ReadLinkOptions) (string, error) {
	log.Trace("S3Storage::ReadLink : Read symlink %s", options.Name)
	data, err := s3.storage.ReadBuffer(options.Name, 0, 0)

	if err != nil {
		s3StatsCollector.PushEvents(readLink, options.Name, nil)
		s3StatsCollector.UpdateStats(stats_manager.Increment, readLink, (int64)(1))
	}

	return string(data), err
}

// Attribute operations
func (s3 *S3Storage) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	//log.Trace("S3Storage::GetAttr : Get attributes of file %s", name)
	return s3.storage.GetAttr(options.Name)
}

func (s3 *S3Storage) Chmod(options internal.ChmodOptions) error {
	log.Trace("S3Storage::Chmod : Change mod of file %s", options.Name)

	s3StatsCollector.PushEvents(chmod, options.Name, map[string]interface{}{mode: options.Mode.String()})
	s3StatsCollector.UpdateStats(stats_manager.Increment, chmod, (int64)(1))

	return nil
}

func (s3 *S3Storage) Chown(options internal.ChownOptions) error {
	log.Trace("S3Storage::Chown : Change ownership of file %s to %d-%d", options.Name, options.Owner, options.Group)
	return nil
}

func (s3 *S3Storage) FlushFile(options internal.FlushFileOptions) error {
	log.Trace("S3Storage::FlushFile : Flush file %s", options.Handle.Path)
	// S3 does not expose blocks the way Azure blob storage does.
	// S3 has object "parts", but they are not meant to be accessed independently of each other.
	// So unless we find a way to bend over backwards to abuse the multi-part upload and download interface, flush has no meaning here.
	// S3 multi-part upload guide: https://docs.aws.amazon.com/AmazonS3/latest/userguide/mpuoverview.html
	return nil
}

// TODO: decide if the TODO below is relevant and delete if not
// TODO : Below methods are pending to be implemented
// SetAttr(string, internal.ObjAttr) error
// UnlinkFile(string) error
// ReleaseFile(*handlemap.Handle) error
// FlushFile(*handlemap.Handle) error

// ------------------------- Factory methods to create objects -------------------------------------------

// Constructor to create object of this component
func News3storageComponent() internal.Component {
	// Init the component with default config
	s3 := &S3Storage{
		stConfig: Config{
			// TODO: add AWS S3 config flags and populate config with them here
		},
	}

	s3.SetName(compName)
	config.AddConfigChangeEventListener(s3)
	return s3
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, News3storageComponent)
	// TODO: add config flags to customize AWS S3 SDK behavior and register them here
	// 	(see how this is done in azstorage for reference).
}
