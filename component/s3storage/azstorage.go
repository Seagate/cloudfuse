/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
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

	"github.com/spf13/cobra"
)

// S3Storage Wrapper type around azure go-sdk (track-1)
type S3Storage struct {
	internal.BaseComponent
	storage     S3Connection
	stConfig    S3StorageConfig
	startTime   time.Time
	listBlocked bool
}

const compName = "s3storage"

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &S3Storage{}

var azStatsCollector *stats_manager.StatsCollector

func (az *S3Storage) Name() string {
	return az.BaseComponent.Name()
}

func (az *S3Storage) SetName(name string) {
	az.BaseComponent.SetName(name)
}

func (az *S3Storage) SetNextComponent(c internal.Component) {
	az.BaseComponent.SetNextComponent(c)
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
func (az *S3Storage) Configure(isParent bool) error {
	log.Trace("S3Storage::Configure : %s", az.Name())

	conf := AzStorageOptions{}
	err := config.UnmarshalKey(az.Name(), &conf)
	if err != nil {
		fmt.Println("Unable to unmarshal")
		log.Err("S3Storage::Configure : config error [invalid config attributes]")
		return fmt.Errorf("config error in %s [%s]", az.Name(), err.Error())
	}

	err = ParseAndValidateConfig(az, conf)
	if err != nil {
		fmt.Println("Unable to parse and validate")
		log.Err("S3Storage::Configure : Config validation failed [%s]", err.Error())
		return fmt.Errorf("config error in %s [%s]", az.Name(), err.Error())
	}

	err = az.configureAndTest(isParent)
	if err != nil {
		log.Err("S3Storage::Configure : Failed to validate storage account [%s]", err.Error())
		return err
	}

	return nil
}

func (az *S3Storage) Priority() internal.ComponentPriority {
	return internal.EComponentPriority.Consumer()
}

// OnConfigChange : When config file is changed, this will be called by pipeline. Refresh required config here
func (az *S3Storage) OnConfigChange() {
	log.Trace("S3Storage::OnConfigChange : %s", az.Name())

	conf := AzStorageOptions{}
	err := config.UnmarshalKey(az.Name(), &conf)
	if err != nil {
		log.Err("S3Storage::OnConfigChange : Config error [invalid config attributes]")
		return
	}

	// err = ParseAndReadDynamicConfig(az, conf, true)
	// if err != nil {
	// 	log.Err("S3Storage::OnConfigChange : failed to reparse config", err.Error())
	// 	return
	// }

	err = az.storage.UpdateConfig(az.stConfig)
	if err != nil {
		log.Err("S3Storage::OnConfigChange : failed to UpdateConfig", err.Error())
		return
	}
}

func (az *S3Storage) configureAndTest(isParent bool) error {
	az.storage = NewS3StorageConnection(az.stConfig)
	return nil
}

// Start : Initialize the go-sdk pipeline here and test auth is working fine
func (az *S3Storage) Start(ctx context.Context) error {
	log.Trace("S3Storage::Start : Starting component %s", az.Name())
	// On mount block the ListBlob call for certain amount of time
	az.startTime = time.Now()
	az.listBlocked = true

	// create stats collector for azstorage
	azStatsCollector = stats_manager.NewStatsCollector(az.Name())

	return nil
}

// Stop : Disconnect all running operations here
func (az *S3Storage) Stop() error {
	log.Trace("S3Storage::Stop : Stopping component %s", az.Name())
	azStatsCollector.Destroy()
	return nil
}

// ------------------------- Container listing -------------------------------------------
func (az *S3Storage) ListContainers() ([]string, error) {
	return az.storage.ListContainers()
}

// ------------------------- Core Operations -------------------------------------------

// Directory operations
func (az *S3Storage) CreateDir(options internal.CreateDirOptions) error {
	log.Trace("S3Storage::CreateDir : %s", options.Name)

	err := az.storage.CreateDirectory(internal.TruncateDirName(options.Name))

	if err == nil {
		azStatsCollector.PushEvents(createDir, options.Name, map[string]interface{}{mode: options.Mode.String()})
		azStatsCollector.UpdateStats(stats_manager.Increment, createDir, (int64)(1))
	}

	return err
}

func (az *S3Storage) DeleteDir(options internal.DeleteDirOptions) error {
	log.Trace("S3Storage::DeleteDir : %s", options.Name)

	err := az.storage.DeleteDirectory(internal.TruncateDirName(options.Name))

	if err == nil {
		azStatsCollector.PushEvents(deleteDir, options.Name, nil)
		azStatsCollector.UpdateStats(stats_manager.Increment, deleteDir, (int64)(1))
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

func (az *S3Storage) IsDirEmpty(options internal.IsDirEmptyOptions) bool {
	log.Trace("S3Storage::IsDirEmpty : %s", options.Name)
	list, _, err := az.storage.List(formatListDirName(options.Name), nil, 1)
	if err != nil {
		log.Err("S3Storage::IsDirEmpty : error listing [%s]", err)
		return false
	}
	if len(list) == 0 {
		return true
	}
	return false
}

func (az *S3Storage) ReadDir(options internal.ReadDirOptions) ([]*internal.ObjAttr, error) {
	log.Trace("S3Storage::ReadDir : %s", options.Name)
	blobList := make([]*internal.ObjAttr, 0)

	// if az.listBlocked {
	// 	diff := time.Since(az.startTime)
	// 	if diff.Seconds() > float64(az.stConfig.cancelListForSeconds) {
	// 		az.listBlocked = false
	// 		log.Info("S3Storage::ReadDir : Unblocked List API")
	// 	} else {
	// 		log.Info("S3Storage::ReadDir : Blocked List API for %d more seconds", int(az.stConfig.cancelListForSeconds)-int(diff.Seconds()))
	// 		return blobList, nil
	// 	}
	// }

	path := formatListDirName(options.Name)
	var iteration int = 0
	var marker *string = nil
	for {
		new_list, new_marker, err := az.storage.List(path, marker, common.MaxDirListCount)
		if err != nil {
			log.Err("S3Storage::ReadDir : Failed to read dir [%s]", err)
			return blobList, err
		}
		blobList = append(blobList, new_list...)
		marker = new_marker
		iteration++

		log.Debug("S3Storage::ReadDir : So far retrieved %d objects in %d iterations", len(blobList), iteration)
		if new_marker == nil || *new_marker == "" {
			break
		}
	}

	return blobList, nil
}

func (az *S3Storage) StreamDir(options internal.StreamDirOptions) ([]*internal.ObjAttr, string, error) {
	log.Trace("S3Storage::StreamDir : Path %s, offset %d, count %d", options.Name, options.Offset, options.Count)

	// if az.listBlocked {
	// 	diff := time.Since(az.startTime)
	// 	if diff.Seconds() > float64(az.stConfig.cancelListForSeconds) {
	// 		az.listBlocked = false
	// 		log.Info("S3Storage::StreamDir : Unblocked List API")
	// 	} else {
	// 		log.Info("S3Storage::StreamDir : Blocked List API for %d more seconds", int(az.stConfig.cancelListForSeconds)-int(diff.Seconds()))
	// 		return make([]*internal.ObjAttr, 0), "", nil
	// 	}
	// }

	path := formatListDirName(options.Name)

	new_list, new_marker, err := az.storage.List(path, &options.Token, options.Count)
	if err != nil {
		log.Err("S3Storage::StreamDir : Failed to read dir [%s]", err)
		return new_list, "", err
	}

	log.Debug("S3Storage::StreamDir : Retrieved %d objects with %s marker for Path %s", len(new_list), options.Token, path)

	if new_marker != nil && *new_marker != "" {
		log.Debug("S3Storage::StreamDir : next-marker %s for Path %s", *new_marker, path)
		if len(new_list) == 0 {
			/* In some customer scenario we have seen that new_list is empty but marker is not empty
			   which means backend has not returned any items this time but there are more left.
			   If we return back this empty list to libfuse layer it will assume listing has completed
			   and will terminate the readdir call. As there are more items left on the server side we
			   need to retry getting a list here.
			*/
			log.Warn("S3Storage::StreamDir : next-marker %s but current list is empty. Need to retry listing", *new_marker)
			options.Token = *new_marker
			return az.StreamDir(options)
		}
	}

	// if path is empty, it means it is the root, relative to the mounted directory
	if len(path) == 0 {
		path = "/"
	}
	azStatsCollector.PushEvents(streamDir, path, map[string]interface{}{count: len(new_list)})

	// increment streamdir call count
	azStatsCollector.UpdateStats(stats_manager.Increment, streamDir, (int64)(1))

	return new_list, *new_marker, nil
}

func (az *S3Storage) RenameDir(options internal.RenameDirOptions) error {
	log.Trace("S3Storage::RenameDir : %s to %s", options.Src, options.Dst)
	options.Src = internal.TruncateDirName(options.Src)
	options.Dst = internal.TruncateDirName(options.Dst)

	err := az.storage.RenameDirectory(options.Src, options.Dst)

	if err == nil {
		azStatsCollector.PushEvents(renameDir, options.Src, map[string]interface{}{src: options.Src, dest: options.Dst})
		azStatsCollector.UpdateStats(stats_manager.Increment, renameDir, (int64)(1))
	}
	return err
}

// File operations
func (az *S3Storage) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	log.Trace("S3Storage::CreateFile : %s", options.Name)

	// Create a handle object for the file being created
	// This handle will be added to handlemap by the first component in pipeline
	handle := handlemap.NewHandle(options.Name)
	if handle == nil {
		log.Err("S3Storage::CreateFile : Failed to create handle for %s", options.Name)
		return nil, syscall.EFAULT
	}

	err := az.storage.CreateFile(options.Name, options.Mode)
	if err != nil {
		return nil, err
	}
	handle.Mtime = time.Now()

	azStatsCollector.PushEvents(createFile, options.Name, map[string]interface{}{mode: options.Mode.String()})

	// increment open file handles count
	azStatsCollector.UpdateStats(stats_manager.Increment, openHandles, (int64)(1))

	return handle, nil
}

func (az *S3Storage) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace("S3Storage::OpenFile : %s", options.Name)

	attr, err := az.storage.GetAttr(options.Name)
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
	azStatsCollector.UpdateStats(stats_manager.Increment, openHandles, (int64)(1))

	return handle, nil
}

func (az *S3Storage) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("S3Storage::CloseFile : %s", options.Handle.Path)

	// decrement open file handles count
	azStatsCollector.UpdateStats(stats_manager.Decrement, openHandles, (int64)(1))

	return nil
}

