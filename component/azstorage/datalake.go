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

package azstorage

import (
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/convertname"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/directory"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/file"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/filesystem"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/service"
)

type Datalake struct {
	AzStorageConnection
	Auth           azAuth
	Service        *service.Client
	Filesystem     *filesystem.Client
	BlockBlob      BlockBlob
	datalakeCPKOpt *file.CPKInfo
}

// Verify that Datalake implements AzConnection interface
var _ AzConnection = &Datalake{}

// transformAccountEndpoint
// Users must set an endpoint to allow cloudfuse to
// 1. support Azure clouds (ex: Public, Zonal DNS, China, Germany, Gov, etc)
// 2. direct REST APIs to a truly custom endpoint (ex: www dot custom-domain dot com)
// We can handle case 1 by simply replacing the .dfs. to .blob. and cloudfuse will work fine.
// However, case 2 will not work since the endpoint likely only redirects to the dfs endpoint and not the blob endpoint, so we don't know what endpoint to use when we call blob endpoints.
// This is also a known problem with the SDKs.
func transformAccountEndpoint(potentialDfsEndpoint string) string {
	if strings.Contains(potentialDfsEndpoint, ".dfs.") {
		return strings.ReplaceAll(potentialDfsEndpoint, ".dfs.", ".blob.")
	} else {
		// Should we just throw here?
		log.Warn("Datalake::transformAccountEndpoint : Detected use of a custom endpoint. Not all operations are guaranteed to work.")
	}
	return potentialDfsEndpoint
}

// transformConfig transforms the adls config to a blob config
func transformConfig(dlConfig AzStorageConfig) AzStorageConfig {
	bbConfig := dlConfig
	bbConfig.authConfig.AccountType = EAccountType.BLOCK()
	bbConfig.authConfig.Endpoint = transformAccountEndpoint(dlConfig.authConfig.Endpoint)
	return bbConfig
}

func (dl *Datalake) Configure(cfg AzStorageConfig) error {
	dl.Config = cfg

	if dl.Config.cpkEnabled {
		dl.datalakeCPKOpt = &file.CPKInfo{
			EncryptionKey:       &dl.Config.cpkEncryptionKey,
			EncryptionKeySHA256: &dl.Config.cpkEncryptionKeySha256,
			EncryptionAlgorithm: to.Ptr(directory.EncryptionAlgorithmTypeAES256),
		}
	}
	return dl.BlockBlob.Configure(transformConfig(cfg))
}

// For dynamic config update the config here
func (dl *Datalake) UpdateConfig(cfg AzStorageConfig) error {
	dl.Config.blockSize = cfg.blockSize
	dl.Config.maxConcurrency = cfg.maxConcurrency
	dl.Config.defaultTier = cfg.defaultTier
	dl.Config.ignoreAccessModifiers = cfg.ignoreAccessModifiers
	return dl.BlockBlob.UpdateConfig(cfg)
}

// UpdateServiceClient : Update the SAS specified by the user and create new service client
func (dl *Datalake) UpdateServiceClient(key, value string) (err error) {
	if key == "saskey" {
		dl.Auth.setOption(key, value)
		// get the service client with updated SAS
		svcClient, err := dl.Auth.getServiceClient(&dl.Config)
		if err != nil {
			log.Err(
				"Datalake::UpdateServiceClient : Failed to get service client [%s]",
				err.Error(),
			)
			return err
		}

		// update the service client
		dl.Service = svcClient.(*service.Client)

		// Update the filesystem client
		dl.Filesystem = dl.Service.NewFileSystemClient(dl.Config.container)
	}
	return dl.BlockBlob.UpdateServiceClient(key, value)
}

// createServiceClient : Create the service client
func (dl *Datalake) createServiceClient() (*service.Client, error) {
	log.Trace("Datalake::createServiceClient : Getting service client")

	dl.Auth = getAzAuth(dl.Config.authConfig)
	if dl.Auth == nil {
		log.Err("Datalake::createServiceClient : Failed to retrieve auth object")
		return nil, fmt.Errorf("failed to retrieve auth object")
	}

	svcClient, err := dl.Auth.getServiceClient(&dl.Config)
	if err != nil {
		log.Err("Datalake::createServiceClient : Failed to get service client [%s]", err.Error())
		return nil, err
	}

	return svcClient.(*service.Client), nil
}

