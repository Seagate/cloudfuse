//go:build !authtest
// +build !authtest

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
	"bytes"
	"container/list"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/handlemap"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var ctx = context.Background()

const MB = 1024 * 1024

func randomString(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func generateBucketName() string {
	return "fuseutc" + randomString(8)
}

func generateDirectoryName() string {
	return "dir" + randomString(8)
}

func generateFileName() string {
	return "file" + randomString(8)
}

type storageTestConfiguration struct {
	BucketName                string `json:"bucket-name"`
	KeyID                     string `json:"access-key"`
	SecretKey                 string `json:"secret-key"`
	Region                    string `json:"region"`
	Profile                   string `json:"profile"`
	Endpoint                  string `json:"endpoint"`
	Prefix                    string `json:"prefix"`
	RestrictedCharsWin        bool   `json:"restricted-characters-windows"`
	PartSizeMb                int64  `json:"part-size-mb"`
	UploadCutoffMb            int64  `json:"upload-cutoff-mb"`
	DisableConcurrentDownload bool   `json:"disable-concurrent-download"`
	UsePathStyle              bool   `json:"use-path-style"`
	DisableUsage              bool   `json:"disable-usage"`
	EnableDirMarker           bool   `json:"enable-dir-marker"`
	EnableChecksum            bool   `json:"enable-checksum"`
}

var storageTestConfigurationParameters storageTestConfiguration

type s3StorageTestSuite struct {
	suite.Suite
	assert      *assert.Assertions
	awsS3Client *s3.Client // S3 client library supplied by AWS
	s3Storage   *S3Storage
	config      string
	bucket      string
}

// s.uploadReaderAtToObject uploads a buffer in parts to an object.
func (s *s3StorageTestSuite) uploadReaderAtToObject(ctx context.Context, reader io.ReaderAt, readerSize int64,
	key string, partSizeMB int64) error {

	// If bufferSize > 5TB, then error
	if readerSize > 5*1024*common.GbToBytes {
		return errors.New("buffer is too large to upload to an object")
	}

	if partSizeMB < 5 { // If the block size is smaller than 5MB, round up to 5MB
		partSizeMB = 5
	}

	partSizeBytes := partSizeMB * common.MbToBytes

	if readerSize <= partSizeBytes {
		// If the size can fit in 1 Upload call, do it this way
		var body io.ReadSeeker = io.NewSectionReader(reader, 0, readerSize)
		_, err := s.awsS3Client.PutObject(context.Background(), &s3.PutObjectInput{
			Bucket:            aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
			Key:               aws.String(key),
			Body:              body,
			ContentType:       aws.String(getContentType(key)),
			ChecksumAlgorithm: s.s3Storage.stConfig.checksumAlgorithm,
		})
		return err
	}

	var numBlocks = int32(((readerSize - 1) / partSizeBytes) + 1)

	//send command to start copy and get the upload id as it is needed later
	var uploadID string
	createOutput, err := s.awsS3Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:            aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:               aws.String(key),
		ContentType:       aws.String(getContentType(key)),
		ChecksumAlgorithm: s.s3Storage.stConfig.checksumAlgorithm,
	})
	if err != nil {
		return err
	}
	if createOutput != nil {
		if createOutput.UploadId != nil {
			uploadID = *createOutput.UploadId
		}
	}
	if uploadID == "" {
		return err
	}

	var partNumber int32 = 1
	var checksumCRC32 *string
	var checksumCRC32C *string
	var checksumSHA256 *string
	var checksumSHA1 *string
	parts := make([]types.CompletedPart, 0)
	for partNumber <= numBlocks {
		endSize := partSizeBytes
		if partNumber == numBlocks {
			endSize = readerSize - int64(partNumber-1)*partSizeBytes
		}
		partResp, err := s.awsS3Client.UploadPart(context.Background(), &s3.UploadPartInput{
			Bucket:            aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
			Key:               aws.String(key),
			PartNumber:        &partNumber,
			UploadId:          &uploadID,
			Body:              io.NewSectionReader(reader, int64(partNumber-1)*partSizeBytes, endSize),
			ChecksumAlgorithm: s.s3Storage.stConfig.checksumAlgorithm,
		})

		checksumCRC32 = partResp.ChecksumCRC32
		checksumCRC32C = partResp.ChecksumCRC32C
		checksumSHA1 = partResp.ChecksumSHA1
		checksumSHA256 = partResp.ChecksumSHA256

		if err != nil {
			s.awsS3Client.AbortMultipartUpload(context.Background(), &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
				Key:      aws.String(key),
				UploadId: &uploadID,
			})

			// AWS states you need to call listparts to verify that multipart upload was properly aborted
			resp, _ := s.awsS3Client.ListParts(context.Background(), &s3.ListPartsInput{
				Bucket:   aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
				Key:      aws.String(key),
				UploadId: &uploadID,
			})
			if len(resp.Parts) != 0 {
				return errors.New("abort multipart upload failed and unable to delete all parts")
			}
			return err
		}

		// copy etag and part number to verify later
		if partResp != nil {
			partNum := partNumber
			etag := strings.Trim(*partResp.ETag, "\"")
			cPart := types.CompletedPart{
				ETag:           &etag,
				PartNumber:     &partNum,
				ChecksumCRC32:  checksumCRC32,
				ChecksumCRC32C: checksumCRC32C,
				ChecksumSHA1:   checksumSHA1,
				ChecksumSHA256: checksumSHA256,
			}
			parts = append(parts, cPart)
		}
		partNumber++
	}

	// complete the upload
	_, err = s.awsS3Client.CompleteMultipartUpload(context.Background(), &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:      aws.String(key),
		UploadId: &uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: parts,
		},
	})
	if err != nil {
		s.awsS3Client.AbortMultipartUpload(context.Background(), &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
			Key:      aws.String(key),
			UploadId: &uploadID,
		})

		// AWS states you need to call listparts to verify that multipart upload was properly aborted
		resp, _ := s.awsS3Client.ListParts(context.Background(), &s3.ListPartsInput{
			Bucket:   aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
			Key:      aws.String(key),
			UploadId: &uploadID,
		})
		if len(resp.Parts) != 0 {
			return errors.New("abort multipart upload failed and unable to delete all parts")
		}
		return err
	}

	return nil
}

func newTestS3Storage(configuration string) (*S3Storage, error) {
	err := config.ReadConfigFromReader(strings.NewReader(configuration))
	if err != nil {
		fmt.Printf("newTestS3Storage : ReadConfigFromReader failed. Here's why: %v\n", err)
		return nil, err
	}
	s3 := News3storageComponent()
	err = s3.Configure(true)

	return s3.(*S3Storage), err
}

func (s *s3StorageTestSuite) SetupTest() {
	// Logging config
	cfg := common.LogConfig{
		FilePath:    "./logfile.txt",
		MaxFileSize: 10,
		FileCount:   10,
		Level:       common.ELogLevel.LOG_DEBUG(),
	}
	err := log.SetDefaultLogger("base", cfg)
	if err != nil {
		fmt.Printf("s3StorageTestSuite::SetupTest : SetDefaultLogger failed. Here's why: %v\n", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Unable to get home directory")
		os.Exit(1)
	}
	cfgFile, err := os.Open(homeDir + "/s3test.json")
	if err != nil {
		fmt.Println("Unable to open config file")
		os.Exit(1)
	}

	cfgData, err := io.ReadAll(cfgFile)
	if err != nil {
		fmt.Println("Failed to read config file")
	}
	err = json.Unmarshal(cfgData, &storageTestConfigurationParameters)
	if err != nil {
		fmt.Println("Failed to parse the config file")
		os.Exit(1)
	}

	cfgFile.Close()
	s.setupTestHelper("", "", true)
}

func (s *s3StorageTestSuite) setupTestHelper(configuration string, bucket string, create bool) {
	// TODO: actually create a bucket for testing (gated by privileges)
	if bucket == "" {
		bucket = generateBucketName()
	}
	s.bucket = bucket
	storageTestConfigurationParameters.EnableDirMarker = true
	storageTestConfigurationParameters.EnableChecksum = true
	if configuration == "" {
		configuration = generateConfigYaml(storageTestConfigurationParameters)
	}
	s.config = configuration

	s.assert = assert.New(s.T())

	var err error
	s.s3Storage, err = newTestS3Storage(configuration)
	if err != nil {
		fmt.Printf("s3StorageTestSuite::setupTestHelper : newTestS3Storage failed. Here's why: %v\n", err)
	}
	err = s.s3Storage.Start(ctx)
	if err != nil {
		fmt.Printf("s3StorageTestSuite::setupTestHelper : s3Storage.Start failed. Here's why: %v\n", err)
	}

	s.awsS3Client = s.s3Storage.storage.(*Client).awsS3Client

	// set a prefix for testing
	// this is only necessary because our demo and our test share a single bucket
	// TODO: once we have a separate test bucket, or (preferably) the ability to create one in testing
	// 	remove this prefix so we can test operations at the root of the bucket
	if s.s3Storage.stConfig.prefixPath == "" {
		s.s3Storage.stConfig.prefixPath = "test" + randomString(8)
		err = s.s3Storage.storage.SetPrefixPath(s.s3Storage.stConfig.prefixPath)
		if err != nil {
			fmt.Printf("s3StorageTestSuite::setupTestHelper : SetPrefixPath failed. Here's why: %v\n", err)
		}
	}
}

func generateConfigYaml(testParams storageTestConfiguration) string {
	return fmt.Sprintf("s3storage:\n  bucket-name: %s\n  key-id: %s\n  secret-key: %s\n"+
		"  region: %s\n  profile: %s\n  endpoint: %s\n  subdirectory: %s\n  restricted-characters-windows: %t\n"+
		"  part-size-mb: %d\n  upload-cutoff-mb: %d\n  disable-concurrent-download: %t\n  use-path-style: %t\n  disable-usage: %t\n"+
		"  enable-dir-marker: %t\n  enable-checksum: %t\n",
		testParams.BucketName, testParams.KeyID, testParams.SecretKey,
		testParams.Region, testParams.Profile, testParams.Endpoint, testParams.Prefix, testParams.RestrictedCharsWin, testParams.PartSizeMb,
		testParams.UploadCutoffMb, testParams.DisableConcurrentDownload, testParams.UsePathStyle, testParams.DisableUsage,
		testParams.EnableDirMarker, testParams.EnableChecksum)
}

func (s *s3StorageTestSuite) tearDownTestHelper(delete bool) {
	err := s.s3Storage.Stop()
	if err != nil {
		fmt.Printf("s3StorageTestSuite::setupTestHelper : s3Storage.Stop failed. Here's why: %v\n", err)
	}
}

func (s *s3StorageTestSuite) cleanupTest() {
	err := s.s3Storage.DeleteDir(internal.DeleteDirOptions{Name: "/"})
	if err != nil {
		fmt.Printf("s3StorageTestSuite::cleanupTest : DeleteDir / failed. Here's why: %v\n", err)
	}
	s.tearDownTestHelper(true)
	err = log.Destroy()
	if err != nil {
		fmt.Printf("s3StorageTestSuite::cleanupTest : log.Destroy failed. Here's why: %v\n", err)
	}
	// Reset this flag if it was set.
	storageTestConfigurationParameters.RestrictedCharsWin = false
}

func (s *s3StorageTestSuite) TestDefault() {
	defer s.cleanupTest()
	// only test required parameters
	s.assert.Equal(storageTestConfigurationParameters.BucketName, s.s3Storage.stConfig.authConfig.BucketName)
	// TODO: Uncomment the following line when we have our own bucket and can remove the default test prefix path
	// s.assert.Empty(s.s3Storage.stConfig.prefixPath)
	s.assert.False(s.s3Storage.stConfig.restrictedCharsWin)
}

func (s *s3StorageTestSuite) TestListBuckets() {
	defer s.cleanupTest()

	// TODO: Fix this so we can create buckets
	// _, err := s.client.CreateBucket(context.Background(), &s3.CreateBucketInput{
	// 	Bucket: aws.String("lens-lab-test-create"),
	// 	CreateBucketConfiguration: &types.CreateBucketConfiguration{
	// 		LocationConstraint: types.BucketLocationConstraint("us-east-1"),
	// 	},
	// })
	// if err != nil {
	// 	fmt.Printf("Couldn't create bucket %v in Region %v. Here's why: %v\n",
	// 		"lens-lab-test-create", "us-east-1", err)
	// }

	buckets, err := s.s3Storage.ListBuckets()
	s.assert.NoError(err)
	s.assert.Contains(buckets, storageTestConfigurationParameters.BucketName)
}

func (s *s3StorageTestSuite) TestCreateDir() {
	defer s.cleanupTest()
	// Testing dir and dir/
	var paths = []string{generateDirectoryName(), generateDirectoryName() + "/"}
	for _, obj_path := range paths {
		log.Debug(obj_path)
		s.Run(obj_path, func() {
			err := s.s3Storage.CreateDir(internal.CreateDirOptions{Name: obj_path})

			s.assert.NoError(err)

			// Directory should be in the account
			key := internal.ExtendDirName(common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, obj_path))
			result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
				Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
				Key:          aws.String(key),
				ChecksumMode: types.ChecksumModeEnabled,
			})
			s.assert.NoError(err)
			s.assert.NotNil(result)
			s.assert.EqualValues(0, *result.ContentLength)
		})
	}
}

