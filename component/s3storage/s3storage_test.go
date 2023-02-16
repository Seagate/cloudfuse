//go:build !authtest
// +build !authtest

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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
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

// A UUID representation compliant with specification in RFC 4122 document.
type uuid [16]byte

const reservedRFC4122 byte = 0x40

func (u uuid) bytes() []byte {
	return u[:]
}

// NewUUID returns a new uuid using RFC 4122 algorithm.
func newUUID() (u uuid) {
	u = uuid{}
	// Set all bits to randomly (or pseudo-randomly) chosen values.
	rand.Read(u[:])
	u[8] = (u[8] | reservedRFC4122) & 0x7F // u.setVariant(ReservedRFC4122)

	var version byte = 4
	u[6] = (u[6] & 0xF) | (version << 4) // u.setVersion(4)
	return
}

func randomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func generateContainerName() string {
	return "fuseutc" + randomString(8)
}

func generateDirectoryName() string {
	return "dir" + randomString(8)
}

func generateFileName() string {
	return "file" + randomString(8)
}

type s3StorageTestSuite struct {
	suite.Suite
	assert    *assert.Assertions
	s3        *S3Storage
	client    *s3.Client
	config    string
	container string
}

func newTestS3Storage(configuration string) (*S3Storage, error) {
	_ = config.ReadConfigFromReader(strings.NewReader(configuration))
	s3 := News3storageComponent()
	err := s3.Configure(true)

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
	_ = log.SetDefaultLogger("base", cfg)

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

	cfgData, _ := ioutil.ReadAll(cfgFile)
	err = json.Unmarshal(cfgData, &storageTestConfigurationParameters)
	if err != nil {
		fmt.Println("Failed to parse the config file")
		os.Exit(1)
	}

	cfgFile.Close()
	s.setupTestHelper("", "", true)
}

func (s *s3StorageTestSuite) setupTestHelper(configuration string, container string, create bool) {
	if container == "" {
		container = generateContainerName()
	}
	s.container = container
	if configuration == "" {
		configuration = fmt.Sprintf("s3storage:\n  bucket-name: %s\n  access-key: %s\n  secret-key: %s\n  endpoint: %s\n  region: %s",
			storageTestConfigurationParameters.BucketName, storageTestConfigurationParameters.AccessKey,
			storageTestConfigurationParameters.SecretKey, storageTestConfigurationParameters.Endpoint, storageTestConfigurationParameters.Region)
	}
	s.config = configuration

	s.assert = assert.New(s.T())

	s.s3, _ = newTestS3Storage(configuration)
	_ = s.s3.Start(ctx) // Note: Start->TestValidation will fail but it doesn't matter. We are creating the container a few lines below anyway.
	// We could create the container before but that requires rewriting the code to new up a service client.

	s.client = s.s3.storage.(*S3Client).Client
}

func (s *s3StorageTestSuite) tearDownTestHelper(delete bool) {
	_ = s.s3.Stop()
}

func (s *s3StorageTestSuite) cleanupTest() {
	s.tearDownTestHelper(true)
	_ = log.Destroy()
}

func (s *s3StorageTestSuite) TestListContainers() {
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

	containers, err := s.s3.ListContainers()
	s.assert.Nil(err)
	s.assert.Equal(containers, []string{"stxe1-srg-lens-lab1"})
}

func (s *s3StorageTestSuite) TestIsDirEmpty() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()
	s.s3.CreateDir(internal.CreateDirOptions{Name: name})

	// Testing dir and dir/
	var paths = []string{name, name + "/"}
	for _, path := range paths {
		log.Debug(path)
		s.Run(path, func() {
			empty := s.s3.IsDirEmpty(internal.IsDirEmptyOptions{Name: name})

			s.assert.True(empty)
		})
	}
}

func (s *s3StorageTestSuite) TestIsDirEmptyFalse() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()
	s.s3.CreateDir(internal.CreateDirOptions{Name: name})
	file := name + "/" + generateFileName()
	s.s3.CreateFile(internal.CreateFileOptions{Name: file})

	empty := s.s3.IsDirEmpty(internal.IsDirEmptyOptions{Name: name})

	s.assert.False(empty)
}

