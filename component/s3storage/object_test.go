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

type blockBlobTestSuite struct {
	suite.Suite
	assert *assert.Assertions
	az     *S3Storage
	client *s3.Client
	// serviceUrl   azblob.ServiceURL
	// containerUrl azblob.ContainerURL
	config    string
	container string
}

func newTestAzStorage(configuration string) (*S3Storage, error) {
	_ = config.ReadConfigFromReader(strings.NewReader(configuration))
	az := NewazstorageComponent()
	err := az.Configure(true)

	return az.(*S3Storage), err
}

func (s *blockBlobTestSuite) SetupTest() {
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

func (s *blockBlobTestSuite) setupTestHelper(configuration string, container string, create bool) {
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

	s.az, _ = newTestAzStorage(configuration)
	_ = s.az.Start(ctx) // Note: Start->TestValidation will fail but it doesn't matter. We are creating the container a few lines below anyway.
	// We could create the container before but that requires rewriting the code to new up a service client.

	s.client = s.az.storage.(*S3Object).Client

	//s.serviceUrl = s.az.storage.(*BlockBlob).Service // Grab the service client to do some validation
	//s.containerUrl = s.serviceUrl.NewContainerURL(s.container)
	// if create {
	// 	_, _ = s.containerUrl.Create(ctx, azblob.Metadata{}, azblob.PublicAccessNone)
	// }
}

func (s *blockBlobTestSuite) tearDownTestHelper(delete bool) {
	_ = s.az.Stop()
	// if delete {
	// 	_, _ = s.containerUrl.Delete(ctx, azblob.ContainerAccessConditions{})
	// }
}

func (s *blockBlobTestSuite) cleanupTest() {
	s.tearDownTestHelper(true)
	_ = log.Destroy()
}

func (s *blockBlobTestSuite) TestListContainers() {
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

	containers, err := s.az.ListContainers()
	s.assert.Nil(err)
	s.assert.Equal(containers, []string{"stxe1-srg-lens-lab1"})
}

func (s *blockBlobTestSuite) TestCreateFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	h, err := s.az.CreateFile(internal.CreateFileOptions{Name: name})

	s.assert.Nil(err)
	s.assert.NotNil(h)
	s.assert.EqualValues(name, h.Path)
	s.assert.EqualValues(0, h.Size)
	// File should be in the account
	result, err := s.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.az.storage.(*S3Object).Config.authConfig.BucketName),
		Key:    aws.String(name),
	})
	s.assert.Nil(err)
	s.assert.NotNil(result)
}

func (s *blockBlobTestSuite) TestCopyFromFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	homeDir, _ := os.UserHomeDir()
	f, _ := ioutil.TempFile(homeDir, name+".tmp")
	defer os.Remove(f.Name())
	f.Write(data)

	err := s.az.CopyFromFile(internal.CopyFromFileOptions{Name: name, File: f})

	s.assert.Nil(err)

	// Object will be updated with new data
	result, err := s.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.az.storage.(*S3Object).Config.authConfig.BucketName),
		Key:    aws.String(name),
	})
	s.assert.Nil(err)
	defer result.Body.Close()
	output, _ := ioutil.ReadAll(result.Body)
	s.assert.EqualValues(testData, output)
}

// func (s *blockBlobTestSuite) TestReadFile() {
// 	defer s.cleanupTest()
// 	// Setup
// 	name := generateFileName()
// 	h, err := s.az.CreateFile(internal.CreateFileOptions{Name: name})
// 	s.assert.Nil(err)
// 	testData := "test data"
// 	data := []byte(testData)
// 	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
// 	s.assert.Nil(err)
// 	h, err = s.az.OpenFile(internal.OpenFileOptions{Name: name})
// 	s.assert.Nil(err)

// 	output, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
// 	s.assert.Nil(err)
// 	s.assert.EqualValues(testData, output)
// }

// func (s *blockBlobTestSuite) TestReadFileError() {
// 	defer s.cleanupTest()
// 	// Setup
// 	name := generateFileName()
// 	h := handlemap.NewHandle(name)

// 	_, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
// 	fmt.Println(err)
// 	s.assert.NotNil(err)
// 	s.assert.EqualValues(syscall.ENOENT, err)
// }

// func (s *blockBlobTestSuite) TestReadInBuffer() {
// 	defer s.cleanupTest()
// 	// Setup
// 	name := generateFileName()
// 	h, err := s.az.CreateFile(internal.CreateFileOptions{Name: name})
// 	s.assert.Nil(err)
// 	testData := "test data"
// 	data := []byte(testData)
// 	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
// 	s.assert.Nil(err)
// 	h, err = s.az.OpenFile(internal.OpenFileOptions{Name: name})
// 	s.assert.Nil(err)

// 	output := make([]byte, 5)
// 	len, err := s.az.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
// 	s.assert.Nil(err)
// 	s.assert.EqualValues(5, len)
// 	s.assert.EqualValues(testData[:5], output)
// }

// func (s *blockBlobTestSuite) TestReadInBufferLargeBuffer() {
// 	defer s.cleanupTest()
// 	// Setup
// 	name := generateFileName()
// 	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
// 	testData := "test data"
// 	data := []byte(testData)
// 	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
// 	h, _ = s.az.OpenFile(internal.OpenFileOptions{Name: name})

// 	output := make([]byte, 1000) // Testing that passing in a super large buffer will still work
// 	len, err := s.az.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
// 	s.assert.Nil(err)
// 	s.assert.EqualValues(h.Size, len)
// 	s.assert.EqualValues(testData, output[:h.Size])
// }

func (s *blockBlobTestSuite) TestReadInBufferEmpty() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})

	output := make([]byte, 10)
	len, err := s.az.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.Nil(err)
	s.assert.EqualValues(0, len)
}

