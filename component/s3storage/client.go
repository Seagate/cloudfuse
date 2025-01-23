/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.

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
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/stats_manager"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/middleware"
	awsHttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyHttp "github.com/aws/smithy-go/transport/http"
)

const (
	symlinkKey    = "is_symlink"
	defaultRegion = "us-east-1"
)

type Client struct {
	Connection
	awsS3Client *s3.Client // S3 client library supplied by AWS
	blockLocks  common.KeyedMutex
}

// Verify that Client implements S3Connection interface
var _ S3Connection = &Client{}

// The text before the : symbol is a magic keyword
// It cannot change as it is parsed by our plugin for network optix to provide more clear errors to the user
var (
	errBucketDoesNotExist = errors.New("Bucket Error: S3 bucket does not exist or you do not have permission to access it. Please check your bucket name and endpoint are correct.")
	errInvalidEndpoint    = errors.New("Endpoint Error: Provided S3 endpoint is invalid. Please check endpoint is correct.")
	errInvalidCredential  = errors.New("Credential or Endpoint Error: S3 credentials or endpoint are invalid. Please check your credentials and endpoint are correct.")
	errInvalidSecretKey   = errors.New("Secret Error: S3 secret key is not valid. Please check that the secret key and endpoint are correct.")
	errNoBucketInAccount  = errors.New("Bucket Error: No bucket exists in S3 account. Please create a bucket in your account.")
)

// getSymlinkBool returns true if the symlink flag is set in the metadata map, false otherwise.
func getSymlinkBool(metadata map[string]*string) bool {
	isSymlink := false
	sym, ok := metadata[symlinkKey]
	if ok && sym != nil {
		isSymlink = (*sym == "true")
	}
	return isSymlink
}

// Configure : Initialize the awsS3Client
func (cl *Client) Configure(cfg Config) error {
	log.Trace("Client::Configure : initialize awsS3Client")
	cl.Config = cfg

	var credentialsProvider aws.CredentialsProvider
	credentialsInConfig := cl.Config.authConfig.KeyID != "" && cl.Config.authConfig.SecretKey != ""
	if credentialsInConfig {
		credentialsProvider = credentials.NewStaticCredentialsProvider(
			cl.Config.authConfig.KeyID,
			cl.Config.authConfig.SecretKey,
			"",
		)
	}

	var err error
	if cl.Config.authConfig.Region == "" {
		region, exists := os.LookupEnv("AWS_REGION")
		if !exists {
			cl.Config.authConfig.Region, err = getRegionFromEndpoint(cl.Config.authConfig.Endpoint)
			if err != nil {
				cl.Config.authConfig.Region = defaultRegion
			}
		} else {
			cl.Config.authConfig.Region = region
		}
	}

	if cl.Config.authConfig.Endpoint == "" {
		cl.Config.authConfig.Endpoint = fmt.Sprintf("https://s3.%s.sv15.lyve.seagate.com", cl.Config.authConfig.Region)
	}

	defaultConfig, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithSharedConfigProfile(cl.Config.authConfig.Profile),
		config.WithCredentialsProvider(credentialsProvider),
		config.WithAppID(UserAgent()),
		config.WithRegion(cl.Config.authConfig.Region),
	)

	if err != nil {
		var e config.SharedConfigProfileNotExistError
		if errors.As(err, &e) {
			// If a config profile is provided the sdk checks that it exists, otherwise it fails and
			// does not try other credentials. So try the other ones here if the profile does not exist
			defaultConfig, err = config.LoadDefaultConfig(
				context.Background(),
				config.WithCredentialsProvider(credentialsProvider),
				config.WithAppID(UserAgent()),
				config.WithRegion(cl.Config.authConfig.Region),
			)
		}
		if err != nil {
			log.Err("Client::Configure : config.LoadDefaultConfig() failed. Here's why: %v", err)
			return err
		}
	}

	// Create an Amazon S3 service client
	if cl.Config.usePathStyle {
		cl.awsS3Client = s3.NewFromConfig(defaultConfig, func(o *s3.Options) {
			o.UsePathStyle = true
			o.BaseEndpoint = aws.String(cl.Config.authConfig.Endpoint)
		})
	} else {
		cl.awsS3Client = s3.NewFromConfig(defaultConfig, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cl.Config.authConfig.Endpoint)
		})
	}

	// ListBuckets here to test connection to S3 backend
	bucketList, err := cl.ListBuckets()
	if err != nil {
		log.Err("Client::Configure : listing buckets failed. Here's why: %v", err)

		var oe *smithy.OperationError
		if errors.As(err, &oe) {
			var re *awsHttp.ResponseError
			if errors.As(err, &re) {
				// Endpoint is invalid or could not connect to endpoint
				if re.HTTPStatusCode() == http.StatusMovedPermanently || re.HTTPStatusCode() == 0 {
					log.Err("Client::Configure : Error trying to connect to endpoint. Invalid endpoint. : %v", err)
					return errInvalidEndpoint
				}
			}
		}

		var ae smithy.APIError
		if errors.As(err, &ae) {
			// If error is forbidden, then credentials were incorrect
			if ae.ErrorCode() == "Forbidden" || ae.ErrorCode() == "AccessDenied" {
				return errInvalidCredential
			}
			// If error is forbidden, then access key is correct but secret key is incorrect
			if ae.ErrorCode() == "SignatureDoesNotMatch" {
				return errInvalidSecretKey
			}
			fmt.Println(ae.Error())
		}
		return err
	}

	// if no bucket-name was set, default to the first bucket in the list
	if cl.Config.authConfig.BucketName == "" {
		if len(bucketList) > 0 {
			cl.Config.authConfig.BucketName = bucketList[0]
			log.Warn("Client::Configure : Bucket defaulted to first listed bucket: %s", bucketList[0])
		} else {
			log.Err("Client::Configure : Error no bucket exists in account: %v", err)
			return errNoBucketInAccount
		}
	}

	// Check that the provided bucket exists and that user has access to bucket
	exists, err := cl.bucketExists()
	if err != nil || !exists {
		// From the aws-sdk-go-v2 documentation
		// If the bucket does not exist or you do not have permission to access it,
		// the HEAD request returns a generic 400 Bad Request , 403 Forbidden or 404 Not Found code.
		// We can't use the above information to reliably determine more of the issue
		log.Err("Client::Configure : Error finding bucket. Here's why: %v", err)
		return errBucketDoesNotExist
	}

	// Use list objects validate user can list objects
	_, _, err = cl.List("/", nil, 1)
	if err != nil {
		log.Err("Client::Configure : listing objects failed. Here's why: %v", err)
		return err
	}

	return nil
}

