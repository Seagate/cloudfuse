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
	"bytes"
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

type clientTestSuite struct {
	suite.Suite
	assert      *assert.Assertions
	awsS3Client *s3.Client // S3 client library supplied by AWS
	client      *Client
	config      string
}

func newTestClient(configuration string) (*Client, error) {
	// push the given config data to config.go
	_ = config.ReadConfigFromReader(strings.NewReader(configuration))
	// ask config to give us the config data back as Options
	conf := Options{}
	err := config.UnmarshalKey(compName, &conf)
	if err != nil {
		log.Err("ClientTest::newTestClient : config error [invalid config attributes]")
		return nil, fmt.Errorf("config error in %s. Here's why: %s", compName, err.Error())
	}
	// now push Options data into an Config
	configForS3Client := Config{
		authConfig: s3AuthConfig{
			BucketName: conf.BucketName,
			KeyID:      conf.KeyID,
			SecretKey:  conf.SecretKey,
			Region:     conf.Region,
			Profile:    conf.Profile,
			Endpoint:   conf.Endpoint,
		},
		prefixPath:                conf.PrefixPath,
		disableConcurrentDownload: conf.DisableConcurrentDownload,
		partSize:                  conf.PartSizeMb * common.MbToBytes,
		uploadCutoff:              conf.UploadCutoffMb * common.MbToBytes,
	}
	// create a Client
	client, err := NewConnection(configForS3Client)

	return client.(*Client), err
}

func (s *clientTestSuite) SetupTest() {
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

	cfgData, _ := io.ReadAll(cfgFile)
	err = json.Unmarshal(cfgData, &storageTestConfigurationParameters)
	if err != nil {
		fmt.Println("Failed to parse the config file")
		os.Exit(1)
	}

	cfgFile.Close()
	s.setupTestHelper("", true)
}

func (s *clientTestSuite) setupTestHelper(configuration string, create bool) error {
	// TODO: actually create a test bucket for testing (flagged with the create parameter)
	if storageTestConfigurationParameters.PartSizeMb == 0 {
		storageTestConfigurationParameters.PartSizeMb = 5
	}
	if storageTestConfigurationParameters.UploadCutoffMb == 0 {
		storageTestConfigurationParameters.UploadCutoffMb = 5
	}
	if configuration == "" {
		configuration = fmt.Sprintf("s3storage:\n  bucket-name: %s\n  key-id: %s\n  secret-key: %s\n  endpoint: %s\n  region: %s\n  part-size-mb: %d\n  upload-cutoff-mb: %d\n",
			storageTestConfigurationParameters.BucketName, storageTestConfigurationParameters.KeyID,
			storageTestConfigurationParameters.SecretKey, storageTestConfigurationParameters.Endpoint, storageTestConfigurationParameters.Region,
			storageTestConfigurationParameters.PartSizeMb, storageTestConfigurationParameters.UploadCutoffMb)
	}
	s.config = configuration

	s.assert = assert.New(s.T())

	var err error
	s.client, err = newTestClient(configuration)
	s.awsS3Client = s.client.awsS3Client
	return err
}

// TODO: do we need s3StatsCollector for this test suite?
// func (s *clientTestSuite) tearDownTestHelper(delete bool) {
// 	_ = s.s3.Stop()
// }

func (s *clientTestSuite) cleanupTest() {
	// s.tearDownTestHelper(true)
	_ = log.Destroy()
}

func (s *clientTestSuite) TestCredentialsError() {
	defer s.cleanupTest()
	// setup
	config := fmt.Sprintf("s3storage:\n  bucket-name: %s\n  key-id: %s\n  secret-key: %s",
		storageTestConfigurationParameters.BucketName, storageTestConfigurationParameters.KeyID,
		"WRONGSECRETKEY")
	// S3 connection creation should fail
	err := s.setupTestHelper(config, false)
	s.assert.NotNil(err)
}

