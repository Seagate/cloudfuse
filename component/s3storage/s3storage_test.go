//go:build !authtest && !unittest
// +build !authtest,!unittest

/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
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
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"strings"
	"syscall"
	"testing"
	"time"

	"lyvecloudfuse/common"
	"lyvecloudfuse/common/config"
	"lyvecloudfuse/common/log"
	"lyvecloudfuse/internal"
	"lyvecloudfuse/internal/handlemap"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var ctx = context.Background()

const MB = 1024 * 1024

func randomString(length int) string {
	rand.Seed(time.Now().UnixNano())
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
	BucketName string `json:"bucket-name"`
	KeyID      string `json:"access-key"`
	SecretKey  string `json:"secret-key"`
	Endpoint   string `json:"endpoint"`
	Region     string `json:"region"`
	Prefix     string `json:"prefix"`
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
		s.s3Storage.stConfig.prefixPath = "test"
		err = s.s3Storage.storage.SetPrefixPath(s.s3Storage.stConfig.prefixPath)
		if err != nil {
			fmt.Printf("s3StorageTestSuite::setupTestHelper : SetPrefixPath failed. Here's why: %v\n", err)
		}
	}
}

func generateConfigYaml(testParams storageTestConfiguration) string {
	return fmt.Sprintf("s3storage:\n  bucket-name: %s\n  key-id: %s\n  secret-key: %s\n"+
		"  endpoint: %s\n  region: %s\n  subdirectory: %s",
		testParams.BucketName, testParams.KeyID, testParams.SecretKey,
		testParams.Endpoint, testParams.Region, testParams.Prefix)
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
}

func (s *s3StorageTestSuite) TestListBuckets() {
	defer s.cleanupTest()

	// TODO: Fix this so we can create buckets
	// _, err := s.client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
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
	s.assert.Nil(err)
	s.assert.Equal(buckets, []string{"stxe1-srg-lens-lab1"})
}

func (s *s3StorageTestSuite) TestCreateDir() {
	defer s.cleanupTest()
	// Testing dir and dir/
	var paths = []string{generateDirectoryName(), generateDirectoryName() + "/"}
	for _, path := range paths {
		log.Debug(path)
		s.Run(path, func() {
			err := s.s3Storage.CreateDir(internal.CreateDirOptions{Name: path})
			// this does nothing, so just make sure it doesn't return an error
			s.assert.Nil(err)
		})
	}
}

func (s *s3StorageTestSuite) TestDeleteDir() {
	defer s.cleanupTest()
	// Setup
	dirName := generateDirectoryName()
	// A directory isn't created unless there is a file in that directory, therefore create a file with
	// 		the directory prefix instead of s.s3Storage.CreateDir(internal.CreateDirOptions{Name: name})
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: path.Join(dirName, generateFileName())})
	s.assert.Nil(err)

	err = s.s3Storage.DeleteDir(internal.DeleteDirOptions{Name: dirName})
	s.assert.Nil(err)

	// Directory should not be in the account
	dirEmpty := s.s3Storage.IsDirEmpty(internal.IsDirEmptyOptions{Name: dirName})
	s.assert.True(dirEmpty)
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
	err := s.s3Storage.CreateDir(internal.CreateDirOptions{Name: base})
	s.assert.Nil(err)
	c1 := base + "/c1"
	err = s.s3Storage.CreateDir(internal.CreateDirOptions{Name: c1})
	s.assert.Nil(err)
	gc1 := c1 + "/gc1"
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: gc1})
	s.assert.Nil(err)
	c2 := base + "/c2"
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: c2})
	s.assert.Nil(err)
	abPath := base + "b"
	err = s.s3Storage.CreateDir(internal.CreateDirOptions{Name: abPath})
	s.assert.Nil(err)
	abc1 := abPath + "/c1"
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: abc1})
	s.assert.Nil(err)
	acPath := base + "c"
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: acPath})
	s.assert.Nil(err)

	a, ab, ac := generateNestedDirectory(base)

	// Validate the paths were setup correctly and all paths exist
	for p := a.Front(); p != nil; p = p.Next() {
		_, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.Nil(err)
	}
	for p := ab.Front(); p != nil; p = p.Next() {
		_, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.Nil(err)
	}
	for p := ac.Front(); p != nil; p = p.Next() {
		_, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.Nil(err)
	}
	return a, ab, ac
}

func (s *s3StorageTestSuite) TestDeleteDirHierarchy() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	a, ab, ac := s.setupHierarchy(base)

	err := s.s3Storage.DeleteDir(internal.DeleteDirOptions{Name: base})

	s.assert.Nil(err)

	/// a paths should be deleted
	for p := a.Front(); p != nil; p = p.Next() {
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.NotNil(err)
	}
	ab.PushBackList(ac) // ab and ac paths should exist
	for p := ab.Front(); p != nil; p = p.Next() {
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.Nil(err)
	}
}