func (s *s3StorageTestSuite) TestDeleteDir() {
	defer s.cleanupTest()
	// Setup
	// Testing dir and dir/
	var paths = []string{generateDirectoryName(), generateDirectoryName() + "/"}
	for _, obj_path := range paths {
		log.Debug(obj_path)
		s.Run(obj_path, func() {
			err := s.s3Storage.CreateDir(internal.CreateDirOptions{Name: obj_path})
			s.assert.NoError(err)
			_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: path.Join(obj_path, generateFileName())})
			s.assert.NoError(err)

			// Directory should be in the account
			key := internal.ExtendDirName(common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, obj_path))
			_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
				Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
				Key:          aws.String(key),
				ChecksumMode: types.ChecksumModeEnabled,
			})
			s.assert.NoError(err)
			err = s.s3Storage.DeleteDir(internal.DeleteDirOptions{Name: obj_path})
			s.assert.NoError(err)

			s.assert.NoError(err)
			// Directory should not be in the account
			key = internal.ExtendDirName(common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, obj_path))
			_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
				Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
				Key:          aws.String(key),
				ChecksumMode: types.ChecksumModeEnabled,
			})
			s.assert.Error(err)

			// Directory be empty
			dirEmpty := s.s3Storage.IsDirEmpty(internal.IsDirEmptyOptions{Name: obj_path})
			s.assert.True(dirEmpty)
		})
	}
}

// Directory structure
// a/
//
//	 a/c1/
//	  a/c1/gc1
//		a/c2
//
// ab/
//
//	ab/c1
//
// ac
func generateNestedDirectory(path string) (*list.List, *list.List, *list.List) {
	aPaths := list.New()
	aPaths.PushBack(internal.TruncateDirName(path))

	aPaths.PushBack(path + "/c1")
	aPaths.PushBack(path + "/c2")
	aPaths.PushBack(path + "/c1" + "/gc1")

	abPaths := list.New()
	path = internal.TruncateDirName(path)
	abPaths.PushBack(path + "b")
	abPaths.PushBack(path + "b" + "/c1")

	acPaths := list.New()
	acPaths.PushBack(path + "c")

	return aPaths, abPaths, acPaths
}

func (s *s3StorageTestSuite) setupHierarchy(base string) (*list.List, *list.List, *list.List) {
	// Hierarchy looks as follows
	// a/
	//  a/c1/
	//   a/c1/gc1
	//	a/c2
	// ab/
	//  ab/c1
	// ac
	s.T().Helper()

	err := s.s3Storage.CreateDir(internal.CreateDirOptions{Name: base})
	s.assert.NoError(err)
	c1 := base + "/c1"
	err = s.s3Storage.CreateDir(internal.CreateDirOptions{Name: c1})
	s.assert.NoError(err)
	gc1 := c1 + "/gc1"
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: gc1})
	s.assert.NoError(err)
	c2 := base + "/c2"
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: c2})
	s.assert.NoError(err)
	abPath := base + "b"
	err = s.s3Storage.CreateDir(internal.CreateDirOptions{Name: abPath})
	s.assert.NoError(err)
	abc1 := abPath + "/c1"
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: abc1})
	s.assert.NoError(err)
	acPath := base + "c"
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: acPath})
	s.assert.NoError(err)

	a, ab, ac := generateNestedDirectory(base)

	// Validate the paths were setup correctly and all paths exist
	for p := a.Front(); p != nil; p = p.Next() {
		_, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.NoError(err)
	}
	for p := ab.Front(); p != nil; p = p.Next() {
		_, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.NoError(err)
	}
	for p := ac.Front(); p != nil; p = p.Next() {
		_, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.NoError(err)
	}
	return a, ab, ac
}

func (s *s3StorageTestSuite) TestDeleteDirHierarchy() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	a, ab, ac := s.setupHierarchy(base)

	err := s.s3Storage.DeleteDir(internal.DeleteDirOptions{Name: base})

	s.assert.NoError(err)

	/// a paths should be deleted
	for p := a.Front(); p != nil; p = p.Next() {
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.Error(err)
	}
	ab.PushBackList(ac) // ab and ac paths should exist
	for p := ab.Front(); p != nil; p = p.Next() {
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.NoError(err)
	}
}

func (s *s3StorageTestSuite) TestDeleteSubDirPrefixPath() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	a, ab, ac := s.setupHierarchy(base)

	err := s.s3Storage.storage.SetPrefixPath(common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, base))
	s.assert.NoError(err)

	attr, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: "c1"})
	s.assert.NoError(err)
	s.assert.NotNil(attr)
	s.assert.True(attr.IsDir())

	err = s.s3Storage.DeleteDir(internal.DeleteDirOptions{Name: "c1"})
	s.assert.NoError(err)

	err = s.s3Storage.storage.SetPrefixPath(s.s3Storage.stConfig.prefixPath)
	s.assert.NoError(err)
	// a paths under c1 should be deleted
	for p := a.Front(); p != nil; p = p.Next() {
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		if strings.HasPrefix(p.Value.(string), base+"/c1") {
			s.assert.Error(err)
		} else {
			s.assert.NoError(err)
		}
	}
	ab.PushBackList(ac) // ab and ac paths should exist
	for p := ab.Front(); p != nil; p = p.Next() {
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.NoError(err)
	}
}

func (s *s3StorageTestSuite) TestDeleteDirError() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()

	err := s.s3Storage.DeleteDir(internal.DeleteDirOptions{Name: name})

	// we have no way of indicating empty folders in the bucket
	// so deleting a non-existent directory should not cause an error
	s.assert.NoError(err)
	// Directory should not be in the account
	_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Error(err)
}

func (s *s3StorageTestSuite) TestIsDirEmpty() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()
	err := s.s3Storage.CreateDir(internal.CreateDirOptions{Name: name})
	s.assert.NoError(err)

	// Testing dir and dir/
	var paths = []string{name, name + "/"}
	for _, obj_path := range paths {
		log.Debug(obj_path)
		s.Run(obj_path, func() {
			empty := s.s3Storage.IsDirEmpty(internal.IsDirEmptyOptions{Name: name})

			s.assert.True(empty)
		})
	}
}

func (s *s3StorageTestSuite) TestIsDirEmptyNoDirectoryMarker() {
	// Setup
	storageTestConfigurationParameters.EnableDirMarker = false
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)
	defer s.cleanupTest()

	// Setup
	name := generateDirectoryName()
	err := s.s3Storage.CreateDir(internal.CreateDirOptions{Name: name})
	s.assert.NoError(err)

	// Testing dir and dir/
	var paths = []string{name, name + "/"}
	for _, obj_path := range paths {
		log.Debug(obj_path)
		s.Run(obj_path, func() {
			empty := s.s3Storage.IsDirEmpty(internal.IsDirEmptyOptions{Name: name})

			s.assert.True(empty)
		})
	}
}

func (s *s3StorageTestSuite) TestIsDirEmptyFalse() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()
	err := s.s3Storage.CreateDir(internal.CreateDirOptions{Name: name})
	s.assert.NoError(err)
	file := name + "/" + generateFileName()
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: file})
	s.assert.NoError(err)

	empty := s.s3Storage.IsDirEmpty(internal.IsDirEmptyOptions{Name: name})

	s.assert.False(empty)
}

func (s *s3StorageTestSuite) TestIsDirEmptyError() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()

	empty := s.s3Storage.IsDirEmpty(internal.IsDirEmptyOptions{Name: name})

	s.assert.True(empty)
	// Directory should not be in the account
}

func (s *s3StorageTestSuite) TestStreamDirNoVirtualDirectory() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()
	childName := name + "/" + generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: childName})
	s.assert.NoError(err)

	// Testing dir and dir/
	var paths = []string{"", "/"}
	for _, obj_path := range paths {
		log.Debug(obj_path)
		s.Run(obj_path, func() {
			entries, _, err := s.s3Storage.StreamDir(internal.StreamDirOptions{Name: obj_path})
			// this only works if the test can create an empty test bucket
			s.assert.NoError(err)
			s.assert.Len(entries, 1)
			s.assert.EqualValues(name, entries[0].Path)
			s.assert.EqualValues(name, entries[0].Name)
			s.assert.True(entries[0].IsDir())
			s.assert.True(entries[0].IsModeDefault())
		})
	}
}

func (s *s3StorageTestSuite) TestStreamDirHierarchy() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	s.setupHierarchy(base)

	// ReadDir only reads the first level of the hierarchy
	entries, _, err := s.s3Storage.StreamDir(internal.StreamDirOptions{Name: base})
	s.assert.NoError(err)
	s.assert.Len(entries, 2)
	// Check the dir
	s.assert.EqualValues(base+"/c1", entries[0].Path)
	s.assert.EqualValues("c1", entries[0].Name)
	s.assert.True(entries[0].IsDir())
	s.assert.True(entries[0].IsModeDefault())
	// Check the file
	s.assert.EqualValues(base+"/c2", entries[1].Path)
	s.assert.EqualValues("c2", entries[1].Name)
	s.assert.False(entries[1].IsDir())
	s.assert.True(entries[1].IsModeDefault())
}