func (s *clientTestSuite) TestEnvVarCredentials() {
	defer s.cleanupTest()
	// setup
	os.Setenv("AWS_ACCESS_KEY_ID", storageTestConfigurationParameters.KeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", storageTestConfigurationParameters.SecretKey)
	config := fmt.Sprintf("s3storage:\n  bucket-name: %s", "stxe1-srg-lens-lab1")
	// S3 connection should find credentials from environment variables
	err := s.setupTestHelper(config, false)
	s.assert.Nil(err)
}

func (s *clientTestSuite) TestCredentialPrecedence() {
	defer s.cleanupTest()
	// setup
	os.Setenv("AWS_ACCESS_KEY_ID", storageTestConfigurationParameters.KeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", storageTestConfigurationParameters.SecretKey)
	config := fmt.Sprintf("s3storage:\n  bucket-name: %s\n  key-id: %s\n  secret-key: %s",
		storageTestConfigurationParameters.BucketName, storageTestConfigurationParameters.KeyID,
		"WRONGSECRETKEY")
	// Wrong credentials should take precedence, so S3 connection should fail
	err := s.setupTestHelper(config, false)
	s.assert.NotNil(err)
}

func (s *clientTestSuite) TestListBuckets() {
	defer s.cleanupTest()
	// TODO: generalize this test by creating, listing, then destroying a bucket
	buckets, err := s.client.ListBuckets()
	s.assert.Nil(err)
	s.assert.Equal(buckets, []string{"stxe1-srg-lens-lab1"})
}
func (s *clientTestSuite) TestSetPrefixPath() {
	defer s.cleanupTest()
	// setup
	prefix := generateDirectoryName()
	fileName := generateFileName()

	err := s.client.SetPrefixPath(prefix)
	s.assert.Nil(err)                                   //stub
	err = s.client.CreateFile(fileName, os.FileMode(0)) // create file uses prefix
	s.assert.Nil(err)

	// object should be at prefix
	_, err = s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(path.Join(prefix, fileName)),
	})
	s.assert.Nil(err)
}
func (s *clientTestSuite) TestCreateFile() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()

	err := s.client.CreateFile(name, os.FileMode(0))
	s.assert.Nil(err)

	// file should be in bucket
	_, err = s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
	})
	s.assert.Nil(err)
}
func (s *clientTestSuite) TestCreateDirectory() {
	defer s.cleanupTest()
	// setup
	name := generateDirectoryName()

	err := s.client.CreateDirectory(name)
	s.assert.Nil(err)
}
func (s *clientTestSuite) TestCreateLink() {
	defer s.cleanupTest()
	// setup
	target := generateFileName()

	_, err := s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(target),
	})

	s.assert.Nil(err)
	source := generateFileName()

	err = s.client.CreateLink(source, target, true)
	s.assert.Nil(err)

	source = s.client.getKey(source, true)

	result, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(source),
	})
	s.assert.Nil(err)

	// object body should match target file name
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.Nil(err)
	s.assert.EqualValues(target, output)

}
func (s *clientTestSuite) TestReadLink() {
	defer s.cleanupTest()
	// setup
	target := generateFileName()

	source := generateFileName()

	err := s.client.CreateLink(source, target, true)
	s.assert.Nil(err)

	source = s.client.getKey(source, true)

	result, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(source),
	})
	s.assert.Nil(err)

	defer result.Body.Close()

	// object body should match target file name
	output, err := io.ReadAll(result.Body)
	s.assert.Nil(err)
	s.assert.EqualValues(target, string(output))

}

func (s *clientTestSuite) TestDeleteLink() {
	defer s.cleanupTest()
	// setup
	target := generateFileName()

	source := generateFileName()

	err := s.client.CreateLink(source, target, true)
	s.assert.Nil(err)

	source = s.client.getKey(source, true)

	_, err = s.awsS3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(source),
	})
	s.assert.Nil(err)

	_, err = s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(source),
	})
	s.assert.NotNil(err)
}

func (s *clientTestSuite) TestDeleteLinks() {
	defer s.cleanupTest()
	// setup

	// generate folder / prefix name

	prefix := generateDirectoryName()

	folder := internal.ExtendDirName(prefix)

	// generate series of file names
	// create link for all file names with prefix name
	var sources [5]string
	var targets [5]string
	for i := 0; i < 5; i++ {
		sources[i] = generateFileName()
		targets[i] = generateFileName()

		err := s.client.CreateLink(folder+sources[i], targets[i], true)
		s.assert.Nil(err)

		sources[i] = s.client.getKey(sources[i], true)

		// make sure the links are there
		result, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(s.client.Config.authConfig.BucketName),
			Key:    aws.String(folder + sources[i]),
		})
		s.assert.Nil(err)

		// object body should match target file name
		defer result.Body.Close()
		buffer, err := io.ReadAll(result.Body)
		s.assert.Nil(err)

		s.assert.EqualValues(targets[i], string(buffer))
	}

	//gather keylist for DeleteObjects
	keyList := make([]types.ObjectIdentifier, len(sources))
	for i, source := range sources {
		key := folder + source
		keyList[i] = types.ObjectIdentifier{
			Key: &key,
		}
	}
	// send keyList for deletion
	_, err := s.awsS3Client.DeleteObjects(context.TODO(), &s3.DeleteObjectsInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Delete: &types.Delete{
			Objects: keyList,
			Quiet:   true,
		},
	})
	s.assert.Nil(err)

	// make sure the links aren't there
	for i := range sources {
		_, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(s.client.Config.authConfig.BucketName),
			Key:    aws.String(folder + sources[i]),
		})
		s.assert.NotNil(err)

	}
}