func (s *s3StorageTestSuite) TestCreateFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	h, err := s.s3.CreateFile(internal.CreateFileOptions{Name: name})

	s.assert.Nil(err)
	s.assert.NotNil(h)
	s.assert.EqualValues(name, h.Path)
	s.assert.EqualValues(0, h.Size)
	// File should be in the account
	result, err := s.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3.storage.(*S3Client).Config.authConfig.BucketName),
		Key:    aws.String(name),
	})
	s.assert.Nil(err)
	s.assert.NotNil(result)
}

func (s *s3StorageTestSuite) TestOpenFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.s3.CreateFile(internal.CreateFileOptions{Name: name})

	h, err := s.s3.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(h)
	s.assert.EqualValues(name, h.Path)
	s.assert.EqualValues(0, h.Size)
}

func (s *s3StorageTestSuite) TestOpenFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	h, err := s.s3.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
	s.assert.Nil(h)
}

func (s *s3StorageTestSuite) TestCloseFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.s3.CreateFile(internal.CreateFileOptions{Name: name})

	// This method does nothing.
	err := s.s3.CloseFile(internal.CloseFileOptions{Handle: h})
	s.assert.Nil(err)
}

func (s *s3StorageTestSuite) TestCloseFileFakeHandle() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)

	// This method does nothing.
	err := s.s3.CloseFile(internal.CloseFileOptions{Handle: h})
	s.assert.Nil(err)
}

func (s *s3StorageTestSuite) TestDeleteFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.s3.CreateFile(internal.CreateFileOptions{Name: name})

	err := s.s3.DeleteFile(internal.DeleteFileOptions{Name: name})
	s.assert.Nil(err)

	// This is similar to the s3 bucket command, use getobject for now
	//_, err = s.s3.GetAttr(internal.GetAttrOptions{name, false})
	// File should not be in the account
	_, err = s.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3.storage.(*S3Client).Config.authConfig.BucketName),
		Key:    aws.String(name),
	})

	s.assert.NotNil(err)
}

// func (s *s3StorageTestSuite) TestDeleteFileError() {
// 	defer s.cleanupTest()
// 	// Setup
// 	name := generateFileName()

// 	err := s.s3.DeleteFile(internal.DeleteFileOptions{Name: name})
// 	s.assert.NotNil(err)
// 	s.assert.EqualValues(syscall.ENOENT, err)

// 	// File should not be in the account
// 	_, err = s.client.GetObject(context.TODO(), &s3.GetObjectInput{
// 		Bucket: aws.String(s.s3.storage.(*S3Client).Config.authConfig.BucketName),
// 		Key:    aws.String(name),
// 	})
// 	s.assert.NotNil(err)
// }

func (s *s3StorageTestSuite) TestCopyFromFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.s3.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	homeDir, _ := os.UserHomeDir()
	f, _ := ioutil.TempFile(homeDir, name+".tmp")
	defer os.Remove(f.Name())
	f.Write(data)

	err := s.s3.CopyFromFile(internal.CopyFromFileOptions{Name: name, File: f})

	s.assert.Nil(err)

	// Object will be updated with new data
	result, err := s.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3.storage.(*S3Client).Config.authConfig.BucketName),
		Key:    aws.String(name),
	})
	s.assert.Nil(err)
	defer result.Body.Close()
	output, _ := ioutil.ReadAll(result.Body)
	s.assert.EqualValues(testData, output)

}

func (s *s3StorageTestSuite) TestReadFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test data"
	data := []byte(testData)
	_, err = s.s3.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	h, err = s.s3.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.Nil(err)

	output, err := s.s3.ReadFile(internal.ReadFileOptions{Handle: h})
	s.assert.Nil(err)
	s.assert.EqualValues(testData, output)
}