func getRegionFromEndpoint(endpoint string) (string, error) {
	if endpoint == "" {
		return "", errors.New("Endpoint is empty")
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}

	hostParts := strings.Split(u.Hostname(), ".")
	if len(hostParts) < 2 {
		return "", errors.New("Invalid Endpoint")
	}

	// the second host part is usually the region
	regionPartIndex := 1
	// but skip "dualstack"
	if hostParts[1] == "dualstack" {
		regionPartIndex = 2
	}
	// reserve two hostparts after the region for the domain
	if len(hostParts)-regionPartIndex < 3 {
		return "", errors.New("Endpoint does not include a region")
	}

	region := hostParts[regionPartIndex]
	return region, nil
}

// For dynamic configuration, update the config here.
func (cl *Client) UpdateConfig(cfg Config) error {
	cl.Config.partSize = cfg.partSize
	return nil
}

// NewCredentialKey : Update the credential key specified by the user.
// Currently not implemented.
func (cl *Client) NewCredentialKey(key, value string) error {
	log.Trace("Client::NewCredentialKey : not implemented")
	// TODO: research whether and how credentials could change on the same bucket
	// If they can, research whether we can change credentials on an existing client object
	// 	(do we replace the credential provider?)
	return nil
}

// Set the prefix path - this overrides "subdirectory" in config.yaml.
// This is only used for testing.
func (cl *Client) SetPrefixPath(path string) error {
	log.Trace("Client::SetPrefixPath : path %s", path)
	cl.Config.prefixPath = path
	return nil
}

func (cl *Client) bucketExists() (bool, error) {
	_, err := cl.headBucket()
	return err != syscall.ENOENT, err
}

// CreateFile : Create a new file in the bucket/virtual directory
func (cl *Client) CreateFile(name string, mode os.FileMode) error {
	log.Trace("Client::CreateFile : name %s", name)
	var data []byte
	return cl.WriteFromBuffer(name, nil, data)
}

// CreateDirectory : Create a new directory in the bucket/virtual directory
func (cl *Client) CreateDirectory(name string) error {
	log.Trace("Client::CreateDirectory : name %s", name)
	// MinIO does not support creating an empty file to indicate a directory
	// directories will be represented only as object prefixes
	// we have no way of representing an empty directory, so do nothing.
	// Note: we could try to list the directory and return EEXIST if it has contents,
	// but that would be a performance penalty for a check that the OS already does.
	// So, let's make it clear: we expect the OS to call GetAttr() on the directory
	// to make sure it doesn't exist before trying to create it.
	return nil
}

// CreateLink : Create a symlink in the bucket/virtual directory
func (cl *Client) CreateLink(source string, target string, isSymlink bool) error {
	log.Trace("Client::CreateLink : %s -> %s", source, target)
	data := []byte(target)

	symlinkMap := map[string]*string{symlinkKey: to.Ptr("false")}
	if isSymlink {
		symlinkMap[symlinkKey] = to.Ptr("true")
	}
	return cl.WriteFromBuffer(source, symlinkMap, data)
}

