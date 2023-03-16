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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"lyvecloudfuse/common"
	"lyvecloudfuse/common/log"
	"lyvecloudfuse/internal"
	"lyvecloudfuse/internal/stats_manager"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

const (
	folderKey  = "hdi_isfolder"
	symlinkKey = "is_symlink"
	// how many times should we retry each call to Lyve Cloud to circumvent the InvalidAccessKeyId issue?
	// TODO: set this to 1 to test whether this issue is fixed
	retryCount = 5
)

type Client struct {
	S3StorageConnection
	awsS3Client *s3.Client // S3 client library supplied by AWS
}

// Verify that Client implements S3Connection interface
var _ S3Connection = &Client{}

func (cl *Client) Configure(cfg S3StorageConfig) error {
	cl.Config = cfg

	// Set the endpoint supplied in the config file
	// TODO: handle the case that the config does not have an endpoint (use Lyve Cloud as default)
	// TODO: handle it when the config does not have a Region (use "us-east-1" as default)
	endpointResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if service == s3.ServiceID {
			return aws.Endpoint{
				PartitionID:   "aws",
				URL:           cl.Config.authConfig.Endpoint,
				SigningRegion: cl.Config.authConfig.Region,
			}, nil
		}
		return aws.Endpoint{}, fmt.Errorf("unknown endpoint requested")
	})

	staticProvider := credentials.NewStaticCredentialsProvider(
		cl.Config.authConfig.AccessKey,
		cl.Config.authConfig.SecretKey,
		"",
	)
	defaultConfig, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithCredentialsProvider(staticProvider),
		config.WithEndpointResolverWithOptions(endpointResolver),
	)
	if err != nil {
		return err
	}

	// Create an Amazon S3 service client
	cl.awsS3Client = s3.NewFromConfig(defaultConfig)

	return nil
}

// For dynamic config update the config here
func (cl *Client) UpdateConfig(cfg S3StorageConfig) error {
	cl.Config.blockSize = cfg.blockSize
	return nil
}

// NewCredentialKey : Update the credential key specified by the user
func (cl *Client) NewCredentialKey(key, value string) (err error) {
	// TODO: research whether and how credentials could change on the same bucket
	// If they can, research whether we can change credentials on an existing client object
	// 	(do we replace the credential provider?)
	return nil
}

// Wrapper function of the GetObject S3 calls.
func (cl *Client) getObject(name string, offset int64, count int64) (*s3.GetObjectOutput, error) {
	log.Trace("Client::getObject : getting object %s (%d+%d)", name, offset, count)

	var rangeString string //string to be used to specify range of object to download from S3
	bucketName := cl.Config.authConfig.BucketName

	//TODO: add handle if the offset+count is greater than the end of Object.
	if count == 0 {
		// if offset is 0 too, leave rangeString empty
		if offset != 0 {
			rangeString = "bytes=" + fmt.Sprint(offset) + "-"
		}
	} else {
		endRange := offset + count
		rangeString = "bytes=" + fmt.Sprint(offset) + "-" + fmt.Sprint(endRange)
	}

	var result *s3.GetObjectOutput
	var err error
	for i := 0; i < retryCount; i++ {
		result, err = cl.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(name),
			Range:  aws.String(rangeString),
		})
		// retry on InvalidAccessKeyId
		if isInvalidAccessKeyID(err) {
			log.Warn("Client::getObject Lyve Cloud \"Invalid Access Key\" bug - retry %d of %d.", i+1, retryCount)
		} else {
			break
		}
	}
	return result, err
}

// Wrapper function for awsS3Client.PutObject
func (cl *Client) putObject(name string, objectData io.Reader) (*s3.PutObjectOutput, error) {
	log.Trace("Client::putObject : putting object %s", name)
	var result *s3.PutObjectOutput
	var err error
	for i := 0; i < retryCount; i++ {
		result, err = cl.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(cl.Config.authConfig.BucketName),
			Key:    aws.String(name),
			Body:   objectData,
		})
		// retry on InvalidAccessKeyId
		if isInvalidAccessKeyID(err) {
			log.Warn("Client::putObject Lyve Cloud \"Invalid Access Key\" bug - retry %d of %d.", i+1, retryCount)
		} else {
			break
		}
	}
	return result, err
}

