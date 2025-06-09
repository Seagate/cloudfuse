/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.

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
	"errors"
	"fmt"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/handlemap"
	"github.com/Seagate/cloudfuse/internal/stats_manager"
	"github.com/awnumar/memguard"
	"github.com/spf13/viper"
)

// S3Storage Wrapper type around aws-sdk-go-v2/service/s3
type S3Storage struct {
	internal.BaseComponent
	storage               S3Connection
	stConfig              Config
	firstOffline          time.Time
	lastConnectionAttempt time.Time
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

	err = config.UnmarshalKey("restricted-characters-windows", &conf.RestrictedCharsWin)
	if err != nil {
		log.Err(
			"S3Storage::Configure : config error [unable to obtain restricted-characters-windows]",
		)
		return err
	}

	secrets := ConfigSecrets{}
	// Securely store key-id and secret-key in enclave
	if viper.GetString("s3storage.key-id") != "" {
		encryptedKeyID := memguard.NewEnclave([]byte(viper.GetString("s3storage.key-id")))

		if encryptedKeyID == nil {
			err := errors.New("unable to store key-id securely")
			log.Err("S3Storage::Configure : ", err.Error())
			return err
		}
		secrets.KeyID = encryptedKeyID
	}

	if viper.GetString("s3storage.secret-key") != "" {
		encryptedSecretKey := memguard.NewEnclave([]byte(viper.GetString("s3storage.secret-key")))

		if encryptedSecretKey == nil {
			err := errors.New("unable to store secret-key securely")
			log.Err("S3Storage::Configure : ", err.Error())
			return err
		}
		secrets.SecretKey = encryptedSecretKey
	}

	err = ParseAndValidateConfig(s3, conf, secrets)
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

	err = ParseAndReadDynamicConfig(s3, conf, true)
	if err != nil {
		log.Err("S3Storage::OnConfigChange : failed to reparse config", err.Error())
		return
	}

	err = s3.storage.UpdateConfig(s3.stConfig)
	if err != nil {
		log.Err("S3Storage::OnConfigChange : failed to UpdateConfig", err.Error())
		return
	}
}