func (s *s3StorageTestSuite) TestDeleteSubDirPrefixPath() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	a, ab, ac := s.setupHierarchy(base)

	err := s.s3Storage.storage.SetPrefixPath(common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, base))
	s.assert.Nil(err)

	attr, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: "c1"})
	s.assert.Nil(err)
	s.assert.NotNil(attr)
	s.assert.True(attr.IsDir())

	err = s.s3Storage.DeleteDir(internal.DeleteDirOptions{Name: "c1"})
	s.assert.Nil(err)

	err = s.s3Storage.storage.SetPrefixPath(s.s3Storage.stConfig.prefixPath)
	s.assert.Nil(err)
	// a paths under c1 should be deleted
	for p := a.Front(); p != nil; p = p.Next() {
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		if strings.HasPrefix(p.Value.(string), base+"/c1") {
			s.assert.NotNil(err)
		} else {
			s.assert.Nil(err)
		}
	}
	ab.PushBackList(ac) // ab and ac paths should exist
	for p := ab.Front(); p != nil; p = p.Next() {
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.Nil(err)
	}
}

func (s *s3StorageTestSuite) TestDeleteDirError() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()

	err := s.s3Storage.DeleteDir(internal.DeleteDirOptions{Name: name})

	// we have no way of indicating empty folders in the bucket
	// so deleting a non-existent directory should not cause an error
	s.assert.Nil(err)
	// Directory should not be in the account
	_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.NotNil(err)
}

func (s *s3StorageTestSuite) TestIsDirEmpty() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()
	err := s.s3Storage.CreateDir(internal.CreateDirOptions{Name: name})
	s.assert.Nil(err)

	// Testing dir and dir/
	var paths = []string{name, name + "/"}
	for _, path := range paths {
		log.Debug(path)
		s.Run(path, func() {
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
	s.assert.Nil(err)
	file := name + "/" + generateFileName()
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: file})
	s.assert.Nil(err)

	empty := s.s3Storage.IsDirEmpty(internal.IsDirEmptyOptions{Name: name})

	s.assert.False(empty)
}

func (s *s3StorageTestSuite) TestReadDirNoVirtualDirectory() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()
	childName := name + "/" + generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: childName})
	s.assert.Nil(err)

	// Testing dir and dir/
	var paths = []string{"", "/"}
	for _, path := range paths {
		log.Debug(path)
		s.Run(path, func() {
			entries, err := s.s3Storage.ReadDir(internal.ReadDirOptions{Name: path})
			// this only works if the test can create an empty test bucket
			s.assert.Nil(err)
			s.assert.EqualValues(1, len(entries))
			s.assert.EqualValues(name, entries[0].Path)
			s.assert.EqualValues(name, entries[0].Name)
			s.assert.True(entries[0].IsDir())
			s.assert.True(entries[0].IsMetadataRetrieved())
			s.assert.True(entries[0].IsModeDefault())
		})
	}
}

func (s *s3StorageTestSuite) TestReadDirHierarchy() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	s.setupHierarchy(base)

	// ReadDir only reads the first level of the hierarchy
	entries, err := s.s3Storage.ReadDir(internal.ReadDirOptions{Name: base})
	s.assert.Nil(err)
	s.assert.EqualValues(2, len(entries))
	// Check the dir
	s.assert.EqualValues(base+"/c1", entries[0].Path)
	s.assert.EqualValues("c1", entries[0].Name)
	s.assert.True(entries[0].IsDir())
	s.assert.True(entries[0].IsMetadataRetrieved())
	s.assert.True(entries[0].IsModeDefault())
	// Check the file
	s.assert.EqualValues(base+"/c2", entries[1].Path)
	s.assert.EqualValues("c2", entries[1].Name)
	s.assert.False(entries[1].IsDir())
	s.assert.True(entries[1].IsMetadataRetrieved())
	s.assert.True(entries[1].IsModeDefault())
}

func (s *s3StorageTestSuite) TestReadDirRoot() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	s.setupHierarchy(base)

	// Testing dir and dir/
	var paths = []string{"", "/"}
	for _, path := range paths {
		log.Debug(path)
		s.Run(path, func() {
			// ReadDir only reads the first level of the hierarchy
			entries, err := s.s3Storage.ReadDir(internal.ReadDirOptions{Name: path})
			s.assert.Nil(err)
			s.assert.EqualValues(3, len(entries))
			// Check the base dir
			s.assert.EqualValues(base, entries[0].Path)
			s.assert.EqualValues(base, entries[0].Name)
			s.assert.True(entries[0].IsDir())
			s.assert.True(entries[0].IsMetadataRetrieved())
			s.assert.True(entries[0].IsModeDefault())
			// Check the baseb dir
			s.assert.EqualValues(base+"b", entries[1].Path)
			s.assert.EqualValues(base+"b", entries[1].Name)
			s.assert.True(entries[1].IsDir())
			s.assert.True(entries[1].IsMetadataRetrieved())
			s.assert.True(entries[1].IsModeDefault())
			// Check the basec file
			s.assert.EqualValues(base+"c", entries[2].Path)
			s.assert.EqualValues(base+"c", entries[2].Name)
			s.assert.False(entries[2].IsDir())
			s.assert.True(entries[2].IsMetadataRetrieved())
			s.assert.True(entries[2].IsModeDefault())
		})
	}
}