// DeleteFile : Delete an object.
// if the file does not exist, this returns an error (ENOENT).
func (cl *Client) DeleteFile(name string) error {
	log.Trace("Client::DeleteFile : name %s", name)
	// first check if the object exists
	attr, err := cl.getFileAttr(name)
	if err == syscall.ENOENT {
		log.Err("Client::DeleteFile : %s does not exist", name)
		return syscall.ENOENT
	} else if err != nil {
		log.Err("Client::DeleteFile : Failed to getFileAttr for object %s. Here's why: %v", name, err)
		return err
	}

	isSymLink := attr.IsSymlink()

	// delete the object
	err = cl.deleteObject(name, isSymLink)
	if err != nil {
		log.Err("Client::DeleteFile : Failed to delete object %s. Here's why: %v", name, err)
		return err
	}

	return nil
}

// DeleteDirectory : Recursively delete all objects with the given prefix.
// If name is given without a trailing slash, a slash will be added.
// If the directory does not exist, no error will be returned.
func (cl *Client) DeleteDirectory(name string) error {
	log.Trace("Client::DeleteDirectory : name %s", name)

	// make sure name has a trailing slash
	name = internal.ExtendDirName(name)

	done := false
	var marker *string
	var err error
	for !done {

		// list all objects with the prefix
		objects, marker, err := cl.List(name, marker, 0)
		if err != nil {
			log.Warn("Client::DeleteDirectory : Failed to list object with prefix %s. Here's why: %v", name, err)
			return err
		}

		// we have no way of indicating empty folders in the bucket
		// so if there are no objects with this prefix we can either:
		// 1. return an error when the user tries to delete an empty directory, or
		// 2. fail to return an error when trying to delete a non-existent directory
		// the second one seems much less risky, so we don't check for an empty list here

		// List only returns the objects and prefixes up to the next "/" character after the prefix
		// This is because List is setting the Delimiter field to "/"
		// This means that recursive directory deletion actually needs to be recursive.
		// Delete all found objects *and prefixes ("directories")*.
		// For improved performance, we'll use one call to delete all objects in this directory.
		// 	To make one call, we need to make a list of objects to delete first.
		var objectsToDelete []*internal.ObjAttr
		for _, object := range objects {
			if object.IsDir() {
				err = cl.DeleteDirectory(object.Path)
				if err != nil {
					log.Err("Client::DeleteDirectory : Failed to delete directory %s. Here's why: %v", object.Path, err)
				}
			} else {
				objectsToDelete = append(objectsToDelete, object) //consider just object instead of object.path to pass down attributes that come from list.
			}
		}
		// Delete the collected files
		err = cl.deleteObjects(objectsToDelete)
		if err != nil {
			log.Err("Client::DeleteDirectory : deleteObjects() failed when called with %d objects. Here's why: %v", len(objectsToDelete), err)
		}

		if marker == nil {
			done = true
		}

	}

	return err
}

// RenameFile : Rename the object (copy then delete).
func (cl *Client) RenameFile(source string, target string, isSymLink bool) error {
	log.Trace("Client::RenameFile : %s -> %s", source, target)

	err := cl.copyObject(source, target, isSymLink)
	if err != nil {
		log.Err("Client::RenameFile : copyObject(%s->%s) failed. Here's why: %v", source, target, err)
		return err
	}
	// Copy of the file is done so now delete the older file
	// in this case we don't need to check if the file exists, so we use deleteObject, not DeleteFile
	// this is what S3's DeleteObject spec is meant for: to make sure the object doesn't exist anymore
	err = cl.deleteObject(source, isSymLink)
	if err != nil {
		log.Err("Client::RenameFile : deleteObject(%s) failed. Here's why: %v", source, err)
	}

	return err
}

// RenameDirectory : Rename the directory
func (cl *Client) RenameDirectory(source string, target string) error {
	log.Trace("Client::RenameDirectory : %s -> %s", source, target)

	// TODO: should this fail when the target directory exists?
	// current behavior merges into the target directory
	// best to check and see what the azstorage code does

	// first we need a list of all the object's we'll be moving
	// make sure to pass source with a trailing forward slash

	done := false
	var marker *string

	for !done {
		sourceObjects, marker, err := cl.List(internal.ExtendDirName(source), marker, 0)
		if err != nil {
			log.Err("Client::RenameDirectory : Failed to list objects with prefix %s. Here's why: %v", source, err)
			return err
		}
		// it's better not to return an error when we don't find any matching objects (see note in DeleteDirectory)
		for _, srcObject := range sourceObjects {
			srcPath := srcObject.Path
			dstPath := strings.Replace(srcPath, source, target, 1)
			if srcObject.IsDir() {
				err = cl.RenameDirectory(srcPath, dstPath)
			} else {
				err = cl.RenameFile(srcPath, dstPath, srcObject.IsSymlink()) //use sourceObjects to pass along symLink bool
			}
			if err != nil {
				log.Err("Client::RenameDirectory : Failed to rename %s -> %s. Here's why: %v", srcPath, dstPath, err)
			}
		}
		if marker == nil {
			done = true
		}
	}
	return nil
}