func (az *S3Storage) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("S3Storage::DeleteFile : %s", options.Name)

	err := az.storage.DeleteFile(options.Name)

	if err == nil {
		azStatsCollector.PushEvents(deleteFile, options.Name, nil)
		azStatsCollector.UpdateStats(stats_manager.Increment, deleteFile, (int64)(1))
	}

	return err
}

func (az *S3Storage) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("S3Storage::RenameFile : %s to %s", options.Src, options.Dst)

	err := az.storage.RenameFile(options.Src, options.Dst)

	if err == nil {
		azStatsCollector.PushEvents(renameFile, options.Src, map[string]interface{}{src: options.Src, dest: options.Dst})
		azStatsCollector.UpdateStats(stats_manager.Increment, renameFile, (int64)(1))
	}
	return err
}

func (az *S3Storage) ReadFile(options internal.ReadFileOptions) (data []byte, err error) {
	//log.Trace("S3Storage::ReadFile : Read %s", h.Path)
	return az.storage.ReadBuffer(options.Handle.Path, 0, 0)
}

func (az *S3Storage) ReadInBuffer(options internal.ReadInBufferOptions) (length int, err error) {
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

	err = az.storage.ReadInBuffer(options.Handle.Path, options.Offset, dataLen, options.Data)
	if err != nil {
		log.Err("S3Storage::ReadInBuffer : Failed to read %s [%s]", options.Handle.Path, err.Error())
	}

	length = int(dataLen)
	return
}