// Wrapper function for awsS3Client.PutObject
func (cl *Client) deleteObject(name string) (*s3.DeleteObjectOutput, error) {
	log.Trace("Client::deleteObject : deleting object %s", name)
	var result *s3.DeleteObjectOutput
	var err error
	for i := 0; i < retryCount; i++ {
		result, err = cl.awsS3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
			Bucket: aws.String(cl.Config.authConfig.BucketName),
			Key:    aws.String(name),
		})
		// retry on InvalidAccessKeyId
		if isInvalidAccessKeyID(err) {
			log.Warn("Client::deleteObject Lyve Cloud \"Invalid Access Key\" bug - retry %d of %d.", i+1, retryCount)
		} else {
			break
		}
	}
	return result, err
}

// Lyve Cloud sometimes returns InvalidAccessKey even if the access key is correct and the request was correct
// Check the returned err value for the associated error code
func isInvalidAccessKeyID(err error) bool {
	var apiErr smithy.APIError
	if err != nil {
		if errors.As(err, &apiErr) {
			code := apiErr.ErrorCode()
			if code == "InvalidAccessKeyId" {
				return true
			}
		}
	}
	return false
}

func (cl *Client) ListContainers() ([]string, error) {
	log.Trace("Client::ListContainers : Listing containers")

	cntList := make([]string, 0)

	var err error
	var result *s3.ListBucketsOutput
	for i := 0; i < retryCount; i++ {
		result, err = cl.awsS3Client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
		// retry on InvalidAccessKeyId
		if isInvalidAccessKeyID(err) {
			log.Warn("Client::ListContainers Lyve Cloud \"Invalid Access Key\" bug - retry %d of %d.", i+1, retryCount)
		} else {
			break
		}
	}

	if err != nil {
		fmt.Printf("Couldn't list buckets for your account. Here's why: %v\n", err)
		return cntList, err
	}

	for _, bucket := range result.Buckets {
		cntList = append(cntList, *bucket.Name)
	}

	return cntList, nil
}

func (cl *Client) SetPrefixPath(path string) error {
	log.Trace("Client::SetPrefixPath : path %s", path)
	cl.Config.prefixPath = path
	return nil
}

// CreateFile : Create a new file in the container/virtual directory
func (cl *Client) CreateFile(name string, mode os.FileMode) error {
	log.Trace("Client::CreateFile : name %s", name)
	var data []byte
	return cl.WriteFromBuffer(name, nil, data)
}

// CreateDirectory : Create a new directory in the container/virtual directory
func (cl *Client) CreateDirectory(name string) error {
	log.Trace("Client::CreateDirectory : name %s", name)
	// Lyve Cloud does not support creating an empty file to indicate a directory
	// directories will be represented only as object prefixes
	// we have no way of representing an empty directory, so do nothing
	// TODO: research: is this supposed to throw an error if the directory already exists?
	return nil
}

// CreateLink : Create a symlink in the container/virtual directory
func (cl *Client) CreateLink(source string, target string) error {
	log.Trace("Client::CreateLink : %s -> %s", source, target)
	data := []byte(target)
	metadata := make(map[string]string)
	metadata[symlinkKey] = "true"
	return cl.WriteFromBuffer(source, metadata, data)
}

// DeleteFile : Delete an object
func (cl *Client) DeleteFile(name string) (err error) {
	log.Trace("Client::DeleteFile : name %s", name)
	_, err = cl.deleteObject(name)
	// TODO: If the object doesn't exist, the command will return success because there's nothing to delete.
	// 		figure out how to force an error
	if err != nil {
		serr := storeBlobErrToErr(err)
		if serr == ErrFileNotFound {
			log.Err("Client::DeleteFile : %s does not exist", name)
			return syscall.ENOENT
		} else if serr == BlobIsUnderLease {
			log.Err("Client::DeleteFile : %s is under lease [%s]", name, err.Error())
			return syscall.EIO
		} else {
			log.Err("Client::DeleteFile : Failed to delete object %s [%s]", name, err.Error())
			return err
		}
	}
	return nil
}