func (s *clientTestSuite) TestDeleteFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	_, err := s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
	})
	s.assert.Nil(err)

	err = s.client.DeleteFile(name)
	s.assert.Nil(err)

	// This is similar to the s3 bucket command, use getobject for now
	//_, err = s.s3.GetAttr(internal.GetAttrOptions{name, false})
	// File should not be in the account
	_, err = s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
	})

	s.assert.NotNil(err)
}
func (s *clientTestSuite) TestDeleteDirectory() {
	defer s.cleanupTest()
	// setup
	dirName := generateDirectoryName()
	fileName := generateFileName() // can't have empty directory
	_, err := s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(path.Join(dirName, fileName)),
	})
	s.assert.Nil(err)

	err = s.client.DeleteDirectory(dirName)
	s.assert.Nil(err)

	// file in directory should no longer be there
	_, err = s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(path.Join(dirName, fileName)),
	})
	s.assert.NotNil(err)
}
func (s *clientTestSuite) TestRenameFile() {
	defer s.cleanupTest()
	// Setup

	src := generateFileName()
	_, err := s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(src),
	})
	s.assert.Nil(err)
	dst := generateFileName()

	err = s.client.RenameFile(src, dst, false)
	s.assert.Nil(err)

	// Src should not be in the account
	_, err = s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(src),
	})
	s.assert.NotNil(err)
	// Dst should be in the account
	_, err = s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(dst),
	})
	s.assert.Nil(err)
}
func (s *clientTestSuite) TestRenameDirectory() {
	defer s.cleanupTest()
	// setup
	srcDir := generateDirectoryName()
	fileName := generateFileName() // can't have empty directory
	_, err := s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(path.Join(srcDir, fileName)),
	})
	s.assert.Nil(err)

	dstDir := generateDirectoryName()
	err = s.client.RenameDirectory(srcDir, dstDir)
	s.assert.Nil(err)

	// file in srcDir should no longer be there
	_, err = s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(path.Join(srcDir, fileName)),
	})
	s.assert.NotNil(err)
	// file in dstDir should be there
	_, err = s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(path.Join(dstDir, fileName)),
	})
	s.assert.Nil(err)
}
func (s *clientTestSuite) TestGetAttrDir() {
	defer s.cleanupTest()
	// setup
	dirName := generateDirectoryName()
	filename := dirName + "/" + generateFileName()

	_, err := s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(filename),
	})
	s.assert.Nil(err)

	attr, err := s.client.GetAttr(dirName)
	s.assert.Nil(err)
	s.assert.NotNil(attr)
	s.assert.True(attr.IsDir())
}
func (s *clientTestSuite) TestGetAttrFile() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	maxBodyLen := 50
	minBodyLen := 10
	bodyLen := rand.Intn(maxBodyLen-minBodyLen) + minBodyLen
	body := []byte(randomString(bodyLen))
	_, err := s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
		Body:   bytes.NewReader(body),
	})
	s.assert.Nil(err)

	before, err := s.client.GetAttr(name)

	// file info
	s.assert.Nil(err)
	s.assert.NotNil(before)
	s.assert.False(before.IsDir())
	s.assert.False(before.IsSymlink())

	// file size
	s.assert.EqualValues(bodyLen, before.Size)

	// file time
	s.assert.NotNil(before.Mtime)

	time.Sleep(time.Second * 3) // Wait 3 seconds and then modify the file again

	_, err = s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
		Body:   bytes.NewReader(body),
	})
	s.assert.Nil(err)

	after, err := s.client.GetAttr(name)
	s.assert.Nil(err)
	s.assert.NotNil(after.Mtime)

	s.assert.True(after.Mtime.After(before.Mtime))
}
func (s *clientTestSuite) TestGetAttrError() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()

	// non existent file should throw error
	_, err := s.client.GetAttr(name)
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}
func (s *clientTestSuite) TestList() {
	defer s.cleanupTest()
	// setup
	base := generateDirectoryName()
	// setup directory hierarchy like setupHierarchy in s3storage_test where 'a' is generated base
	// a/c1/gc1
	gc1 := base + "/c1" + "/gc1"
	_, err := s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(gc1),
	})
	s.assert.Nil(err)
	// a/c2
	c2 := base + "/c2"
	_, err = s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(c2),
	})
	s.assert.Nil(err)
	// ab/c1
	abc1 := base + "b/c1"
	_, err = s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(abc1),
	})
	s.assert.Nil(err)
	// ac
	ac := base + "c"
	_, err = s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(ac),
	})
	s.assert.Nil(err)

	// with trailing "/" should return only the directory c1 and the file c2
	baseTrail := base + "/"
	objects, _, err := s.client.List(baseTrail, nil, 0)
	s.assert.Nil(err)
	s.assert.NotNil(objects)
	s.assert.EqualValues(len(objects), 2)
	s.assert.EqualValues(objects[0].Name, "c1")
	s.assert.True(objects[0].IsDir())
	s.assert.EqualValues(objects[1].Name, "c2")
	s.assert.False(objects[1].IsDir())

	// without trailing "/" only get file ac
	// if not including the trailing "/", List will return any files with the given prefix
	// but no directories
	objects, _, err = s.client.List(base, nil, 0)
	s.assert.Nil(err)
	s.assert.NotNil(objects)
	s.assert.EqualValues(len(objects), 1)
	s.assert.EqualValues(objects[0].Name, base+"c")
	s.assert.False(objects[0].IsDir())
}
func (s *clientTestSuite) TestReadToFile() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	maxBodyLen := 50
	minBodyLen := 10
	bodyLen := rand.Intn(maxBodyLen-minBodyLen) + minBodyLen
	body := []byte(randomString(bodyLen))
	_, err := s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
		Body:   bytes.NewReader(body),
	})
	s.assert.Nil(err)

	f, err := os.CreateTemp("", name+".tmp")
	s.assert.Nil(err)
	defer os.Remove(f.Name())

	err = s.client.ReadToFile(name, 0, 0, f)
	s.assert.Nil(err)

	// file content should match generated body
	output := make([]byte, bodyLen)
	f, err = os.Open(f.Name())
	s.assert.Nil(err)
	outputLen, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(bodyLen, outputLen)
	s.assert.EqualValues(body, output)
	f.Close()
}

