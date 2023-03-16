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
	bucket      string
}

func newTestClient(configuration string) (*Client, error) {
	// push the given config data to config.go
	_ = config.ReadConfigFromReader(strings.NewReader(configuration))
	// ask config to give us the config data back as S3StorageOptions
	conf := S3StorageOptions{}
	err := config.UnmarshalKey(compName, &conf)
	if err != nil {
		fmt.Println("Unable to unmarshal")
		log.Err("ClientTest::newTestClient : config error [invalid config attributes]")
		return nil, fmt.Errorf("config error in %s. Here's why: %s", compName, err.Error())
	}
	// now push S3StorageOptions data into an S3StorageConfig
	configForS3Client := S3StorageConfig{
		authConfig: s3AuthConfig{
			BucketName: conf.BucketName,
			AccessKey:  conf.AccessKey,
			SecretKey:  conf.SecretKey,
			Region:     conf.Region,
			Endpoint:   conf.Endpoint,
		},
		prefixPath: conf.PrefixPath,
	}
	// Validate endpoint
	if conf.Endpoint == "" {
		log.Warn("ParseAndValidateConfig : account endpoint not provided, assuming the default .lyvecloud.seagate.com style endpoint")
		configForS3Client.authConfig.Endpoint = fmt.Sprintf("s3.%s.lyvecloud.seagate.com", conf.Region)
	}
	// create a Client
	client := NewS3StorageConnection(configForS3Client)

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
	s.setupTestHelper("", "", true)
}

func (s *clientTestSuite) setupTestHelper(configuration string, bucket string, create bool) {
	if bucket == "" {
		bucket = generateBucketName()
	}
	s.bucket = bucket
	if configuration == "" {
		configuration = fmt.Sprintf("s3storage:\n  bucket-name: %s\n  access-key: %s\n  secret-key: %s\n  endpoint: %s\n  region: %s",
			storageTestConfigurationParameters.BucketName, storageTestConfigurationParameters.AccessKey,
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
	// We need to get permissions to create buckets in Lyve Cloud, or implement this against AWS S3.
	buckets, err := s.client.ListBuckets()
	s.assert.Nil(err)
	s.assert.Equal(buckets, []string{"stxe1-srg-lens-lab1"})
}
func (s *clientTestSuite) TestSetPrefixPath() {
}
func (s *clientTestSuite) TestCreateFile() {
}
func (s *clientTestSuite) TestCreateDirectory() {
}
func (s *clientTestSuite) TestCreateLink() {
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
}
func (s *clientTestSuite) TestReadBuffer() {
}
func (s *clientTestSuite) TestReadInBuffer() {
}
func (s *clientTestSuite) TestcalculateBlockSize() {
}
func (s *clientTestSuite) TestWriteFromFile() {
}
func (s *clientTestSuite) TestWriteFromBuffer() {
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