// GetAttr : Get attributes for a given file or folder.
// If name is a file, it should not have a trailing slash.
// If name is a directory, the trailing slash is optional.
func (cl *Client) GetAttr(name string) (*internal.ObjAttr, error) {
	log.Trace("Client::GetAttr : name %s", name)

	// first let's suppose the caller is looking for a file
	// there are no objects with trailing slashes (MinIO doesn't support them)
	// 	and trailing slashes aren't allowed in filenames
	// so if this was called with a trailing slash, don't look for an object
	if len(name) > 0 && name[len(name)-1] != '/' {
		attr, err := cl.getFileAttr(name)
		if err == nil {
			return attr, err
		}
		if err != syscall.ENOENT {
			log.Err("Client::GetAttr : Failed to getFileAttr(%s). Here's why: %v", name, err)
		}
	}

	// ensure a trailing slash
	dirName := internal.ExtendDirName(name)
	// now search for that as a directory
	return cl.getDirectoryAttr(dirName)
}

// Get attributes for the given file path.
// Return ENOENT if there is no corresponding object in the bucket.
// name should not have a trailing slash (or nothing will be found!).
func (cl *Client) getFileAttr(name string) (*internal.ObjAttr, error) {
	log.Trace("Client::getFileAttr : name %s", name)
	isSymlink := false
	object, err := cl.headObject(name, isSymlink)
	if err == syscall.ENOENT && !cl.Config.disableSymlink {
		isSymlink = true
		return cl.headObject(name, isSymlink)
	}
	return object, err
}

func (cl *Client) getDirectoryAttr(dirName string) (*internal.ObjAttr, error) {
	log.Trace("Client::getDirectoryAttr : name %s", dirName)

	// to do this, accept anything that comes back from List()
	objects, _, err := cl.List(dirName, nil, 1)
	if err != nil {
		log.Err("Client::getDirectoryAttr : List(%s) failed. Here's why: %v", dirName, err)
		return nil, err
	} else if len(objects) > 0 {
		// create and return an objAttr for the directory
		attr := internal.CreateObjAttrDir(dirName)
		return attr, nil
	}

	// directory not found in bucket
	log.Err("Client::getDirectoryAttr : not found: %s", dirName)
	return nil, syscall.ENOENT
}

// Download object data to a file handle.
// Read starting at a byte offset from the start of the object, with length in bytes = count.
// count = 0 reads to the end of the object.
func (cl *Client) ReadToFile(name string, offset int64, count int64, fi *os.File) error {
	log.Trace("Client::ReadToFile : name %s, offset : %d, count %d -> file %s", name, offset, count, fi.Name())

	// If we are reading the entire object, then we can use a multipart download
	if !cl.Config.disableConcurrentDownload && offset == 0 && count == 0 {
		err := cl.getObjectMultipartDownload(name, fi)
		if err != nil {
			log.Err("Client::ReadToFile : getObjectMultipartDownload(%s) failed. Here's why: %v", name, err)
			return err
		}
		return nil
	}

	// get object data
	objectDataReader, err := cl.getObject(name, offset, count, false)
	if err != nil {
		log.Err("Client::ReadToFile : getObject(%s) failed. Here's why: %v", name, err)
		return err
	}
	// read object data
	defer objectDataReader.Close()
	objectData, err := io.ReadAll(objectDataReader)
	if err != nil {
		log.Err("Couldn't read object data from %v. Here's why: %v", name, err)
		return err
	}
	// write data to file
	_, err = fi.Write(objectData)
	if err != nil {
		log.Err("Couldn't write to file %v. Here's why: %v", name, err)
		return err
	}

	return err
}

// Download object with the given name and return the data as a byte array.
// Reads starting at a byte offset from the start of the object, with length in bytes = len.
// len = 0 reads to the end of the object.
// name is the file path
func (cl *Client) ReadBuffer(name string, offset int64, length int64, isSymlink bool) ([]byte, error) {
	log.Trace("Client::ReadBuffer : name %s (%d+%d)", name, offset, length)
	// get object data
	objectDataReader, err := cl.getObject(name, offset, length, isSymlink)
	if err != nil {
		log.Err("Client::ReadBuffer : getObject(%s) failed. Here's why: %v", name, err)
		return nil, err
	}
	// read object data
	defer objectDataReader.Close()
	buff, err := io.ReadAll(objectDataReader)
	if err != nil {
		log.Err("Client::ReadBuffer : Failed to read data from GetObject result. Here's why: %v", err)
		return nil, err
	}

	return buff, nil
}

// Download object to provided byte array.
// Reads starting at a byte offset from the start of the object, with length in bytes = len.
// len = 0 reads to the end of the object.
// name is the file path.
func (cl *Client) ReadInBuffer(name string, offset int64, length int64, data []byte) error {
	log.Trace("Client::ReadInBuffer : name %s offset %d len %d", name, offset, length)
	// get object data
	objectDataReader, err := cl.getObject(name, offset, length, false)
	if err != nil {
		log.Err("Client::ReadInBuffer : getObject(%s) failed. Here's why: %v", name, err)
		return err
	}
	// read object data
	defer objectDataReader.Close()
	_, err = io.ReadFull(objectDataReader, data)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		// If we reached the EOF then all the data was correctly read
		return nil
	}

	return err
}