// SetupPipeline : Based on the config setup the ***URLs
func (dl *Datalake) SetupPipeline() error {
	log.Trace("Datalake::SetupPipeline : Setting up")
	var err error

	// create the service client
	dl.Service, err = dl.createServiceClient()
	if err != nil {
		log.Err("Datalake::SetupPipeline : Failed to get service client [%s]", err.Error())
		return err
	}

	// create the filesystem client
	dl.Filesystem = dl.Service.NewFileSystemClient(dl.Config.container)

	return dl.BlockBlob.SetupPipeline()
}

// TestPipeline : Validate the credentials specified in the auth config
func (dl *Datalake) TestPipeline() error {
	log.Trace("Datalake::TestPipeline : Validating")

	if dl.Config.mountAllContainers {
		return nil
	}

	if dl.Filesystem == nil || dl.Filesystem.DFSURL() == "" || dl.Filesystem.BlobURL() == "" {
		log.Err("Datalake::TestPipeline : Filesystem Client is not built, check your credentials")
		return nil
	}

	maxResults := int32(2)
	listPathPager := dl.Filesystem.NewListPathsPager(false, &filesystem.ListPathsOptions{
		MaxResults: &maxResults,
		Prefix:     &dl.Config.prefixPath,
	})

	// we are just validating the auth mode used. So, no need to iterate over the pages
	_, err := listPathPager.NextPage(context.Background())
	if err != nil {
		log.Err("Datalake::TestPipeline : Failed to validate account with given auth %s", err.Error)
		return err
	}

	return dl.BlockBlob.TestPipeline()
}

// check the connection to the service by calling GetProperties on the container
func (dl *Datalake) ConnectionOkay(ctx context.Context) error {
	log.Trace("BlockBlob::ConnectionOkay : checking connection to cloud service")
	return dl.BlockBlob.ConnectionOkay(ctx)
}

func (dl *Datalake) ListContainers(ctx context.Context) ([]string, error) {
	log.Trace("Datalake::ListContainers : Listing containers")
	return dl.BlockBlob.ListContainers(ctx)
}

func (dl *Datalake) SetPrefixPath(path string) error {
	log.Trace("Datalake::SetPrefixPath : path %s", path)
	dl.Config.prefixPath = path
	return dl.BlockBlob.SetPrefixPath(path)
}

// CreateFile : Create a new file in the filesystem/directory
func (dl *Datalake) CreateFile(ctx context.Context, name string, mode os.FileMode) error {
	log.Trace("Datalake::CreateFile : name %s", name)
	err := dl.BlockBlob.CreateFile(ctx, name, mode)
	if err != nil {
		log.Err("Datalake::CreateFile : Failed to create file %s [%s]", name, err.Error())
		return err
	}
	err = dl.ChangeMod(ctx, name, mode)
	if err != nil {
		log.Err(
			"Datalake::CreateFile : Failed to set permissions on file %s [%s]",
			name,
			err.Error(),
		)
		return err
	}

	return nil
}

// CreateDirectory : Create a new directory in the filesystem/directory
func (dl *Datalake) CreateDirectory(ctx context.Context, name string) error {
	log.Trace("Datalake::CreateDirectory : name %s", name)

	directoryURL := dl.getDirectoryClient(name)
	_, err := directoryURL.Create(ctx, &directory.CreateOptions{
		CPKInfo: dl.datalakeCPKOpt,
		AccessConditions: &directory.AccessConditions{
			ModifiedAccessConditions: &directory.ModifiedAccessConditions{
				IfNoneMatch: to.Ptr(azcore.ETagAny),
			},
		},
	})

	if err != nil {
		serr := storeDatalakeErrToErr(err)
		switch serr {
		case InvalidPermission:
			log.Err(
				"Datalake::CreateDirectory : Insufficient permissions for %s [%s]",
				name,
				err.Error(),
			)
			return syscall.EACCES
		case ErrFileAlreadyExists:
			log.Err(
				"Datalake::CreateDirectory : Path already exists for %s [%s]",
				name,
				err.Error(),
			)
			return syscall.EEXIST
		default:
			log.Err(
				"Datalake::CreateDirectory : Failed to create directory %s [%s]",
				name,
				err.Error(),
			)
			return err
		}
	}

	return nil
}