// DeleteDirectory : Delete a virtual directory in the container/virtual directory
func (cl *Client) DeleteDirectory(name string) (err error) {

	log.Trace("Client::DeleteDirectory : name %s", name)

	reconstructedPath := filepath.Join(cl.Config.prefixPath, name) + "/"

	var marker *string = nil

	objects, _, err := cl.List(reconstructedPath, marker, 0)
	if err != nil {
		e := storeBlobErrToErr(err)
		if e == ErrFileNotFound {
			return syscall.ENOENT
		} else if e == InvalidPermission {
			return syscall.EPERM
		} else {
			log.Warn("Client::getAttr : Failed to list object properties for %s [%s]", name, err.Error())
		}
		return err
	}

	if len(objects) == 0 {
		return syscall.ENOENT
	}

	for _, object := range objects {
		fullPath := filepath.Join(cl.Config.prefixPath, object.Path)
		deleteErr := cl.DeleteFile(fullPath)
		if deleteErr != nil {
			log.Err("Client::DeleteDirectory : Failed to delete file %s [%s]", fullPath, deleteErr.Error)
			return deleteErr
		}
	}

	return nil
}

// RenameFile : Rename the file
func (cl *Client) RenameFile(source string, target string) (err error) {
	log.Trace("Client::RenameFile : %s -> %s", source, target)

	for i := 0; i < retryCount; i++ {
		_, err = cl.awsS3Client.CopyObject(context.TODO(), &s3.CopyObjectInput{
			Bucket:     aws.String(cl.Config.authConfig.BucketName),
			CopySource: aws.String(fmt.Sprintf("%v/%v", cl.Config.authConfig.BucketName, source)),
			Key:        aws.String(target),
		})
		// retry on InvalidAccessKeyId
		if isInvalidAccessKeyID(err) {
			log.Warn("Client::RenameFile Lyve Cloud \"Invalid Access Key\" bug - retry %d of %d.", i+1, retryCount)
		} else {
			break
		}
	}

	if err != nil {
		// No such key found so object is not in S3
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			log.Err("Client::RenameFile : %s does not exist", source)
			return syscall.ENOENT
		}
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			// No Such Key
			// TODO: Fix this error. For some reason, CopyObject gives a no such key error, but as a Smithy error
			// so need a different case to catch this. Understand why this does not give a no such key error type
			code := apiErr.ErrorCode()
			if code == "NoSuchKey" {
				log.Err("Client::RenameFile : %s does not exist", source)
				return syscall.ENOENT
			}
		}
		log.Err("Client::RenameFile : Failed to start copy of file %s [%s]", source, err.Error())
		return err
	}

	log.Trace("Client::RenameFile : %s -> %s done", source, target)

	// Copy of the file is done so now delete the older file
	err = cl.DeleteFile(source)
	for retry := 0; retry < 3 && err == syscall.ENOENT; retry++ {
		// Sometimes backend is able to copy source file to destination but when we try to delete the
		// source files it returns back with ENOENT. If file was just created on backend it might happen
		// that it has not been synced yet at all layers and hence delete is not able to find the source file
		log.Trace("Client::RenameFile : %s -> %s, unable to find source. Retrying %d", source, target, retry)
		time.Sleep(1 * time.Second)
		err = cl.DeleteFile(source)
	}

	if err == syscall.ENOENT {
		// Even after 3 retries, 1 second apart if server returns 404 then source file no longer
		// exists on the backend and its safe to assume rename was successful
		err = nil
	}

	return err
}

// RenameDirectory : Rename the directory
func (cl *Client) RenameDirectory(source string, target string) error {
	return nil
}

func (cl *Client) getAttrUsingRest(name string) (attr *internal.ObjAttr, err error) {
	return nil, err
}

func (cl *Client) getAttrUsingList(name string) (attr *internal.ObjAttr, err error) {
	return nil, err
}

