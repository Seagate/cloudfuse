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
	"sort" // to sort List() results
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
	symlinkKey = "is_symlink"
)

type Client struct {
	Connection
	awsS3Client *s3.Client // S3 client library supplied by AWS
}

// Verify that Client implements S3Connection interface
var _ S3Connection = &Client{}

// Configure : Initialize the awsS3Client
func (cl *Client) Configure(cfg Config) error {
	log.Trace("Client::Configure : initialize awsS3Client")
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

	// TODO: check if the config is missing credentials
	// 	and allow the default config to find them in the environment on its own
	staticProvider := credentials.NewStaticCredentialsProvider(
		cl.Config.authConfig.KeyID,
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

// For dynamic configuration, update the config here.
// Not implemented.
func (cl *Client) UpdateConfig(cfg Config) error {
	log.Trace("Client::UpdateConfig : no dynamic config update support")
	return nil
}

// NewCredentialKey : Update the credential key specified by the user.
// Currently not implemented.
func (cl *Client) NewCredentialKey(key, value string) (err error) {
	log.Trace("Client::NewCredentialKey : not implemented")
	// TODO: research whether and how credentials could change on the same bucket
	// If they can, research whether we can change credentials on an existing client object
	// 	(do we replace the credential provider?)
	return nil
}

// Wrapper for awsS3Client.GetObject.
// Set count = 0 to read to the end of the object.
// key is the full path to the object (with the prefixPath).
func (cl *Client) getObject(key string, offset int64, count int64) (body io.ReadCloser, err error) {
	log.Trace("Client::getObject : get object %s (%d+%d)", key, offset, count)

	// deal with the range
	var rangeString string //string to be used to specify range of object to download from S3
	//TODO: add handle if the offset+count is greater than the end of Object.
	if count == 0 {
		// sending Range:"bytes=0-" gives errors from Lyve Cloud ("InvalidRange: The requested range is not satisfiable")
		// so if offset is 0 too, leave rangeString empty
		if offset != 0 {
			rangeString = "bytes=" + fmt.Sprint(offset) + "-"
		}
	} else {
		endRange := offset + count
		rangeString = "bytes=" + fmt.Sprint(offset) + "-" + fmt.Sprint(endRange)
	}

	result, err := cl.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(cl.Config.authConfig.BucketName),
		Key:    aws.String(key),
		Range:  aws.String(rangeString),
	})

	// check for errors
	if err != nil {
		log.Err("Client::getObject : Failed to get object %s. Here's why: %v", key, err)
		// No such key found so object is not in S3
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			err = syscall.ENOENT
		}
		// Invalid range
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			code := apiErr.ErrorCode()
			if code == "InvalidRange" {
				err = syscall.ERANGE
			}
		}
		return nil, err
	}
	// return body
	return result.Body, err
}

// Wrapper for awsS3Client.PutObject.
// Takes an io.Reader to work with both files and byte arrays.
// key is the full path to the object (with the prefixPath).
func (cl *Client) putObject(key string, objectData io.Reader) (*s3.PutObjectOutput, error) {
	log.Trace("Client::putObject : putting object %s", key)

	result, err := cl.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(cl.Config.authConfig.BucketName),
		Key:         aws.String(key),
		Body:        objectData,
		ContentType: aws.String(getContentType(key)),
	})

	return result, err
}

// Wrapper for awsS3Client.DeleteObject.
// key is the full path to the object (with the prefixPath).
func (cl *Client) deleteObject(key string) (*s3.DeleteObjectOutput, error) {
	log.Trace("Client::deleteObject : deleting object %s", key)

	result, err := cl.awsS3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(cl.Config.authConfig.BucketName),
		Key:    aws.String(key),
	})

	return result, err
}

// Wrapper for awsS3Client.DeleteObjects.
// keys is a list of full paths to the objects (with the prefixPath)
func (cl *Client) deleteObjects(keys []string) (result *s3.DeleteObjectsOutput, err error) {
	log.Trace("Client::deleteObjects : deleting %d objects", len(keys))
	// build list to send to DeleteObjects
	keyList := make([]types.ObjectIdentifier, len(keys))
	for i := 0; i < len(keys); i++ {
		keyList[i] = types.ObjectIdentifier{
			Key: &keys[i],
		}
	}
	// send keyList for deletion
	result, err = cl.awsS3Client.DeleteObjects(context.TODO(), &s3.DeleteObjectsInput{
		Bucket: &cl.Config.authConfig.BucketName,
		Delete: &types.Delete{
			Objects: keyList,
			Quiet:   true,
		},
	})

	return
}