// CreateLink : Create a symlink in the filesystem/directory
func (dl *Datalake) CreateLink(ctx context.Context, source string, target string) error {
	log.Trace("Datalake::CreateLink : %s -> %s", source, target)
	return dl.BlockBlob.CreateLink(ctx, source, target)
}

// DeleteFile : Delete a file in the filesystem/directory
func (dl *Datalake) DeleteFile(ctx context.Context, name string) (err error) {
	log.Trace("Datalake::DeleteFile : name %s", name)
	fileClient := dl.getFileClient(name)
	_, err = fileClient.Delete(ctx, nil)
	if err != nil {
		serr := storeDatalakeErrToErr(err)
		switch serr {
		case ErrFileNotFound:
			log.Err("Datalake::DeleteFile : %s does not exist", name)
			return syscall.ENOENT
		case BlobIsUnderLease:
			log.Err("Datalake::DeleteFile : %s is under lease [%s]", name, err.Error())
			return syscall.EIO
		case InvalidPermission:
			log.Err(
				"Datalake::DeleteFile : Insufficient permissions for %s [%s]",
				name,
				err.Error(),
			)
			return syscall.EACCES
		default:
			log.Err("Datalake::DeleteFile : Failed to delete file %s [%s]", name, err.Error())
			return err
		}
	}

	return nil
}

// DeleteDirectory : Delete a directory in the filesystem/directory
func (dl *Datalake) DeleteDirectory(ctx context.Context, name string) (err error) {
	log.Trace("Datalake::DeleteDirectory : name %s", name)

	directoryClient := dl.getDirectoryClient(name)
	_, err = directoryClient.Delete(ctx, nil)
	// TODO : There is an ability to pass a continuation token here for recursive delete, should we implement this logic to follow continuation token? The SDK does not currently do this.
	if err != nil {
		serr := storeDatalakeErrToErr(err)
		if serr == ErrFileNotFound {
			log.Err("Datalake::DeleteDirectory : %s does not exist", name)
			return syscall.ENOENT
		} else {
			log.Err("Datalake::DeleteDirectory : Failed to delete directory %s [%s]", name, err.Error())
			return err
		}
	}

	return nil
}

// RenameFile : Rename the file
func (dl *Datalake) RenameFile(ctx context.Context, source string, target string) error {
	log.Trace("Datalake::RenameFile : %s -> %s", source, target)

	fileClient := dl.getFileClientPathEscape(source)

	_, err := fileClient.Rename(
		ctx,
		dl.getFormattedPath(target),
		&file.RenameOptions{
			CPKInfo: dl.datalakeCPKOpt,
		},
	)
	if err != nil {
		serr := storeDatalakeErrToErr(err)
		if serr == ErrFileNotFound {
			log.Err("Datalake::RenameFile : %s does not exist", source)
			return syscall.ENOENT
		} else {
			log.Err("Datalake::RenameFile : Failed to rename file %s to %s [%s]", source, target, err.Error())
			return err
		}
	}

	return nil
}