// GetAttr : Retrieve attributes of the object
func (cl *Client) GetAttr(name string) (attr *internal.ObjAttr, err error) {
	log.Trace("Client::GetAttr : name %s", name)

	var marker *string = nil

	// TODO: Call cl.List with a limit of 1 to reduce wasted resources
	// TODO: Handle markers
	objects, _, err := cl.List(name, marker, 1)
	if err != nil {
		e := storeBlobErrToErr(err)
		if e == ErrFileNotFound {
			return attr, syscall.ENOENT
		} else if e == InvalidPermission {
			return attr, syscall.EPERM
		} else {
			log.Warn("Client::getAttrUsingList : Failed to list object properties for %s [%s]", name, err.Error())
		}
		return nil, err
	}

	numObjects := len(objects)
	for i, object := range objects {
		log.Trace("Client::GetAttr : Item %d Object %s", i+numObjects, object.Name)
		if object.Path == name {
			// we found it!
			return object, nil
		}
	}

	// not found
	log.Err("GetAttr was asked for %s, but found no match (among %d results).\n", name, numObjects)
	return nil, syscall.ENOENT
}

// List : Get a list of objects matching the given prefix
// This fetches the list using a marker so the caller code should handle marker logic
// If count=0 - fetch max entries
func (cl *Client) List(prefix string, marker *string, count int32) ([]*internal.ObjAttr, *string, error) {
	log.Trace("Client::List : prefix %s, marker %s", prefix, func(marker *string) string {
		if marker != nil {
			return *marker
		} else {
			return ""
		}
	}(marker))

	// prepare parameters
	bucketName := cl.Config.authConfig.BucketName
	if count == 0 {
		count = common.MaxDirListCount
	}
	// combine the configured prefix and the prefix being given to List to get a full listPath
	listPath := filepath.Join(cl.Config.prefixPath, prefix)
	// replace trailing forward slash stripped by filepath.Join
	if (prefix != "" && prefix[len(prefix)-1] == '/') || (prefix == "" && cl.Config.prefixPath != "") {
		listPath += "/"
	}

	// create a map to keep track of all directories
	var dirList = make(map[string]bool)

	// using paginator from here: https://aws.github.io/aws-sdk-go-v2/docs/making-requests/#using-paginators
	params := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucketName),
		MaxKeys:   count,
		Prefix:    aws.String(listPath),
		Delimiter: aws.String("/"), // delimeter is needed to get CommonPrefixes
	}

	paginator := s3.NewListObjectsV2Paginator(cl.awsS3Client, params)

	// initialize list to be returned
	objectAttrList := make([]*internal.ObjAttr, 0)
	// fetch and process result pages
	for paginator.HasMorePages() {
		var err error
		var output *s3.ListObjectsV2Output
		for i := 0; i < retryCount; i++ {
			output, err = paginator.NextPage(context.TODO())
			// retry on InvalidAccessKeyId
			if isInvalidAccessKeyID(err) {
				log.Warn("Client::List Lyve Cloud \"Invalid Access Key\" bug - retry %d of %d.", i+1, retryCount)
			} else {
				break
			}
		}
		if err != nil {
			log.Err("Failed to list objects in bucket %v with prefix %v. Here's why: %v", prefix, bucketName, err)
			return objectAttrList, nil, err
		}
		// documentation for this S3 data structure:
		// 	https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3@v1.30.2#ListObjectsV2Output
		for _, value := range output.Contents {
			// parse the ETag, on the chance that it is MD5
			// md5, err := hex.DecodeString(*value.ETag)
			// if err != nil {
			// 	log.Warn("Failed to parse ETag %v of object %v as an MD5 hash. Here's why: %v", value.ETag, value.Key, err)
			// 	md5 = nil
			// }
			// push object info into the list
			attr := &internal.ObjAttr{
				Path:   split(cl.Config.prefixPath, *value.Key),
				Name:   filepath.Base(*value.Key),
				Size:   value.Size,
				Mode:   0,
				Mtime:  *value.LastModified,
				Atime:  *value.LastModified,
				Ctime:  *value.LastModified,
				Crtime: *value.LastModified,
				Flags:  internal.NewFileBitMap(),
				// MD5:    md5,
			}

			// set flags
			attr.Flags.Set(internal.PropFlagMetadataRetrieved)
			attr.Flags.Set(internal.PropFlagModeDefault)
			attr.Metadata = make(map[string]string)
			objectAttrList = append(objectAttrList, attr)
		}
		// documentation for CommonPrefixes:
		// 	https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3@v1.30.2/types#CommonPrefix
		for _, value := range output.CommonPrefixes {
			dir := *value.Prefix
			dirList[dir] = true
			// let's extract and add intermediate directories
			// first cut the listPath (the full prefix path) off of the directory path
			_, intermediatePath, listPathFound := strings.Cut(dir, listPath)
			// if the listPath isn't here, that's weird
			if !listPathFound {
				log.Warn("Prefix mismatch with path %v when listing objects in %v.", dir, listPath)
			}
			// get an array of intermediate directories
			intermediatDirectories := strings.Split(intermediatePath, "/")
			// walk up the tree and add each one until we find an already existing parent
			// we have to iterate in descending order
			suffixToTrim := ""
			for i := len(intermediatDirectories) - 1; i >= 0; i-- {
				// ignore empty strings (split does not ommit them)
				if intermediatDirectories[i] == "" {
					continue
				}
				// add to the suffix we're trimming off
				suffixToTrim = intermediatDirectories[i] + "/" + suffixToTrim
				// get the trimmed (parent) directory
				parentDir := strings.TrimSuffix(dir, suffixToTrim)
				// have we seen this one already?
				if dirList[parentDir] {
					break
				}
				dirList[parentDir] = true
			}
		}
	}

	// for _, objAttr := range objectAttrList {
	// 	fmt.Println(objAttr.Path)
	// }

	// now let's add attributes for all the directories in dirList
	for dir := range dirList {
		if dir == listPath {
			continue
		}
		name := strings.TrimSuffix(dir, "/")
		attr := &internal.ObjAttr{
			Path:  split(cl.Config.prefixPath, name),
			Name:  filepath.Base(name),
			Size:  4096,
			Mode:  os.ModeDir,
			Mtime: time.Now(),
			Flags: internal.NewDirBitMap(),
		}
		attr.Atime = attr.Mtime
		attr.Crtime = attr.Mtime
		attr.Ctime = attr.Mtime
		attr.Flags.Set(internal.PropFlagMetadataRetrieved)
		attr.Flags.Set(internal.PropFlagModeDefault)
		attr.Metadata = make(map[string]string)
		attr.Metadata[folderKey] = "true"
		objectAttrList = append(objectAttrList, attr)
	}

	// fmt.Println("Printing again with directories")
	// for _, objAttr := range objectAttrList {
	// 	fmt.Println(objAttr.Path)
	// }

	// Clean up the temp map as its no more needed
	for k := range dirList {
		delete(dirList, k)
	}

	newMarker := ""
	return objectAttrList, &newMarker, nil
}