func (s *s3StorageTestSuite) TestReadDirSubDir() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	s.setupHierarchy(base)

	// ReadDir only reads the first level of the hierarchy
	entries, err := s.s3Storage.ReadDir(internal.ReadDirOptions{Name: base + "/c1"})
	s.assert.Nil(err)
	s.assert.EqualValues(1, len(entries))
	// Check the dir
	s.assert.EqualValues(base+"/c1"+"/gc1", entries[0].Path)
	s.assert.EqualValues("gc1", entries[0].Name)
	s.assert.False(entries[0].IsDir())
	s.assert.True(entries[0].IsMetadataRetrieved())
	s.assert.True(entries[0].IsModeDefault())
}

func (s *s3StorageTestSuite) TestReadDirSubDirPrefixPath() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	s.setupHierarchy(base)

	err := s.s3Storage.storage.SetPrefixPath(common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, base))
	s.assert.Nil(err)

	// ReadDir only reads the first level of the hierarchy
	entries, err := s.s3Storage.ReadDir(internal.ReadDirOptions{Name: "/c1"})
	s.assert.Nil(err)
	s.assert.EqualValues(1, len(entries))
	// Check the dir
	s.assert.EqualValues("c1"+"/gc1", entries[0].Path)
	s.assert.EqualValues("gc1", entries[0].Name)
	s.assert.False(entries[0].IsDir())
	s.assert.True(entries[0].IsMetadataRetrieved())
	s.assert.True(entries[0].IsModeDefault())
}

func (s *s3StorageTestSuite) TestReadDirError() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()

	entries, err := s.s3Storage.ReadDir(internal.ReadDirOptions{Name: name})

	s.assert.Nil(err) // Note: See comment in BlockBlob.List. BlockBlob behaves differently from Datalake
	s.assert.Empty(entries)
	// Directory should not be in the account
	_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.NotNil(err)
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
			// We don't keep track of empty directories, so let's create an object with the src prefix
			_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: common.JoinUnixFilepath(input.src, generateFileName())})
			s.assert.Nil(err)

			err = s.s3Storage.RenameDir(internal.RenameDirOptions{Src: input.src, Dst: input.dst})
			s.assert.Nil(err)

			// Src should not be in the account
			_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: input.src})
			s.assert.NotNil(err)

			// Dst should be in the account
			_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: input.dst})
			s.assert.Nil(err)
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
	s.assert.Nil(err)

	// Source
	// aSrc paths should be deleted
	for p := aSrc.Front(); p != nil; p = p.Next() {
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.NotNil(err)
	}
	abSrc.PushBackList(acSrc) // abSrc and acSrc paths should exist
	for p := abSrc.Front(); p != nil; p = p.Next() {
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.Nil(err)
	}
	// Destination
	// aDst paths should exist
	for p := aDst.Front(); p != nil; p = p.Next() {
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.Nil(err)
	}
	abDst.PushBackList(acDst) // abDst and acDst paths should not exist
	for p := abDst.Front(); p != nil; p = p.Next() {
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: p.Value.(string)})
		s.assert.NotNil(err)
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
	s.assert.Nil(err)
	err = s.s3Storage.RenameDir(internal.RenameDirOptions{Src: "c1", Dst: baseDst})
	s.assert.Nil(err)

	// remove extra prefix to check results
	err = s.s3Storage.storage.SetPrefixPath(s.s3Storage.stConfig.prefixPath)
	s.assert.Nil(err)
	// aSrc paths under c1 should be deleted
	for p := aSrc.Front(); p != nil; p = p.Next() {
		path := p.Value.(string)
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: path})
		if strings.HasPrefix(path, baseSrc+"/c1") {
			s.assert.NotNil(err)
		} else {
			s.assert.Nil(err)
		}
	}
	abSrc.PushBackList(acSrc) // abSrc and acSrc paths should exist
	for p := abSrc.Front(); p != nil; p = p.Next() {
		path := p.Value.(string)
		_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: path})
		s.assert.Nil(err)
	}
	// Destination
	// aDst paths should exist -> aDst and aDst/gc1
	_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: baseSrc + "/" + baseDst})
	s.assert.Nil(err)
	_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: baseSrc + "/" + baseDst + "/gc1"})
	s.assert.Nil(err)
}

func (s *s3StorageTestSuite) TestRenameDirError() {
	defer s.cleanupTest()
	// Setup
	src := generateDirectoryName()
	dst := generateDirectoryName()

	err := s.s3Storage.RenameDir(internal.RenameDirOptions{Src: src, Dst: dst})

	// we have no way of indicating empty folders in the bucket
	// so renaming a non-existent directory should not cause an error
	s.assert.Nil(err)
	// Neither directory should be in the account
	_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: src})
	s.assert.NotNil(err)
	_, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: dst})
	s.assert.NotNil(err)
}

func (s *s3StorageTestSuite) TestCreateFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})

	s.assert.Nil(err)
	s.assert.NotNil(h)
	s.assert.EqualValues(name, h.Path)
	s.assert.EqualValues(0, h.Size)
	// File should be in the account
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:    aws.String(key),
	})
	s.assert.Nil(err)
	s.assert.NotNil(result)
}