func (s *clientTestSuite) TestReadToFileRanged() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	maxBodyLen := 50
	minBodyLen := 10
	bodyLen := rand.Intn(maxBodyLen-minBodyLen) + minBodyLen
	body := []byte(randomString(bodyLen))
	_, err := s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
		Body:   bytes.NewReader(body),
	})
	s.assert.Nil(err)

	f, err := os.CreateTemp("", name+".tmp")
	s.assert.Nil(err)
	defer os.Remove(f.Name())

	err = s.client.ReadToFile(name, 0, int64(bodyLen), f)
	s.assert.Nil(err)

	// file content should match generated body
	output := make([]byte, bodyLen)
	f, err = os.Open(f.Name())
	s.assert.Nil(err)
	outputLen, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(bodyLen, outputLen)
	s.assert.EqualValues(body, output)
	f.Close()
}

func (s *clientTestSuite) TestReadToFileNoMultipart() {
	storageTestConfigurationParameters.DisableConcurrentDownload = true
	vdConfig := generateConfigYaml(storageTestConfigurationParameters)
	s.setupTestHelper(vdConfig, false)
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	maxBodyLen := 50
	minBodyLen := 10
	bodyLen := rand.Intn(maxBodyLen-minBodyLen) + minBodyLen
	body := []byte(randomString(bodyLen))
	_, err := s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
		Body:   bytes.NewReader(body),
	})
	s.assert.Nil(err)

	f, err := os.CreateTemp("", name+".tmp")
	s.assert.Nil(err)
	defer os.Remove(f.Name())

	err = s.client.ReadToFile(name, 0, 0, f)
	s.assert.Nil(err)

	// file content should match generated body
	output := make([]byte, bodyLen)
	f, err = os.Open(f.Name())
	s.assert.Nil(err)
	outputLen, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(bodyLen, outputLen)
	s.assert.EqualValues(body, output)
	f.Close()
}