// Upload from a file handle to an object.
// The metadata parameter is not used.
func (cl *Client) WriteFromFile(name string, metadata map[string]*string, fi *os.File) error {
	isSymlink := getSymlinkBool(metadata)

	log.Trace("Client::WriteFromFile : file %s -> name %s", fi.Name(), name)
	// track time for performance testing
	defer log.TimeTrack(time.Now(), "Client::WriteFromFile", name)
	// get the size of the file
	stat, err := fi.Stat()
	if err != nil {
		log.Err("Client::WriteFromFile : Failed to get file size %s. Here's why: %v", name, err)
		return err
	}
	// The aws-sdk-go-v2 does not seem to see to the end of the file
	// so let's seek to the start before uploading
	// TODO: Is there a more elegant way to do this?
	_, err = fi.Seek(0, 0)
	if err != nil {
		log.Err("Client::WriteFromFile : Failed to seek to beginning of input file %s", fi.Name())
		return err
	}

	// upload file data
	err = cl.putObject(name, fi, stat.Size(), isSymlink)
	if err != nil {
		log.Err("Client::WriteFromFile : putObject(%s) failed. Here's why: %v", name, err)
		return err
	}

	// TODO: Add monitor tracking
	// if common.MonitorCfs() && stat.Size() > 0 {
	// 	uploadOptions.Progress = func(bytesTransferred int64) {
	// 		trackUpload(name, bytesTransferred, stat.Size(), uploadPtr)
	// 	}
	// }
	log.Debug("Client::WriteFromFile : Upload complete of object %v", name)

	// store total bytes uploaded so far
	if stat.Size() > 0 {
		s3StatsCollector.UpdateStats(stats_manager.Increment, bytesUploaded, stat.Size())
	}

	return nil
}

// WriteFromBuffer : Upload from a buffer to an object.
// name is the file path.
func (cl *Client) WriteFromBuffer(name string, metadata map[string]*string, data []byte) error {
	log.Trace("Client::WriteFromBuffer : name %s", name)
	isSymlink := getSymlinkBool(metadata)

	// convert byte array to io.Reader
	dataReader := bytes.NewReader(data)
	// upload data to object
	// TODO: handle metadata with S3
	err := cl.putObject(name, dataReader, int64(len(data)), isSymlink)
	if err != nil {
		log.Err("Client::WriteFromBuffer : putObject(%s) failed. Here's why: %v", name, err)
	}
	return err
}

// GetFileBlockOffsets: store blocks ids and corresponding offsets.
func (cl *Client) GetFileBlockOffsets(name string) (*common.BlockOffsetList, error) {
	log.Trace("Client::GetFileBlockOffsets : name %s", name)
	blockList := common.BlockOffsetList{}
	result, err := cl.headObject(name, false)
	if err != nil {
		log.Err("Client::GetFileBlockOffsets : Unable to headObject with name %v", name)
		return &blockList, err
	}

	cutoff := cl.Config.uploadCutoff
	var objectSize int64

	// if file is smaller than the uploadCutoff it is small, otherwise it is a multipart
	// upload
	if result.Size < cutoff {
		blockList.Flags.Set(common.SmallFile)
		return &blockList, nil
	}

	partSize := cl.Config.partSize

	// Create a list of blocks that are the partSize except for the last block
	for objectSize <= result.Size {
		if objectSize+partSize >= result.Size {
			// This is the last block to add
			blk := &common.Block{
				Id:         base64.StdEncoding.EncodeToString(common.NewUUID().Bytes()),
				StartIndex: objectSize,
				EndIndex:   result.Size,
			}
			blockList.BlockList = append(blockList.BlockList, blk)
			break
		}

		blk := &common.Block{
			Id:         base64.StdEncoding.EncodeToString(common.NewUUID().Bytes()),
			StartIndex: objectSize,
			EndIndex:   objectSize + partSize,
		}
		blockList.BlockList = append(blockList.BlockList, blk)
		objectSize += partSize
	}
	blockList.BlockIdLength = common.GetIdLength(blockList.BlockList[0].Id)

	return &blockList, nil
}

// Truncate object to size in bytes.
// name is the file path.
func (cl *Client) TruncateFile(name string, size int64) error {
	log.Trace("Client::TruncateFile : Truncating %s to %dB.", name, size)

	// get object data
	objectDataReader, err := cl.getObject(name, 0, 0, false)
	if err != nil {
		log.Err("Client::TruncateFile : getObject(%s) failed. Here's why: %v", name, err)
		return err
	}
	// read object data
	defer objectDataReader.Close()
	objectData, err := io.ReadAll(objectDataReader)
	if err != nil {
		log.Err("Client::TruncateFile : Failed to read object data from %v. Here's why: %v", name, err)
		return err
	}
	// ensure data is of the expected length
	if int64(len(objectData)) > size {
		// truncate
		objectData = objectData[:size]
	} else if int64(len(objectData)) < size {
		// pad the data with zeros
		log.Warn("Client::TruncateFile : Padding file %s with zeros to truncate its original size (%dB) UP to %dB.", name, len(objectData), size)
		oldObjectData := objectData
		newObjectData := make([]byte, size)
		copy(newObjectData, oldObjectData)
		objectData = newObjectData
	}
	// overwrite the object with the truncated data
	truncatedDataReader := bytes.NewReader(objectData)
	err = cl.putObject(name, truncatedDataReader, int64(len(objectData)), false)
	if err != nil {
		log.Err("Client::TruncateFile : Failed to write truncated data to object %s", name)
	}

	return err
}

