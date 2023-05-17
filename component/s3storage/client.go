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
	"fmt"
	"io"
	"os"
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
	endpointResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if service == s3.ServiceID {
			// figure out the region
			if cl.Config.authConfig.Region == "" && region == "" {
				region = "us-east-1"
			}
			// figure out the endpoint URL
			var url string
			if cl.Config.authConfig.Endpoint != "" {
				url = cl.Config.authConfig.Endpoint
			} else {
				// TODO: default to another S3 provider
				switch region {
				case "us-east-1":
					url = "https://s3.us-east-1.lyvecloud.seagate.com"
				case "us-west-1":
					url = "https://s3.us-west-1.lyvecloud.seagate.com"
				case "ap-southeast-1":
					url = "https://s3.ap-southeast-1.lyvecloud.seagate.com"
				case "us-central-1":
					url = "https://s3.us-central-1.lyvecloud.seagate.com"
				case "eu-west-1":
					url = "https://s3.eu-west-1.lyvecloud.seagate.com"
				case "us-central-2":
					url = "https://s3.us-central-2.lyvecloud.seagate.com"
				default:
					return aws.Endpoint{}, fmt.Errorf("unrecognized region \"%s\"", region)
				}
				// save the results back to the config
				cl.Config.authConfig.Region = region
				cl.Config.authConfig.Endpoint = url
			}
			// create the endpoint
			return aws.Endpoint{
				PartitionID:   "aws",
				URL:           url,
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
		log.Err("Client::Configure : config.LoadDefaultConfig() failed. Here's why: %v", err)
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

// CreateFile : Create a new file in the bucket/virtual directory
func (cl *Client) CreateFile(name string, mode os.FileMode) error {
	log.Trace("Client::CreateFile : name %s", name)
	var data []byte
	return cl.WriteFromBuffer(name, false, data)
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
	symLink := true
	data := []byte(target)
	return cl.WriteFromBuffer(source, symLink, data)
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
func (cl *Client) RenameFile(source string, target string, symLink bool) error {
	log.Trace("Client::RenameFile : %s -> %s", source, target)

	err := cl.copyObject(source, target, symLink)
	if err != nil {
		log.Err("Client::RenameFile : copyObject(%s->%s) failed. Here's why: %v", source, target, err)
		return err
	}
	// Copy of the file is done so now delete the older file
	// in this case we don't need to check if the file exists, so we use deleteObject, not DeleteFile
	// this is what S3's DeleteObject spec is meant for: to make sure the object doesn't exist anymore
	err = cl.deleteObject(source, symLink)
	if err != nil {
		log.Err("Client::RenameFile : deleteObject(%s) failed. Here's why: %v", source, err)
	}

	return err
}

// RenameDirectory : Rename the directory
func (cl *Client) RenameDirectory(source string, target string) error {
	log.Trace("Client::RenameDirectory : %s -> %s", source, target)

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
// name should not have a trailing slash (nothing will be found!).
func (cl *Client) getFileAttr(name string) (*internal.ObjAttr, error) {
	log.Trace("Client::getFileAttr : name %s", name)
	isSymLink := false
	object, err := cl.headObject(name, isSymLink) //revisit
	if err == syscall.ENOENT {                    //if object does not exist in bucket, check if it is asymlink.
		isSymLink = true
		return cl.headObject(name, isSymLink)
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
		attr := createObjAttrDir(dirName)
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
func (cl *Client) ReadBuffer(name string, offset int64, len int64, isSymlink bool) ([]byte, error) {
	log.Trace("Client::ReadBuffer : name %s (%d+%d)", name, offset, len)
	// get object data
	objectDataReader, err := cl.getObject(name, offset, len, isSymlink)
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
func (cl *Client) ReadInBuffer(name string, offset int64, len int64, data []byte) error {
	log.Trace("Client::ReadInBuffer : name %s", name)
	// get object data
	objectDataReader, err := cl.getObject(name, offset, len, false)
	if err != nil {
		log.Err("Client::ReadInBuffer : getObject(%s) failed. Here's why: %v", name, err)
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
func (cl *Client) WriteFromFile(name string, fi *os.File) error {
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
	err = cl.putObject(name, fi, false)
	if err != nil {
		log.Err("Client::WriteFromFile : putObject(%s) failed. Here's why: %v", name, err)
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
func (cl *Client) WriteFromBuffer(name string, symLink bool, data []byte) error {
	log.Trace("Client::WriteFromBuffer : name %s", name)

	// convert byte array to io.Reader
	dataReader := bytes.NewReader(data)
	// upload data to object
	// TODO: handle metadata with S3
	err := cl.putObject(name, dataReader, symLink)
	log.Err("Client::WriteFromBuffer : putObject(%s) failed. Here's why: %v", name, err)
	return err
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
	err = cl.putObject(name, truncatedDataReader, false) //because I have to pass along the symLink bool, I have to go get it from getObject()
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

	// get the existing object data
	oldData, _ := cl.ReadBuffer(name, 0, 0, false)
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
	err := cl.WriteFromBuffer(name, false, *dataBuffer) //to replace options.Metadata with an object. should I create a new object or use list?
	if err != nil {
		log.Err("Client::Write : Failed to upload to object. Here's why: %v ", name, err)
		return err
	}
	return nil
}
