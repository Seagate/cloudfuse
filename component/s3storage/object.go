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
	"encoding/hex"
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

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

const (
	folderKey  = "hdi_isfolder"
	symlinkKey = "is_symlink"
)

type S3Object struct {
	S3StorageConnection
	//Auth      azAuth
	Client    *s3.Client
	Container azblob.ContainerURL
}

// Verify that S3Object implements AzConnection interface
var _ S3Connection = &S3Object{}

const (
	MaxBlocksSize = azblob.BlockBlobMaxStageBlockBytes * azblob.BlockBlobMaxBlocks
)

func (bb *S3Object) Configure(cfg S3StorageConfig) error {
	bb.Config = cfg

	endpointResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		//if service == s3.ServiceID && region == "us-east-1" {
		if service == s3.ServiceID {
			return aws.Endpoint{
				PartitionID:   "aws",
				URL:           bb.Config.authConfig.Endpoint,
				SigningRegion: bb.Config.authConfig.Region,
			}, nil
		}
		return aws.Endpoint{}, fmt.Errorf("unknown endpoint requested")
	})

	staticProvider := credentials.NewStaticCredentialsProvider(
		bb.Config.authConfig.AccessKey,
		bb.Config.authConfig.SecretKey,
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
	bb.Client = s3.NewFromConfig(defaultConfig)

	return nil
}

// For dynamic config update the config here
func (bb *S3Object) UpdateConfig(cfg S3StorageConfig) error {
	return nil
}

// NewCredentialKey : Update the credential key specified by the user
func (bb *S3Object) NewCredentialKey(key, value string) (err error) {
	return nil
}

// getCredential : Create the credential object
func (bb *S3Object) getCredential() azblob.Credential {
	return nil
}

func (bb *S3Object) ListContainers() ([]string, error) {
	log.Trace("S3Object::ListContainers : Listing containers")

	cntList := make([]string, 0)
	result, err := bb.Client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	if err != nil {
		fmt.Printf("Couldn't list buckets for your account. Here's why: %v\n", err)
		return cntList, err
	}

	for _, bucket := range result.Buckets {
		cntList = append(cntList, *bucket.Name)
	}

	return cntList, nil
}

func exitErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func (bb *S3Object) SetPrefixPath(path string) error {
	log.Trace("BlockBlob::SetPrefixPath : path %s", path)
	bb.Config.prefixPath = path
	return nil
}

// CreateFile : Create a new file in the container/virtual directory
func (bb *S3Object) CreateFile(name string, mode os.FileMode) error {
	log.Trace("S3Object::CreateFile : name %s", name)
	var data []byte
	return bb.WriteFromBuffer(name, nil, data)
}

// CreateDirectory : Create a new directory in the container/virtual directory
func (bb *S3Object) CreateDirectory(name string) error {
	log.Trace("S3Object::CreateDirectory : name %s", name)

	// Lyve Cloud does not support creating an empty file to indicate a directory
	// so do nothing
	return nil
}

// CreateLink : Create a symlink in the container/virtual directory
func (bb *S3Object) CreateLink(source string, target string) error {
	log.Trace("S3Object::CreateLink : %s -> %s", source, target)
	data := []byte(target)
	metadata := make(azblob.Metadata)
	metadata[symlinkKey] = "true"
	return bb.WriteFromBuffer(source, metadata, data)
}

// DeleteFile : Delete a blob in the container/virtual directory
func (bb *S3Object) DeleteFile(name string) (err error) {
	return err
}

// DeleteDirectory : Delete a virtual directory in the container/virtual directory
func (bb *S3Object) DeleteDirectory(name string) (err error) {
	return err
}