func (az *S3Storage) WriteFile(options internal.WriteFileOptions) (int, error) {
	err := az.storage.Write(options)
	return len(options.Data), err
}

func (az *S3Storage) GetFileBlockOffsets(options internal.GetFileBlockOffsetsOptions) (*common.BlockOffsetList, error) {
	return az.storage.GetFileBlockOffsets(options.Name)

}

func (az *S3Storage) TruncateFile(options internal.TruncateFileOptions) error {
	log.Trace("S3Storage::TruncateFile : %s to %d bytes", options.Name, options.Size)
	err := az.storage.TruncateFile(options.Name, options.Size)

	if err == nil {
		azStatsCollector.PushEvents(truncateFile, options.Name, map[string]interface{}{size: options.Size})
		azStatsCollector.UpdateStats(stats_manager.Increment, truncateFile, (int64)(1))
	}
	return err
}

func (az *S3Storage) CopyToFile(options internal.CopyToFileOptions) error {
	log.Trace("S3Storage::CopyToFile : Read file %s", options.Name)
	return az.storage.ReadToFile(options.Name, options.Offset, options.Count, options.File)
}

func (az *S3Storage) CopyFromFile(options internal.CopyFromFileOptions) error {
	log.Trace("S3Storage::CopyFromFile : Upload file %s", options.Name)
	return az.storage.WriteFromFile(options.Name, options.Metadata, options.File)
}