// Wrapper for awsS3Client.HeadObject.
// HeadObject() acts just like GetObject, except no contents are returned.
// So this is used to get metadata / attributes for an object.
// key is the full path to the object (with the prefixPath)
func (cl *Client) headObject(key string) (*s3.HeadObjectOutput, error) {
	log.Trace("Client::headObject : object %s", key)

	result, err := cl.awsS3Client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(cl.Config.authConfig.BucketName),
		Key:    aws.String(key),
	})

	return result, err
}

// Convert file name to object getKey
func (cl *Client) getKey(name string) string {
	return common.JoinUnixFilepath(cl.Config.prefixPath, name)
}

// Wrapper for awsS3Client.ListBuckets
func (cl *Client) ListBuckets() ([]string, error) {
	log.Trace("Client::ListBuckets : Listing buckets")

	cntList := make([]string, 0)

	result, err := cl.awsS3Client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})

	if err != nil {
		log.Err("Couldn't list buckets for your account. Here's why: %v", err)
		return cntList, err
	}

	for _, bucket := range result.Buckets {
		cntList = append(cntList, *bucket.Name)
	}

	return cntList, nil
}

// Set the prefix path - this overrides "subdirectory" in config.yaml.
// This is only used for testing.
func (cl *Client) SetPrefixPath(path string) error {
	log.Trace("Client::SetPrefixPath : path %s", path)
	cl.Config.prefixPath = path
	return nil
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
	// Lyve Cloud does not support creating an empty file to indicate a directory
	// directories will be represented only as object prefixes
	// we have no way of representing an empty directory, so do nothing
	// TODO: research: is this supposed to throw an error if the directory already exists?
	return nil
}

// CreateLink : Create a symlink in the bucket/virtual directory
func (cl *Client) CreateLink(source string, target string) error {
	log.Trace("Client::CreateLink : %s -> %s", source, target)
	data := []byte(target)
	metadata := make(map[string]string)
	metadata[symlinkKey] = "true"
	return cl.WriteFromBuffer(source, metadata, data)
}

// DeleteFile : Delete an object.
// if the file does not exist, this returns an error (ENOENT).
func (cl *Client) DeleteFile(name string) (err error) {
	log.Trace("Client::DeleteFile : name %s", name)
	// first check if the object exists
	_, err = cl.getFileAttr(name)
	if err == syscall.ENOENT {
		log.Err("Client::DeleteFile : %s does not exist", name)
		return syscall.ENOENT
	} else if err != nil {
		log.Err("Client::DeleteFile : Failed to getFileAttr for object %s. Here's why: %v", name, err)
		return err
	}
	// delete the object
	key := cl.getKey(name)
	_, err = cl.deleteObject(key)
	if err != nil {
		log.Err("Client::DeleteFile : Failed to delete object %s. Here's why: %v", name, err)
		return err
	}

	return nil
}

// DeleteDirectory : Delete all objects with the given prefix.
// If name is given without a trailing slash, a slash will be added.
// If the directory does not exist, no error will be returned.
func (cl *Client) DeleteDirectory(name string) (err error) {
	log.Trace("Client::DeleteDirectory : name %s", name)

	// make sure name has a trailing slash
	name = internal.ExtendDirName(name)

	// list all objects with the prefix
	objects, _, err := cl.List(name, nil, 0)
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
	var keysToDelete []string
	for _, object := range objects {
		if object.IsDir() {
			err = cl.DeleteDirectory(object.Path)
			if err != nil {
				log.Err("Client::DeleteDirectory : Failed to delete directory %s. Here's why: %v", object.Path, err)
			}
		} else {
			keysToDelete = append(keysToDelete, cl.getKey(object.Path))
		}
	}
	// Delete the collected keys
	result, err := cl.deleteObjects(keysToDelete)
	if err != nil {
		log.Err("Client::DeleteDirectory : Failed to delete %d files. Here's why: %v", len(keysToDelete), err)
		for i := 0; i < len(result.Errors); i++ {
			log.Err("Client::DeleteDirectory : Failed to delete key %s. Here's why: %s", result.Errors[i].Key, result.Errors[i].Message)
		}
	}

	return err
}