// RenameFile : Rename the file
func (bb *S3Object) RenameFile(source string, target string) (err error) {
	log.Trace("Object::RenameFile : %s -> %s", source, target)

	_, err = bb.Client.CopyObject(context.TODO(), &s3.CopyObjectInput{
		Bucket:     aws.String(bb.Config.authConfig.BucketName),
		CopySource: aws.String(fmt.Sprintf("%v/%v", bb.Config.authConfig.BucketName, source)),
		Key:        aws.String(target),
	})

	if err != nil {
		// No such key found so object is not in S3
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			log.Err("Object::RenameFile : %s does not exist", source)
			return syscall.ENOENT
		}
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			// No Such Key
			// TODO: Fix this error. For some reason, CopyObject gives a no such key error, but as a Smithy error
			// so need a different case to catch this. Understand why this does not give a no such key error type
			code := apiErr.ErrorCode()
			if code == "NoSuchKey" {
				log.Err("Object::RenameFile : %s does not exist", source)
				return syscall.ENOENT
			}
		}
		log.Err("Object::RenameFile : Failed to start copy of file %s [%s]", source, err.Error())
		return err
	}

	log.Trace("BlockBlob::RenameFile : %s -> %s done", source, target)

	// Copy of the file is done so now delete the older file
	err = bb.DeleteFile(source)
	for retry := 0; retry < 3 && err == syscall.ENOENT; retry++ {
		// Sometimes backend is able to copy source file to destination but when we try to delete the
		// source files it returns back with ENOENT. If file was just created on backend it might happen
		// that it has not been synced yet at all layers and hence delete is not able to find the source file
		log.Trace("BlockBlob::RenameFile : %s -> %s, unable to find source. Retrying %d", source, target, retry)
		time.Sleep(1 * time.Second)
		err = bb.DeleteFile(source)
	}

	if err == syscall.ENOENT {
		// Even after 3 retries, 1 second apart if server returns 404 then source file no longer
		// exists on the backend and its safe to assume rename was successful
		err = nil
	}

	return err
}

// RenameDirectory : Rename the directory
func (bb *S3Object) RenameDirectory(source string, target string) error {
	return nil
}

func (bb *S3Object) getAttrUsingRest(name string) (attr *internal.ObjAttr, err error) {
	return nil, err
}

func (bb *S3Object) getAttrUsingList(name string) (attr *internal.ObjAttr, err error) {
	return nil, err
}

// GetAttr : Retrieve attributes of the blob
func (bb *S3Object) GetAttr(name string) (attr *internal.ObjAttr, err error) {
	return nil, err
}