func (s *s3StorageTestSuite) TestReadFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)

	_, err := s.s3.ReadFile(internal.ReadFileOptions{Handle: h})
	fmt.Println(err)
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *s3StorageTestSuite) TestReadInBuffer() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.s3.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test data"
	data := []byte(testData)
	_, err = s.s3.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	h, err = s.s3.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.Nil(err)

	output := make([]byte, 5)
	len, err := s.s3.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.Nil(err)
	s.assert.EqualValues(5, len)
	s.assert.EqualValues(testData[:5], output)
}

func (s *s3StorageTestSuite) TestReadInBufferLargeBuffer() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.s3.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	s.s3.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	h, _ = s.s3.OpenFile(internal.OpenFileOptions{Name: name})

	output := make([]byte, 1000) // Testing that passing in a super large buffer will still work
	len, err := s.s3.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.Nil(err)
	s.assert.EqualValues(h.Size, len)
	s.assert.EqualValues(testData, output[:h.Size])
}

func (s *s3StorageTestSuite) TestReadInBufferEmpty() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.s3.CreateFile(internal.CreateFileOptions{Name: name})

	output := make([]byte, 10)
	len, err := s.s3.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.Nil(err)
	s.assert.EqualValues(0, len)
}

func (s *s3StorageTestSuite) TestReadInBufferBadRange() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)
	h.Size = 10

	_, err := s.s3.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 20, Data: make([]byte, 2)})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ERANGE, err)
}

func (s *s3StorageTestSuite) TestReadInBufferError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)
	h.Size = 10

	_, err := s.s3.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: make([]byte, 2)})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *s3StorageTestSuite) TestWriteFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.s3.CreateFile(internal.CreateFileOptions{Name: name})

	testData := "test data"
	data := []byte(testData)
	count, err := s.s3.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	s.assert.EqualValues(len(data), count)

	// Blob should have updated data
	result, err := s.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3.storage.(*S3Client).Config.authConfig.BucketName),
		Key:    aws.String(name),
	})
	s.assert.Nil(err)
	defer result.Body.Close()
	output, _ := ioutil.ReadAll(result.Body)
	s.assert.EqualValues(testData, output)
}

func (s *s3StorageTestSuite) TestWriteSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.s3.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	dataLen := len(data)
	_, err := s.s3.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())

	err = s.s3.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	output := make([]byte, len(data))
	f, _ = os.Open(f.Name())
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
	h, _ := s.s3.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test-replace-data"
	data := []byte(testData)
	dataLen := len(data)
	_, err := s.s3.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("newdata")
	_, err = s.s3.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 5, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("test-newdata-data")
	output := make([]byte, len(currentData))

	err = s.s3.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
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
	h, _ := s.s3.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test-data"
	data := []byte(testData)

	_, err := s.s3.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("newdata")
	_, err = s.s3.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 5, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("test-newdata")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.s3.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
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
	h, _ := s.s3.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test-data"
	data := []byte(testData)

	_, err := s.s3.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("-newdata")
	_, err = s.s3.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 9, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("test-data-newdata")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.s3.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
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
	h, _ := s.s3.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test-data"
	data := []byte(testData)

	_, err := s.s3.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("newdata")
	_, err = s.s3.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 12, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("test-data\x00\x00\x00newdata")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.s3.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
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
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())

	err := s.s3.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *s3StorageTestSuite) TestReadDir() {
	defer s.cleanupTest()
	// This tests the default listBlocked = 0. It should return the expected paths.
	// Setup
	name := generateDirectoryName()
	s.s3.CreateDir(internal.CreateDirOptions{Name: name})
	childName := name + "/" + generateFileName()
	s.s3.CreateFile(internal.CreateFileOptions{Name: childName})

	// Testing dir and dir/
	var paths = []string{name, name + "/"}
	for _, path := range paths {
		log.Debug(path)
		s.Run(path, func() {
			entries, err := s.s3.ReadDir(internal.ReadDirOptions{Name: path})
			s.assert.Nil(err)
			s.assert.EqualValues(1, len(entries))
		})
	}
}