func (s *s3StorageTestSuite) TestOpenFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)

	h, err := s.s3Storage.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(h)
	s.assert.EqualValues(name, h.Path)
	s.assert.EqualValues(0, h.Size)
}

func (s *s3StorageTestSuite) TestOpenFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	h, err := s.s3Storage.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
	s.assert.Nil(h)
}

func (s *s3StorageTestSuite) TestOpenFileSize() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	size := 10
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	err = s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(size)})
	s.assert.Nil(err)

	// TODO: There is a sort of bug in S3 where writing zeros to the object causes it to be unreadable.
	// I think it's related to this link, but this discussion is about the key, whereas this is the value...
	// Is this another Lyve Cloud bug?
	h, err := s.s3Storage.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(h)
	s.assert.EqualValues(name, h.Path)
	s.assert.EqualValues(size, h.Size)
}

func (s *s3StorageTestSuite) TestCloseFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)

	// This method does nothing.
	err = s.s3Storage.CloseFile(internal.CloseFileOptions{Handle: h})
	s.assert.Nil(err)
}

func (s *s3StorageTestSuite) TestCloseFileFakeHandle() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)

	// This method does nothing.
	err := s.s3Storage.CloseFile(internal.CloseFileOptions{Handle: h})
	s.assert.Nil(err)
}

func (s *s3StorageTestSuite) TestDeleteFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)

	err = s.s3Storage.DeleteFile(internal.DeleteFileOptions{Name: name})
	s.assert.Nil(err)

	// This is similar to the s3 bucket command, use getObject for now
	//_, err = s.s3.GetAttr(internal.GetAttrOptions{name, false})
	// File should not be in the account
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	_, err = s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:    aws.String(key),
	})

	s.assert.NotNil(err)
}

func (s *s3StorageTestSuite) TestDeleteFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	err := s.s3Storage.DeleteFile(internal.DeleteFileOptions{Name: name})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)

	// File should not be in the account
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	_, err = s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:    aws.String(key),
	})
	s.assert.NotNil(err)
}

func (s *s3StorageTestSuite) TestCopyFromFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test data"
	data := []byte(testData)
	homeDir, err := os.UserHomeDir()
	s.assert.Nil(err)
	f, err := os.CreateTemp(homeDir, name+".tmp")
	s.assert.Nil(err)
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	s.assert.Nil(err)

	err = s.s3Storage.CopyFromFile(internal.CopyFromFileOptions{Name: name, File: f})

	s.assert.Nil(err)

	// Object will be updated with new data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:    aws.String(key),
	})
	s.assert.Nil(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.Nil(err)
	s.assert.EqualValues(testData, output)

}

func (s *s3StorageTestSuite) TestReadFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test data"
	data := []byte(testData)
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	h, err = s.s3Storage.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.Nil(err)

	output, err := s.s3Storage.ReadFile(internal.ReadFileOptions{Handle: h})
	s.assert.Nil(err)
	s.assert.EqualValues(testData, output)
}

func (s *s3StorageTestSuite) TestReadFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)

	_, err := s.s3Storage.ReadFile(internal.ReadFileOptions{Handle: h})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *s3StorageTestSuite) TestReadInBuffer() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test data"
	data := []byte(testData)
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	h, err = s.s3Storage.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.Nil(err)

	output := make([]byte, 5)
	len, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.Nil(err)
	s.assert.EqualValues(5, len)
	s.assert.EqualValues(testData[:5], output)
}

func (s *s3StorageTestSuite) TestReadInBufferRange() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test data test data "
	data := []byte(testData)
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	h, err = s.s3Storage.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.Nil(err)

	output := make([]byte, 15)
	len, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 5, Data: output})
	s.assert.Nil(err)
	s.assert.EqualValues(15, len)
	s.assert.EqualValues(testData[5:], output)
}

func (s *s3StorageTestSuite) TestReadInBufferRange1Byte() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test data test data "
	data := []byte(testData)
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	h, err = s.s3Storage.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.Nil(err)

	output := make([]byte, 1)
	len, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.Nil(err)
	s.assert.EqualValues(1, len)
	s.assert.EqualValues(testData[:1], output)
}

func (s *s3StorageTestSuite) TestReadInBufferLargeBuffer() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test data"
	data := []byte(testData)
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	h, err = s.s3Storage.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.Nil(err)

	output := make([]byte, 1000) // Testing that passing in a super large buffer will still work
	len, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.Nil(err)
	s.assert.EqualValues(h.Size, len)
	s.assert.EqualValues(testData, output[:h.Size])
}

func (s *s3StorageTestSuite) TestReadInBufferEmpty() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)

	output := make([]byte, 10)
	len, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.Nil(err)
	s.assert.EqualValues(0, len)
}

func (s *s3StorageTestSuite) TestReadInBufferBadRange() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)
	h.Size = 10

	_, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 20, Data: make([]byte, 2)})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ERANGE, err)
}

func (s *s3StorageTestSuite) TestReadInBufferError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)
	h.Size = 10

	_, err := s.s3Storage.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: make([]byte, 2)})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *s3StorageTestSuite) TestWriteFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)

	testData := "test data"
	data := []byte(testData)
	count, err := s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	s.assert.EqualValues(len(data), count)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:    aws.String(key),
	})
	s.assert.Nil(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.Nil(err)
	s.assert.EqualValues(testData, output)
}