// List : Get a list of blobs matching the given prefix
// This fetches the list using a marker so the caller code should handle marker logic
// If count=0 - fetch max entries
func (bb *S3Object) List(prefix string, marker *string, count int32) ([]*internal.ObjAttr, *string, error) {
	log.Trace("BlockBlob::List : prefix %s, marker %s", prefix, func(marker *string) string {
		if marker != nil {
			return *marker
		} else {
			return ""
		}
	}(marker))

	// prepare parameters
	bucketName := bb.Config.authConfig.BucketName
	if count == 0 {
		count = common.MaxDirListCount
	}
	// combine the configured prefix and the prefix being given to List to get a full listPath
	listPath := filepath.Join(bb.Config.prefixPath, prefix)
	// make sure the listPath ends with a forward slash
	if (prefix != "" && prefix[len(prefix)-1] == '/') || (prefix == "" && bb.Config.prefixPath != "") {
		listPath += "/"
	}

	// create a map to keep track of all directories
	var dirList = make(map[string]bool)
	dirList[listPath] = true

	// using paginator from here: https://aws.github.io/aws-sdk-go-v2/docs/making-requests/#using-paginators
	params := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucketName),
		MaxKeys:   count,
		Prefix:    aws.String(listPath),
		Delimiter: aws.String("/"), // delimeter is needed to get CommonPrefixes
	}

	paginator := s3.NewListObjectsV2Paginator(bb.Client, params)

	// initialize list to be returned
	objectAttrList := make([]*internal.ObjAttr, 0)
	// fetch and process result pages
	for paginator.HasMorePages() {
		retryCount := 5
		var err error
		var output *s3.ListObjectsV2Output
		for i := 0; i < retryCount; i++ {
			output, err = paginator.NextPage(context.TODO())
			if err == nil {
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
			md5, err := hex.DecodeString(*value.ETag)
			if err != nil {
				log.Warn("Failed to parse ETag %v of object %v as an MD5 hash. Here's why: %v", value.ETag, value.Key, err)
				md5 = nil
			}
			// push object info into the list
			attr := &internal.ObjAttr{
				Path:   split(bb.Config.prefixPath, *value.Key),
				Name:   filepath.Base(*value.Key),
				Size:   value.Size,
				Mode:   0,
				Mtime:  *value.LastModified,
				Atime:  *value.LastModified,
				Ctime:  *value.LastModified,
				Crtime: *value.LastModified,
				Flags:  internal.NewFileBitMap(),
				MD5:    md5,
			}

			// this is cargo code from block_blob
			// not sure if we need this
			attr.Flags.Set(internal.PropFlagMetadataRetrieved)
			attr.Flags.Set(internal.PropFlagModeDefault)
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

	for _, objAttr := range objectAttrList {
		fmt.Println(objAttr.Path)
	}

	// now let's add attributes for all the directories in dirList
	for dir, _ := range dirList {
		if dir == listPath {
			continue
		}
		name := strings.TrimSuffix(dir, "/")
		attr := &internal.ObjAttr{
			Path:  split(bb.Config.prefixPath, name),
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
		objectAttrList = append(objectAttrList, attr)
	}

	fmt.Println("Printing again with directories")
	for _, objAttr := range objectAttrList {
		fmt.Println(objAttr.Path)
	}

	// Clean up the temp map as its no more needed
	for k := range dirList {
		delete(dirList, k)
	}
	newMarker := ""

	return objectAttrList, &newMarker, nil
}

// track the progress of download of blobs where every 100MB of data downloaded is being tracked. It also tracks the completion of download
func trackDownload(name string, bytesTransferred int64, count int64, downloadPtr *int64) {
}

// ReadToFile : Download a blob to a local file

/*
what dis do?
*/
func (bb *S3Object) ReadToFile(name string, offset int64, count int64, fi *os.File) (err error) {

	log.Trace("Object::ReadToFile : name %s, offset : %d, count %d", name, offset, count)
	//defer exectime.StatTimeCurrentBlock("BlockBlob::ReadToFile")()
	//blobURL := bb.Container.NewBlobURL(filepath.Join(bb.Config.prefixPath, name))

	//what is this?
	var downloadPtr *int64 = new(int64)
	*downloadPtr = 1

	//defer log.TimeTrack(time.Now(), "BlockBlob::ReadToFile", name)
	//err = azblob.DownloadBlobToFile(context.Background(), blobURL, offset, count, fi, bb.downloadOptions)

	bucketName := aws.String(bb.Config.authConfig.BucketName)
	result, err := bb.Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: bucketName,
		Key:    aws.String(name),
	})
	if err != nil {
		// No such key found so object is not in S3
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			log.Err("Object::ReadToFile : Failed to download object %s [%s]", name, err.Error())
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

	return err

}

// ReadBuffer : Download a specific range from a blob to a buffer
func (bb *S3Object) ReadBuffer(name string, offset int64, len int64) ([]byte, error) {
	log.Trace("BlockBlob::ReadBuffer : name %s", name)
	var buff []byte

	// If the len is 0, that means we need to read till the end of the object
	if len == 0 {
		attr, err := bb.GetAttr(name)
		if err != nil {
			return buff, err
		}
		buff = make([]byte, attr.Size)
		len = attr.Size
	} else {
		buff = make([]byte, len)
	}

	result, err := bb.Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bb.Config.authConfig.BucketName),
		Key:    aws.String(name),
		Range:  aws.String("bytes=" + fmt.Sprint(offset) + "-" + fmt.Sprint(len)),
	})

	if err != nil {
		// No such key found so object is not in S3
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			log.Err("Object::ReadBuffer : Failed to download object %s [%s]", name, err.Error())
			return buff, syscall.ENOENT
		}
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			// Range is incorrect
			code := apiErr.ErrorCode()
			if code == "InvalidRange" {
				log.Err("Object::ReadBuffer : Failed to download object %s [%s]", name, err.Error())
				return buff, syscall.ERANGE
			}
		}
		log.Err("Object::ReadBuffer : Failed to download object %s [%s]", name, err.Error())
		return buff, err
	}

	// Read the object into the buffer
	defer result.Body.Close()
	buff, _ = io.ReadAll(result.Body)

	return buff, nil
}

