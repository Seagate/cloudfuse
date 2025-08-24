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
	"context"
	"net/url"
	"os"
	"time"

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
	healthCheckInterval       time.Duration
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

	ConnectionOkay(ctx context.Context) error
	ListBuckets(ctx context.Context) ([]string, error)
	ListAuthorizedBuckets(ctx context.Context) ([]string, error)

	// This is just for test, shall not be used otherwise
	SetPrefixPath(string) error

	CreateFile(ctx context.Context, name string, mode os.FileMode) error
	CreateDirectory(ctx context.Context, name string) error
	CreateLink(ctx context.Context, source string, target string, isSymlink bool) error

	DeleteFile(ctx context.Context, name string) error
	DeleteDirectory(ctx context.Context, name string) error

	RenameFile(ctx context.Context, source string, target string, isSymLink bool) error
	RenameDirectory(ctx context.Context, source string, target string) error

	GetAttr(ctx context.Context, name string) (attr *internal.ObjAttr, err error)

	// Standard operations to be supported by any account type
	List(
		ctx context.Context,
		prefix string,
		marker *string,
		count int32,
	) ([]*internal.ObjAttr, *string, error)

	ReadToFile(ctx context.Context, name string, offset int64, count int64, fi *os.File) error
	ReadBuffer(
		ctx context.Context,
		name string,
		offset int64,
		length int64,
		isSymlink bool,
	) ([]byte, error)
	ReadInBuffer(ctx context.Context, name string, offset int64, length int64, data []byte) error

	WriteFromFile(ctx context.Context, name string, metadata map[string]*string, fi *os.File) error
	WriteFromBuffer(
		ctx context.Context,
		name string,
		metadata map[string]*string,
		data []byte,
	) error
	Write(ctx context.Context, options internal.WriteFileOptions) error
	GetFileBlockOffsets(ctx context.Context, name string) (*common.BlockOffsetList, error)

	TruncateFile(ctx context.Context, name string, size int64) error
	StageAndCommit(ctx context.Context, name string, bol *common.BlockOffsetList) error

	GetCommittedBlockList(ctx context.Context, name string) (*internal.CommittedBlockList, error)
	StageBlock(name string, data []byte, id string) error
	CommitBlocks(ctx context.Context, name string, blockList []string) error

	NewCredentialKey(_, _ string) error
	GetUsedSize(ctx context.Context) (uint64, error)
}