func (s *s3StorageTestSuite) TestTruncateSmallFileSmaller() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 5
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)

	err = s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.Nil(err)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:    aws.String(key),
		Range:  aws.String("bytes=0-" + fmt.Sprint(truncatedLength)),
	})
	s.assert.Nil(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.Nil(err)
	s.assert.EqualValues(testData[:truncatedLength], output)
}

func (s *s3StorageTestSuite) TestTruncateChunkedFileSmaller() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 5
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)

	err = s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.Nil(err)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:    aws.String(key),
		Range:  aws.String("bytes=0-" + fmt.Sprint(truncatedLength)),
	})
	s.assert.Nil(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.Nil(err)
	s.assert.EqualValues(testData[:truncatedLength], output)
}

func (s *s3StorageTestSuite) TestTruncateSmallFileEqual() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 9
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)

	err = s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.Nil(err)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:    aws.String(key),
		Range:  aws.String("bytes=0-" + fmt.Sprint(truncatedLength)),
	})
	s.assert.Nil(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.Nil(err)
	s.assert.EqualValues(testData, output)
}

func (s *s3StorageTestSuite) TestTruncateChunkedFileEqual() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 9
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)

	err = s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.Nil(err)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:    aws.String(key),
		Range:  aws.String("bytes=0-" + fmt.Sprint(truncatedLength)),
	})
	s.assert.Nil(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.Nil(err)
	s.assert.EqualValues(testData, output)
}

func (s *s3StorageTestSuite) TestTruncateSmallFileBigger() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 15
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)

	err = s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.Nil(err)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:    aws.String(key),
		Range:  aws.String("bytes=0-" + fmt.Sprint(truncatedLength)),
	})
	s.assert.Nil(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.Nil(err)
	s.assert.EqualValues(testData, output[:len(data)])
}

func (s *s3StorageTestSuite) TestTruncateEmptyFileBigger() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	truncatedLength := 15

	err = s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.Nil(err)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:    aws.String(key),
	})
	s.assert.Nil(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.Nil(err)
	s.assert.EqualValues(truncatedLength, len(output))
	s.assert.EqualValues(make([]byte, truncatedLength), output[:])
}

func (s *s3StorageTestSuite) TestTruncateChunkedFileBigger() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 15
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)

	err = s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.Nil(err)

	// Object should have updated data
	key := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, name)
	result, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:    aws.String(key),
		Range:  aws.String("bytes=0-" + fmt.Sprint(truncatedLength)),
	})
	s.assert.Nil(err)
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.Nil(err)
	s.assert.EqualValues(testData, output[:len(data)])
}

func (s *s3StorageTestSuite) TestTruncateFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	err := s.s3Storage.TruncateFile(internal.TruncateFileOptions{Name: name})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *s3StorageTestSuite) TestWriteSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test data"
	data := []byte(testData)
	dataLen := len(data)
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, err := os.CreateTemp("", name+".tmp")
	s.assert.Nil(err)
	defer os.Remove(f.Name())

	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	output := make([]byte, len(data))
	f, err = os.Open(f.Name())
	s.assert.Nil(err)
	len, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(testData, output)
	f.Close()
}

func (s *s3StorageTestSuite) TestOverwriteSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test-replace-data"
	data := []byte(testData)
	dataLen := len(data)
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, err := os.CreateTemp("", name+".tmp")
	s.assert.Nil(err)
	defer os.Remove(f.Name())
	newTestData := []byte("newdata")
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 5, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("test-newdata-data")
	output := make([]byte, len(currentData))

	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, err = os.Open(f.Name())
	s.assert.Nil(err)
	len, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *s3StorageTestSuite) TestOverwriteAndAppendToSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test-data"
	data := []byte(testData)

	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, err := os.CreateTemp("", name+".tmp")
	s.assert.Nil(err)
	defer os.Remove(f.Name())
	newTestData := []byte("newdata")
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 5, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("test-newdata")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, err = os.Open(f.Name())
	s.assert.Nil(err)
	len, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *s3StorageTestSuite) TestAppendToSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test-data"
	data := []byte(testData)

	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, err := os.CreateTemp("", name+".tmp")
	s.assert.Nil(err)
	defer os.Remove(f.Name())
	newTestData := []byte("-newdata")
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 9, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("test-data-newdata")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, err = os.Open(f.Name())
	s.assert.Nil(err)
	len, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *s3StorageTestSuite) TestAppendOffsetLargerThanSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test-data"
	data := []byte(testData)

	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, err := os.CreateTemp("", name+".tmp")
	s.assert.Nil(err)
	defer os.Remove(f.Name())
	newTestData := []byte("newdata")
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 12, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("test-data\x00\x00\x00newdata")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, err = os.Open(f.Name())
	s.assert.Nil(err)
	len, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *s3StorageTestSuite) TestAppendOffsetLargerThanSize() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "testdatates1dat1tes2dat2tes3dat3tes4dat4"
	data := []byte(testData)

	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Data: data})
	s.assert.Nil(err)
	f, err := os.CreateTemp("", name+".tmp")
	s.assert.Nil(err)
	defer os.Remove(f.Name())
	newTestData := []byte("43211234cake")
	_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 45, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("testdatates1dat1tes2dat2tes3dat3tes4dat4\x00\x00\x00\x00\x0043211234cake")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, err = os.Open(f.Name())
	s.assert.Nil(err)
	len, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *s3StorageTestSuite) TestCopyToFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	f, err := os.CreateTemp("", name+".tmp")
	s.assert.Nil(err)
	defer os.Remove(f.Name())

	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *s3StorageTestSuite) TestReadDir() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()
	err := s.s3Storage.CreateDir(internal.CreateDirOptions{Name: name})
	s.assert.Nil(err)
	childName := name + "/" + generateFileName()
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: childName})
	s.assert.Nil(err)

	// Testing dir and dir/
	var paths = []string{name, name + "/"}
	for _, path := range paths {
		log.Debug(path)
		s.Run(path, func() {
			entries, err := s.s3Storage.ReadDir(internal.ReadDirOptions{Name: path})
			s.assert.Nil(err)
			s.assert.EqualValues(1, len(entries))
		})
	}
}