func (s *s3StorageTestSuite) TestStreamDirRoot() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	s.setupHierarchy(base)

	// Testing dir and dir/
	var paths = []string{"", "/"}
	for _, obj_path := range paths {
		log.Debug(obj_path)
		s.Run(obj_path, func() {
			// ReadDir only reads the first level of the hierarchy
			entries, _, err := s.s3Storage.StreamDir(internal.StreamDirOptions{Name: obj_path})
			s.assert.NoError(err)
			s.assert.Len(entries, 3)
			// Check the base dir
			s.assert.EqualValues(base, entries[0].Path)
			s.assert.EqualValues(base, entries[0].Name)
			s.assert.True(entries[0].IsDir())
			s.assert.True(entries[0].IsModeDefault())
			// Check the baseb dir
			s.assert.EqualValues(base+"b", entries[1].Path)
			s.assert.EqualValues(base+"b", entries[1].Name)
			s.assert.True(entries[1].IsDir())
			s.assert.True(entries[1].IsModeDefault())
			// Check the basec file
			s.assert.EqualValues(base+"c", entries[2].Path)
			s.assert.EqualValues(base+"c", entries[2].Name)
			s.assert.False(entries[2].IsDir())
			s.assert.True(entries[2].IsModeDefault())
		})
	}
}

func (s *s3StorageTestSuite) TestStreamDirSubDir() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	s.setupHierarchy(base)

	// ReadDir only reads the first level of the hierarchy
	entries, _, err := s.s3Storage.StreamDir(internal.StreamDirOptions{Name: base + "/c1"})
	s.assert.NoError(err)
	s.assert.Len(entries, 1)
	// Check the dir
	s.assert.EqualValues(base+"/c1"+"/gc1", entries[0].Path)
	s.assert.EqualValues("gc1", entries[0].Name)
	s.assert.False(entries[0].IsDir())
	s.assert.True(entries[0].IsModeDefault())
}

func (s *s3StorageTestSuite) TestStreamDirSubDirPrefixPath() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	s.setupHierarchy(base)

	err := s.s3Storage.storage.SetPrefixPath(common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, base))
	s.assert.NoError(err)

	// ReadDir only reads the first level of the hierarchy
	entries, _, err := s.s3Storage.StreamDir(internal.StreamDirOptions{Name: "/c1"})
	s.assert.NoError(err)
	s.assert.Len(entries, 1)
	// Check the dir
	s.assert.EqualValues("c1"+"/gc1", entries[0].Path)
	s.assert.EqualValues("gc1", entries[0].Name)
	s.assert.False(entries[0].IsDir())
	s.assert.True(entries[0].IsModeDefault())
}

func (s *s3StorageTestSuite) TestStreamDirWindowsNameConvert() {
	// Skip test if not running on Windows
	if runtime.GOOS != "windows" {
		return
	}
	storageTestConfigurationParameters.RestrictedCharsWin = true
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()
	windowsDirName := "＂＊：＜＞？｜" + name
	err := s.s3Storage.CreateDir(internal.CreateDirOptions{Name: windowsDirName})
	s.assert.NoError(err)
	childName := generateFileName()
	windowsChildName := windowsDirName + "/" + childName + "＂＊：＜＞？｜"
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: windowsChildName})
	s.assert.NoError(err)

	// Testing dir and dir/
	var paths = []string{windowsDirName, windowsDirName + "/"}
	for _, obj_path := range paths {
		log.Debug(obj_path)
		s.Run(obj_path, func() {
			entries, _, err := s.s3Storage.StreamDir(internal.StreamDirOptions{Name: obj_path})
			s.assert.NoError(err)
			s.assert.Len(entries, 1)
			s.assert.Equal(windowsChildName, entries[0].Path)
		})
	}
}

func (s *s3StorageTestSuite) TestStreamDirError() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()

	entries, _, err := s.s3Storage.StreamDir(internal.StreamDirOptions{Name: name})

	s.assert.NoError(err) // Note: See comment in BlockBlob.List. BlockBlob behaves differently from Datalake
	s.assert.Empty(entries)
	// Directory should not be in the account
	_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Error(err)
}

func (s *s3StorageTestSuite) TestRenameDir() {
	defer s.cleanupTest()
	// Test handling "dir" and "dir/"
	var inputs = []struct {
		src string
		dst string
	}{
		{src: generateDirectoryName(), dst: generateDirectoryName()},
		{src: generateDirectoryName() + "/", dst: generateDirectoryName()},
		{src: generateDirectoryName(), dst: generateDirectoryName() + "/"},
		{src: generateDirectoryName() + "/", dst: generateDirectoryName() + "/"},
	}

	for _, input := range inputs {
		s.Run(input.src+"->"+input.dst, func() {
			// Setup
			err := s.s3Storage.CreateDir(internal.CreateDirOptions{Name: input.src})
			s.assert.NoError(err)

			_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: common.JoinUnixFilepath(input.src, generateFileName())})
			s.assert.NoError(err)

			err = s.s3Storage.RenameDir(internal.RenameDirOptions{Src: input.src, Dst: input.dst})
			s.assert.NoError(err)

			// Src should not be in the account
			_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: input.src})
			s.assert.Error(err)

			// Dst should be in the account
			_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: input.dst})
			s.assert.NoError(err)
		})
	}

}

func (s *s3StorageTestSuite) TestRenameDirHierarchy() {
	defer s.cleanupTest()
	// Setup
	baseSrc := generateDirectoryName()
	aSrc, abSrc, acSrc := s.setupHierarchy(baseSrc)
	baseDst := generateDirectoryName()
	aDst, abDst, acDst := generateNestedDirectory(baseDst)

	err := s.s3Storage.RenameDir(internal.RenameDirOptions{Src: baseSrc, Dst: baseDst})
	s.assert.NoError(err)

	// Source
	// aSrc paths should be deleted
	for p := aSrc.Front(); p != nil; p = p.Next() {
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.Error(err)
	}
	abSrc.PushBackList(acSrc) // abSrc and acSrc paths should exist
	for p := abSrc.Front(); p != nil; p = p.Next() {
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.NoError(err)
	}
	// Destination
	// aDst paths should exist
	for p := aDst.Front(); p != nil; p = p.Next() {
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.NoError(err)
	}
	abDst.PushBackList(acDst) // abDst and acDst paths should not exist
	for p := abDst.Front(); p != nil; p = p.Next() {
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.Error(err)
	}
}

func (s *s3StorageTestSuite) TestRenameDirSubDirPrefixPath() {
	defer s.cleanupTest()
	// Setup
	baseSrc := generateDirectoryName()
	aSrc, abSrc, acSrc := s.setupHierarchy(baseSrc)
	baseDst := generateDirectoryName()

	// Test rename directory with prefix set
	err := s.s3Storage.storage.SetPrefixPath(common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, baseSrc))
	s.assert.NoError(err)
	err = s.s3Storage.RenameDir(internal.RenameDirOptions{Src: "c1", Dst: baseDst})
	s.assert.NoError(err)

	// remove extra prefix to check results
	err = s.s3Storage.storage.SetPrefixPath(s.s3Storage.stConfig.prefixPath)
	s.assert.NoError(err)
	// aSrc paths under c1 should be deleted
	for p := aSrc.Front(); p != nil; p = p.Next() {
		path := p.Value.(string)
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: path})
		if strings.HasPrefix(path, baseSrc+"/c1") {
			s.assert.Error(err)
		} else {
			s.assert.NoError(err)
		}
	}
	abSrc.PushBackList(acSrc) // abSrc and acSrc paths should exist
	for p := abSrc.Front(); p != nil; p = p.Next() {
		path := p.Value.(string)
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: path})
		s.assert.NoError(err)
	}
	// Destination
	// aDst paths should exist -> aDst and aDst/gc1
	_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: baseSrc + "/" + baseDst})
	s.assert.NoError(err)
	_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: baseSrc + "/" + baseDst + "/gc1"})
	s.assert.NoError(err)
}

func (s *s3StorageTestSuite) TestRenameDirError() {
	defer s.cleanupTest()
	// Setup
	src := generateDirectoryName()
	dst := generateDirectoryName()

	err := s.s3Storage.RenameDir(internal.RenameDirOptions{Src: src, Dst: dst})

	// we have no way of indicating empty folders in the bucket
	// so renaming a non-existent directory should not cause an error
	s.assert.NoError(err)
	// Neither directory should be in the account
	_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: src})
	s.assert.Error(err)
	_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: dst})
	s.assert.Error(err)
}

func (s *s3StorageTestSuite) TestCreateFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})

	s.assert.NoError(err)
	s.assert.NotNil(h)
	s.assert.EqualValues(name, h.Path)
	s.assert.EqualValues(0, h.Size)
	// File should be in the account
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(key),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
	s.assert.NotNil(result)
}

func (s *s3StorageTestSuite) TestCreateFileWindowsNameConvert() {
	// Skip test if not running on Windows
	if runtime.GOOS != "windows" {
		return
	}
	storageTestConfigurationParameters.RestrictedCharsWin = true
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)
	defer s.cleanupTest()
	// Setup
	// Test with characters in folder and filepath
	name := generateFileName()
	windowsName := "＂＊：＜＞？｜" + "/" + name + "＂＊：＜＞？｜"
	objectName := "\"*:<>?|" + "/" + name + "\"*:<>?|"

	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: windowsName})

	s.assert.NoError(err)
	s.assert.NotNil(h)
	s.assert.EqualValues(windowsName, h.Path)
	s.assert.EqualValues(0, h.Size)
	// File should be in the account with the correct object key
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, objectName)
	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(key),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
	s.assert.NotNil(result)
}

func (s *s3StorageTestSuite) TestOpenFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)

	h, err := s.s3Storage.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.NoError(err)
	s.assert.NotNil(h)
	s.assert.EqualValues(name, h.Path)
	s.assert.EqualValues(0, h.Size)
}

func (s *s3StorageTestSuite) TestOpenFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	h, err := s.s3Storage.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.Error(err)
	s.assert.EqualValues(syscall.ENOENT, err)
	s.assert.Nil(h)
}

func (s *s3StorageTestSuite) TestOpenFileSize() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	size := 10
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	err = s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(size)})
	s.assert.NoError(err)

	// TODO: There is a sort of bug in S3 where writing zeros to the object causes it to be unreadable.
	// I think it's related to this link, but this discussion is about the key, whereas this is the value...
	h, err := s.s3Storage.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.NoError(err)
	s.assert.NotNil(h)
	s.assert.EqualValues(name, h.Path)
	s.assert.EqualValues(size, h.Size)
}

func (s *s3StorageTestSuite) TestCloseFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)

	// This method does nothing.
	err = s.s3Storage.CloseFile(internal.CloseFileOptions{Handle: h})
	s.assert.NoError(err)
}

func (s *s3StorageTestSuite) TestCloseFileFakeHandle() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)

	// This method does nothing.
	err := s.s3Storage.CloseFile(internal.CloseFileOptions{Handle: h})
	s.assert.NoError(err)
}

func (s *s3StorageTestSuite) TestDeleteFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)

	err = s.s3Storage.DeleteFile(internal.DeleteFileOptions{Name: name})
	s.assert.NoError(err)

	// This is similar to the s3 bucket command, use getObject for now
	//_, err = s.s3.GetAttr(internal.GetAttrOptions{name, false})
	// File should not be in the account
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(key),
		ChecksumMode: types.ChecksumModeEnabled,
	})

	s.assert.Error(err)
}

func (s *s3StorageTestSuite) TestDeleteFileWindowsNameConvert() {
	// Skip test if not running on Windows
	if runtime.GOOS != "windows" {
		return
	}
	defer s.cleanupTest()
	// Setup
	// Test with characters in folder and filepath
	name := generateFileName()
	windowsName := "＂＊：＜＞？｜" + "/" + name + "＂＊：＜＞？｜"
	objectName := "\"*:<>?|" + "/" + name + "\"*:<>?|"
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: windowsName})
	s.assert.NoError(err)

	err = s.s3Storage.DeleteFile(internal.DeleteFileOptions{Name: windowsName})
	s.assert.NoError(err)

	// File should not be in the account
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, objectName)
	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:    aws.String(key),
	})

	s.assert.Error(err)
}