// RenameFile : Rename the object (copy then delete).
func (cl *Client) RenameFile(source string, target string) (err error) {
	log.Trace("Client::RenameFile : %s -> %s", source, target)

	// copy the object to its new key
	sourceKey := cl.getKey(source)
	targetKey := cl.getKey(target)
	_, err = cl.awsS3Client.CopyObject(context.TODO(), &s3.CopyObjectInput{
		Bucket: aws.String(cl.Config.authConfig.BucketName),
		// TODO: URL-encode CopySource
		CopySource: aws.String(fmt.Sprintf("%v/%v", cl.Config.authConfig.BucketName, sourceKey)),
		Key:        aws.String(targetKey),
	})
	// check for errors on copy
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
		log.Err("Client::RenameFile : Failed to start copy of file %s. Here's why: %v", source, err)
		return err
	}

	log.Debug("Client::RenameFile : copy %s -> %s done", source, target)

	// Copy of the file is done so now delete the older file
	// in this case we don't need to check if the file exists, so we use deleteObject, not DeleteFile
	// this is what S3's DeleteObject spec is meant for: to make sure the object doesn't exist anymore
	_, err = cl.deleteObject(sourceKey)
	if err != nil {
		log.Err("Client::RenameFile : Failed to delete source object after copy. Here's why: %v", err)
	}

	return err
}

// RenameDirectory : Rename the directory
func (cl *Client) RenameDirectory(source string, target string) error {
	log.Trace("Client::RenameDirectory : %s -> %s", source, target)

	// first we need a list of all the object's we'll be moving
	// make sure to pass source with a trailing forward slash
	sourceObjects, _, err := cl.List(internal.ExtendDirName(source), nil, 0)
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
			err = cl.RenameFile(srcPath, dstPath)
		}
		if err != nil {
			log.Err("Client::RenameDirectory : Failed to rename %s -> %s. Here's why: %v", srcPath, dstPath, err)
		}
	}

	return nil
}

// GetAttr : Get attributes for a given file or folder.
// If name is a file, it should not have a trailing slash.
// If name is a directory, the trailing slash is optional.
func (cl *Client) GetAttr(name string) (attr *internal.ObjAttr, err error) {
	log.Trace("Client::GetAttr : name %s", name)

	// first let's suppose the caller is looking for a file
	// there are no objects with trailing slashes (MinIO doesn't support them)
	// 	and trailing slashes aren't allowed in filenames
	// so if this was called with a trailing slash, don't look for an object
	if len(name) > 0 && name[len(name)-1] != '/' {
		attr, err = cl.getFileAttr(name)
		if err == nil {
			return attr, err
		}
	}

	// ensure a trailing slash
	dirName := internal.ExtendDirName(name)
	// now search for that as a directory
	return cl.getDirectoryAttr(dirName)
}

// Get attributes for the given file path.
// Return ENOENT if there is no corresponding object in the bucket.
// name should not have a trailing slash (nothing will be found!).
func (cl *Client) getFileAttr(name string) (attr *internal.ObjAttr, err error) {
	log.Trace("Client::getFileAttr : name %s", name)

	// no trailing slash, so we can use headObject
	key := cl.getKey(name)
	result, err := cl.headObject(key)
	if err == nil {
		// create and return an objAttr
		attr = createObjAttr(name, result.ContentLength, *result.LastModified)
		return attr, err
	}
	// err is not nil
	log.Debug("Client::getFileAttr : headObject(%s) failed. Here's why: %v", key, err)
	// No such key found so object is not in S3
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return nil, syscall.ENOENT
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		if code == "NotFound" {
			return nil, syscall.ENOENT
		}
	}

	// unknown err - we have bigger problems than a non-existent object
	log.Err("Client::getFileAttr : headObject(%s) failed. Here's why: %v", key, err)
	return nil, err
}

func (cl *Client) getDirectoryAttr(dirName string) (attr *internal.ObjAttr, err error) {
	log.Trace("Client::getDirectoryAttr : name %s", dirName)

	// to do this, accept anything that comes back from List()
	objects, _, err := cl.List(dirName, nil, 1)
	if err != nil {
		log.Err("Client::getDirectoryAttr : List(%s) failed. Here's why: %v", dirName, err)
		return nil, err
	} else if len(objects) > 0 {
		// create and return an objAttr for the directory
		attr = createObjAttrDir(dirName)
		return attr, nil
	}

	// directory not found in bucket
	log.Err("Client::getDirectoryAttr : not found: %s", dirName)
	return nil, syscall.ENOENT
}