func (s *s3StorageTestSuite) TestStreamDirSmallCountNoDuplicates() {
	defer s.cleanupTest()
	// Setup
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: "TestStreamDirSmallCountNoDuplicates/object1.txt"})
	s.assert.Nil(err)
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: "TestStreamDirSmallCountNoDuplicates/object2.txt"})
	s.assert.Nil(err)
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: "TestStreamDirSmallCountNoDuplicates/newObject1.txt"})
	s.assert.Nil(err)
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: "TestStreamDirSmallCountNoDuplicates/newObject2.txt"})
	s.assert.Nil(err)
	err = s.s3Storage.CreateDir(internal.CreateDirOptions{Name: "TestStreamDirSmallCountNoDuplicates/myFolder"})
	s.assert.Nil(err)
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: "TestStreamDirSmallCountNoDuplicates/myFolder/newObjectA.txt"})
	s.assert.Nil(err)
	_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: "TestStreamDirSmallCountNoDuplicates/myFolder/newObjectB.txt"})
	s.assert.Nil(err)

	var iteration int // = 0
	var marker string // = ""
	objectList := make([]*internal.ObjAttr, 0)

	for {
		newList, newMarker, err := s.s3Storage.StreamDir(internal.StreamDirOptions{Name: "TestStreamDirSmallCountNoDuplicates/", Token: marker, Count: 1})
		s.assert.Nil(err)
		objectList = append(objectList, newList...)
		marker = newMarker
		iteration++

		log.Debug("s3StorageTestSuite::TestStreamDirSmallCountNoDuplicates : So far retrieved %d objects in %d iterations", len(objectList), iteration)
		if newMarker == "" {
			break
		}
	}

	s.assert.EqualValues(5, len(objectList))
}

func (s *s3StorageTestSuite) TestRenameFile() {
	defer s.cleanupTest()
	// Setup
	src := generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: src})
	s.assert.Nil(err)
	dst := generateFileName()

	err = s.s3Storage.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	s.assert.Nil(err)

	// Src should not be in the account
	srcKey := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, src)
	_, err = s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:    aws.String(srcKey),
	})
	s.assert.NotNil(err)
	// Dst should be in the account
	dstKey := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, dst)
	_, err = s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:    aws.String(dstKey),
	})
	s.assert.Nil(err)
}

func (s *s3StorageTestSuite) TestRenameFileError() {
	defer s.cleanupTest()
	// Setup
	src := generateFileName()
	dst := generateFileName()

	err := s.s3Storage.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)

	// Src and destination should not be in the account
	srcKey := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, src)
	_, err = s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:    aws.String(srcKey),
	})
	s.assert.NotNil(err)
	dstKey := common.JoinUnixFilepath(s.s3Storage.stConfig.prefixPath, dst)
	_, err = s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3Storage.storage.(*Client).Config.authConfig.BucketName),
		Key:    aws.String(dstKey),
	})
	s.assert.NotNil(err)
}

func (s *s3StorageTestSuite) TestCreateLink() {
	defer s.cleanupTest()
	// Setup
	target := generateFileName()
	s.s3Storage.CreateFile(internal.CreateFileOptions{Name: target})
	name := generateFileName()

	err := s.s3Storage.CreateLink(internal.CreateLinkOptions{Name: name, Target: target})
	s.assert.Nil(err)

	// now we check the link exists
	attr, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(attr)
	s.assert.NotEmpty(attr.Metadata)
	s.assert.Contains(attr.Metadata, symlinkKey)
	s.assert.Equal("true", attr.Metadata[symlinkKey])

	//download and make sure the data is correct
	result, err := s.s3Storage.ReadLink(internal.ReadLinkOptions{Name: name})
	s.assert.Nil(err)
	s.assert.Equal(target, result)
}