func (s *s3StorageTestSuite) TestDeleteFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	err := s.s3Storage.DeleteFile(internal.DeleteFileOptions{Name: name})
	s.assert.Error(err)
	s.assert.EqualValues(syscall.ENOENT, err)

	// File should not be in the account
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(key),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.Error(err)
}

func (s *s3StorageTestSuite) TestCopyFromFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	testData := "test data"
	data := []byte(testData)
	homeDir, err := os.UserHomeDir()
	s.assert.NoError(err)
	f, err := os.CreateTemp(homeDir, name+".tmp")
	s.assert.NoError(err)
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	s.assert.NoError(err)

	err = s.s3Storage.CopyFromFile(internal.CopyFromFileOptions{Name: name, File: f})

	s.assert.NoError(err)

	// Object will be updated with new data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(key),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.EqualValues(testData, output)
}

func (s *s3StorageTestSuite) TestCopyFromFileWindowsNameConvert() {
	// Skip test if not running on Windows
	if runtime.GOOS != "windows" {
		return
	}
	storageTestConfigurationParameters.RestrictedCharsWin = true
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	windowsName := name + "＂＊：＜＞？｜"
	objectName := name + "\"*:<>?|"
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: windowsName})
	s.assert.NoError(err)
	testData := "test data"
	data := []byte(testData)
	homeDir, err := os.UserHomeDir()
	s.assert.NoError(err)
	f, err := os.CreateTemp(homeDir, windowsName+".tmp")
	s.assert.NoError(err)
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	s.assert.NoError(err)

	err = s.s3Storage.CopyFromFile(internal.CopyFromFileOptions{Name: windowsName, File: f})

	s.assert.NoError(err)

	// Object will be updated with new data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, objectName)
	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(key),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.EqualValues(testData, output)
}

func (s *s3StorageTestSuite) TestReadInBuffer() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	testData := "test data"
	data := []byte(testData)
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)
	h, err = s.s3Storage.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.NoError(err)

	output := make([]byte, 5)
	len, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.NoError(err)
	s.assert.EqualValues(5, len)
	s.assert.EqualValues(testData[:5], output)
}

func (s *s3StorageTestSuite) TestReadInBufferRange() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	testData := "test data test data "
	data := []byte(testData)
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)
	h, err = s.s3Storage.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.NoError(err)

	output := make([]byte, 15)
	len, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 5, Data: output})
	s.assert.NoError(err)
	s.assert.EqualValues(15, len)
	s.assert.EqualValues(testData[5:], output)
}

func (s *s3StorageTestSuite) TestReadInBufferLargeBuffer() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	testData := "test data"
	data := []byte(testData)
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)
	h, err = s.s3Storage.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.NoError(err)

	output := make([]byte, 1000) // Testing that passing in a super large buffer will still work
	len, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.NoError(err)
	s.assert.EqualValues(h.Size, len)
	s.assert.EqualValues(testData, output[:h.Size])
}

func (s *s3StorageTestSuite) TestReadInBufferEmpty() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)

	output := make([]byte, 10)
	len, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.NoError(err)
	s.assert.EqualValues(0, len)
}

func (s *s3StorageTestSuite) TestReadInBufferBadRange() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)
	h.Size = 10

	_, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 20, Data: make([]byte, 2)})
	s.assert.Error(err)
	s.assert.EqualValues(syscall.ERANGE, err)
}

func (s *s3StorageTestSuite) TestReadInBufferError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)
	h.Size = 10

	_, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: make([]byte, 2)})
	s.assert.Error(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *s3StorageTestSuite) TestWriteFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)

	testData := "test data"
	data := []byte(testData)
	count, err := s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)
	s.assert.EqualValues(len(data), count)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(key),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.EqualValues(testData, output)
}

func (s *s3StorageTestSuite) TestWriteFileMultipartUpload() {
	defer s.cleanupTest()
	storageTestConfigurationParameters.PartSizeMb = 5
	storageTestConfigurationParameters.UploadCutoffMb = 5
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)

	// Setup
	name := generateFileName()
	fileSize := 6 * MB
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	data := make([]byte, fileSize)
	rand.Read(data)

	count, err := s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)
	s.assert.EqualValues(len(data), count)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(key),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.EqualValues(data, output)

	// The etag in AWS indicates if it was uploaded as a multipart upload
	// After the etag there is a dash with the number of parts in the object
	res, err := s.awsS3Client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:    aws.String(key),
	})
	s.assert.NoError(err)
	etag := strings.Split(*res.ETag, "-")
	numParts, err := strconv.Atoi(strings.Trim(etag[1], "\""))
	s.assert.NoError(err)
	s.assert.Equal(2, numParts)
}

func (s *s3StorageTestSuite) TestWriteFileWindowsNameConvert() {
	// Skip test if not running on Windows
	if runtime.GOOS != "windows" {
		return
	}
	storageTestConfigurationParameters.RestrictedCharsWin = true
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)
	defer s.cleanupTest()
	// Setup
	// Test with characters in folder and filepath
	name := generateFileName()
	windowsName := "＂＊：＜＞？｜" + "/" + name + "＂＊：＜＞？｜"
	objectName := "\"*:<>?|" + "/" + name + "\"*:<>?|"
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: windowsName})
	s.assert.NoError(err)

	testData := "test data"
	data := []byte(testData)
	count, err := s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)
	s.assert.EqualValues(len(data), count)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, objectName)
	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(key),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.EqualValues(testData, output)
}

func (s *s3StorageTestSuite) TestTruncateSmallFileSmaller() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 5
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)

	err = s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.NoError(err)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(key),
		Range:        aws.String("bytes=0-" + fmt.Sprint(truncatedLength)),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.EqualValues(testData[:truncatedLength], output[:])
}

func (s *s3StorageTestSuite) TestTruncateSmallFileSmallerWindowsNameConvert() {
	// Skip test if not running on Windows
	if runtime.GOOS != "windows" {
		return
	}
	storageTestConfigurationParameters.RestrictedCharsWin = true
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	windowsName := "＂＊：＜＞？｜" + "/" + name + "＂＊：＜＞？｜"
	objectName := "\"*:<>?|" + "/" + name + "\"*:<>?|"
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: windowsName})
	s.assert.NoError(err)
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 5
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)

	err = s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: windowsName, Size: int64(truncatedLength)})
	s.assert.NoError(err)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, objectName)
	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(key),
		Range:        aws.String("bytes=0-" + fmt.Sprint(truncatedLength)),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.EqualValues(testData[:truncatedLength], output)
}

func (s *s3StorageTestSuite) TestTruncateChunkedFileSmaller() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 5
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)

	err = s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.NoError(err)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(key),
		Range:        aws.String("bytes=0-" + fmt.Sprint(truncatedLength)),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.EqualValues(testData[:truncatedLength], output)
}

func (s *s3StorageTestSuite) TestTruncateSmallFileEqual() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 9
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)

	err = s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.NoError(err)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(key),
		Range:        aws.String("bytes=0-" + fmt.Sprint(truncatedLength)),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.EqualValues(testData, output)
}

func (s *s3StorageTestSuite) TestTruncateChunkedFileEqual() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 9
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)

	err = s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.NoError(err)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(key),
		Range:        aws.String("bytes=0-" + fmt.Sprint(truncatedLength)),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.EqualValues(testData, output)
}

func (s *s3StorageTestSuite) TestTruncateSmallFileBigger() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 15
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)

	err = s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.NoError(err)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(key),
		Range:        aws.String("bytes=0-" + fmt.Sprint(truncatedLength)),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.EqualValues(testData, output[:len(data)])
}

func (s *s3StorageTestSuite) TestTruncateEmptyFileBigger() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	truncatedLength := 15

	err = s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.NoError(err)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(key),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.Len(output, truncatedLength)
	s.assert.EqualValues(make([]byte, truncatedLength), output[:])
}

func (s *s3StorageTestSuite) TestTruncateChunkedFileBigger() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 15
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)

	err = s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.NoError(err)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(key),
		Range:        aws.String("bytes=0-" + fmt.Sprint(truncatedLength)),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.EqualValues(testData, output[:len(data)])
}

func (s *s3StorageTestSuite) TestTruncateFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	err := s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name})
	s.assert.Error(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *s3StorageTestSuite) TestWriteSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	testData := "test data"
	data := []byte(testData)
	dataLen := len(data)
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)
	f, err := os.CreateTemp("", name+".tmp")
	s.assert.NoError(err)
	defer os.Remove(f.Name())

	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.NoError(err)

	output := make([]byte, len(data))
	f, err = os.Open(f.Name())
	s.assert.NoError(err)
	len, err := f.Read(output)
	s.assert.NoError(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(testData, output)
	f.Close()
}

func (s *s3StorageTestSuite) TestOverwriteSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	testData := "test-replace-data"
	data := []byte(testData)
	dataLen := len(data)
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)
	f, err := os.CreateTemp("", name+".tmp")
	s.assert.NoError(err)
	defer os.Remove(f.Name())
	newTestData := []byte("newdata")
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 5, Data: newTestData})
	s.assert.NoError(err)

	currentData := []byte("test-newdata-data")
	output := make([]byte, len(currentData))

	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.NoError(err)

	f, err = os.Open(f.Name())
	s.assert.NoError(err)
	len, err := f.Read(output)
	s.assert.NoError(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *s3StorageTestSuite) TestOverwriteAndAppendToSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	testData := "test-data"
	data := []byte(testData)

	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)
	f, err := os.CreateTemp("", name+".tmp")
	s.assert.NoError(err)
	defer os.Remove(f.Name())
	newTestData := []byte("newdata")
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 5, Data: newTestData})
	s.assert.NoError(err)

	currentData := []byte("test-newdata")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.NoError(err)

	f, err = os.Open(f.Name())
	s.assert.NoError(err)
	len, err := f.Read(output)
	s.assert.NoError(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *s3StorageTestSuite) TestAppendToSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	testData := "test-data"
	data := []byte(testData)

	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)
	f, err := os.CreateTemp("", name+".tmp")
	s.assert.NoError(err)
	defer os.Remove(f.Name())
	newTestData := []byte("-newdata")
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 9, Data: newTestData})
	s.assert.NoError(err)

	currentData := []byte("test-data-newdata")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.NoError(err)

	f, err = os.Open(f.Name())
	s.assert.NoError(err)
	len, err := f.Read(output)
	s.assert.NoError(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *s3StorageTestSuite) TestAppendOffsetLargerThanSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	testData := "test-data"
	data := []byte(testData)

	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)
	f, err := os.CreateTemp("", name+".tmp")
	s.assert.NoError(err)
	defer os.Remove(f.Name())
	newTestData := []byte("newdata")
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 12, Data: newTestData})
	s.assert.NoError(err)

	currentData := []byte("test-data\x00\x00\x00newdata")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.NoError(err)

	f, err = os.Open(f.Name())
	s.assert.NoError(err)
	len, err := f.Read(output)
	s.assert.NoError(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *s3StorageTestSuite) TestOverwriteBlocks() {
	// TODO Fix this test for localstack
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestEnvVarCredentials using LocalStack.")
		return
	}

	defer s.cleanupTest()
	blockSizeMB := 5
	storageTestConfigurationParameters.PartSizeMb = int64(blockSizeMB)
	storageTestConfigurationParameters.UploadCutoffMb = 5
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)

	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	data := make([]byte, 10*MB)
	rand.Read(data)

	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	err = s.uploadReaderAtToObject(ctx, bytes.NewReader(data), int64(len(data)), key, int64(blockSizeMB))
	s.assert.NoError(err)
	f, err := os.CreateTemp("", name+".tmp")
	s.assert.NoError(err)
	defer os.Remove(f.Name())
	newTestData := []byte("cake")
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 5, Data: newTestData})
	s.assert.NoError(err)

	dataLen := len(data)
	output := make([]byte, dataLen)

	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.NoError(err)

	f, err = os.Open(f.Name())
	s.assert.NoError(err)
	len, err := f.Read(output)
	s.assert.NoError(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(data[:5], output[:5])
	s.assert.EqualValues("cake", output[5:9])
	s.assert.EqualValues(data[9:], output[9:])
	f.Close()
}