// Write : write data at given offset to an object
func (cl *Client) Write(options internal.WriteFileOptions) error {
	name := options.Handle.Path
	offset := options.Offset
	data := options.Data
	length := int64(len(data))

	defer log.TimeTrack(time.Now(), "Client::Write", options.Handle.Path)
	log.Trace("Client::Write : name %s offset %v", name, offset)
	// tracks the case where our offset is great than our current file size (appending only - not modifying pre-existing data)
	var dataBuffer *[]byte

	fileOffsets, err := cl.GetFileBlockOffsets(name)
	if err != nil {
		return err
	}

	if fileOffsets.SmallFile() {
		// case 1: file consists of no parts (small file)

		// get the existing object data
		isSymlink := getSymlinkBool(options.Metadata)

		oldData, _ := cl.ReadBuffer(name, 0, 0, isSymlink)
		// update the data with the new data
		// if we're only overwriting existing data
		if int64(len(oldData)) >= offset+length {
			copy(oldData[offset:], data)
			dataBuffer = &oldData
			// else appending and/or overwriting
		} else {
			// if the file is not empty then we need to combine the data
			if len(oldData) > 0 {
				// new data buffer with the size of old and new data
				newDataBuffer := make([]byte, offset+length)
				// copy the old data into it
				// TODO: better way to do this?
				if offset != 0 {
					copy(newDataBuffer, oldData)
					oldData = nil
				}
				// overwrite with the new data we want to add
				copy(newDataBuffer[offset:], data)
				dataBuffer = &newDataBuffer
			} else {
				dataBuffer = &data
			}
		}

		// WriteFromBuffer should be able to handle the case where now the block is too big and gets split into multiple parts
		err := cl.WriteFromBuffer(name, options.Metadata, *dataBuffer)
		if err != nil {
			log.Err("Client::Write : Failed to upload to object. Here's why: %v ", name, err)
			return err
		}
	} else {
		// case 2: given offset is within the size of the object - and the object consists of multiple parts
		// case 3: new parts need to be added

		index, oldDataSize, exceedsFileBlocks, appendOnly := fileOffsets.FindBlocksToModify(offset, length)
		// keeps track of how much new data will be appended to the end of the file (applicable only to case 3)
		newBufferSize := int64(0)
		// case 3?
		if exceedsFileBlocks {
			newBufferSize = cl.createNewBlocks(fileOffsets, offset, length)
		}
		// buffer that holds that pre-existing data in those blocks we're interested in
		oldDataBuffer := make([]byte, oldDataSize+newBufferSize)
		if !appendOnly {
			// fetch the parts that will be impacted by the new changes so we can overwrite them
			err = cl.ReadInBuffer(name, fileOffsets.BlockList[index].StartIndex, oldDataSize, oldDataBuffer)
			if err != nil {
				log.Err("BlockBlob::Write : Failed to read data in buffer %s [%s]", name, err.Error())
			}
		}
		// this gives us where the offset with respect to the buffer that holds our old data - so we can start writing the new data
		blockOffset := offset - fileOffsets.BlockList[index].StartIndex
		copy(oldDataBuffer[blockOffset:], data)
		err := cl.stageAndCommitModifiedBlocks(name, oldDataBuffer, fileOffsets)
		return err
	}

	return nil
}

func (cl *Client) createBlock(blockIdLength, startIndex, size int64) *common.Block {
	newBlockId := base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(blockIdLength))
	newBlock := &common.Block{
		Id:         newBlockId,
		StartIndex: startIndex,
		EndIndex:   startIndex + size,
	}
	// mark truncated since it is a new empty block
	newBlock.Flags.Set(common.TruncatedBlock)
	newBlock.Flags.Set(common.DirtyBlock)
	return newBlock
}

func (cl *Client) createNewBlocks(blockList *common.BlockOffsetList, offset, length int64) int64 {
	partSize := cl.Config.partSize
	prevIndex := blockList.BlockList[len(blockList.BlockList)-1].EndIndex
	if partSize == 0 {
		partSize = DefaultPartSize
	}
	// BufferSize is the size of the buffer that will go beyond our current object
	var bufferSize int64
	for i := prevIndex; i < offset+length; i += partSize {
		blkSize := int64(math.Min(float64(partSize), float64((offset+length)-i)))
		newBlock := cl.createBlock(blockList.BlockIdLength, i, blkSize)
		blockList.BlockList = append(blockList.BlockList, newBlock)
		// reset the counter to determine if there are leftovers at the end
		bufferSize += blkSize
	}
	return bufferSize
}