func (s3 *S3Storage) configureAndTest(isParent bool) error {
	var err error
	s3.storage, err = NewConnection(s3.stConfig)
	return err
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

// Online check
func (s3 *S3Storage) CloudConnected() bool {
	log.Trace("S3Storage::CloudConnected")
	if !s3.timeToRetry() {
		log.Debug("S3Storage::CloudConnected : Exponential backoff triggered")
		return false
	}
	// Use ListBuckets to check if the S3 connection is online
	_, err := s3.ListBuckets()
	currentTime := time.Now()
	if err != nil {
		log.Err("S3Storage::CloudConnected : S3 connection is offline [%v]", err)
		s3.lastConnectionAttempt = currentTime
		if s3.firstOffline.IsZero() {
			s3.firstOffline = currentTime
		}
		return false
	}
	// update state
	s3.firstOffline = time.Time{}
	s3.lastConnectionAttempt = currentTime
	return true
}

func (s3 *S3Storage) timeToRetry() bool {
	// If the firstOffline is not set, it means we are online
	if s3.firstOffline.IsZero() {
		return true
	}
	// If we haven't checked for over 30 seconds, we can retry
	timeSinceLastAttempt := time.Since(s3.lastConnectionAttempt)
	if timeSinceLastAttempt > 30*time.Second {
		// Reset the firstOffline time if we are retrying after a long time
		s3.firstOffline = time.Time{}
		return true
	}
	// Minimum delay before retrying
	initialDelay := 5 * time.Second
	if timeSinceLastAttempt < initialDelay {
		return false
	}
	// Formula between 5 seconds and 30 seconds
	timeOffline := s3.lastConnectionAttempt.Sub(s3.firstOffline)
	if time.Since(s3.lastConnectionAttempt) < timeOffline {
		return false
	}
	return true
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
		s3StatsCollector.PushEvents(
			createDir,
			options.Name,
			map[string]interface{}{mode: options.Mode.String()},
		)
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
	// List up to two objects, since one could be the directory with a trailing slash
	list, _, err := s3.storage.List(formatListDirName(options.Name), nil, 2)
	if err != nil {
		log.Err("S3Storage::IsDirEmpty : error listing [%s]", err)
		return false
	}

	return len(list) == 0
}

func (s3 *S3Storage) StreamDir(
	options internal.StreamDirOptions,
) ([]*internal.ObjAttr, string, error) {
	log.Trace(
		"S3Storage::StreamDir : %s, offset %d, count %d",
		options.Name,
		options.Offset,
		options.Count,
	)
	objectList := make([]*internal.ObjAttr, 0)

	path := formatListDirName(options.Name)
	var iteration int           // = 0
	var marker = &options.Token // = nil
	var totalEntriesFetched int32
	entriesRemaining := options.Count
	if options.Count == 0 {
		entriesRemaining = maxResultsPerListCall
	}
	for entriesRemaining > 0 {
		newList, nextMarker, err := s3.storage.List(path, marker, entriesRemaining)
		if err != nil {
			log.Err("S3Storage::StreamDir : %s Failed to read dir [%s]", options.Name, err)
			return objectList, "", err
		}
		objectList = append(objectList, newList...)
		marker = nextMarker
		iteration++
		totalEntriesFetched += int32(len(newList))

		log.Debug("S3Storage::StreamDir : %s So far retrieved %d objects in %d iterations",
			options.Name, totalEntriesFetched, iteration)
		if marker == nil || *marker == "" {
			break
		} else {
			log.Debug("S3Storage::StreamDir : %s List iteration %d nextMarker=\"%s\"",
				options.Name, iteration, *nextMarker)
		}
		// decrement and loop
		entriesRemaining -= totalEntriesFetched
		// in one case, the response will be missing one entry (see comment above `count++` in Client::List)
		if entriesRemaining == 1 && options.Token == "" {
			// don't make a request just for that one leftover entry
			break
		}
	}

	if marker == nil {
		blnkStr := ""
		marker = &blnkStr
	}

	// if path is empty, it means it is the root, relative to the mounted directory
	if len(path) == 0 {
		path = "/"
	}
	s3StatsCollector.PushEvents(streamDir, path, map[string]interface{}{count: totalEntriesFetched})

	// increment streamDir call count
	s3StatsCollector.UpdateStats(stats_manager.Increment, streamDir, (int64)(1))

	return objectList, *marker, nil
}

func (s3 *S3Storage) RenameDir(options internal.RenameDirOptions) error {
	log.Trace("S3Storage::RenameDir : %s to %s", options.Src, options.Dst)
	options.Src = internal.TruncateDirName(options.Src)
	options.Dst = internal.TruncateDirName(options.Dst)

	err := s3.storage.RenameDirectory(options.Src, options.Dst)

	if err == nil {
		s3StatsCollector.PushEvents(
			renameDir,
			options.Src,
			map[string]interface{}{src: options.Src, dest: options.Dst},
		)
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

	s3StatsCollector.PushEvents(
		createFile,
		options.Name,
		map[string]interface{}{mode: options.Mode.String()},
	)

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

	err := s3.storage.RenameFile(options.Src, options.Dst, false)

	if err == nil {
		s3StatsCollector.PushEvents(
			renameFile,
			options.Src,
			map[string]interface{}{src: options.Src, dest: options.Dst},
		)
		s3StatsCollector.UpdateStats(stats_manager.Increment, renameFile, (int64)(1))
	}
	return err
}

// Read file data into the buffer given in options.Data.
func (s3 *S3Storage) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	//log.Trace("S3Storage::ReadInBuffer : Read %s from %d offset", h.Path, offset)

	if options.Offset > atomic.LoadInt64(&options.Handle.Size) {
		return 0, syscall.ERANGE
	}

	var dataLen = int64(len(options.Data))
	if atomic.LoadInt64(&options.Handle.Size) < (options.Offset + int64(len(options.Data))) {
		dataLen = options.Handle.Size - options.Offset
	}

	if dataLen == 0 {
		return 0, nil
	}

	err := s3.storage.ReadInBuffer(options.Handle.Path, options.Offset, dataLen, options.Data)
	if err != nil {
		log.Err(
			"S3Storage::ReadInBuffer : Failed to read %s [%s]",
			options.Handle.Path,
			err.Error(),
		)
	}

	length := int(dataLen)
	return length, err
}

func (s3 *S3Storage) WriteFile(options internal.WriteFileOptions) (int, error) {
	err := s3.storage.Write(options)
	return len(options.Data), err
}

func (s3 *S3Storage) GetFileBlockOffsets(
	options internal.GetFileBlockOffsetsOptions,
) (*common.BlockOffsetList, error) {
	return s3.storage.GetFileBlockOffsets(options.Name)

}

func (s3 *S3Storage) TruncateFile(options internal.TruncateFileOptions) error {
	log.Trace("S3Storage::TruncateFile : %s to %d bytes", options.Name, options.Size)
	err := s3.storage.TruncateFile(options.Name, options.Size)

	if err == nil {
		s3StatsCollector.PushEvents(
			truncateFile,
			options.Name,
			map[string]interface{}{size: options.Size},
		)
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
	if s3.stConfig.disableSymlink {
		log.Err(
			"S3Storage::CreateLink : %s -> %s - Symlink support not enabled",
			options.Name,
			options.Target,
		)
		return syscall.ENOTSUP
	}
	log.Trace("S3Storage::CreateLink : Create symlink %s -> %s", options.Name, options.Target)
	err := s3.storage.CreateLink(options.Name, options.Target, true)

	if err == nil {
		s3StatsCollector.PushEvents(
			createLink,
			options.Name,
			map[string]interface{}{target: options.Target},
		)
		s3StatsCollector.UpdateStats(stats_manager.Increment, createLink, (int64)(1))
	}

	return err
}

func (s3 *S3Storage) ReadLink(options internal.ReadLinkOptions) (string, error) {
	if s3.stConfig.disableSymlink {
		log.Err("S3Storage::ReadLink : %s - Symlink support not enabled", options.Name)
		return "", syscall.ENOENT
	}
	log.Trace("S3Storage::ReadLink : Read symlink %s", options.Name)

	data, err := s3.storage.ReadBuffer(options.Name, 0, 0, true)

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
	log.Trace("S3Storage::Chmod : Change mode of file %s", options.Name)

	s3StatsCollector.PushEvents(
		chmod,
		options.Name,
		map[string]interface{}{mode: options.Mode.String()},
	)
	s3StatsCollector.UpdateStats(stats_manager.Increment, chmod, (int64)(1))

	return nil
}

func (s3 *S3Storage) Chown(options internal.ChownOptions) error {
	log.Trace(
		"S3Storage::Chown : Change ownership of file %s to %d-%d",
		options.Name,
		options.Owner,
		options.Group,
	)
	return nil
}

func (s3 *S3Storage) FlushFile(options internal.FlushFileOptions) error {
	log.Trace("S3Storage::FlushFile : Flush file %s", options.Handle.Path)
	return s3.storage.StageAndCommit(options.Handle.Path, options.Handle.CacheObj.BlockOffsetList)
}

const blockSize = 4096

func (s3 *S3Storage) StatFs() (*common.Statfs_t, bool, error) {
	if s3.stConfig.disableUsage {
		return nil, false, nil
	}

	log.Trace("S3Storage::StatFs")
	// cache_size = f_blocks * f_frsize/1024
	// cache_size - used = f_frsize * f_bavail/1024
	// cache_size - used = vfs.f_bfree * vfs.f_frsize / 1024
	// if cache size is set to 0 then we have the root mount usage
	sizeUsed, err := s3.storage.GetUsedSize()
	if err != nil {
		// TODO: will returning EIO break any applications that depend on StatFs?
		return nil, true, err
	}

	stat := common.Statfs_t{
		Blocks: sizeUsed / blockSize,
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
		"S3Storage::StatFs : responding with free=%d avail=%d blocks=%d (bsize=%d)",
		stat.Bfree,
		stat.Bavail,
		stat.Blocks,
		stat.Bsize,
	)

	return &stat, true, nil
}

// TODO: decide if the TODO below is relevant and delete if not
// TODO : Below methods are pending to be implemented
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
}
