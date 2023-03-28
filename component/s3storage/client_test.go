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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"lyvecloudfuse/common"
	"lyvecloudfuse/common/config"
	"lyvecloudfuse/common/log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
			Endpoint:   conf.Endpoint,
		},
		prefixPath: conf.PrefixPath,
	}
	// create a Client
	client := NewConnection(configForS3Client)

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

	cfgData, _ := ioutil.ReadAll(cfgFile)
	err = json.Unmarshal(cfgData, &storageTestConfigurationParameters)
	if err != nil {
		fmt.Println("Failed to parse the config file")
		os.Exit(1)
	}

	cfgFile.Close()
	s.setupTestHelper("", true)
}

func (s *clientTestSuite) setupTestHelper(configuration string, create bool) {
	if configuration == "" {
		configuration = fmt.Sprintf("s3storage:\n  bucket-name: %s\n  key-id: %s\n  secret-key: %s\n  endpoint: %s\n  region: %s",
			storageTestConfigurationParameters.BucketName, storageTestConfigurationParameters.KeyID,
			storageTestConfigurationParameters.SecretKey, storageTestConfigurationParameters.Endpoint, storageTestConfigurationParameters.Region)
	}
	s.config = configuration

	s.assert = assert.New(s.T())

	s.client, _ = newTestClient(configuration)
	s.awsS3Client = s.client.awsS3Client
}

// TODO: do we need s3StatsCollector for this test suite?
// func (s *clientTestSuite) tearDownTestHelper(delete bool) {
// 	_ = s.s3.Stop()
// }

func (s *clientTestSuite) cleanupTest() {
	// s.tearDownTestHelper(true)
	_ = log.Destroy()
}

func (s *clientTestSuite) TestUpdateConfig() {
}
func (s *clientTestSuite) TestNewCredentialKey() {
}
func (s *clientTestSuite) TestListBuckets() {
	// TODO: generalize this test by creating, listing, then destroying a bucket
	// 	We need to get permissions to create buckets in Lyve Cloud, or implement this against AWS S3.
	// 	For now, the bucket parameter has been removed from the test suite for tidiness sake
	buckets, err := s.client.ListBuckets()
	s.assert.Nil(err)
	s.assert.Equal(buckets, []string{"stxe1-srg-lens-lab1"})
}
func (s *clientTestSuite) TestSetPrefixPath() {
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

	err = s.client.CreateLink(source, target)
	s.assert.Nil(err)

	result, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(source),
	})
	s.assert.Nil(err)

	// object body should match target file name
	defer result.Body.Close()
	output, err := ioutil.ReadAll(result.Body)
	s.assert.Nil(err)
	s.assert.EqualValues(target, output)

	// TODO : test metadata
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

	err = s.client.RenameFile(src, dst)
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
}
func (s *clientTestSuite) TestgetAttrUsingRest() {
}
func (s *clientTestSuite) TestgetAttrUsingList() {
}
func (s *clientTestSuite) TestGetAttr() {
}
func (s *clientTestSuite) TestList() {
}
func (s *clientTestSuite) TestReadToFile() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	bodyLen := 20
	body := []byte(randomString(bodyLen))
	_, err := s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
		Body:   bytes.NewReader(body),
	})
	s.assert.Nil(err)

	f, err := os.CreateTemp("", name+".tmp") // s3storage_test uses ioutil.TempFile which is deprecated
	s.assert.Nil(err)
	defer os.Remove(f.Name())

	err = s.client.ReadToFile(name, 0, int64(bodyLen), f)
	s.assert.Nil(err)

	output := make([]byte, bodyLen)
	f, _ = os.Open(f.Name())
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
	len := 20
	body := []byte(randomString(len))
	_, err := s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
		Body:   bytes.NewReader(body),
	})
	s.assert.Nil(err)

	result, err := s.client.ReadBuffer(name, 0, int64(len))

	// result should match generated body
	s.assert.Nil(err)
	s.assert.EqualValues(body, result)
}
func (s *clientTestSuite) TestReadInBuffer() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	bodyLen := 20
	body := []byte(randomString(bodyLen))
	_, err := s.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
		Body:   bytes.NewReader(body),
	})
	s.assert.Nil(err)

	outputLen := bodyLen - 5
	output := make([]byte, outputLen)
	err = s.client.ReadInBuffer(name, 0, int64(outputLen), output)

	// read in buffer should match first outputLen characters of generated body
	s.assert.Nil(err)
	s.assert.EqualValues(body[:outputLen], output)
}
func (s *clientTestSuite) TestcalculateBlockSize() {
}
func (s *clientTestSuite) TestWriteFromFile() {
	// TODO: (assert nil errors where needed)
	// generate file name and body
	// create local temp file with body - ioutil.TempFile
	// call write from file
	// get object
	// object body should match generated body
	// close temp file
}
func (s *clientTestSuite) TestWriteFromBuffer() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	body := []byte(randomString(20))

	err := s.client.WriteFromBuffer(name, nil, body)
	s.assert.Nil(err)

	result, err := s.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.client.Config.authConfig.BucketName),
		Key:    aws.String(name),
	})
	s.assert.Nil(err)

	// object body should match generated body
	defer result.Body.Close()
	output, err := ioutil.ReadAll(result.Body)
	s.assert.Nil(err)
	s.assert.EqualValues(body, output)
}
func (s *clientTestSuite) TestGetFileBlockOffsets() {
}
func (s *clientTestSuite) TestcreateBlock() {
}
func (s *clientTestSuite) TestcreateNewBlocks() {
}
func (s *clientTestSuite) TestremoveBlocks() {
}
func (s *clientTestSuite) TestTruncateFile() {
}
func (s *clientTestSuite) TestWrite() {
}
func (s *clientTestSuite) TeststageAndCommitModifiedBlocks() {
}
func (s *clientTestSuite) TestStageAndCommit() {
}
func (s *clientTestSuite) TestChangeMod() {
}
func (s *clientTestSuite) TestChangeOwner() {
}

func TestClient(t *testing.T) {
	suite.Run(t, new(clientTestSuite))
}
