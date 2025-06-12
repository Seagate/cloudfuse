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
	"net/url"
	"os"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/internal"

	"github.com/awnumar/memguard"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type Connection struct {
	Config   Config
	Endpoint *url.URL
}

type Config struct {
	authConfig                s3AuthConfig
	prefixPath                string
	restrictedCharsWin        bool
	partSize                  int64
	uploadCutoff              int64
	concurrency               int
	disableConcurrentDownload bool
	enableChecksum            bool
	checksumAlgorithm         types.ChecksumAlgorithm
	usePathStyle              bool
	disableSymlink            bool
	disableUsage              bool
	enableDirMarker           bool
}

// TODO: move s3AuthConfig to s3auth.go
// TODO: handle different types of authentication
// TODO: write tests in s3auth_test.go

// s3AuthConfig : Config to authenticate to storage
type s3AuthConfig struct {
	BucketName string
	KeyID      *memguard.Enclave
	SecretKey  *memguard.Enclave
	Region     string
	Profile    string
	Endpoint   string
}

// NewConnection : Create S3Connection Object
func NewConnection(cfg Config) (S3Connection, error) {
	stg := &Client{}
	err := stg.Configure(cfg)
	return stg, err
}

type S3Connection interface {
	Configure(cfg Config) error
	UpdateConfig(cfg Config) error

	ConnectionOkay() bool
	ListBuckets() ([]string, error)

	// This is just for test, shall not be used otherwise
	SetPrefixPath(string) error

	CreateFile(name string, mode os.FileMode) error
	CreateDirectory(name string) error
	CreateLink(source string, target string, isSymlink bool) error

	DeleteFile(name string) error
	DeleteDirectory(name string) error

	RenameFile(string, string, bool) error
	RenameDirectory(string, string) error

	GetAttr(name string) (attr *internal.ObjAttr, err error)

	// Standard operations to be supported by any account type
	List(prefix string, marker *string, count int32) ([]*internal.ObjAttr, *string, error)

	ReadToFile(name string, offset int64, count int64, fi *os.File) error
	ReadBuffer(name string, offset int64, length int64, isSymlink bool) ([]byte, error)
	ReadInBuffer(name string, offset int64, length int64, data []byte) error

	WriteFromFile(name string, metadata map[string]*string, fi *os.File) error
	WriteFromBuffer(name string, metadata map[string]*string, data []byte) error
	Write(options internal.WriteFileOptions) error
	GetFileBlockOffsets(name string) (*common.BlockOffsetList, error)

	TruncateFile(string, int64) error
	StageAndCommit(name string, bol *common.BlockOffsetList) error

	GetCommittedBlockList(string) (*internal.CommittedBlockList, error)
	StageBlock(string, []byte, string) error
	CommitBlocks(string, []string) error

	NewCredentialKey(_, _ string) error
	GetUsedSize() (uint64, error)
}