func (cl *Client) stageAndCommitModifiedBlocks(name string, data []byte, offsetList *common.BlockOffsetList) error {
	blockOffset := int64(0)
	for _, blk := range offsetList.BlockList {
		if blk.Dirty() {
			blk.Data = data[blockOffset : (blk.EndIndex-blk.StartIndex)+blockOffset]
			blockOffset = (blk.EndIndex - blk.StartIndex) + blockOffset
			// Clear the truncated flag if we are writing data to this block
			if blk.Truncated() {
				blk.Flags.Clear(common.TruncatedBlock)
			}
		}
	}

	return cl.StageAndCommit(name, offsetList)
}

func (cl *Client) StageAndCommit(name string, bol *common.BlockOffsetList) error {
	// lock on the object name so that no stage and commit race condition occur causing failure
	objectMtx := cl.blockLocks.GetLock(name)
	objectMtx.Lock()
	defer objectMtx.Unlock()

	// Return early if blocklist is empty
	if len(bol.BlockList) == 0 {
		return nil
	}

	// Return early if there are no dirty blocks
	staged := false
	for _, blk := range bol.BlockList {
		if blk.Dirty() {
			staged = true
			break
		}
	}
	if !staged {
		return nil
	}

	// For loop to determine if interior blocks are too small for AWS multipart upload and we need to change
	// the size of interior blocks
	combineBlocks := false
	for i, blk := range bol.BlockList {
		// If an interior block is small and not the last block, then we cannot upload using multipart upload
		// so we need to combine blocks
		if len(blk.Data) < 5*common.MbToBytes && i < len(bol.BlockList)-1 {
			combineBlocks = true
			break
		}
	}

	var err error
	if combineBlocks {
		bol.BlockList, err = cl.combineSmallBlocks(name, bol.BlockList)
		if err != nil {
			log.Err("Client::StageAndCommit : Failed to combine small blocks: %v ", name, err)
			return err
		}
	}

	//struct for starting a multipart upload
	ctx := context.Background()
	key := cl.getKey(name, false)

	//send command to start copy and get the upload id as it is needed later
	var uploadID string
	createMultipartUploadInput := &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(cl.Config.authConfig.BucketName),
		Key:         aws.String(key),
		ContentType: aws.String(getContentType(key)),
	}

	if cl.Config.enableChecksum {
		createMultipartUploadInput.ChecksumAlgorithm = cl.Config.checksumAlgorithm
	}

	createOutput, err := cl.awsS3Client.CreateMultipartUpload(ctx, createMultipartUploadInput)
	if err != nil {
		log.Err("Client::StageAndCommit : Failed to create multipart upload. Here's why: %v ", name, err)
		return err
	}
	if createOutput != nil {
		if createOutput.UploadId != nil {
			uploadID = *createOutput.UploadId
		}
	}
	if uploadID == "" {
		log.Err("Client::StageAndCommit : No upload id found in start upload request. Here's why: %v ", name, err)
		return err
	}

	var partNumber int32 = 1
	parts := make([]types.CompletedPart, 0)
	var data []byte

	for _, blk := range bol.BlockList {
		if blk.Truncated() {
			data = make([]byte, blk.EndIndex-blk.StartIndex)
			blk.Flags.Clear(common.TruncatedBlock)
		} else {
			data = blk.Data
		}

		var err error
		var eTag *string
		var checksumCRC32 *string
		var checksumCRC32C *string
		var checksumSHA256 *string
		var checksumSHA1 *string
		if blk.Dirty() || len(data) > 0 {
			// This block has data that is not yet in the bucket
			uploadPartInput := &s3.UploadPartInput{
				Bucket:     aws.String(cl.Config.authConfig.BucketName),
				Key:        aws.String(key),
				PartNumber: &partNumber,
				UploadId:   &uploadID,
				Body:       bytes.NewReader(data),
			}

			if cl.Config.enableChecksum {
				uploadPartInput.ChecksumAlgorithm = cl.Config.checksumAlgorithm
			}

			var partResp *s3.UploadPartOutput
			partResp, err = cl.awsS3Client.UploadPart(ctx, uploadPartInput)
			eTag = partResp.ETag
			blk.Flags.Clear(common.DirtyBlock)

			// Collect the checksums
			// It is easier to just collect all checksums and then upload them together
			// as ones that are not used will just be nil and an object can only ever
			// have one valid checksum
			checksumCRC32 = partResp.ChecksumCRC32
			checksumCRC32C = partResp.ChecksumCRC32C
			checksumSHA1 = partResp.ChecksumSHA1
			checksumSHA256 = partResp.ChecksumSHA256
		} else {
			// This block is already in the bucket, so we need to copy this part
			var partResp *s3.UploadPartCopyOutput
			partResp, err = cl.awsS3Client.UploadPartCopy(ctx, &s3.UploadPartCopyInput{
				Bucket:          aws.String(cl.Config.authConfig.BucketName),
				Key:             aws.String(key),
				CopySource:      aws.String(fmt.Sprintf("%v/%v", cl.Config.authConfig.BucketName, key)),
				CopySourceRange: aws.String("bytes=" + fmt.Sprint(blk.StartIndex) + "-" + fmt.Sprint(blk.EndIndex-1)),
				PartNumber:      &partNumber,
				UploadId:        &uploadID,
			})
			eTag = partResp.CopyPartResult.ETag

			// Collect the checksums
			// It is easier to just collect all checksums and then upload them together
			// as ones that are not used will just be nil and an object can only ever
			// have one valid checksum
			checksumCRC32 = partResp.CopyPartResult.ChecksumCRC32
			checksumCRC32C = partResp.CopyPartResult.ChecksumCRC32C
			checksumSHA1 = partResp.CopyPartResult.ChecksumSHA1
			checksumSHA256 = partResp.CopyPartResult.ChecksumSHA256
		}

		if err != nil {
			log.Info("Client::StageAndCommit : Attempting to abort upload due to error: ", err.Error())
			abortErr := cl.abortMultipartUpload(key, uploadID)
			return errors.Join(err, abortErr)
		}

		// copy etag and part number to verify later
		if eTag != nil {
			partNum := partNumber
			etag := strings.Trim(*eTag, "\"")
			cPart := types.CompletedPart{
				ETag:       &etag,
				PartNumber: &partNum,
			}
			if cl.Config.enableChecksum {
				cPart.ChecksumCRC32 = checksumCRC32
				cPart.ChecksumCRC32C = checksumCRC32C
				cPart.ChecksumSHA1 = checksumSHA1
				cPart.ChecksumSHA256 = checksumSHA256
			}
			parts = append(parts, cPart)
		}

		partNumber++
	}

	// complete the upload
	_, err = cl.awsS3Client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(cl.Config.authConfig.BucketName),
		Key:      aws.String(key),
		UploadId: &uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: parts,
		},
	})
	if err != nil {
		log.Info("Client::StageAndCommit : Attempting to abort upload due to error: ", err.Error())
		abortErr := cl.abortMultipartUpload(key, uploadID)
		return errors.Join(err, abortErr)
	}

	return nil
}