// ReadInBuffer : Download specific range from a file to a user provided buffer
func (bb *S3Object) ReadInBuffer(name string, offset int64, len int64, data []byte) error {
	log.Trace("Object::ReadInBuffer : name %s", name)
	result, err := bb.Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bb.Config.authConfig.BucketName),
		Key:    aws.String(name),
		Range:  aws.String("bytes=" + fmt.Sprint(offset) + "-" + fmt.Sprint(len)),
	})

	if err != nil {
		// No such key found so object is not in S3
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			log.Err("Object::ReadBuffer : Failed to download object %s [%s]", name, err.Error())
			return syscall.ENOENT
		}
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			// Range is incorrect
			code := apiErr.ErrorCode()
			if code == "InvalidRange" {
				log.Err("Object::ReadBuffer : Failed to download object %s [%s]", name, err.Error())
				return syscall.ERANGE
			}
		}
		log.Err("Object::ReadBuffer : Failed to download object %s [%s]", name, err.Error())
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

func (bb *S3Object) calculateBlockSize(name string, fileSize int64) (blockSize int64, err error) {
	return 0, nil
}

// track the progress of upload of blobs where every 100MB of data uploaded is being tracked. It also tracks the completion of upload
func trackUpload(name string, bytesTransferred int64, count int64, uploadPtr *int64) {
}

// WriteFromFile : Upload local file to blob
func (bb *S3Object) WriteFromFile(name string, metadata map[string]string, fi *os.File) (err error) {
	log.Trace("Object::WriteFromFile : name %s", name)
	//defer exectime.StatTimeCurrentBlock("WriteFromFile::WriteFromFile")()

	defer log.TimeTrack(time.Now(), "BlockBlob::WriteFromFile", name)

	var uploadPtr *int64 = new(int64)
	*uploadPtr = 1

	// TODO: Move this variable into the config file
	var partMiBs int64 = 16

	// get the size of the file
	stat, err := fi.Stat()
	if err != nil {
		log.Err("Object::WriteFromFile : Failed to get file size %s [%s]", name, err.Error())
		return err
	}

	// if the block size is not set then we configure it based on file size
	// if blockSize == 0 {
	// 	// based on file-size calculate block size
	// 	blockSize, err = bb.calculateBlockSize(name, stat.Size())
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	// TODO: Add md5 hash support in S3
	// Compute md5 of this file is requested by user
	// If file is uploaded in one shot (no blocks created) then server is populating md5 on upload automatically.
	// hence we take cost of calculating md5 only for files which are bigger in size and which will be converted to blocks.
	// md5sum := []byte{}
	// if bb.Config.updateMD5 && stat.Size() >= azblob.BlockBlobMaxUploadBlobBytes {
	// 	md5sum, err = getMD5(fi)
	// 	if err != nil {
	// 		// Md5 sum generation failed so set nil while uploading
	// 		log.Warn("Object::WriteFromFile : Failed to generate md5 of %s", name)
	// 		md5sum = []byte{0}
	// 	}
	// }

	// TODO: Is there a more elegant way to do this?
	// The aws-sdk-go does not seem to see to the end of the file
	// so let's seek to the start before uploading
	fi.Seek(0, 0)

	uploader := manager.NewUploader(bb.Client, func(u *manager.Uploader) {
		u.PartSize = partMiBs * 1024 * 1024
	})
	_, err = uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bb.Config.authConfig.BucketName),
		Key:    aws.String(name),
		Body:   fi,
	})

	// TODO: Add monitor tracking
	// if common.MonitorBfs() && stat.Size() > 0 {
	// 	uploadOptions.Progress = func(bytesTransferred int64) {
	// 		trackUpload(name, bytesTransferred, stat.Size(), uploadPtr)
	// 	}
	// }

	if err != nil {
		log.Err("Object::WriteFromFile : Failed to upload blob %s [%s]", name, err.Error())
		return err
	} else {
		log.Debug("Object::WriteFromFile : Upload complete of object %v", name)

		// store total bytes uploaded so far
		if stat.Size() > 0 {
			azStatsCollector.UpdateStats(stats_manager.Increment, bytesUploaded, stat.Size())
		}
	}

	return nil
}

