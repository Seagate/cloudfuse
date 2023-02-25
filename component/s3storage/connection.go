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
	"net/url"
	"os"

	"lyvecloudfuse/common"
	"lyvecloudfuse/internal"
)

// Example for azblob usage : https://godoc.org/github.com/Azure/azure-storage-blob-go/azblob#pkg-examples
// For methods help refer : https://godoc.org/github.com/Azure/azure-storage-blob-go/azblob#ContainerURL
type Config struct {
	authConfig s3AuthConfig
	prefixPath string
	// TODO: use a fake block size to improve streaming performance
	// 	even though S3 doesn't expose a block size
	blockSize int64
}

type Connection struct {
	Config Config

	Endpoint *url.URL
}

type S3Connection interface {
	Configure(cfg Config) error
	UpdateConfig(cfg Config) error

	ListBuckets() ([]string, error)

	// This is just for test, shall not be used otherwise
	SetPrefixPath(string) error

	CreateFile(name string, mode os.FileMode) error
	CreateDirectory(name string) error
	CreateLink(source string, target string) error

	DeleteFile(name string) error
	DeleteDirectory(name string) error

	RenameFile(string, string) error
	RenameDirectory(string, string) error

	GetAttr(name string) (attr *internal.ObjAttr, err error)

	// Standard operations to be supported by any account type
	List(prefix string, marker *string, count int32) ([]*internal.ObjAttr, *string, error)

	ReadToFile(name string, offset int64, count int64, fi *os.File) error
	ReadBuffer(name string, offset int64, len int64) ([]byte, error)
	ReadInBuffer(name string, offset int64, len int64, data []byte) error

	WriteFromFile(name string, metadata map[string]string, fi *os.File) error
	WriteFromBuffer(name string, metadata map[string]string, data []byte) error
	Write(options internal.WriteFileOptions) error
	GetFileBlockOffsets(name string) (*common.BlockOffsetList, error)

	TruncateFile(string, int64) error

	NewCredentialKey(_, _ string) error
}

// NewConnection : Based on account type create respective S3Connection Object
func NewConnection(cfg Config) S3Connection {
	stg := &Client{}
	_ = stg.Configure(cfg)
	return stg
}