func (s *s3StorageTestSuite) TestReadLink() {
	defer s.cleanupTest()
	// Setup
	target := generateFileName()

	s.s3Storage.CreateFile(internal.CreateFileOptions{Name: target})

	name := generateFileName()

	s.s3Storage.CreateLink(internal.CreateLinkOptions{Name: name, Target: target})

	read, err := s.s3Storage.ReadLink(internal.ReadLinkOptions{Name: name})
	s.assert.Nil(err)
	s.assert.EqualValues(target, read)
}

func (s *s3StorageTestSuite) TestReadLinkError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	_, err := s.s3Storage.ReadLink(internal.ReadLinkOptions{Name: name})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *s3StorageTestSuite) TestGetAttrDir() {
	defer s.cleanupTest()
	vdConfig := generateConfigYaml(storageTestConfigurationParameters)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.bucket, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			dirName := generateDirectoryName()
			err := s.s3Storage.CreateDir(internal.CreateDirOptions{Name: dirName})
			s.assert.Nil(err)
			// since CreateDir doesn't do anything, let's put an object with that prefix
			filename := dirName + "/" + generateFileName()
			_, err = s.s3Storage.CreateFile(internal.CreateFileOptions{Name: filename})
			s.assert.Nil(err)
			// Now we should be able to see the directory
			props, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: dirName})
			deleteError := s.s3Storage.DeleteFile(internal.DeleteFileOptions{Name: filename})
			s.assert.Nil(err)
			s.assert.NotNil(props)
			s.assert.True(props.IsDir())
			s.assert.Nil(deleteError)
		})
	}
}

func (s *s3StorageTestSuite) TestGetAttrVirtualDir() {
	defer s.cleanupTest()
	vdConfig := generateConfigYaml(storageTestConfigurationParameters)
	// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
	s.tearDownTestHelper(false)
	s.setupTestHelper(vdConfig, s.bucket, true)
	// Setup
	dirName := generateFileName()
	name := dirName + "/" + generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)

	props, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: dirName})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.True(props.IsDir())
	s.assert.False(props.IsSymlink())

	// Check file in dir too
	props, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.False(props.IsDir())
	s.assert.False(props.IsSymlink())
}

func (s *s3StorageTestSuite) TestGetAttrVirtualDirSubDir() {
	defer s.cleanupTest()
	vdConfig := generateConfigYaml(storageTestConfigurationParameters)
	// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
	s.tearDownTestHelper(false)
	s.setupTestHelper(vdConfig, s.bucket, true)
	// Setup
	dirName := generateFileName()
	subDirName := dirName + "/" + generateFileName()
	name := subDirName + "/" + generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)

	props, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: dirName})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.True(props.IsDir())
	s.assert.False(props.IsSymlink())

	// Check subdir in dir too
	props, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: subDirName})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.True(props.IsDir())
	s.assert.False(props.IsSymlink())

	// Check file in subdir too
	props, err = s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.False(props.IsDir())
	s.assert.False(props.IsSymlink())
}

func (s *s3StorageTestSuite) TestGetAttrFile() {
	defer s.cleanupTest()
	vdConfig := generateConfigYaml(storageTestConfigurationParameters)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.bucket, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			name := generateFileName()
			_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
			s.assert.Nil(err)

			props, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
			s.assert.Nil(err)
			s.assert.NotNil(props)
			s.assert.False(props.IsDir())
			s.assert.False(props.IsSymlink())
		})
	}
}

func (s *s3StorageTestSuite) TestGetAttrLink() {
	defer s.cleanupTest()
	vdConfig := generateConfigYaml(storageTestConfigurationParameters)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.bucket, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			target := generateFileName()
			s.s3Storage.CreateFile(internal.CreateFileOptions{Name: target})
			name := generateFileName()
			s.s3Storage.CreateLink(internal.CreateLinkOptions{Name: name, Target: target})

			props, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
			s.assert.Nil(err)
			s.assert.NotNil(props)
			s.assert.True(props.IsSymlink())
			s.assert.NotEmpty(props.Metadata)
			s.assert.Contains(props.Metadata, symlinkKey)
			s.assert.EqualValues("true", props.Metadata[symlinkKey])
		})
	}
}

func (s *s3StorageTestSuite) TestGetAttrFileSize() {
	defer s.cleanupTest()
	vdConfig := generateConfigYaml(storageTestConfigurationParameters)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.bucket, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			name := generateFileName()
			h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
			s.assert.Nil(err)
			testData := "test data"
			data := []byte(testData)
			_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
			s.assert.Nil(err)

			props, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
			s.assert.Nil(err)
			s.assert.NotNil(props)
			s.assert.False(props.IsDir())
			s.assert.False(props.IsSymlink())
			s.assert.EqualValues(len(testData), props.Size)
		})
	}
}

func (s *s3StorageTestSuite) TestGetAttrFileTime() {
	defer s.cleanupTest()
	vdConfig := generateConfigYaml(storageTestConfigurationParameters)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.bucket, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			name := generateFileName()
			h, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
			s.assert.Nil(err)
			testData := "test data"
			data := []byte(testData)
			_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
			s.assert.Nil(err)

			before, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
			s.assert.Nil(err)
			s.assert.NotNil(before.Mtime)

			time.Sleep(time.Second * 3) // Wait 3 seconds and then modify the file again

			_, err = s.s3Storage.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
			s.assert.Nil(err)

			after, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
			s.assert.Nil(err)
			s.assert.NotNil(after.Mtime)

			s.assert.True(after.Mtime.After(before.Mtime))
		})
	}
}