// List : Get a list of objects matching the given prefix, up to the next "/", similar to listing a directory.
// For predictable results, include the trailing slash in the prefix.
// When prefix has no trailing slash, List has unintuitive behavior (e.g. prefix "file" would match "filet-o-fish").
// This fetches the list using a marker so the caller code should handle marker logic.
// If count=0 - fetch max entries.
func (cl *Client) List(prefix string, marker *string, count int32) ([]*internal.ObjAttr, *string, error) {
	log.Trace("Client::List : prefix %s, marker %s", prefix, func(marker *string) string {
		if marker != nil {
			return *marker
		}
		return ""
	}(marker))

	// prepare parameters
	bucketName := cl.Config.authConfig.BucketName
	if count == 0 {
		count = common.MaxDirListCount
	}

	// combine the configured prefix and the prefix being given to List to get a full listPath
	listPath := cl.getKey(prefix)
	// replace any trailing forward slash stripped by common.JoinUnixFilepath
	if (prefix != "" && prefix[len(prefix)-1] == '/') || (prefix == "" && cl.Config.prefixPath != "") {
		listPath += "/"
	}

	// Only look for CommonPrefixes (subdirectories) if List was called with a prefix ending in a slash.
	// If prefix does not end in a slash, CommonPrefixes would find unwanted results.
	// For example, it would find "filet-of-fish/" when searching for "file".
	findCommonPrefixes := listPath[len(listPath)-1] == '/'

	// create a map to keep track of all directories
	var dirList = make(map[string]bool)

	// using paginator from here: https://aws.github.io/aws-sdk-go-v2/docs/making-requests/#using-paginators
	// List is a tricky function. Here is a great explanation of how list works:
	// 	https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-prefixes.html
	params := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucketName),
		MaxKeys:   count,
		Prefix:    aws.String(listPath),
		Delimiter: aws.String("/"), // delimeter limits results and provides CommonPrefixes
	}
	paginator := s3.NewListObjectsV2Paginator(cl.awsS3Client, params)
	// initialize list to be returned
	objectAttrList := make([]*internal.ObjAttr, 0)
	// fetch and process result pages
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(context.TODO())
		if err != nil {
			log.Err("Failed to list objects in bucket %v with prefix %v. Here's why: %v", prefix, bucketName, err)
			return objectAttrList, nil, err
		}
		// documentation for this S3 data structure:
		// 	https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3@v1.30.2#ListObjectsV2Output
		for _, value := range output.Contents {
			// push object info into the list
			path := split(cl.Config.prefixPath, *value.Key)
			attr := createObjAttr(path, value.Size, *value.LastModified)
			objectAttrList = append(objectAttrList, attr)
		}

		if findCommonPrefixes {
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
				intermediateDirectories := strings.Split(intermediatePath, "/")
				// walk up the tree and add each one until we find an already existing parent
				// we have to iterate in descending order
				suffixToTrim := ""
				for i := len(intermediateDirectories) - 1; i >= 0; i-- {
					// ignore empty strings (split does not ommit them)
					if intermediateDirectories[i] == "" {
						continue
					}
					// add to the suffix we're trimming off
					suffixToTrim = intermediateDirectories[i] + "/" + suffixToTrim
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
	}

	// now let's add attributes for all the directories in dirList
	for dir := range dirList {
		if dir == listPath {
			continue
		}
		path := split(cl.Config.prefixPath, dir)
		attr := createObjAttrDir(path)
		objectAttrList = append(objectAttrList, attr)
	}

	// values should be returned in ascending order by key
	// sort the list before returning it
	sort.Slice(objectAttrList, func(i, j int) bool {
		return objectAttrList[i].Path < objectAttrList[j].Path
	})

	newMarker := ""
	return objectAttrList, &newMarker, nil
}

// create an object attributes struct
func createObjAttr(path string, size int64, lastModified time.Time) (attr *internal.ObjAttr) {
	attr = &internal.ObjAttr{
		Path:   path,
		Name:   filepath.Base(path),
		Size:   size,
		Mode:   0,
		Mtime:  lastModified,
		Atime:  lastModified,
		Ctime:  lastModified,
		Crtime: lastModified,
		Flags:  internal.NewFileBitMap(),
	}
	// set flags
	attr.Flags.Set(internal.PropFlagMetadataRetrieved)
	attr.Flags.Set(internal.PropFlagModeDefault)
	attr.Metadata = make(map[string]string)

	return attr
}