// combineSmallBlocks will combine blocks in a blocklist, except for the last block, if the block is smaller
// than the smallest size for a part in AWS, which is 5 MB. Blocks smaller than 5MB will be combined with the
// next block in the list.
func (cl *Client) combineSmallBlocks(name string, blockList []*common.Block) ([]*common.Block, error) {
	newBlockList := []*common.Block{}
	newBlockList = append(newBlockList, &common.Block{})
	addIndex := 0
	beginNewBlock := true
	for i, blk := range blockList {
		if beginNewBlock && blk.EndIndex-blk.StartIndex >= 5*common.MbToBytes {
			// This block is large enough so we copy the whole block
			newBlockList[addIndex] = blk
		} else {
			// We have a small block and need to keep adding data to the block
			if beginNewBlock {
				newBlockList[addIndex].StartIndex = blk.StartIndex
				newBlockList[addIndex].Flags.Set(common.DirtyBlock)
				newBlockList[addIndex].Id = blk.Id
				beginNewBlock = false
			}

			var addData []byte
			// If there is no data in the block and it is not truncated, we need to get it from the cloud. Otherwise we can just copy it.
			if len(blk.Data) == 0 && !blk.Truncated() {
				result, err := cl.getObject(name, blk.StartIndex, blk.EndIndex-blk.StartIndex, false)
				if err != nil {
					log.Err("Client::combineSmallBlocks : Unable to get object with error: ", err.Error())
					return nil, err
				}

				defer result.Close()
				addData, err = io.ReadAll(result)
				if err != nil {
					log.Err("Client::combineSmallBlocks : Unable to read bytes from object with error: ", err.Error())
					return nil, err
				}
			} else {
				addData = blk.Data
			}

			// Combine these two blocks
			newBlockList[addIndex].Data = append(newBlockList[addIndex].Data, addData...)
			newBlockList[addIndex].EndIndex = blk.EndIndex
		}

		// If our current block is large enough and it is not the last block
		if newBlockList[addIndex].EndIndex-newBlockList[addIndex].StartIndex >= 5*common.MbToBytes && i < len(blockList)-1 {
			beginNewBlock = true
			newBlockList = append(newBlockList, &common.Block{})
			addIndex++
		}
	}
	return newBlockList, nil
}

func (cl *Client) GetUsedSize() (uint64, error) {
	headBucketOutput, err := cl.headBucket()
	if err != nil {
		return 0, err
	}

	response, ok := middleware.GetRawResponse(headBucketOutput.ResultMetadata).(*smithyHttp.Response)
	if !ok || response == nil {
		return 0, fmt.Errorf("Failed GetRawResponse from HeadBucketOutput")
	}

	headerValue, ok := response.Header["X-Rstor-Size"]
	if !ok {
		headerValue, ok = response.Header["X-Lyve-Size"]
	}
	if !ok || len(headerValue) == 0 {
		return 0, fmt.Errorf("HeadBucket response has no size header (is the endpoint not Lyve Cloud?)")
	}

	bucketSizeBytes, err := strconv.ParseUint(headerValue[0], 10, 64)
	if err != nil {
		return 0, err
	}

	return bucketSizeBytes, nil
}