func (s *s3StorageTestSuite) TestStreamDirSmallCountNoDuplicates() {
	defer s.cleanupTest()
	// Setup
	s.s3.CreateFile(internal.CreateFileOptions{Name: "TestStreamDirSmallCountNoDuplicates/blob1.txt"})
	s.s3.CreateFile(internal.CreateFileOptions{Name: "TestStreamDirSmallCountNoDuplicates/blob2.txt"})
	s.s3.CreateFile(internal.CreateFileOptions{Name: "TestStreamDirSmallCountNoDuplicates/newblob1.txt"})
	s.s3.CreateFile(internal.CreateFileOptions{Name: "TestStreamDirSmallCountNoDuplicates/newblob2.txt"})
	s.s3.CreateDir(internal.CreateDirOptions{Name: "TestStreamDirSmallCountNoDuplicates/myfolder"})
	s.s3.CreateFile(internal.CreateFileOptions{Name: "TestStreamDirSmallCountNoDuplicates/myfolder/newblobA.txt"})
	s.s3.CreateFile(internal.CreateFileOptions{Name: "TestStreamDirSmallCountNoDuplicates/myfolder/newblobB.txt"})

	var iteration int = 0
	var marker string = ""
	blobList := make([]*internal.ObjAttr, 0)

	for {
		new_list, new_marker, err := s.s3.StreamDir(internal.StreamDirOptions{Name: "TestStreamDirSmallCountNoDuplicates/", Token: marker, Count: 1})
		fmt.Println(err)
		s.assert.Nil(err)
		blobList = append(blobList, new_list...)
		marker = new_marker
		iteration++

		log.Debug("s3StorageTestSuite::TestStreamDirSmallCountNoDuplicates : So far retrieved %d objects in %d iterations", len(blobList), iteration)
		if new_marker == "" {
			break
		}
	}

	s.assert.EqualValues(5, len(blobList))
}

func (s *s3StorageTestSuite) TestRenameFile() {
	defer s.cleanupTest()
	// Setup
	src := generateFileName()
	s.s3.CreateFile(internal.CreateFileOptions{Name: src})
	dst := generateFileName()

	err := s.s3.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	s.assert.Nil(err)

	// Src should not be in the account
	_, err = s.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3.storage.(*S3Client).Config.authConfig.BucketName),
		Key:    aws.String(src),
	})
	s.assert.NotNil(err)
	// Dst should be in the account
	_, err = s.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3.storage.(*S3Client).Config.authConfig.BucketName),
		Key:    aws.String(dst),
	})
	s.assert.Nil(err)
}

func (s *s3StorageTestSuite) TestRenameFileError() {
	defer s.cleanupTest()
	// Setup
	src := generateFileName()
	dst := generateFileName()

	err := s.s3.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)

	// Src and destination should not be in the account
	_, err = s.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3.storage.(*S3Client).Config.authConfig.BucketName),
		Key:    aws.String(src),
	})
	s.assert.NotNil(err)
	_, err = s.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.s3.storage.(*S3Client).Config.authConfig.BucketName),
		Key:    aws.String(dst),
	})
	s.assert.NotNil(err)
}

func (s *s3StorageTestSuite) TestGetAttrDir() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("s3storage:\n  bucket-name: %s\n  access-key: %s\n  secret-key: %s\n  endpoint: %s\n  region: %s",
		storageTestConfigurationParameters.BucketName, storageTestConfigurationParameters.AccessKey,
		storageTestConfigurationParameters.SecretKey, storageTestConfigurationParameters.Endpoint, storageTestConfigurationParameters.Region)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.container, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			dirName := generateDirectoryName()
			s.s3.CreateDir(internal.CreateDirOptions{Name: dirName})
			// since CreateDir doesn't do anything, let's put an object with that prefix
			filename := dirName + "/" + generateFileName()
			s.s3.CreateFile(internal.CreateFileOptions{Name: filename})
			// Now we should be able to see the directory
			props, err := s.s3.GetAttr(internal.GetAttrOptions{Name: dirName})
			deleteError := s.s3.DeleteFile(internal.DeleteFileOptions{Name: filename})
			s.assert.Nil(err)
			s.assert.NotNil(props)
			s.assert.True(props.IsDir())
			s.assert.NotEmpty(props.Metadata)
			s.assert.Contains(props.Metadata, folderKey)
			s.assert.EqualValues("true", props.Metadata[folderKey])
			s.assert.Nil(deleteError)
		})
	}
}