// RenameDirectory : Rename the directory
func (dl *Datalake) RenameDirectory(ctx context.Context, source string, target string) error {
	log.Trace("Datalake::RenameDirectory : %s -> %s", source, target)

	directoryClient := dl.getDirectoryClientPathEscape(source)
	_, err := directoryClient.Rename(
		ctx,
		dl.getFormattedPath(target),
		&directory.RenameOptions{
			CPKInfo: dl.datalakeCPKOpt,
		},
	)
	if err != nil {
		serr := storeDatalakeErrToErr(err)
		if serr == ErrFileNotFound {
			log.Err("Datalake::RenameDirectory : %s does not exist", source)
			return syscall.ENOENT
		} else {
			log.Err("Datalake::RenameDirectory : Failed to rename directory %s to %s [%s]", source, target, err.Error())
			return err
		}
	}

	return nil
}

// GetAttr : Retrieve attributes of the path
func (dl *Datalake) GetAttr(ctx context.Context, name string) (attr *internal.ObjAttr, err error) {
	log.Trace("Datalake::GetAttr : name %s", name)

	fileClient := dl.getFileClient(name)
	prop, err := fileClient.GetProperties(ctx, &file.GetPropertiesOptions{
		CPKInfo: dl.datalakeCPKOpt,
	})
	if err != nil {
		e := storeDatalakeErrToErr(err)
		switch e {
		case ErrFileNotFound:
			return attr, syscall.ENOENT
		case InvalidPermission:
			log.Err("Datalake::GetAttr : Insufficient permissions for %s [%s]", name, err.Error())
			return attr, syscall.EACCES
		default:
			log.Err(
				"Datalake::GetAttr : Failed to get path properties for %s [%s]",
				name,
				err.Error(),
			)
			return attr, err
		}
	}

	mode, err := getFileMode(*prop.Permissions)
	if err != nil {
		log.Err("Datalake::GetAttr : Failed to get file mode for %s [%s]", name, err.Error())
		return attr, err
	}

	attr = &internal.ObjAttr{
		Path:   name,
		Name:   filepath.Base(name),
		Size:   *prop.ContentLength,
		Mode:   mode,
		Mtime:  *prop.LastModified,
		Atime:  *prop.LastModified,
		Ctime:  *prop.LastModified,
		Crtime: *prop.LastModified,
		Flags:  internal.NewFileBitMap(),
	}
	parseMetadata(attr, prop.Metadata)

	if *prop.ResourceType == "directory" {
		attr.Flags = internal.NewDirBitMap()
		attr.Mode = attr.Mode | os.ModeDir
	}

	if dl.Config.honourACL && dl.Config.authConfig.ObjectID != "" {
		acl, err := fileClient.GetAccessControl(ctx, nil)
		if err != nil {
			// Just ignore the error here as rest of the attributes have been retrieved
			log.Err("Datalake::GetAttr : Failed to get ACL for %s [%s]", name, err.Error())
		} else {
			mode, err := getFileModeFromACL(dl.Config.authConfig.ObjectID, *acl.ACL, *acl.Owner)
			if err != nil {
				log.Err("Datalake::GetAttr : Failed to get file mode from ACL for %s [%s]", name, err.Error())
			} else {
				attr.Mode = mode
			}
		}
	}

	return attr, nil
}