func (s *s3StorageTestSuite) TestOverwriteAndAppendBlocks() {
	defer s.cleanupTest()
	blockSizeMB := 5
	storageTestConfigurationParameters.PartSizeMb = int64(blockSizeMB)
	storageTestConfigurationParameters.UploadCutoffMb = 5
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)

	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	data := make([]byte, 5*MB)
	rand.Read(data)

	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	err = s.uploadReaderAtToObject(ctx, bytes.NewReader(data), int64(len(data)), key, int64(blockSizeMB))
	s.assert.NoError(err)
	f, _ := os.CreateTemp("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("43211234cake")
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 5*MB - 4, Data: newTestData})
	s.assert.NoError(err)

	currentData := append(data[:len(data)-4], []byte("43211234cake")...)
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.NoError(err)

	f, err = os.Open(f.Name())
	s.assert.NoError(err)
	len, err := f.Read(output)
	s.assert.NoError(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *s3StorageTestSuite) TestAppendBlocks() {
	// TODO Fix this test for localstack
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestEnvVarCredentials using LocalStack.")
		return
	}

	defer s.cleanupTest()
	blockSizeMB := 5
	storageTestConfigurationParameters.PartSizeMb = int64(blockSizeMB)
	storageTestConfigurationParameters.UploadCutoffMb = 5
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)

	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	data := make([]byte, 5*MB)
	rand.Read(data)

	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	err = s.uploadReaderAtToObject(ctx, bytes.NewReader(data), int64(len(data)), key, int64(blockSizeMB))
	s.assert.NoError(err)
	f, _ := os.CreateTemp("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("43211234cake")
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 5 * MB, Data: newTestData})
	s.assert.NoError(err)

	currentData := append(data, []byte("43211234cake")...)
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.NoError(err)

	f, err = os.Open(f.Name())
	s.assert.NoError(err)
	len, err := f.Read(output)
	s.assert.NoError(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *s3StorageTestSuite) TestOverwriteAndAppendBlocksLargeFile() {
	// TODO Fix this test for localstack
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestEnvVarCredentials using LocalStack.")
		return
	}

	defer s.cleanupTest()
	blockSizeMB := 5
	storageTestConfigurationParameters.PartSizeMb = int64(blockSizeMB)
	storageTestConfigurationParameters.UploadCutoffMb = 5
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)

	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	data := make([]byte, 15*MB)
	rand.Read(data)

	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	err = s.uploadReaderAtToObject(ctx, bytes.NewReader(data), int64(len(data)), key, int64(blockSizeMB))
	s.assert.NoError(err)
	f, _ := os.CreateTemp("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("43211234cake")
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 15*MB - 4, Data: newTestData})
	s.assert.NoError(err)

	currentData := append(data[:len(data)-4], []byte("43211234cake")...)
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.NoError(err)

	f, err = os.Open(f.Name())
	s.assert.NoError(err)
	len, err := f.Read(output)
	s.assert.NoError(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *s3StorageTestSuite) TestOverwriteAndAppendBlocksMiddleLargeFile() {
	// TODO Fix this test for localstack
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestEnvVarCredentials using LocalStack.")
		return
	}

	defer s.cleanupTest()
	blockSizeMB := 5
	storageTestConfigurationParameters.PartSizeMb = int64(blockSizeMB)
	storageTestConfigurationParameters.UploadCutoffMb = 5
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)

	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	data := make([]byte, 15*MB)
	rand.Read(data)

	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	err = s.uploadReaderAtToObject(ctx, bytes.NewReader(data), int64(len(data)), key, int64(blockSizeMB))
	s.assert.NoError(err)
	f, _ := os.CreateTemp("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("43211234cake")
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 5*MB - 4, Data: newTestData})
	s.assert.NoError(err)

	currentData := append(data[:5*MB-4], []byte("43211234cake")...)
	currentData = append(currentData, data[5*MB+8:]...)
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.NoError(err)

	f, err = os.Open(f.Name())
	s.assert.NoError(err)
	len, err := f.Read(output)
	s.assert.NoError(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *s3StorageTestSuite) TestAppendOffsetLargerThanSize() {
	defer s.cleanupTest()
	// Setup
	storageTestConfigurationParameters.UploadCutoffMb = 5
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	testData := "testdatates1dat1tes2dat2tes3dat3tes4dat4"
	data := []byte(testData)

	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Data: data})
	s.assert.NoError(err)
	f, err := os.CreateTemp("", name+".tmp")
	s.assert.NoError(err)
	defer os.Remove(f.Name())
	newTestData := []byte("43211234cake")
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 45, Data: newTestData})
	s.assert.NoError(err)

	currentData := []byte("testdatates1dat1tes2dat2tes3dat3tes4dat4\x00\x00\x00\x00\x0043211234cake")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.NoError(err)

	f, err = os.Open(f.Name())
	s.assert.NoError(err)
	len, err := f.Read(output)
	s.assert.NoError(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *s3StorageTestSuite) TestCopyToFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	f, err := os.CreateTemp("", name+".tmp")
	s.assert.NoError(err)
	defer os.Remove(f.Name())

	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Error(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *s3StorageTestSuite) TestStreamDir() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()
	err := s.s3Storage.CreateDir(internal.CreateDirOptions{Name: name})
	s.assert.NoError(err)
	childName := name + "/" + generateFileName()
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: childName})
	s.assert.NoError(err)

	// Testing dir and dir/
	var paths = []string{name, name + "/"}
	for _, obj_path := range paths {
		log.Debug(obj_path)
		s.Run(obj_path, func() {
			entries, _, err := s.s3Storage.StreamDir(internal.StreamDirOptions{Name: obj_path})
			s.assert.NoError(err)
			s.assert.Len(entries, 1)
		})
	}
}

func (s *s3StorageTestSuite) TestStreamDirSmallCountNoDuplicates() {
	defer s.cleanupTest()
	// Setup
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: "TestStreamDirSmallCountNoDuplicates/object1.txt"})
	s.assert.NoError(err)
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: "TestStreamDirSmallCountNoDuplicates/object2.txt"})
	s.assert.NoError(err)
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: "TestStreamDirSmallCountNoDuplicates/newObject1.txt"})
	s.assert.NoError(err)
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: "TestStreamDirSmallCountNoDuplicates/newObject2.txt"})
	s.assert.NoError(err)
	err = s.s3Storage.CreateDir(internal.CreateDirOptions{Name: "TestStreamDirSmallCountNoDuplicates/myFolder"})
	s.assert.NoError(err)
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: "TestStreamDirSmallCountNoDuplicates/myFolder/newObjectA.txt"})
	s.assert.NoError(err)
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: "TestStreamDirSmallCountNoDuplicates/myFolder/newObjectB.txt"})
	s.assert.NoError(err)

	var iteration int // = 0
	var marker string // = ""
	objectList := make([]*internal.ObjAttr, 0)

	for {
		newList, nextMarker, err := s.s3Storage.StreamDir(internal.StreamDirOptions{Name: "TestStreamDirSmallCountNoDuplicates/", Token: marker, Count: 1})
		s.assert.NoError(err)
		objectList = append(objectList, newList...)
		marker = nextMarker
		iteration++

		log.Debug("s3StorageTestSuite::TestStreamDirSmallCountNoDuplicates : So far retrieved %d objects in %d iterations", len(objectList), iteration)
		if nextMarker == "" {
			break
		}
	}

	s.assert.Len(objectList, 5)
}

func (s *s3StorageTestSuite) TestRenameFile() {
	defer s.cleanupTest()
	// Setup
	src := generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: src})
	s.assert.NoError(err)
	dst := generateFileName()

	err = s.s3Storage.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	s.assert.NoError(err)

	// Src should not be in the account
	srcKey := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, src)
	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(srcKey),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.Error(err)
	// Dst should be in the account
	dstKey := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, dst)
	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(dstKey),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
}

func (s *s3StorageTestSuite) TestRenameFileWindowsNameConvert() {
	// Skip test if not running on Windows
	if runtime.GOOS != "windows" {
		return
	}
	storageTestConfigurationParameters.RestrictedCharsWin = true
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)
	defer s.cleanupTest()
	// Setup
	src := generateFileName()
	srcWindowsName := "＂＊：＜＞？｜" + "/" + src + "＂＊：＜＞？｜"
	srcObjectName := "\"*:<>?|" + "/" + src + "\"*:<>?|"
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: srcWindowsName})
	s.assert.NoError(err)

	dst := generateFileName()
	dstWindowsName := "＂＊：＜＞？｜" + "/" + dst + "＂＊：＜＞？｜"
	dstObjectName := "\"*:<>?|" + "/" + dst + "\"*:<>?|"

	err = s.s3Storage.RenameFile(internal.RenameFileOptions{Src: srcWindowsName, Dst: dstWindowsName})
	s.assert.NoError(err)

	// Src should not be in the account
	srcKey := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, srcObjectName)
	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(srcKey),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.Error(err)
	// Dst should be in the account
	dstKey := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, dstObjectName)
	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(dstKey),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
}

func (s *s3StorageTestSuite) TestRenameFileError() {
	defer s.cleanupTest()
	// Setup
	src := generateFileName()
	dst := generateFileName()

	err := s.s3Storage.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	s.assert.Error(err)
	s.assert.EqualValues(syscall.ENOENT, err)

	// Src and destination should not be in the account
	srcKey := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, src)
	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(srcKey),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.Error(err)
	dstKey := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, dst)
	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(dstKey),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.Error(err)
}

func (s *s3StorageTestSuite) TestCreateLink() {
	defer s.cleanupTest()
	// enable symlinks in config
	config := generateConfigYaml(storageTestConfigurationParameters) + "attr_cache:\n  enable-symlinks: true\n"
	s.setupTestHelper(config, s.bucket, true)
	s.assert.False(s.s3Storage.stConfig.disableSymlink)
	// Setup
	target := generateFileName()
	s.s3Storage.CreateFile(internal.CreateFileOptions{Name: target})
	name := generateFileName()

	err := s.s3Storage.CreateLink(internal.CreateLinkOptions{Name: name, Target: target})
	s.assert.NoError(err)

	// now we check the link exists
	attr, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.NoError(err)
	s.assert.NotNil(attr)
	s.assert.NotEmpty(attr.Metadata)
	s.assert.Contains(attr.Metadata, symlinkKey)
	s.assert.Equal("true", *attr.Metadata[symlinkKey])

	//download and make sure the data is correct
	result, err := s.s3Storage.ReadLink(internal.ReadLinkOptions{Name: name})
	s.assert.NoError(err)
	s.assert.Equal(target, result)
}