func (s *s3StorageTestSuite) TestGetAttrVirtualDir() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("s3storage:\n  bucket-name: %s\n  access-key: %s\n  secret-key: %s\n  endpoint: %s\n  region: %s",
		storageTestConfigurationParameters.BucketName, storageTestConfigurationParameters.AccessKey,
		storageTestConfigurationParameters.SecretKey, storageTestConfigurationParameters.Endpoint, storageTestConfigurationParameters.Region)
	// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
	s.tearDownTestHelper(false)
	s.setupTestHelper(vdConfig, s.container, true)
	// Setup
	dirName := generateFileName()
	name := dirName + "/" + generateFileName()
	s.s3.CreateFile(internal.CreateFileOptions{Name: name})

	props, err := s.s3.GetAttr(internal.GetAttrOptions{Name: dirName})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.True(props.IsDir())
	s.assert.False(props.IsSymlink())

	// Check file in dir too
	props, err = s.s3.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.False(props.IsDir())
	s.assert.False(props.IsSymlink())
}

func (s *s3StorageTestSuite) TestGetAttrVirtualDirSubDir() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("s3storage:\n  bucket-name: %s\n  access-key: %s\n  secret-key: %s\n  endpoint: %s\n  region: %s",
		storageTestConfigurationParameters.BucketName, storageTestConfigurationParameters.AccessKey,
		storageTestConfigurationParameters.SecretKey, storageTestConfigurationParameters.Endpoint, storageTestConfigurationParameters.Region)
	// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
	s.tearDownTestHelper(false)
	s.setupTestHelper(vdConfig, s.container, true)
	// Setup
	dirName := generateFileName()
	subDirName := dirName + "/" + generateFileName()
	name := subDirName + "/" + generateFileName()
	s.s3.CreateFile(internal.CreateFileOptions{Name: name})

	props, err := s.s3.GetAttr(internal.GetAttrOptions{Name: dirName})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.True(props.IsDir())
	s.assert.False(props.IsSymlink())

	// Check subdir in dir too
	props, err = s.s3.GetAttr(internal.GetAttrOptions{Name: subDirName})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.True(props.IsDir())
	s.assert.False(props.IsSymlink())

	// Check file in subdir too
	props, err = s.s3.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.False(props.IsDir())
	s.assert.False(props.IsSymlink())
}

func (s *s3StorageTestSuite) TestGetAttrFile() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("s3storage:\n  bucket-name: %s\n  access-key: %s\n  secret-key: %s\n  endpoint: %s\n  region: %s",
		storageTestConfigurationParameters.BucketName, storageTestConfigurationParameters.AccessKey,
		storageTestConfigurationParameters.SecretKey, storageTestConfigurationParameters.Endpoint, storageTestConfigurationParameters.Region)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.container, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			name := generateFileName()
			s.s3.CreateFile(internal.CreateFileOptions{Name: name})

			props, err := s.s3.GetAttr(internal.GetAttrOptions{Name: name})
			s.assert.Nil(err)
			s.assert.NotNil(props)
			s.assert.False(props.IsDir())
			s.assert.False(props.IsSymlink())
		})
	}
}