func (s *s3StorageTestSuite) TestGetAttrError() {
	defer s.cleanupTest()
	vdConfig := generateConfigYaml(storageTestConfigurationParameters)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.bucket, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			name := generateFileName()

			_, err := s.s3Storage.GetAttr(internal.GetAttrOptions{Name: name})
			s.assert.NotNil(err)
			s.assert.EqualValues(syscall.ENOENT, err)
		})
	}
}

func TestS3Storage(t *testing.T) {
	suite.Run(t, new(s3StorageTestSuite))
}

// uploads data from a temp file and downloads the full object and tests the correct data was received
func (s *s3StorageTestSuite) TestFullRangedDownload() {
	defer s.cleanupTest()

	//create a temp file containing "test data"
	name := generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	data := []byte("test data")
	homeDir, err := os.UserHomeDir()
	s.assert.Nil(err)
	uploadFile, err := os.CreateTemp(homeDir, name+".tmp")
	s.assert.Nil(err)
	defer os.Remove(uploadFile.Name())
	_, err = uploadFile.Write(data)
	s.assert.Nil(err)

	// upload the temp file
	err = s.s3Storage.CopyFromFile(internal.CopyFromFileOptions{Name: name, File: uploadFile})
	s.assert.Nil(err)

	//create empty file for object download to write into
	file, err := os.CreateTemp("", generateFileName()+".tmp")
	s.assert.Nil(err)
	defer os.Remove(file.Name())

	//download to testDownload file
	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, Offset: 0, Count: 0, File: file})
	s.assert.Nil(err)

	//create byte array of characters that are identical to what we should have downloaded
	dataLen := len(data)
	output := make([]byte, dataLen) //empty byte array of that only holds 5 chars
	file, err = os.Open(file.Name())
	s.assert.Nil(err)

	//downloaded data in file is being read and dumped into the byte array.
	len, err := file.Read(output)

	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(data, output)
}

// uploads data from a temp file. downloads a portion/range of that data from S3 and tests the correct range was received.
func (s *s3StorageTestSuite) TestRangedDownload() {
	defer s.cleanupTest()

	//create a temp file containing "test data"
	name := generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	data := []byte("test data")
	homeDir, err := os.UserHomeDir()
	s.assert.Nil(err)
	uploadFile, err := os.CreateTemp(homeDir, name+".tmp")
	s.assert.Nil(err)
	defer os.Remove(uploadFile.Name())
	_, err = uploadFile.Write(data)
	s.assert.Nil(err)

	// upload the temp file
	err = s.s3Storage.CopyFromFile(internal.CopyFromFileOptions{Name: name, File: uploadFile})
	s.assert.Nil(err)

	//create empty file for object download to write into
	file, err := os.CreateTemp("", generateFileName()+".tmp")
	s.assert.Nil(err)
	defer os.Remove(file.Name())

	//download portion of object to file
	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, Offset: 2, Count: 5, File: file})
	s.assert.Nil(err)

	//create byte array of characters that are identical to what we should have downloaded
	currentData := []byte("st da")
	dataLen := len(currentData)
	output := make([]byte, dataLen) //empty byte array of that only holds 5 chars
	file, err = os.Open(file.Name())
	s.assert.Nil(err)

	//downloaded data in file is being read and dumped into the byte array.
	len, err := file.Read(output)

	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)

}

// uploads data from a temp file. downloads all except the first 3 bytes (based on offset) of that data from S3 and tests the correct portion was received.
func (s *s3StorageTestSuite) TestOffsetToEndDownload() {
	defer s.cleanupTest()

	//create a temp file containing "test data"
	name := generateFileName()
	_, err := s.s3Storage.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	data := []byte("test data")
	homeDir, err := os.UserHomeDir()
	s.assert.Nil(err)
	uploadFile, err := os.CreateTemp(homeDir, name+".tmp")
	s.assert.Nil(err)
	defer os.Remove(uploadFile.Name())
	_, err = uploadFile.Write(data)
	s.assert.Nil(err)

	// upload the temp file
	err = s.s3Storage.CopyFromFile(internal.CopyFromFileOptions{Name: name, File: uploadFile})
	s.assert.Nil(err)

	//create empty file for object download to write into
	file, err := os.CreateTemp("", generateFileName()+".tmp")
	s.assert.Nil(err)
	defer os.Remove(file.Name())

	//download to testDownload file
	err = s.s3Storage.CopyToFile(internal.CopyToFileOptions{Name: name, Offset: 3, Count: 0, File: file})
	s.assert.Nil(err)

	//create byte array of characters that are identical to what we should have downloaded
	currentData := []byte("t data")
	dataLen := len(currentData)
	output := make([]byte, dataLen) //empty byte array of that only holds 5 chars
	file, err = os.Open(file.Name())
	s.assert.Nil(err)

	//downloaded data in file is being read and dumped into the byte array.
	len, err := file.Read(output)

	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)

}