// track the progress of download of objects where every 100MB of data downloaded is being tracked. It also tracks the completion of download
func trackDownload(name string, bytesTransferred int64, count int64, downloadPtr *int64) {
}

// Download object to a local file with parameters: filename, bytes offset from start of object, bytes to include from offset, file to write to.
func (cl *Client) ReadToFile(name string, offset int64, count int64, fi *os.File) (err error) {

	log.Trace("Client::ReadToFile : name %s, offset : %d, count %d", name, offset, count)
	// var downloadPtr *int64 = new(int64)
	// *downloadPtr = 1

	bucketName := cl.Config.authConfig.BucketName

	result, err := cl.getObject(name, offset, count)

	if err != nil {
		// No such key found so object is not in S3
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			log.Err("Client::ReadToFile : Failed to download object %s [%s]", name, err.Error())
			return syscall.ENOENT
		}
		log.Err("Couldn't get object %v:%v. Here's why: %v\n", bucketName, name, err)
		return err
	}

	defer result.Body.Close()
	body, err := io.ReadAll(result.Body)
	if err != nil {
		log.Err("Couldn't read object body from %v. Here's why: %v\n", name, err)
		return err
	}

	_, err = fi.Write(body)
	if err != nil {
		log.Err("Couldn't write to file %v. Here's why: %v\n", name, err)
		return err
	}

	return err
}

