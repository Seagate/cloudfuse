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
	"os"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/vibhansa-msft/blobfilter"
)

// Example for azblob usage : https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/storage/azblob#pkg-examples
// For methods help refer : https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/storage/azblob#Client
type AzStorageConfig struct {
	authConfig azAuthConfig

	container      string
	prefixPath     string
	blockSize      int64
	maxConcurrency uint16

	// tier to be set on every upload
	defaultTier *blob.AccessTier

	// Return back readDir on mount for given amount of time
	cancelListForSeconds uint16

	// Retry policy config
	maxRetries            int32
	maxTimeout            int32
	backoffTime           int32
	maxRetryDelay         int32
	proxyAddress          string
	ignoreAccessModifiers bool
	mountAllContainers    bool

	updateMD5          bool
	validateMD5        bool
	virtualDirectory   bool
	maxResultsForList  int32
	disableCompression bool

	restrictedCharsWin bool
	telemetry          string
	honourACL          bool
	disableSymlink     bool
	preserveACL        bool

	// CPK related config
	cpkEnabled             bool
	cpkEncryptionKey       string
	cpkEncryptionKeySha256 string

	// Blob filters
	filter *blobfilter.BlobFilter
}

type AzStorageConnection struct {
	Config AzStorageConfig
}

type AzConnection interface {
	Configure(cfg AzStorageConfig) error
	UpdateConfig(cfg AzStorageConfig) error

	ConnectionOkay(ctx context.Context) error
	SetupPipeline() error
	TestPipeline() error

	ListContainers(ctx context.Context) ([]string, error)

	// This is just for test, shall not be used otherwise
	SetPrefixPath(string) error

	CreateFile(ctx context.Context, name string, mode os.FileMode) error
	CreateDirectory(ctx context.Context, name string) error
	CreateLink(ctx context.Context, source string, target string) error

	DeleteFile(ctx context.Context, name string) error
	DeleteDirectory(ctx context.Context, name string) error

	RenameFile(context.Context, string, string, *internal.ObjAttr) error
	RenameDirectory(context.Context, string, string) error

	GetAttr(ctx context.Context, name string) (attr *internal.ObjAttr, err error)

	// Standard operations to be supported by any account type
	List(
		ctx context.Context,
		prefix string,
		marker *string,
		count int32,
	) ([]*internal.ObjAttr, *string, error)

	ReadToFile(ctx context.Context, name string, offset int64, count int64, fi *os.File) error
	ReadBuffer(ctx context.Context, name string, offset int64, length int64) ([]byte, error)
	ReadInBuffer(ctx context.Context, name string, offset int64, length int64, data []byte, etag *string) error

	WriteFromFile(ctx context.Context, name string, metadata map[string]*string, fi *os.File) error
	WriteFromBuffer(
		ctx context.Context,
		name string,
		metadata map[string]*string,
		data []byte,
	) error
	Write(ctx context.Context, options internal.WriteFileOptions) error
	GetFileBlockOffsets(ctx context.Context, name string) (*common.BlockOffsetList, error)

	ChangeMod(context.Context, string, os.FileMode) error
	ChangeOwner(context.Context, string, int, int) error
	TruncateFile(context.Context, string, int64) error
	StageAndCommit(ctx context.Context, name string, bol *common.BlockOffsetList) error

	GetCommittedBlockList(context.Context, string) (*internal.CommittedBlockList, error)
	StageBlock(context.Context, string, []byte, string) error
	CommitBlocks(context.Context, string, []string, *string) error

	UpdateServiceClient(_, _ string) error

	SetFilter(string) error
}

// NewAzStorageConnection : Based on account type create respective AzConnection Object
func NewAzStorageConnection(cfg AzStorageConfig) AzConnection {
	if cfg.authConfig.AccountType == EAccountType.INVALID_ACC() {
		log.Err("NewAzStorageConnection : Invalid account type")
	} else if cfg.authConfig.AccountType == EAccountType.BLOCK() {
		stg := &BlockBlob{}
		_ = stg.Configure(cfg)
		return stg
	} else if cfg.authConfig.AccountType == EAccountType.ADLS() {
		stg := &Datalake{}
		_ = stg.Configure(cfg)
		return stg
	}

	return nil
}