// List : Get a list of path matching the given prefix
// This fetches the list using a marker so the caller code should handle marker logic
// If count=0 - fetch max entries
func (dl *Datalake) List(
	ctx context.Context,
	prefix string,
	marker *string,
	count int32,
) ([]*internal.ObjAttr, *string, error) {
	log.Trace("Datalake::List : prefix %s, marker %s", prefix, func(marker *string) string {
		if marker != nil {
			return *marker
		} else {
			return ""
		}
	}(marker))

	pathList := make([]*internal.ObjAttr, 0)

	if count == 0 {
		count = common.MaxDirListCount
	}

	prefixPath := dl.getFormattedPath(prefix)
	if prefix != "" && prefix[len(prefix)-1] == '/' {
		prefixPath += "/"
	}

	// Get a result segment starting with the path indicated by the current Marker.
	pager := dl.Filesystem.NewListPathsPager(false, &filesystem.ListPathsOptions{
		Marker:     marker,
		MaxResults: &count,
		Prefix:     &prefixPath,
	})

	// Process the paths returned in this result segment (if the segment is empty, the loop body won't execute)
	listPath, err := pager.NextPage(ctx)
	if err != nil {
		log.Err("Datalake::List : Failed to validate account with given auth %s", err.Error())
		m := ""
		e := storeDatalakeErrToErr(err)
		switch e {
		case ErrFileNotFound: // TODO: should this be checked for list calls
			return pathList, &m, syscall.ENOENT
		case InvalidPermission:
			return pathList, &m, syscall.EACCES
		default:
			return pathList, &m, err
		}
	}

	// Process the paths returned in this result segment (if the segment is empty, the loop body won't execute)
	for _, pathInfo := range listPath.Paths {
		// convert path name
		conv_name := dl.getFileName(*pathInfo.Name)
		pathInfo.Name = &conv_name

		var attr *internal.ObjAttr
		var lastModifiedTime time.Time
		if dl.Config.disableSymlink {
			var mode fs.FileMode
			if pathInfo.Permissions != nil {
				mode, err = getFileMode(*pathInfo.Permissions)
				if err != nil {
					log.Err(
						"Datalake::List : Failed to get file mode for %s [%s]",
						*pathInfo.Name,
						err.Error(),
					)
					m := ""
					return pathList, &m, err
				}
			} else {
				// This happens when a blob account is mounted with type:adls
				log.Err("Datalake::List : Failed to get file permissions for %s", *pathInfo.Name)
			}

			var contentLength int64 = 0
			if pathInfo.ContentLength != nil {
				contentLength = *pathInfo.ContentLength
			} else {
				// This happens when a blob account is mounted with type:adls
				log.Err("Datalake::List : Failed to get file length for %s", *pathInfo.Name)
			}

			if pathInfo.LastModified != nil {
				lastModifiedTime, err = time.Parse(time.RFC1123, *pathInfo.LastModified)
				if err != nil {
					log.Err(
						"Datalake::List : Failed to get last modified time for %s [%s]",
						*pathInfo.Name,
						err.Error(),
					)
				}
			}
			attr = &internal.ObjAttr{
				Path:   *pathInfo.Name,
				Name:   filepath.Base(*pathInfo.Name),
				Size:   contentLength,
				Mode:   mode,
				Mtime:  lastModifiedTime,
				Atime:  lastModifiedTime,
				Ctime:  lastModifiedTime,
				Crtime: lastModifiedTime,
				Flags:  internal.NewFileBitMap(),
			}
			if pathInfo.IsDirectory != nil && *pathInfo.IsDirectory {
				attr.Flags = internal.NewDirBitMap()
				attr.Mode = attr.Mode | os.ModeDir
			}
		} else {
			attr, err = dl.GetAttr(ctx, *pathInfo.Name)
			if err != nil {
				log.Err("Datalake::List : Failed to get properties for %s [%s]", *pathInfo.Name, err.Error())
				m := ""
				return pathList, &m, err
			}
		}

		// Note: Datalake list paths does not return metadata/properties.
		// To account for this and accurately return attributes when needed,
		// we have a flag for whether or not metadata has been retrieved.
		// If this flag is not set the attribute cache will call get attributes
		// to fetch metadata properties.
		// Any method that populates the metadata should set the attribute flag.
		// Alternatively, if you want Datalake list paths to return metadata/properties as well.
		// pass CLI parameter --enable-symlinks=true in the mount command.
		pathList = append(pathList, attr)

	}

	return pathList, listPath.Continuation, nil
}

// ReadToFile : Download a file to a local file
func (dl *Datalake) ReadToFile(
	ctx context.Context,
	name string,
	offset int64,
	count int64,
	fi *os.File,
) (err error) {
	return dl.BlockBlob.ReadToFile(ctx, name, offset, count, fi)
}

// ReadBuffer : Download a specific range from a file to a buffer
func (dl *Datalake) ReadBuffer(
	ctx context.Context,
	name string,
	offset int64,
	length int64,
) ([]byte, error) {
	return dl.BlockBlob.ReadBuffer(ctx, name, offset, length)
}