func (s *clientTestSuite) TestReadBuffer() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	maxBodyLen := 50
	minBodyLen := 10
	bodyLen := rand.Intn(maxBodyLen-minBodyLen) + minBodyLen
	body := []byte(randomString(bodyLen))
	_, err := s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
		Body:   bytes.NewReader(body),
	})
	s.assert.Nil(err)

	result, err := s.client.ReadBuffer(name, 0, int64(bodyLen), false)

	// result should match generated body
	s.assert.Nil(err)
	s.assert.EqualValues(body, result)
}
func (s *clientTestSuite) TestReadInBuffer() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	maxBodyLen := 50
	minBodyLen := 10
	bodyLen := rand.Intn(maxBodyLen-minBodyLen) + minBodyLen
	body := []byte(randomString(bodyLen))
	_, err := s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
		Body:   bytes.NewReader(body),
	})
	s.assert.Nil(err)

	outputLen := rand.Intn(bodyLen-1) + 1 // minimum buffer length of 1
	output := make([]byte, outputLen)
	err = s.client.ReadInBuffer(name, 0, int64(outputLen), output)

	// read in buffer should match first outputLen characters of generated body
	s.assert.Nil(err)
	s.assert.EqualValues(body[:outputLen], output)
}
func (s *clientTestSuite) TestWriteFromFile() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	maxBodyLen := 50
	minBodyLen := 10
	bodyLen := rand.Intn(maxBodyLen-minBodyLen) + minBodyLen
	body := []byte(randomString(bodyLen))
	f, err := os.CreateTemp("", name+".tmp")
	s.assert.Nil(err)
	defer os.Remove(f.Name())
	outputLen, err := f.Write(body)
	s.assert.Nil(err)
	s.assert.EqualValues(bodyLen, outputLen)
	var options internal.WriteFileOptions //stub

	err = s.client.WriteFromFile(name, options.Metadata, f)
	s.assert.Nil(err)
	f.Close()

	//todo: create another test like this one that does getObject here with and without the .rclonelink suffix
	// this checks the integration between attr cache and s3storage for metadata.make sure the flag passed down is
	// respected.
	result, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
	})
	s.assert.Nil(err)

	// object body should match generated body written to file
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.Nil(err)
	s.assert.EqualValues(body, output)
}
func (s *clientTestSuite) TestWriteFromBuffer() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	maxBodyLen := 50
	minBodyLen := 10
	bodyLen := rand.Intn(maxBodyLen-minBodyLen) + minBodyLen
	body := []byte(randomString(bodyLen))

	var options internal.WriteFileOptions //stub

	err := s.client.WriteFromBuffer(name, options.Metadata, body)
	s.assert.Nil(err)

	result, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
	})
	s.assert.Nil(err)

	// object body should match generated body
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.Nil(err)
	s.assert.EqualValues(body, output)
}
func (s *clientTestSuite) TestTruncateFile() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	maxBodyLen := 50
	minBodyLen := 10
	bodyLen := rand.Intn(maxBodyLen-minBodyLen) + minBodyLen
	body := []byte(randomString(bodyLen))
	_, err := s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
		Body:   bytes.NewReader(body),
	})
	s.assert.Nil(err)

	size := rand.Intn(bodyLen-1) + 1 // minimum size of 1
	err = s.client.TruncateFile(name, int64(size))
	s.assert.Nil(err)

	result, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
	})
	s.assert.Nil(err)

	// object body should match truncated initial body
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.Nil(err)
	s.assert.EqualValues(body[:size], output)
}
func (s *clientTestSuite) TestWrite() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	maxBodyLen := 50
	minBodyLen := 10
	bodyLen := rand.Intn(maxBodyLen-minBodyLen) + minBodyLen
	oldBody := []byte(randomString(bodyLen))
	_, err := s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
		Body:   bytes.NewReader(oldBody),
	})
	s.assert.Nil(err)

	offset := rand.Intn(bodyLen-1) + 1 // minimum offset of 1
	newData := []byte(randomString(bodyLen - offset))
	h := handlemap.NewHandle(name)
	err = s.client.Write(internal.WriteFileOptions{Handle: h, Offset: int64(offset), Data: newData})
	s.assert.Nil(err)

	result, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
	})
	s.assert.Nil(err)

	// object body should match generated combo of old and new data
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.Nil(err)
	s.assert.EqualValues(oldBody[:offset], output[:offset])
	s.assert.EqualValues(newData, output[offset:])
}

func TestClient(t *testing.T) {
	suite.Run(t, new(clientTestSuite))
}