// ReadBuffer : Download a specific range from an object to a buffer
func (cl *Client) ReadBuffer(name string, offset int64, len int64) ([]byte, error) {
	log.Trace("Client::ReadBuffer : name %s (%d+%d)", name, offset, len)
	var buff []byte

	// If the len is 0, that means we need to read till the end of the object
	if len == 0 {
		attr, err := cl.GetAttr(name)
		if err != nil {
			return buff, err
		}
		buff = make([]byte, attr.Size)
		len = attr.Size
	} else {
		buff = make([]byte, len)
	}

	result, err := cl.getObject(name, offset, len)

	if err != nil {
		// No such key found so object is not in S3
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			log.Err("Client::ReadBuffer : Failed to download object %s [%s]", name, err.Error())
			return buff, syscall.ENOENT
		}
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			// Range is incorrect
			code := apiErr.ErrorCode()
			if code == "InvalidRange" {
				log.Err("Client::ReadBuffer : Failed to download object %s [%s]", name, err.Error())
				return buff, syscall.ERANGE
			}
		}
		log.Err("Client::ReadBuffer : Failed to download object %s [%s]", name, err.Error())
		return buff, err
	}

	// Read the object into the buffer
	defer result.Body.Close()
	buff, _ = io.ReadAll(result.Body)

	return buff, nil
}

// ReadInBuffer : Download specific range from a file to a user provided buffer
func (cl *Client) ReadInBuffer(name string, offset int64, len int64, data []byte) error {
	log.Trace("Client::ReadInBuffer : name %s", name)

	result, err := cl.getObject(name, offset, len)

	if err != nil {
		// No such key found so object is not in S3
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			log.Err("Client::ReadBuffer : Failed to download object %s [%s]", name, err.Error())
			return syscall.ENOENT
		}
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			// Range is incorrect
			code := apiErr.ErrorCode()
			if code == "InvalidRange" {
				log.Err("Client::ReadBuffer : Failed to download object %s [%s]", name, err.Error())
				return syscall.ERANGE
			}
		}
		log.Err("Client::ReadBuffer : Failed to download object %s [%s]", name, err.Error())
		return err
	}

	defer result.Body.Close()
	_, err = result.Body.Read(data)

	if err != nil {
		// If we reached the EOF then all the data was correctly read so return
		if err == io.EOF {
			return nil
		}
		return err
	}

	return nil
}

func (cl *Client) calculateBlockSize(name string, fileSize int64) (blockSize int64, err error) {
	return 0, nil
}

// track the progress of upload of objects where every 100MB of data uploaded is being tracked. It also tracks the completion of upload
func trackUpload(name string, bytesTransferred int64, count int64, uploadPtr *int64) {
}

// WriteFromFile : Upload local file to object
func (cl *Client) WriteFromFile(name string, metadata map[string]string, fi *os.File) (err error) {
	log.Trace("Client::WriteFromFile : name %s", name)
	//defer exectime.StatTimeCurrentBlock("WriteFromFile::WriteFromFile")()

	defer log.TimeTrack(time.Now(), "Client::WriteFromFile", name)

	var uploadPtr *int64 = new(int64)
	*uploadPtr = 1

	// TODO: Move this variable into the config file
	// var partMiBs int64 = 16

	// get the size of the file
	stat, err := fi.Stat()
	if err != nil {
		log.Err("Client::WriteFromFile : Failed to get file size %s [%s]", name, err.Error())
		return err
	}

	// if the block size is not set then we configure it based on file size
	// if blockSize == 0 {
	// 	// based on file-size calculate block size
	// 	blockSize, err = cl.calculateBlockSize(name, stat.Size())
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	// TODO: Add md5 hash support in S3
	// Compute md5 of this file is requested by user
	// If file is uploaded in one shot (no blocks created) then server is populating md5 on upload automatically.
	// hence we take cost of calculating md5 only for files which are bigger in size and which will be converted to blocks.
	// md5sum := []byte{}
	// if cl.Config.updateMD5 && stat.Size() >= azblob.BlockBlobMaxUploadBlobBytes {
	// 	md5sum, err = getMD5(fi)
	// 	if err != nil {
	// 		// Md5 sum generation failed so set nil while uploading
	// 		log.Warn("Client::WriteFromFile : Failed to generate md5 of %s", name)
	// 		md5sum = []byte{0}
	// 	}
	// }

	// TODO: Is there a more elegant way to do this?
	// The aws-sdk-go does not seem to see to the end of the file
	// so let's seek to the start before uploading
	_, err = fi.Seek(0, 0)
	if err != nil {
		log.Err("Client::WriteFromFile : Failed to seek to beginning of input file %s", fi.Name())
		return err
	}

	// uploader := manager.NewUploader(cl.Client, func(u *manager.Uploader) {
	// 	u.PartSize = partMiBs * 1024 * 1024
	// })

	_, err = cl.putObject(name, fi)

	// TODO: Add monitor tracking
	// if common.MonitorBfs() && stat.Size() > 0 {
	// 	uploadOptions.Progress = func(bytesTransferred int64) {
	// 		trackUpload(name, bytesTransferred, stat.Size(), uploadPtr)
	// 	}
	// }

	if err != nil {
		log.Err("Client::WriteFromFile : Failed to upload object %s [%s]", name, err.Error())
		return err
	}

	log.Debug("Client::WriteFromFile : Upload complete of object %v", name)

	// store total bytes uploaded so far
	if stat.Size() > 0 {
		s3StatsCollector.UpdateStats(stats_manager.Increment, bytesUploaded, stat.Size())
	}

	return nil
}