// ReadInBuffer : Download specific range from a file to a user provided buffer
func (dl *Datalake) ReadInBuffer(
	ctx context.Context,
	name string,
	offset int64,
	length int64,
	data []byte,
) error {
	return dl.BlockBlob.ReadInBuffer(ctx, name, offset, length, data)
}

// WriteFromFile : Upload local file to file
func (dl *Datalake) WriteFromFile(
	ctx context.Context,
	name string,
	metadata map[string]*string,
	fi *os.File,
) (err error) {
	// File in DataLake may have permissions and ACL set. Just uploading the file will override them.
	// So, we need to get the existing permissions and ACL and set them back after uploading the file.

	var acl = ""
	var fileClient *file.Client = nil

	if dl.Config.preserveACL {
		fileClient = dl.Filesystem.NewFileClient(filepath.Join(dl.Config.prefixPath, name))
		resp, err := fileClient.GetAccessControl(ctx, nil)
		if err != nil {
			log.Err("Datalake::getACL : Failed to get ACLs for file %s [%s]", name, err.Error())
		} else if resp.ACL != nil {
			acl = *resp.ACL
		}
	}

	// Upload the file, which will override the permissions and ACL
	retCode := dl.BlockBlob.WriteFromFile(ctx, name, metadata, fi)

	if acl != "" {
		// Cannot set both permissions and ACL in one call. ACL includes permission as well so just setting those back
		// Just setting up the permissions will delete existing ACLs applied on the blob so do not convert this code to
		// just set the permissions.
		_, err := fileClient.SetAccessControl(ctx, &file.SetAccessControlOptions{
			ACL: &acl,
		})

		if err != nil {
			// Earlier code was ignoring this so it might break customer cases where they do not have auth to update ACL
			log.Err("Datalake::WriteFromFile : Failed to set ACL for %s [%s]", name, err.Error())
		}
	}

	return retCode
}

// WriteFromBuffer : Upload from a buffer to a file
func (dl *Datalake) WriteFromBuffer(
	ctx context.Context,
	name string,
	metadata map[string]*string,
	data []byte,
) error {
	return dl.BlockBlob.WriteFromBuffer(ctx, name, metadata, data)
}

// Write : Write to a file at given offset
func (dl *Datalake) Write(ctx context.Context, options internal.WriteFileOptions) error {
	return dl.BlockBlob.Write(ctx, options)
}

func (dl *Datalake) StageAndCommit(
	ctx context.Context,
	name string,
	bol *common.BlockOffsetList,
) error {
	return dl.BlockBlob.StageAndCommit(ctx, name, bol)
}

func (dl *Datalake) GetFileBlockOffsets(
	ctx context.Context,
	name string,
) (*common.BlockOffsetList, error) {
	return dl.BlockBlob.GetFileBlockOffsets(ctx, name)
}

func (dl *Datalake) TruncateFile(ctx context.Context, name string, size int64) error {
	return dl.BlockBlob.TruncateFile(ctx, name, size)
}

// ChangeMod : Change mode of a path
func (dl *Datalake) ChangeMod(ctx context.Context, name string, mode os.FileMode) error {
	log.Trace("Datalake::ChangeMod : Change mode of file %s to %s", name, mode)
	fileClient := dl.getFileClient(name)

	/*
		// If we need to call the ACL set api then we need to get older acl string here
		// and create new string with the username included in the string
		// Keeping this code here so in future if its required we can get the string and manipulate

		currPerm, err := fileURL.getACL(ctx)
		e := storeDatalakeErrToErr(err)
		if e == ErrFileNotFound {
			return syscall.ENOENT
		} else if err != nil {
			log.Err("Datalake::ChangeMod : Failed to get mode of file %s [%s]", name, err.Error())
			return err
		}
	*/

	newPerm := getACLPermissions(mode)
	_, err := fileClient.SetAccessControl(ctx, &file.SetAccessControlOptions{
		Permissions: &newPerm,
	})
	if err != nil {
		log.Err(
			"Datalake::ChangeMod : Failed to change mode of file %s to %s [%s]",
			name,
			mode,
			err.Error(),
		)
		e := storeDatalakeErrToErr(err)
		switch e {
		case ErrFileNotFound:
			return syscall.ENOENT
		case InvalidPermission:
			return syscall.EACCES
		default:
			return err
		}
	}

	return nil
}