func (s *s3StorageTestSuite) TestCreateLinkDisabled() {
	defer s.cleanupTest()
	// Setup
	target := generateFileName()
	s.s3Storage.CreateFile(internal.CreateFileOptions{Name: target})
	name := generateFileName()
	notSupported := syscall.ENOTSUP

	err := s.s3Storage.CreateLink(internal.CreateLinkOptions{Name: name, Target: target})
	s.assert.Error(err)
	s.assert.EqualError(err, notSupported.Error())

	// link should not exist
	attr, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Nil(attr)
	s.assert.Error(err)
	s.assert.True(os.IsNotExist(err))
}

func (s *s3StorageTestSuite) TestReadLink() {
	defer s.cleanupTest()
	// enable symlinks in config
	config := generateConfigYaml(storageTestConfigurationParameters) + "attr_cache:\n  enable-symlinks: true\n"
	s.setupTestHelper(config, s.bucket, true)
	s.assert.False(s.s3Storage.stConfig.disableSymlink)
	// Setup
	target := generateFileName()

	s.s3Storage.CreateFile(internal.CreateFileOptions{Name: target})

	name := generateFileName()

	s.s3Storage.CreateLink(internal.CreateLinkOptions{Name: name, Target: target})

	read, err := s.s3Storage.ReadLink(internal.ReadLinkOptions{Name: name})
	s.assert.NoError(err)
	s.assert.EqualValues(target, read)
}

func (s *s3StorageTestSuite) TestReadLinkError() {
	defer s.cleanupTest()
	// enable symlinks in config
	config := generateConfigYaml(storageTestConfigurationParameters) + "attr_cache:\n  enable-symlinks: true\n"
	s.setupTestHelper(config, s.bucket, true)
	s.assert.False(s.s3Storage.stConfig.disableSymlink)
	// Setup
	name := generateFileName()

	_, err := s.s3Storage.ReadLink(internal.ReadLinkOptions{Name: name})
	s.assert.Error(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *s3StorageTestSuite) TestReadLinkDisabled() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	_, err := s.s3Storage.ReadLink(internal.ReadLinkOptions{Name: name})
	s.assert.Error(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *s3StorageTestSuite) TestGetAttrDir() {
	defer s.cleanupTest()
	// Setup
	dirName := generateDirectoryName()
	err := s.s3Storage.CreateDir(internal.CreateDirOptions{Name: dirName})
	s.assert.NoError(err)
	// Now we should be able to see the directory
	props, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: dirName})
	deleteError := s.s3Storage.DeleteDir(internal.DeleteDirOptions{Name: dirName})
	s.assert.NoError(err)
	s.assert.NotNil(props)
	s.assert.True(props.IsDir())
	s.assert.NoError(deleteError)
}

func (s *s3StorageTestSuite) TestGetAttrVirtualDir() {
	defer s.cleanupTest()
	// Setup
	dirName := generateFileName()
	name := dirName + "/" + generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)

	props, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: dirName})
	s.assert.NoError(err)
	s.assert.NotNil(props)
	s.assert.True(props.IsDir())
	s.assert.False(props.IsSymlink())

	// Check file in dir too
	props, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.NoError(err)
	s.assert.NotNil(props)
	s.assert.False(props.IsDir())
	s.assert.False(props.IsSymlink())
}

func (s *s3StorageTestSuite) TestGetAttrVirtualDirSubDir() {
	defer s.cleanupTest()
	// Setup
	dirName := generateFileName()
	subDirName := dirName + "/" + generateFileName()
	name := subDirName + "/" + generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)

	props, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: dirName})
	s.assert.NoError(err)
	s.assert.NotNil(props)
	s.assert.True(props.IsDir())
	s.assert.False(props.IsSymlink())

	// Check subdir in dir too
	props, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: subDirName})
	s.assert.NoError(err)
	s.assert.NotNil(props)
	s.assert.True(props.IsDir())
	s.assert.False(props.IsSymlink())

	// Check file in subdir too
	props, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.NoError(err)
	s.assert.NotNil(props)
	s.assert.False(props.IsDir())
	s.assert.False(props.IsSymlink())
}

func (s *s3StorageTestSuite) TestGetAttrFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)

	props, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.NoError(err)
	s.assert.NotNil(props)
	s.assert.False(props.IsDir())
	s.assert.False(props.IsSymlink())
}

func (s *s3StorageTestSuite) TestGetAttrLink() {
	defer s.cleanupTest()
	// enable symlinks in config
	config := generateConfigYaml(storageTestConfigurationParameters) + "attr_cache:\n  enable-symlinks: true\n"
	s.setupTestHelper(config, s.bucket, true)
	s.assert.False(s.s3Storage.stConfig.disableSymlink)
	// Setup
	target := generateFileName()
	s.s3Storage.CreateFile(internal.CreateFileOptions{Name: target})
	name := generateFileName()
	s.s3Storage.CreateLink(internal.CreateLinkOptions{Name: name, Target: target})

	props, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.NoError(err)
	s.assert.NotNil(props)
	s.assert.True(props.IsSymlink())
	s.assert.NotEmpty(props.Metadata)
	s.assert.Contains(props.Metadata, symlinkKey)
	s.assert.EqualValues("true", *props.Metadata[symlinkKey])
}

func (s *s3StorageTestSuite) TestGetAttrFileSize() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	testData := "test data"
	data := []byte(testData)
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)

	props, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.NoError(err)
	s.assert.NotNil(props)
	s.assert.False(props.IsDir())
	s.assert.False(props.IsSymlink())
	s.assert.EqualValues(len(testData), props.Size)
}

func (s *s3StorageTestSuite) TestGetAttrFileTime() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	testData := "test data"
	data := []byte(testData)
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)

	before, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.NoError(err)
	s.assert.NotNil(before.Mtime)

	time.Sleep(1 * time.Second) // Wait and then modify the file again

	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.NoError(err)

	after, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.NoError(err)
	s.assert.NotNil(after.Mtime)

	s.assert.True(after.Mtime.After(before.Mtime))
}

func (s *s3StorageTestSuite) TestGetAttrError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	_, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Error(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

// uploads data from a temp file and downloads the full object and tests the correct data was received
func (s *s3StorageTestSuite) TestFullRangedDownload() {
	defer s.cleanupTest()

	//create a temp file containing "test data"
	name := generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	data := []byte("test data")
	homeDir, err := os.UserHomeDir()
	s.assert.NoError(err)
	uploadFile, err := os.CreateTemp(homeDir, name+".tmp")
	s.assert.NoError(err)
	defer os.Remove(uploadFile.Name())
	_, err = uploadFile.Write(data)
	s.assert.NoError(err)

	// upload the temp file
	err = s.s3Storage.CopyFromFile(internal.CopyFromFileOptions{Name: name, File: uploadFile})
	s.assert.NoError(err)

	//create empty file for object download to write into
	file, err := os.CreateTemp("", generateFileName()+".tmp")
	s.assert.NoError(err)
	defer os.Remove(file.Name())

	//download to testDownload file
	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, Offset: 0, Count: 0, File: file})
	s.assert.NoError(err)

	//create byte array of characters that are identical to what we should have downloaded
	dataLen := len(data)
	output := make([]byte, dataLen) //empty byte array of that only holds 5 chars
	file, err = os.Open(file.Name())
	s.assert.NoError(err)

	//downloaded data in file is being read and dumped into the byte array.
	len, err := file.Read(output)

	s.assert.NoError(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(data, output)
}

// uploads data from a temp file. downloads a portion/range of that data from S3 and tests the correct range was received.
func (s *s3StorageTestSuite) TestRangedDownload() {
	defer s.cleanupTest()

	//create a temp file containing "test data"
	name := generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	data := []byte("test data")
	homeDir, err := os.UserHomeDir()
	s.assert.NoError(err)
	uploadFile, err := os.CreateTemp(homeDir, name+".tmp")
	s.assert.NoError(err)
	defer os.Remove(uploadFile.Name())
	_, err = uploadFile.Write(data)
	s.assert.NoError(err)

	// upload the temp file
	err = s.s3Storage.CopyFromFile(internal.CopyFromFileOptions{Name: name, File: uploadFile})
	s.assert.NoError(err)

	//create empty file for object download to write into
	file, err := os.CreateTemp("", generateFileName()+".tmp")
	s.assert.NoError(err)
	defer os.Remove(file.Name())

	//download portion of object to file
	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, Offset: 2, Count: 5, File: file})
	s.assert.NoError(err)

	//create byte array of characters that are identical to what we should have downloaded
	currentData := []byte("st da")
	dataLen := len(currentData)
	output := make([]byte, dataLen) //empty byte array of that only holds 5 chars
	file, err = os.Open(file.Name())
	s.assert.NoError(err)

	//downloaded data in file is being read and dumped into the byte array.
	len, err := file.Read(output)

	s.assert.NoError(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)

}

// uploads data from a temp file. downloads all except the first 3 bytes (based on offset) of that data from S3 and tests the correct portion was received.
func (s *s3StorageTestSuite) TestOffsetToEndDownload() {
	defer s.cleanupTest()

	//create a temp file containing "test data"
	name := generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)
	data := []byte("test data")
	homeDir, err := os.UserHomeDir()
	s.assert.NoError(err)
	uploadFile, err := os.CreateTemp(homeDir, name+".tmp")
	s.assert.NoError(err)
	defer os.Remove(uploadFile.Name())
	_, err = uploadFile.Write(data)
	s.assert.NoError(err)

	// upload the temp file
	err = s.s3Storage.CopyFromFile(internal.CopyFromFileOptions{Name: name, File: uploadFile})
	s.assert.NoError(err)

	//create empty file for object download to write into
	file, err := os.CreateTemp("", generateFileName()+".tmp")
	s.assert.NoError(err)
	defer os.Remove(file.Name())

	//download to testDownload file
	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, Offset: 3, Count: 0, File: file})
	s.assert.NoError(err)

	//create byte array of characters that are identical to what we should have downloaded
	currentData := []byte("t data")
	dataLen := len(currentData)
	output := make([]byte, dataLen) //empty byte array of that only holds 5 chars
	file, err = os.Open(file.Name())
	s.assert.NoError(err)

	//downloaded data in file is being read and dumped into the byte array.
	len, err := file.Read(output)

	s.assert.NoError(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)

}

func (s *s3StorageTestSuite) TestGetFileBlockOffsetsSmallFile() {
	defer s.cleanupTest()
	blockSizeMB := 5
	storageTestConfigurationParameters.PartSizeMb = int64(blockSizeMB)
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)

	// Setup
	name := generateFileName()
	h, _ := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "testdatates1dat1tes2dat2tes3dat3tes4dat4"
	data := []byte(testData)

	s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})

	// GetFileBlockOffsets
	offsetList, err := s.s3Storage.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	s.assert.NoError(err)
	s.assert.Empty(offsetList.BlockList)
	s.assert.True(offsetList.SmallFile())
	s.assert.EqualValues(0, offsetList.BlockIdLength)
}

func (s *s3StorageTestSuite) TestGetFileBlockOffsetsChunkedFile() {
	defer s.cleanupTest()
	blockSizeMB := 5
	storageTestConfigurationParameters.PartSizeMb = int64(blockSizeMB)
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)

	// Setup
	name := generateFileName()
	s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	data := make([]byte, 10*MB)
	rand.Read(data)

	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	err := s.uploadReaderAtToObject(context.Background(), bytes.NewReader(data), int64(len(data)), key, int64(blockSizeMB))
	s.assert.NoError(err)

	// GetFileBlockOffsets
	offsetList, err := s.s3Storage.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	s.assert.NoError(err)
	s.assert.Len(offsetList.BlockList, 2)
	s.assert.Zero(offsetList.Flags)
	s.assert.EqualValues(1, offsetList.BlockIdLength)
}