// func (s *s3StorageTestSuite) TestGetAttrLink() {
// 	defer s.cleanupTest()
// 	vdConfig := fmt.Sprintf("s3storage:\n  bucket-name: %s\n  access-key: %s\n  secret-key: %s\n  endpoint: %s\n  region: %s",
// 		storageTestConfigurationParameters.BucketName, storageTestConfigurationParameters.AccessKey,
// 		storageTestConfigurationParameters.SecretKey, storageTestConfigurationParameters.Endpoint, storageTestConfigurationParameters.Region)
// 	configs := []string{"", vdConfig}
// 	for _, c := range configs {
// 		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
// 		s.tearDownTestHelper(false)
// 		s.setupTestHelper(c, s.container, true)
// 		testName := ""
// 		if c != "" {
// 			testName = "virtual-directory"
// 		}
// 		s.Run(testName, func() {
// 			// Setup
// 			target := generateFileName()
// 			s.s3.CreateFile(internal.CreateFileOptions{Name: target})
// 			name := generateFileName()
// 			s.s3.CreateLink(internal.CreateLinkOptions{Name: name, Target: target})

// 			props, err := s.s3.GetAttr(internal.GetAttrOptions{Name: name})
// 			s.assert.Nil(err)
// 			s.assert.NotNil(props)
// 			s.assert.True(props.IsSymlink())
// 			s.assert.NotEmpty(props.Metadata)
// 			s.assert.Contains(props.Metadata, symlinkKey)
// 			s.assert.EqualValues("true", props.Metadata[symlinkKey])
// 		})
// 	}
// }

func (s *s3StorageTestSuite) TestGetAttrFileSize() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("s3storage:\n  bucket-name: %s\n  access-key: %s\n  secret-key: %s\n  endpoint: %s\n  region: %s",
		storageTestConfigurationParameters.BucketName, storageTestConfigurationParameters.AccessKey,
		storageTestConfigurationParameters.SecretKey, storageTestConfigurationParameters.Endpoint, storageTestConfigurationParameters.Region)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.container, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			name := generateFileName()
			h, _ := s.s3.CreateFile(internal.CreateFileOptions{Name: name})
			testData := "test data"
			data := []byte(testData)
			s.s3.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})

			props, err := s.s3.GetAttr(internal.GetAttrOptions{Name: name})
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
	vdConfig := fmt.Sprintf("s3storage:\n  bucket-name: %s\n  access-key: %s\n  secret-key: %s\n  endpoint: %s\n  region: %s",
		storageTestConfigurationParameters.BucketName, storageTestConfigurationParameters.AccessKey,
		storageTestConfigurationParameters.SecretKey, storageTestConfigurationParameters.Endpoint, storageTestConfigurationParameters.Region)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.container, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			name := generateFileName()
			h, _ := s.s3.CreateFile(internal.CreateFileOptions{Name: name})
			testData := "test data"
			data := []byte(testData)
			s.s3.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})

			before, err := s.s3.GetAttr(internal.GetAttrOptions{Name: name})
			s.assert.Nil(err)
			s.assert.NotNil(before.Mtime)

			time.Sleep(time.Second * 3) // Wait 3 seconds and then modify the file again

			s.s3.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})

			after, err := s.s3.GetAttr(internal.GetAttrOptions{Name: name})
			s.assert.Nil(err)
			s.assert.NotNil(after.Mtime)

			s.assert.True(after.Mtime.After(before.Mtime))
		})
	}
}

func (s *s3StorageTestSuite) TestGetAttrError() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("s3storage:\n  bucket-name: %s\n  access-key: %s\n  secret-key: %s\n  endpoint: %s\n  region: %s",
		storageTestConfigurationParameters.BucketName, storageTestConfigurationParameters.AccessKey,
		storageTestConfigurationParameters.SecretKey, storageTestConfigurationParameters.Endpoint, storageTestConfigurationParameters.Region)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.container, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			name := generateFileName()

			_, err := s.s3.GetAttr(internal.GetAttrOptions{Name: name})
			s.assert.NotNil(err)
			s.assert.EqualValues(syscall.ENOENT, err)
		})
	}
}

func TestS3Storage(t *testing.T) {
	suite.Run(t, new(s3StorageTestSuite))
}