// Symlink operations
func (az *S3Storage) CreateLink(options internal.CreateLinkOptions) error {
	log.Trace("S3Storage::CreateLink : Create symlink %s -> %s", options.Name, options.Target)
	err := az.storage.CreateLink(options.Name, options.Target)

	if err == nil {
		azStatsCollector.PushEvents(createLink, options.Name, map[string]interface{}{target: options.Target})
		azStatsCollector.UpdateStats(stats_manager.Increment, createLink, (int64)(1))
	}

	return err
}

func (az *S3Storage) ReadLink(options internal.ReadLinkOptions) (string, error) {
	log.Trace("S3Storage::ReadLink : Read symlink %s", options.Name)
	data, err := az.storage.ReadBuffer(options.Name, 0, 0)

	if err != nil {
		azStatsCollector.PushEvents(readLink, options.Name, nil)
		azStatsCollector.UpdateStats(stats_manager.Increment, readLink, (int64)(1))
	}

	return string(data), err
}

// Attribute operations
func (az *S3Storage) GetAttr(options internal.GetAttrOptions) (attr *internal.ObjAttr, err error) {
	//log.Trace("S3Storage::GetAttr : Get attributes of file %s", name)
	return az.storage.GetAttr(options.Name)
}

func (az *S3Storage) Chmod(options internal.ChmodOptions) error {
	log.Trace("S3Storage::Chmod : Change mod of file %s", options.Name)
	err := az.storage.ChangeMod(options.Name, options.Mode)

	if err == nil {
		azStatsCollector.PushEvents(chmod, options.Name, map[string]interface{}{mode: options.Mode.String()})
		azStatsCollector.UpdateStats(stats_manager.Increment, chmod, (int64)(1))
	}

	return err
}

func (az *S3Storage) Chown(options internal.ChownOptions) error {
	log.Trace("S3Storage::Chown : Change ownership of file %s to %d-%d", options.Name, options.Owner, options.Group)
	return az.storage.ChangeOwner(options.Name, options.Owner, options.Group)
}

func (az *S3Storage) FlushFile(options internal.FlushFileOptions) error {
	log.Trace("S3Storage::FlushFile : Flush file %s", options.Handle.Path)
	return az.storage.StageAndCommit(options.Handle.Path, options.Handle.CacheObj.BlockOffsetList)
}

// TODO : Below methods are pending to be implemented
// SetAttr(string, internal.ObjAttr) error
// UnlinkFile(string) error
// ReleaseFile(*handlemap.Handle) error
// FlushFile(*handlemap.Handle) error