func (s *s3StorageTestSuite) TestGetFileBlockOffsetsError() {
	defer s.cleanupTest()
	blockSizeMB := 5
	storageTestConfigurationParameters.PartSizeMb = int64(blockSizeMB)
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)
	// Setup
	name := generateFileName()

	// GetFileBlockOffsets
	_, err := s.s3Storage.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	s.assert.Error(err)
}

func (s *s3StorageTestSuite) TestFlushFileEmptyFile() {
	defer s.cleanupTest()
	blockSizeMB := 5
	storageTestConfigurationParameters.PartSizeMb = int64(blockSizeMB)
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)

	// Setup
	name := generateFileName()
	h, _ := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})

	bol, _ := s.s3Storage.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(16*MB), h)
	h.CacheObj.BlockOffsetList = bol

	err := s.s3Storage.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.NoError(err)

	output := make([]byte, 1)
	length, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.NoError(err)
	s.assert.EqualValues(0, length)
	s.assert.EqualValues("", output[:length])
}

func (s *s3StorageTestSuite) TestFlushFileChunkedFile() {
	defer s.cleanupTest()
	blockSizeMB := 5
	storageTestConfigurationParameters.PartSizeMb = int64(blockSizeMB)
	storageTestConfigurationParameters.UploadCutoffMb = 5
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)

	// Setup
	name := generateFileName()
	h, _ := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	data := make([]byte, 15*MB)
	rand.Read(data)

	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	err := s.uploadReaderAtToObject(ctx, bytes.NewReader(data), int64(len(data)), key, int64(blockSizeMB))
	s.assert.NoError(err)
	bol, _ := s.s3Storage.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(15*MB), h)
	h.CacheObj.BlockOffsetList = bol
	s.assert.Len(bol.BlockList, 3)
	h.Size = 15 * MB

	err = s.s3Storage.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.NoError(err)

	output := make([]byte, 15*MB)
	length, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.NoError(err)
	s.assert.EqualValues(15*MB, length)
	s.assert.EqualValues(data, output)
}

func (s *s3StorageTestSuite) TestFlushFileUpdateChunkedFile() {
	// TODO Fix this test for localstack
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestEnvVarCredentials using LocalStack.")
		return
	}

	defer s.cleanupTest()
	blockSizeMB := 5
	blockSizeBytes := blockSizeMB * common.MbToBytes
	storageTestConfigurationParameters.PartSizeMb = int64(blockSizeMB)
	storageTestConfigurationParameters.UploadCutoffMb = 5
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)

	// Setup
	name := generateFileName()
	h, _ := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	data := make([]byte, 15*MB)
	rand.Read(data)

	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	err := s.uploadReaderAtToObject(ctx, bytes.NewReader(data), int64(len(data)), key, int64(blockSizeMB))
	s.assert.NoError(err)
	bol, _ := s.s3Storage.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(15*MB), h)
	h.CacheObj.BlockOffsetList = bol
	h.Size = 15 * MB

	updatedBlock := make([]byte, 2*MB)
	rand.Read(updatedBlock)
	h.CacheObj.BlockOffsetList.BlockList[1].Data = make([]byte, blockSizeBytes)
	s.s3Storage.storage.ReadInBuffer(name, int64(blockSizeBytes), int64(blockSizeBytes), h.CacheObj.BlockOffsetList.BlockList[1].Data)
	copy(h.CacheObj.BlockOffsetList.BlockList[1].Data[MB:2*MB+MB], updatedBlock)
	h.CacheObj.BlockOffsetList.BlockList[1].Flags.Set(common.DirtyBlock)

	err = s.s3Storage.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.NoError(err)

	output := make([]byte, 15*MB)
	length, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.NoError(err)
	s.assert.EqualValues(15*MB, length)
	s.assert.NotEqualValues(data, output)
	s.assert.EqualValues(data[:6*MB], output[:6*MB])
	s.assert.EqualValues(updatedBlock, output[6*MB:6*MB+2*MB])
	s.assert.EqualValues(data[8*MB:], output[8*MB:])
}

func (s *s3StorageTestSuite) TestFlushFileTruncateUpdateChunkedFile() {
	// TODO Fix this test for localstack
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestEnvVarCredentials using LocalStack.")
		return
	}

	defer s.cleanupTest()
	blockSizeMB := 5
	blockSizeBytes := blockSizeMB * common.MbToBytes
	storageTestConfigurationParameters.PartSizeMb = int64(blockSizeMB)
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)

	// Setup
	name := generateFileName()
	h, _ := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	data := make([]byte, 15*MB)
	rand.Read(data)

	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	err := s.uploadReaderAtToObject(ctx, bytes.NewReader(data), int64(len(data)), key, int64(blockSizeMB))
	s.assert.NoError(err)
	bol, _ := s.s3Storage.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(15*MB), h)
	h.CacheObj.BlockOffsetList = bol
	h.Size = 16 * MB

	// truncate block
	h.CacheObj.BlockOffsetList.BlockList[1].Data = make([]byte, blockSizeBytes/2)
	h.CacheObj.BlockOffsetList.BlockList[1].EndIndex = int64(blockSizeBytes + blockSizeBytes/2)
	s.s3Storage.storage.ReadInBuffer(name, int64(blockSizeBytes), int64(blockSizeBytes)/2, h.CacheObj.BlockOffsetList.BlockList[1].Data)
	h.CacheObj.BlockOffsetList.BlockList[1].Flags.Set(common.DirtyBlock)

	// remove 1 block
	h.CacheObj.BlockOffsetList.BlockList = h.CacheObj.BlockOffsetList.BlockList[:2]

	err = s.s3Storage.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.NoError(err)

	output := make([]byte, 16*MB)
	length, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.NoError(err)
	s.assert.EqualValues(16*MB, length)
	s.assert.NotEqualValues(data, output)
	s.assert.EqualValues(data[:7.5*MB], output[:7.5*MB])
}

func (s *s3StorageTestSuite) TestFlushFileAppendBlocksEmptyFile() {
	defer s.cleanupTest()
	blockSizeMB := 5
	blockSizeBytes := blockSizeMB * common.MbToBytes
	storageTestConfigurationParameters.PartSizeMb = int64(blockSizeMB)
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)

	// Setup
	name := generateFileName()
	h, _ := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})

	bol, _ := s.s3Storage.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(30*MB), h)
	h.CacheObj.BlockOffsetList = bol
	h.CacheObj.BlockIdLength = 16
	h.Size = int64(3 * blockSizeBytes)

	data1 := make([]byte, blockSizeBytes)
	rand.Read(data1)
	blk1 := &common.Block{
		StartIndex: 0,
		EndIndex:   int64(blockSizeMB * MB),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
		Data:       data1,
	}
	blk1.Flags.Set(common.DirtyBlock)

	data2 := make([]byte, blockSizeBytes)
	rand.Read(data2)
	blk2 := &common.Block{
		StartIndex: int64(blockSizeMB * MB),
		EndIndex:   2 * int64(blockSizeBytes),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
		Data:       data2,
	}
	blk2.Flags.Set(common.DirtyBlock)

	data3 := make([]byte, blockSizeBytes)
	rand.Read(data3)
	blk3 := &common.Block{
		StartIndex: 2 * int64(blockSizeBytes),
		EndIndex:   3 * int64(blockSizeBytes),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
		Data:       data3,
	}
	blk3.Flags.Set(common.DirtyBlock)
	h.CacheObj.BlockOffsetList.BlockList = append(h.CacheObj.BlockOffsetList.BlockList, blk1, blk2, blk3)
	bol.Flags.Clear(common.SmallFile)

	err := s.s3Storage.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.NoError(err)

	output := make([]byte, 3*blockSizeBytes)
	length, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.NoError(err)
	s.assert.EqualValues(3*blockSizeBytes, length)
	s.assert.EqualValues(blk1.Data, output[0:blockSizeBytes])
	s.assert.EqualValues(blk2.Data, output[blockSizeBytes:2*blockSizeBytes])
	s.assert.EqualValues(blk3.Data, output[2*blockSizeBytes:3*blockSizeBytes])
}

func (s *s3StorageTestSuite) TestFlushFileAppendBlocksChunkedFile() {
	// TODO Fix this test for localstack
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestEnvVarCredentials using LocalStack.")
		return
	}

	defer s.cleanupTest()
	blockSizeMB := 5
	blockSizeBytes := blockSizeMB * common.MbToBytes
	storageTestConfigurationParameters.PartSizeMb = int64(blockSizeMB)
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)

	// Setup
	name := generateFileName()
	fileSize := 30 * MB
	h, _ := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	data := make([]byte, fileSize)
	rand.Read(data)

	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	err := s.uploadReaderAtToObject(ctx, bytes.NewReader(data), int64(len(data)), key, int64(blockSizeMB))
	s.assert.NoError(err)
	bol, _ := s.s3Storage.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(30*MB), h)
	h.CacheObj.BlockOffsetList = bol
	h.CacheObj.BlockIdLength = 16
	h.Size = int64(fileSize + 3*blockSizeBytes)

	data1 := make([]byte, blockSizeBytes)
	rand.Read(data1)
	blk1 := &common.Block{
		StartIndex: int64(fileSize),
		EndIndex:   int64(fileSize + blockSizeBytes),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
		Data:       data1,
	}
	blk1.Flags.Set(common.DirtyBlock)

	data2 := make([]byte, blockSizeBytes)
	rand.Read(data2)
	blk2 := &common.Block{
		StartIndex: int64(fileSize + blockSizeBytes),
		EndIndex:   int64(fileSize + 2*blockSizeBytes),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
		Data:       data2,
	}
	blk2.Flags.Set(common.DirtyBlock)

	data3 := make([]byte, blockSizeBytes)
	rand.Read(data3)
	blk3 := &common.Block{
		StartIndex: int64(fileSize + 2*blockSizeBytes),
		EndIndex:   int64(fileSize + 3*blockSizeBytes),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
		Data:       data3,
	}
	blk3.Flags.Set(common.DirtyBlock)
	h.CacheObj.BlockOffsetList.BlockList = append(h.CacheObj.BlockOffsetList.BlockList, blk1, blk2, blk3)
	bol.Flags.Clear(common.SmallFile)

	err = s.s3Storage.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.NoError(err)

	output := make([]byte, fileSize+3*blockSizeBytes)
	length, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.NoError(err)
	s.assert.EqualValues(fileSize+3*blockSizeBytes, length)
	s.assert.EqualValues(data, output[0:fileSize])
	s.assert.EqualValues(blk1.Data, output[fileSize:fileSize+blockSizeBytes])
	s.assert.EqualValues(blk2.Data, output[fileSize+blockSizeBytes:fileSize+2*blockSizeBytes])
	s.assert.EqualValues(blk3.Data, output[fileSize+2*blockSizeBytes:fileSize+3*blockSizeBytes])
}