/*
description:

	creates a file with bytes and uploads it to S3 and deletes it from local machine afgter upload.
	to be called from other test functions. Hense why there is no cleanupTest() call

input:

	N/A

output:
 1. string of file name that was uploaded
 2. []byte of the data that was written to the file.
*/
func (s *s3StorageTestSuite) UploadFile() (string, []byte) {

	// Setup
	name := generateFileName()
	s.s3.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	homeDir, _ := os.UserHomeDir()
	file, _ := ioutil.TempFile(homeDir, name+".tmp")
	defer os.Remove(file.Name())
	file.Write(data)

	err := s.s3.CopyFromFile(internal.CopyFromFileOptions{Name: name, File: file})
	s.assert.Nil(err)

	return name, data

}

/*
description:

	tests downloading full object from S3 bucket
	does an assertion of two equal values between the follwoing:
			1. A string of the data written to file before it was uploaded to the S3 Bucket
			2. The data contained in the file downloaded of the object

input:

	N/A

output:

	N/A
*/
func (s *s3StorageTestSuite) TestFullRangedDownload() {
	defer s.cleanupTest()

	//create and upload file to S3 for download testing
	name, data := s.UploadFile()

	//create empty file for object download to write into
	file, err := os.Create("testDownload")
	s.assert.Nil(err)

	//download to testDownload file
	err = s.s3.CopyToFile(internal.CopyToFileOptions{Name: name, Offset: 0, Count: 0, File: file})
	s.assert.Nil(err)

	//reading downloaded file to compare with original data string slice.
	dat, err := os.ReadFile(file.Name())
	s.assert.Nil(err)
	dataString := string(data)
	s.assert.EqualValues(string(dat), dataString)
}

/*
description:

	tests handling of non zero values provided for offset and count parameters to ReadToFile()
	does an assertion of two equal values between the follwoing:
		1. A sub string of the data written to file before it was uploaded to the S3 Bucket
		2. The data contained in the file downloaded from a specified range of the object

	example:
		-file uploaded with data "test data".
		-substring of "st da" taken from the file that was uploaded
		-download the range of data from the object into a file
		-compare "st da" from file upload with "st da" from downloaded file.

input:

	N/A

output:

	N/A
*/
func (s *s3StorageTestSuite) TestRangedDownload() {
	defer s.cleanupTest()

	//create and upload file to S3 for download testing
	name, data := s.UploadFile()

	//create empty file for object download to write into
	file, err := os.Create("testDownload")
	s.assert.Nil(err)

	//download to testDownload file
	err = s.s3.CopyToFile(internal.CopyToFileOptions{Name: name, Offset: 2, Count: 5, File: file})
	s.assert.Nil(err)

	//reading downloaded file to compare with original data string slice.
	dat, err := os.ReadFile(file.Name())
	s.assert.Nil(err)
	dataString := string(data)
	dataSlice := dataString[2:8]
	s.assert.EqualValues(string(dat), dataSlice)
}

/*
description:

	tests handling of parameters, offset set to non zero value and count set to zero, for ReadToFile().
	count set to zero means to read to end of object from offset.

	does an assertion of two equal values between the follwoing:
		1. A sub string of the data written to file before it was uploaded to the S3 Bucket
		2. The data contained in the file downloaded from a specified range of the object

	example:
		-file uploaded with data "test data".
		-substring of "st data" taken from the file that was uploaded
		-download the range (offset to end) of data from the object into a file
		-compare "st data" from file upload with "st data" from downloaded file.

input:

	N/A

output:

	N/A
*/
func (s *s3StorageTestSuite) TestOffsetToEndDownload() {
	defer s.cleanupTest()

	//create and upload file to S3 for download testing
	name, data := s.UploadFile()

	//create empty file for object download to write into
	file, err := os.Create("testDownload")
	s.assert.Nil(err)

	//download to testDownload file
	err = s.s3.CopyToFile(internal.CopyToFileOptions{Name: name, Offset: 3, Count: 0, File: file})
	s.assert.Nil(err)

	//reading downloaded file to compare with original data string slice.
	dat, err := os.ReadFile(file.Name())
	s.assert.Nil(err)
	dataString := string(data)
	dataSlice := dataString[3:]
	s.assert.EqualValues(string(dat), dataSlice)
}