// WriteFromBuffer : Upload from a buffer to a object
func (cl *Client) WriteFromBuffer(name string, metadata map[string]string, data []byte) (err error) {
	largeBuffer := bytes.NewReader(data)
	// TODO: Move this variable into the config file
	// var partMiBs int64 = 16
	// uploader := manager.NewUploader(cl.Client, func(u *manager.Uploader) {
	// 	u.PartSize = partMiBs * 1024 * 1024
	// })

	_, err = cl.putObject(name, largeBuffer)

	if err != nil {
		log.Err("Couldn't upload object to %v:%v. Here's why: %v\n",
			cl.Config.authConfig.BucketName, name, err)
		return err
	}
	return nil
}

// GetFileBlockOffsets: store blocks ids and corresponding offsets
func (cl *Client) GetFileBlockOffsets(name string) (*common.BlockOffsetList, error) {
	return nil, nil
}

func (cl *Client) createBlock(blockIdLength, startIndex, size int64) *common.Block {
	return nil
}

// create new blocks based on the offset and total length we're adding to the file
func (cl *Client) createNewBlocks(blockList *common.BlockOffsetList, offset, length int64) int64 {
	return 0
}

func (cl *Client) removeBlocks(blockList *common.BlockOffsetList, size int64, name string) *common.BlockOffsetList {
	return nil
}

func (cl *Client) TruncateFile(name string, size int64) error {
	return nil
}

// Write : write data at given offset to a object
func (cl *Client) Write(options internal.WriteFileOptions) error {
	name := options.Handle.Path
	offset := options.Offset
	data := options.Data
	length := int64(len(data))
	defer log.TimeTrack(time.Now(), "Client::Write", options.Handle.Path)
	log.Trace("Client::Write : name %s offset %v", name, offset)
	// tracks the case where our offset is great than our current file size (appending only - not modifying pre-existing data)
	var dataBuffer *[]byte

	// get the existing object data
	oldData, _ := cl.ReadBuffer(name, 0, 0)
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
	// WriteFromBuffer should be able to handle the case where now the block is too big and gets split into multiple blocks
	err := cl.WriteFromBuffer(name, options.Metadata, *dataBuffer)
	if err != nil {
		log.Err("Client::Write : Failed to upload to object %s ", name, err.Error())
		return err
	}
	return nil
}

// TODO: make a similar method facing stream that would enable us to write to cached blocks then stage and commit
func (cl *Client) stageAndCommitModifiedBlocks(name string, data []byte, offsetList *common.BlockOffsetList) error {
	return nil
}

func (cl *Client) StageAndCommit(name string, bol *common.BlockOffsetList) error {
	return nil
}

// ChangeMod : Change mode of a object
func (cl *Client) ChangeMod(name string, _ os.FileMode) error {
	return nil
}

// ChangeOwner : Change owner of a object
func (cl *Client) ChangeOwner(name string, _ int, _ int) error {
	return nil
}