func (s *s3StorageTestSuite) TestFlushFileTruncateBlocksEmptyFile() {
	defer s.cleanupTest()
	blockSizeMB := 5
	blockSizeBytes := blockSizeMB * common.MbToBytes
	storageTestConfigurationParameters.PartSizeMb = int64(blockSizeMB)
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)

	// Setup
	name := generateFileName()
	h, _ := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})

	bol, _ := s.s3Storage.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(15*MB), h)
	h.CacheObj.BlockOffsetList = bol
	h.CacheObj.BlockIdLength = 16
	h.Size = int64(3 * int64(blockSizeBytes))

	blk1 := &common.Block{
		StartIndex: 0,
		EndIndex:   int64(blockSizeMB * MB),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk1.Flags.Set(common.TruncatedBlock)
	blk1.Flags.Set(common.DirtyBlock)

	blk2 := &common.Block{
		StartIndex: int64(blockSizeMB * MB),
		EndIndex:   2 * int64(blockSizeBytes),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk2.Flags.Set(common.TruncatedBlock)
	blk2.Flags.Set(common.DirtyBlock)

	blk3 := &common.Block{
		StartIndex: 2 * int64(blockSizeBytes),
		EndIndex:   3 * int64(blockSizeBytes),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk3.Flags.Set(common.TruncatedBlock)
	blk3.Flags.Set(common.DirtyBlock)
	h.CacheObj.BlockOffsetList.BlockList = append(h.CacheObj.BlockOffsetList.BlockList, blk1, blk2, blk3)
	bol.Flags.Clear(common.SmallFile)

	err := s.s3Storage.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.NoError(err)

	output := make([]byte, 3*int64(blockSizeBytes))
	length, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.NoError(err)
	s.assert.EqualValues(3*int64(blockSizeBytes), length)
	data := make([]byte, 3*blockSizeBytes)
	s.assert.EqualValues(data, output)
}

func (s *s3StorageTestSuite) TestFlushFileTruncateBlocksChunkedFile() {
	// TODO Fix this test for localstack
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestEnvVarCredentials using LocalStack.")
		return
	}

	defer s.cleanupTest()
	blockSizeMB := 5
	blockSizeBytes := blockSizeMB * common.MbToBytes
	storageTestConfigurationParameters.PartSizeMb = int64(blockSizeMB)
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)

	// Setup
	name := generateFileName()
	fileSize := 30 * MB
	h, _ := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	data := make([]byte, fileSize)
	rand.Read(data)

	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	err := s.uploadReaderAtToObject(ctx, bytes.NewReader(data), int64(len(data)), key, int64(blockSizeMB))
	s.assert.NoError(err)
	bol, _ := s.s3Storage.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(30*MB), h)
	h.CacheObj.BlockOffsetList = bol
	h.CacheObj.BlockIdLength = 16
	h.Size = int64(fileSize + 3*blockSizeBytes)

	blk1 := &common.Block{
		StartIndex: int64(fileSize),
		EndIndex:   int64(fileSize + blockSizeBytes),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk1.Flags.Set(common.TruncatedBlock)
	blk1.Flags.Set(common.DirtyBlock)

	blk2 := &common.Block{
		StartIndex: int64(fileSize + blockSizeBytes),
		EndIndex:   int64(fileSize + 2*blockSizeBytes),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk2.Flags.Set(common.TruncatedBlock)
	blk2.Flags.Set(common.DirtyBlock)

	blk3 := &common.Block{
		StartIndex: int64(fileSize + 2*blockSizeBytes),
		EndIndex:   int64(fileSize + 3*blockSizeBytes),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk3.Flags.Set(common.TruncatedBlock)
	blk3.Flags.Set(common.DirtyBlock)
	h.CacheObj.BlockOffsetList.BlockList = append(h.CacheObj.BlockOffsetList.BlockList, blk1, blk2, blk3)
	bol.Flags.Clear(common.SmallFile)

	err = s.s3Storage.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.NoError(err)

	output := make([]byte, fileSize+3*blockSizeBytes)
	length, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.NoError(err)
	s.assert.EqualValues(fileSize+3*blockSizeBytes, length)
	s.assert.EqualValues(data, output[:fileSize])
	emptyData := make([]byte, 3*blockSizeBytes)
	s.assert.EqualValues(emptyData, output[fileSize:])
}

func (s *s3StorageTestSuite) TestFlushFileAppendAndTruncateBlocksEmptyFile() {
	defer s.cleanupTest()
	blockSizeMB := 7
	blockSizeBytes := blockSizeMB * common.MbToBytes
	storageTestConfigurationParameters.PartSizeMb = int64(blockSizeMB)
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)

	// Setup
	name := generateFileName()
	h, _ := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})

	bol, _ := s.s3Storage.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(12*MB), h)
	h.CacheObj.BlockOffsetList = bol
	h.CacheObj.BlockIdLength = 16
	h.Size = int64(3 * blockSizeBytes)

	data1 := make([]byte, blockSizeBytes)
	rand.Read(data1)
	blk1 := &common.Block{
		StartIndex: 0,
		EndIndex:   int64(blockSizeMB * MB),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
		Data:       data1,
	}
	blk1.Flags.Set(common.DirtyBlock)

	blk2 := &common.Block{
		StartIndex: int64(blockSizeMB * MB),
		EndIndex:   2 * int64(blockSizeBytes),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk2.Flags.Set(common.DirtyBlock)
	blk2.Flags.Set(common.TruncatedBlock)

	blk3 := &common.Block{
		StartIndex: 2 * int64(blockSizeBytes),
		EndIndex:   3 * int64(blockSizeBytes),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk3.Flags.Set(common.DirtyBlock)
	blk3.Flags.Set(common.TruncatedBlock)
	h.CacheObj.BlockOffsetList.BlockList = append(h.CacheObj.BlockOffsetList.BlockList, blk1, blk2, blk3)
	bol.Flags.Clear(common.SmallFile)

	err := s.s3Storage.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.NoError(err)

	output := make([]byte, 3*blockSizeBytes)
	length, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.NoError(err)
	s.assert.EqualValues(3*blockSizeBytes, length)
	data := make([]byte, blockSizeBytes)
	s.assert.EqualValues(blk1.Data, output[0:blockSizeBytes])
	s.assert.EqualValues(data, output[blockSizeBytes:2*blockSizeBytes])
	s.assert.EqualValues(data, output[2*blockSizeBytes:3*blockSizeBytes])
}

func (s *s3StorageTestSuite) TestFlushFileAppendAndTruncateBlocksChunkedFile() {
	// TODO Fix this test for localstack
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestEnvVarCredentials using LocalStack.")
		return
	}

	defer s.cleanupTest()
	blockSizeMB := 7
	blockSizeBytes := blockSizeMB * common.MbToBytes
	storageTestConfigurationParameters.PartSizeMb = int64(blockSizeMB)
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(config, s.bucket, true)

	// Setup
	name := generateFileName()
	fileSize := 16 * MB
	h, _ := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	data := make([]byte, fileSize)
	rand.Read(data)

	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	err := s.uploadReaderAtToObject(ctx, bytes.NewReader(data), int64(len(data)), key, int64(blockSizeMB))
	s.assert.NoError(err)
	bol, _ := s.s3Storage.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(16*MB), h)
	h.CacheObj.BlockOffsetList = bol
	h.CacheObj.BlockIdLength = 16
	h.Size = int64(fileSize + 3*blockSizeBytes)

	data1 := make([]byte, blockSizeBytes)
	rand.Read(data1)
	blk1 := &common.Block{
		StartIndex: int64(fileSize),
		EndIndex:   int64(fileSize + blockSizeBytes),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
		Data:       data1,
	}
	blk1.Flags.Set(common.DirtyBlock)

	blk2 := &common.Block{
		StartIndex: int64(fileSize + blockSizeBytes),
		EndIndex:   int64(fileSize + 2*blockSizeBytes),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk2.Flags.Set(common.DirtyBlock)
	blk2.Flags.Set(common.TruncatedBlock)

	blk3 := &common.Block{
		StartIndex: int64(fileSize + 2*blockSizeBytes),
		EndIndex:   int64(fileSize + 3*blockSizeBytes),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk3.Flags.Set(common.DirtyBlock)
	blk3.Flags.Set(common.TruncatedBlock)
	h.CacheObj.BlockOffsetList.BlockList = append(h.CacheObj.BlockOffsetList.BlockList, blk1, blk2, blk3)
	bol.Flags.Clear(common.SmallFile)

	err = s.s3Storage.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.NoError(err)

	// file should be empty
	output := make([]byte, fileSize+3*blockSizeBytes)
	length, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.NoError(err)
	s.assert.EqualValues(fileSize+3*blockSizeBytes, length)
	s.assert.EqualValues(data, output[:fileSize])
	emptyData := make([]byte, blockSizeBytes)
	s.assert.EqualValues(blk1.Data, output[fileSize:fileSize+blockSizeBytes])
	s.assert.EqualValues(emptyData, output[fileSize+blockSizeBytes:fileSize+2*blockSizeBytes])
	s.assert.EqualValues(emptyData, output[fileSize+2*blockSizeBytes:fileSize+3*blockSizeBytes])
}

func (s *s3StorageTestSuite) TestUpdateConfig() {
	defer s.cleanupTest()

	s.s3Storage.storage.UpdateConfig(Config{
		partSize:     7 * MB,
		uploadCutoff: 15 * MB,
	})

	s.assert.EqualValues(7*MB, s.s3Storage.storage.(*Client).Config.partSize)
}

func (s *s3StorageTestSuite) TestTruncateSmallFileToSmaller() {
	s.UtilityFunctionTestTruncateFileToSmaller(2*MB, 1*MB)
}

func (s *s3StorageTestSuite) TestTruncateSmallFileToLarger() {
	s.UtilityFunctionTruncateFileToLarger(1*MB, 2*MB)
}

func (s *s3StorageTestSuite) TestTruncateBlockFileToSmaller() {
	s.UtilityFunctionTestTruncateFileToSmaller(10*MB, 8*MB)
}

func (s *s3StorageTestSuite) TestTruncateBlockFileToLarger() {
	s.UtilityFunctionTruncateFileToLarger(8*MB, 10*MB)
}

func (s *s3StorageTestSuite) TestTruncateNoBlockFileToLarger() {
	s.UtilityFunctionTruncateFileToLarger(10*MB, 20*MB)
}

func (s *s3StorageTestSuite) UtilityFunctionTestTruncateFileToSmaller(size int, truncatedLength int) {
	s.T().Helper()

	defer s.cleanupTest()
	// Setup
	storageTestConfigurationParameters.PartSizeMb = 5
	storageTestConfigurationParameters.UploadCutoffMb = 5
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.tearDownTestHelper(false)
	s.setupTestHelper(config, s.bucket, true)
	// // This is a little janky but required since testify suite does not support running setup or clean up for subtests.

	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)

	data := make([]byte, size)
	s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})

	err = s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.NoError(err)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(key),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.Len(output, truncatedLength)
	s.assert.EqualValues(data[:truncatedLength], output[:])
}

func (s *s3StorageTestSuite) UtilityFunctionTruncateFileToLarger(size int, truncatedLength int) {
	s.T().Helper()

	defer s.cleanupTest()
	// Setup
	storageTestConfigurationParameters.PartSizeMb = 5
	storageTestConfigurationParameters.UploadCutoffMb = 5
	config := generateConfigYaml(storageTestConfigurationParameters)
	s.tearDownTestHelper(false)
	s.setupTestHelper(config, s.bucket, true)

	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NoError(err)

	data := make([]byte, size)
	s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})

	err = s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.NoError(err)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:          aws.String(key),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.Len(output, truncatedLength)
	s.assert.EqualValues(data[:], output[:size])
}



func TestS3Storage(t *testing.T) {
	suite.Run(t, new(s3StorageTestSuite))
}