// ------------------------- Factory methods to create objects -------------------------------------------

// Constructor to create object of this component
func NewazstorageComponent() internal.Component {
	// Init the component with default config
	az := &S3Storage{
		stConfig: S3StorageConfig{
			// blockSize:      0,
			// maxConcurrency: 32,
			// defaultTier:    getAccessTierType("none"),
			// authConfig: azAuthConfig{
			// 	AuthMode: EAuthType.KEY(),
			// 	UseHTTP:  false,
			// },
		},
	}

	az.SetName(compName)
	config.AddConfigChangeEventListener(az)
	return az
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewazstorageComponent)
	RegisterEnvVariables()

	useHttps := config.AddBoolFlag("use-https", true, "Enables HTTPS communication with Blob storage.")
	config.BindPFlag(compName+".use-https", useHttps)
	useHttps.Hidden = true

	blockListSecFlag := config.AddInt32Flag("cancel-list-on-mount-seconds", 0, "Number of seconds list call is blocked post mount")
	config.BindPFlag(compName+".block-list-on-mount-sec", blockListSecFlag)
	blockListSecFlag.Hidden = true

	containerNameFlag := config.AddStringFlag("container-name", "", "Configures the name of the container to be mounted")
	config.BindPFlag(compName+".container", containerNameFlag)

	useAdls := config.AddBoolFlag("use-adls", false, "Enables blobfuse to access Azure DataLake storage account.")
	config.BindPFlag(compName+".use-adls", useAdls)
	useAdls.Hidden = true

	maxConcurrency := config.AddUint16Flag("max-concurrency", 32, "Option to override default number of concurrent storage connections")
	config.BindPFlag(compName+".max-concurrency", maxConcurrency)
	maxConcurrency.Hidden = true

	httpProxy := config.AddStringFlag("http-proxy", "", "HTTP Proxy address.")
	config.BindPFlag(compName+".http-proxy", httpProxy)
	httpProxy.Hidden = true

	httpsProxy := config.AddStringFlag("https-proxy", "", "HTTPS Proxy address.")
	config.BindPFlag(compName+".https-proxy", httpsProxy)
	httpsProxy.Hidden = true

	maxRetry := config.AddUint16Flag("max-retry", 3, "Maximum retry count if the failure codes are retryable.")
	config.BindPFlag(compName+".max-retries", maxRetry)
	maxRetry.Hidden = true

	maxRetryInterval := config.AddUint16Flag("max-retry-interval-in-seconds", 3, "Maximum number of seconds between 2 retries.")
	config.BindPFlag(compName+".max-retry-timeout-sec", maxRetryInterval)
	maxRetryInterval.Hidden = true

	retryDelayFactor := config.AddUint16Flag("retry-delay-factor", 1, "Retry delay between two tries")
	config.BindPFlag(compName+".retry-backoff-sec", retryDelayFactor)
	retryDelayFactor.Hidden = true

	setContentType := config.AddBoolFlag("set-content-type", true, "Turns on automatic 'content-type' property based on the file extension.")
	config.BindPFlag(compName+".set-content-type", setContentType)
	setContentType.Hidden = true

	caCertFile := config.AddStringFlag("ca-cert-file", "", "Specifies the proxy pem certificate path if its not in the default path.")
	config.BindPFlag(compName+".ca-cert-file", caCertFile)
	caCertFile.Hidden = true

	debugLibcurl := config.AddStringFlag("debug-libcurl", "", "Flag to allow users to debug libcurl calls.")
	config.BindPFlag(compName+".debug-libcurl", debugLibcurl)
	debugLibcurl.Hidden = true

	virtualDir := config.AddBoolFlag("virtual-directory", false, "Support virtual directories without existence of a special marker blob.")
	config.BindPFlag(compName+".virtual-directory", virtualDir)

	subDirectory := config.AddStringFlag("subdirectory", "", "Mount only this sub-directory from given container.")
	config.BindPFlag(compName+".subdirectory", subDirectory)

	config.RegisterFlagCompletionFunc("container-name", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	})
}