// WriteFromBuffer : Upload from a buffer to a blob
func (bb *S3Object) WriteFromBuffer(name string, metadata map[string]string, data []byte) (err error) {
	largeBuffer := bytes.NewReader(data)
	// TODO: Move this variable into the config file
	var partMiBs int64 = 16
	uploader := manager.NewUploader(bb.Client, func(u *manager.Uploader) {
		u.PartSize = partMiBs * 1024 * 1024
	})
	retryCount := 5
	for i := 0; i < retryCount; i++ {
		_, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(bb.Config.authConfig.BucketName),
			Key:    aws.String(name),
			Body:   largeBuffer,
		})
		if err == nil {
			break
		}
	}
	if err != nil {
		log.Err("Couldn't upload object to %v:%v. Here's why: %v\n",
			bb.Config.authConfig.BucketName, name, err)
		return err
	}
	return nil
}

// GetFileBlockOffsets: store blocks ids and corresponding offsets
func (bb *S3Object) GetFileBlockOffsets(name string) (*common.BlockOffsetList, error) {
	return nil, nil
}

func (bb *S3Object) createBlock(blockIdLength, startIndex, size int64) *common.Block {
	return nil
}

// create new blocks based on the offset and total length we're adding to the file
func (bb *S3Object) createNewBlocks(blockList *common.BlockOffsetList, offset, length int64) int64 {
	return 0
}

func (bb *S3Object) removeBlocks(blockList *common.BlockOffsetList, size int64, name string) *common.BlockOffsetList {
	return nil
}

func (bb *S3Object) TruncateFile(name string, size int64) error {
	return nil
}

// Write : write data at given offset to a blob
func (bb *S3Object) Write(options internal.WriteFileOptions) error {
	name := options.Handle.Path
	offset := options.Offset
	defer log.TimeTrack(time.Now(), "BlockBlob::Write", options.Handle.Path)
	log.Trace("BlockBlob::Write : name %s offset %v", name, offset)
	// tracks the case where our offset is great than our current file size (appending only - not modifying pre-existing data)
	var dataBuffer *[]byte

	length := int64(len(options.Data))
	data := options.Data

	// get all the data
	oldData, _ := bb.ReadBuffer(name, 0, 0)
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
	err := bb.WriteFromBuffer(name, options.Metadata, *dataBuffer)
	if err != nil {
		log.Err("BlockBlob::Write : Failed to upload to blob %s ", name, err.Error())
		return err
	}
	return nil
}

// TODO: make a similar method facing stream that would enable us to write to cached blocks then stage and commit
func (bb *S3Object) stageAndCommitModifiedBlocks(name string, data []byte, offsetList *common.BlockOffsetList) error {
	return nil
}

func (bb *S3Object) StageAndCommit(name string, bol *common.BlockOffsetList) error {
	return nil
}

// ChangeMod : Change mode of a blob
func (bb *S3Object) ChangeMod(name string, _ os.FileMode) error {
	return nil
}

// ChangeOwner : Change owner of a blob
func (bb *S3Object) ChangeOwner(name string, _ int, _ int) error {
	return nil
}