func (s *blockBlobTestSuite) TestReadInBufferBadRange() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)
	h.Size = 10

	_, err := s.az.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 20, Data: make([]byte, 2)})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ERANGE, err)
}

func (s *blockBlobTestSuite) TestReadInBufferError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)
	h.Size = 10

	_, err := s.az.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: make([]byte, 2)})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *blockBlobTestSuite) TestWriteSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	dataLen := len(data)
	_, err := s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())

	err = s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	output := make([]byte, len(data))
	f, _ = os.Open(f.Name())
	len, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(testData, output)
	f.Close()
}

func (s *blockBlobTestSuite) TestOverwriteSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test-replace-data"
	data := []byte(testData)
	dataLen := len(data)
	_, err := s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("newdata")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 5, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("test-newdata-data")
	output := make([]byte, len(currentData))

	err = s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
	len, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *blockBlobTestSuite) TestOverwriteAndAppendToSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test-data"
	data := []byte(testData)

	_, err := s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("newdata")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 5, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("test-newdata")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
	len, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *blockBlobTestSuite) TestAppendToSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test-data"
	data := []byte(testData)

	_, err := s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("-newdata")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 9, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("test-data-newdata")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
	len, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *blockBlobTestSuite) TestAppendOffsetLargerThanSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test-data"
	data := []byte(testData)

	_, err := s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("newdata")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 12, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("test-data\x00\x00\x00newdata")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
	len, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *blockBlobTestSuite) TestCopyToFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())

	err := s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *blockBlobTestSuite) TestReadDir() {
	defer s.cleanupTest()
	// This tests the default listBlocked = 0. It should return the expected paths.
	// Setup
	name := generateDirectoryName()
	s.az.CreateDir(internal.CreateDirOptions{Name: name})
	childName := name + "/" + generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: childName})

	// Testing dir and dir/
	var paths = []string{name, name + "/"}
	for _, path := range paths {
		log.Debug(path)
		s.Run(path, func() {
			entries, err := s.az.ReadDir(internal.ReadDirOptions{Name: path})
			s.assert.Nil(err)
			s.assert.EqualValues(1, len(entries))
		})
	}
}

func (s *blockBlobTestSuite) TestStreamDirSmallCountNoDuplicates() {
	defer s.cleanupTest()
	// Setup
	s.az.CreateFile(internal.CreateFileOptions{Name: "blob1.txt"})
	s.az.CreateFile(internal.CreateFileOptions{Name: "blob2.txt"})
	s.az.CreateFile(internal.CreateFileOptions{Name: "newblob1.txt"})
	s.az.CreateFile(internal.CreateFileOptions{Name: "newblob2.txt"})
	s.az.CreateDir(internal.CreateDirOptions{Name: "myfolder"})
	s.az.CreateFile(internal.CreateFileOptions{Name: "myfolder/newblobA.txt"})
	s.az.CreateFile(internal.CreateFileOptions{Name: "myfolder/newblobB.txt"})

	var iteration int = 0
	var marker string = ""
	blobList := make([]*internal.ObjAttr, 0)

	for {
		new_list, new_marker, err := s.az.StreamDir(internal.StreamDirOptions{Name: "/", Token: marker, Count: 1})
		fmt.Println(err)
		s.assert.Nil(err)
		blobList = append(blobList, new_list...)
		marker = new_marker
		iteration++

		log.Debug("AzStorage::ReadDir : So far retrieved %d objects in %d iterations", len(blobList), iteration)
		if new_marker == "" {
			break
		}
	}

	s.assert.EqualValues(5, len(blobList))
}

// func (s *blockBlobTestSuite) TestRenameFile() {
// 	defer s.cleanupTest()
// 	// Setup
// 	src := generateFileName()
// 	s.az.CreateFile(internal.CreateFileOptions{Name: src})
// 	dst := generateFileName()

// 	err := s.az.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
// 	s.assert.Nil(err)

// 	// Src should not be in the account
// 	_, err = s.client.GetObject(context.TODO(), &s3.GetObjectInput{
// 		Bucket: aws.String(s.az.storage.(*S3Object).Config.authConfig.BucketName),
// 		Key:    aws.String(src),
// 	})
// 	s.assert.NotNil(err)
// 	// Dst should be in the account
// 	_, err = s.client.GetObject(context.TODO(), &s3.GetObjectInput{
// 		Bucket: aws.String(s.az.storage.(*S3Object).Config.authConfig.BucketName),
// 		Key:    aws.String(dst),
// 	})
// 	s.assert.Nil(err)
// }

// func (s *blockBlobTestSuite) TestRenameFileError() {
// 	defer s.cleanupTest()
// 	// Setup
// 	src := generateFileName()
// 	dst := generateFileName()

// 	err := s.az.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
// 	s.assert.NotNil(err)
// 	s.assert.EqualValues(syscall.ENOENT, err)

// 	// Src and destination should not be in the account
// 	_, err = s.client.GetObject(context.TODO(), &s3.GetObjectInput{
// 		Bucket: aws.String(s.az.storage.(*S3Object).Config.authConfig.BucketName),
// 		Key:    aws.String(src),
// 	})
// 	s.assert.NotNil(err)
// 	_, err = s.client.GetObject(context.TODO(), &s3.GetObjectInput{
// 		Bucket: aws.String(s.az.storage.(*S3Object).Config.authConfig.BucketName),
// 		Key:    aws.String(dst),
// 	})
// 	s.assert.NotNil(err)
// }

func TestBlockBlob(t *testing.T) {
	suite.Run(t, new(blockBlobTestSuite))
}