// ChangeOwner : Change owner of a path
func (dl *Datalake) ChangeOwner(ctx context.Context, name string, _ int, _ int) error {
	log.Trace("Datalake::ChangeOwner : name %s", name)

	if dl.Config.ignoreAccessModifiers {
		// for operations like git clone where transaction fails if chown is not successful
		// return success instead of ENOSYS
		return nil
	}

	// TODO: This is not supported for now.
	// fileURL := dl.Filesystem.NewRootDirectoryURL().NewFileURL(common.JoinUnixFilepath(dl.Config.prefixPath, name))
	// group := strconv.Itoa(gid)
	// owner := strconv.Itoa(uid)
	// _, err := fileURL.SetAccessControl(ctx, azbfs.BlobFSAccessControl{Group: group, Owner: owner})
	// e := storeDatalakeErrToErr(err)
	// if e == ErrFileNotFound {
	// 	return syscall.ENOENT
	// } else if err != nil {
	// 	log.Err("Datalake::ChangeOwner : Failed to change ownership of file %s to %s [%s]", name, mode, err.Error())
	// 	return err
	// }
	return syscall.ENOTSUP
}

// GetCommittedBlockList : Get the list of committed blocks
func (dl *Datalake) GetCommittedBlockList(
	ctx context.Context,
	name string,
) (*internal.CommittedBlockList, error) {
	return dl.BlockBlob.GetCommittedBlockList(ctx, name)
}

// StageBlock : stages a block and returns its blockid
func (dl *Datalake) StageBlock(ctx context.Context, name string, data []byte, id string) error {
	return dl.BlockBlob.StageBlock(ctx, name, data, id)
}

// CommitBlocks : persists the block list
func (dl *Datalake) CommitBlocks(ctx context.Context, name string, blockList []string) error {
	return dl.BlockBlob.CommitBlocks(ctx, name, blockList)
}

// getDirectoryClient returns a new directory url. On Windows this will also convert special characters.
func (dl *Datalake) getDirectoryClient(name string) *directory.Client {
	return dl.Filesystem.NewDirectoryClient(dl.getFormattedPath(name))
}

// getDirectoryClientPathEscape returns a new directory url that is properly escaped. On Windows this will also convert
// special characters.
func (dl *Datalake) getDirectoryClientPathEscape(name string) *directory.Client {
	return dl.Filesystem.NewDirectoryClient(url.PathEscape(dl.getFormattedPath(name)))
}

// getFileClient returns a new file client. On Windows this will also convert special characters.
func (dl *Datalake) getFileClient(name string) *file.Client {
	return dl.Filesystem.NewFileClient(dl.getFormattedPath(name))
}

// getFileClientPathEscape returns a new root directory url that is properly escaped. On Windows this will also convert
// special characters.
func (dl *Datalake) getFileClientPathEscape(name string) *file.Client {
	return dl.Filesystem.NewFileClient(url.PathEscape(dl.getFormattedPath(name)))
}

// getFileName takes a blob name and will convert the special characters into similar unicode characters
// on Windows.
func (dl *Datalake) getFileName(name string) string {
	if runtime.GOOS == "windows" && dl.Config.restrictedCharsWin {
		name = convertname.WindowsCloudToFile(name)
	}
	return name
}

// getFormattedPath takes a file name and converts special characters to the original ASCII
// on Windows and adds the prefixPath.
func (dl *Datalake) getFormattedPath(name string) string {
	name = common.JoinUnixFilepath(dl.Config.prefixPath, name)
	if runtime.GOOS == "windows" && dl.Config.restrictedCharsWin {
		name = convertname.WindowsFileToCloud(name)
	}
	return name
}
