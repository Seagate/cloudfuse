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
	"os"

	"lyvecloudfuse/common"
	"lyvecloudfuse/common/log"
	"lyvecloudfuse/internal"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

	result, err := bb.Client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	if err != nil {
		fmt.Printf("Couldn't list buckets for your account. Here's why: %v\n", err)
	}

	cntList := make([]string, 0)
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

	var data []byte
	metadata := make(azblob.Metadata)
	metadata[folderKey] = "true"

	return bb.WriteFromBuffer(name, metadata, data)
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
func (bb *S3Object) RenameFile(source string, target string) error {
	return nil
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
	return nil, nil, nil
}

// track the progress of download of blobs where every 100MB of data downloaded is being tracked. It also tracks the completion of download
func trackDownload(name string, bytesTransferred int64, count int64, downloadPtr *int64) {
}

// ReadToFile : Download a blob to a local file
func (bb *S3Object) ReadToFile(name string, offset int64, count int64, fi *os.File) (err error) {
	return nil
}

// ReadBuffer : Download a specific range from a blob to a buffer
func (bb *S3Object) ReadBuffer(name string, offset int64, len int64) ([]byte, error) {
	return nil, nil
}

// ReadInBuffer : Download specific range from a file to a user provided buffer
func (bb *S3Object) ReadInBuffer(name string, offset int64, len int64, data []byte) error {
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
	return nil
}

// WriteFromBuffer : Upload from a buffer to a blob
func (bb *S3Object) WriteFromBuffer(name string, metadata map[string]string, data []byte) error {
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