// create an object attributes struct for a directory
func createObjAttrDir(path string) (attr *internal.ObjAttr) {
	// strip any trailing slash
	path = internal.TruncateDirName(path)
	// For these dirs we get only the name and no other properties so hardcoding time to current time
	currentTime := time.Now()

	attr = createObjAttr(path, 4096, currentTime)
	// Change the relevant fields for a directory
	attr.Mode = os.ModeDir
	// set flags
	attr.Flags = internal.NewDirBitMap()
	attr.Flags.Set(internal.PropFlagMetadataRetrieved)
	attr.Flags.Set(internal.PropFlagModeDefault)

	return attr
}

// Download object data to a file handle.
// Read starting at a byte offset from the start of the object, with length in bytes = count.
// count = 0 reads to the end of the object.
func (cl *Client) ReadToFile(name string, offset int64, count int64, fi *os.File) (err error) {
	log.Trace("Client::ReadToFile : name %s, offset : %d, count %d", name, offset, count)
	// get object data
	objectDataReader, err := cl.getObject(cl.getKey(name), offset, count)
	if err != nil {
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
func (cl *Client) ReadBuffer(name string, offset int64, len int64) ([]byte, error) {
	log.Trace("Client::ReadBuffer : name %s (%d+%d)", name, offset, len)
	// get object data
	objectDataReader, err := cl.getObject(cl.getKey(name), offset, len)
	if err != nil {
		return nil, err
	}
	// read object data
	defer objectDataReader.Close()
	buff, err := io.ReadAll(objectDataReader)
	if err != nil {
		log.Err("Failed to read data from GetObject result. Here's why: %v", err)
		return nil, err
	}

	return buff, nil
}

// Download object to provided byte array.
// Reads starting at a byte offset from the start of the object, with length in bytes = len.
// len = 0 reads to the end of the object.
// name is the file path.
func (cl *Client) ReadInBuffer(name string, offset int64, len int64, data []byte) error {
	log.Trace("Client::ReadInBuffer : name %s", name)
	// get object data
	objectDataReader, err := cl.getObject(cl.getKey(name), offset, len)
	if err != nil {
		return err
	}
	// read object data
	defer objectDataReader.Close()
	_, err = objectDataReader.Read(data)
	if err == io.EOF {
		// If we reached the EOF then all the data was correctly read
		return nil
	}

	return err
}

// Upload from a file handle to an object.
// The metadata parameter is not used.
func (cl *Client) WriteFromFile(name string, metadata map[string]string, fi *os.File) (err error) {
	log.Trace("Client::WriteFromFile : name %s", name)
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
	key := cl.getKey(name)
	_, err = cl.putObject(key, fi)
	// TODO: decide when to use this higher-level API
	// uploader := manager.NewUploader(cl.Client, func(u *manager.Uploader) {
	//  // TODO: Move this variable into the config file
	// 	u.PartSize = partMiBs * 1024 * 1024
	// })
	// check for errors
	if err != nil {
		log.Err("Client::WriteFromFile : Failed to upload object %s. Here's why: %v", name, err)
		return err
	}

	// TODO: Add monitor tracking
	// if common.MonitorBfs() && stat.Size() > 0 {
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
func (cl *Client) WriteFromBuffer(name string, metadata map[string]string, data []byte) (err error) {
	log.Trace("Client::WriteFromBuffer : name %s", name)

	// convert byte array to io.Reader
	dataReader := bytes.NewReader(data)
	// upload data to object
	key := cl.getKey(name)
	// TODO: handle metadata with S3
	_, err = cl.putObject(key, dataReader)
	if err != nil {
		log.Err("Couldn't upload object to %v. Here's why: %v", name, err)
		return err
	}
	return nil
}

// GetFileBlockOffsets: store blocks ids and corresponding offsets.
func (cl *Client) GetFileBlockOffsets(name string) (*common.BlockOffsetList, error) {
	// TODO: decide whether we have any use for this function
	// if not, we can just skip this and return nil, nil in s3storage.go:GetFileBlockOffsets()
	return nil, nil
}

// Truncate object to size in bytes.
// name is the file path.
func (cl *Client) TruncateFile(name string, size int64) error {
	log.Trace("Client::TruncateFile : Truncating %s to %dB.", name, size)

	// get object data
	objectDataReader, err := cl.getObject(cl.getKey(name), 0, 0)
	if err != nil {
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
	key := cl.getKey(name)
	_, err = cl.putObject(key, truncatedDataReader)
	if err != nil {
		log.Err("Client::TruncateFile : Failed to write truncated data to object %s. Here's why: %v", name, err)
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
		log.Err("Client::Write : Failed to upload to object. Here's why: %v ", name, err)
		return err
	}
	return nil
}
