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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
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
	"github.com/awnumar/memguard"
	"github.com/spf13/viper"

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

	// Secure keyID in enclave
	var encryptedKeyID *memguard.Enclave
	if viper.GetString("s3storage.key-id") != "" {
		encryptedKeyID = memguard.NewEnclave([]byte(viper.GetString("s3storage.key-id")))
		if encryptedKeyID == nil {
			return nil, fmt.Errorf(
				"config error in %s. Here's why: %s",
				compName,
				"Error storing key ID securely",
			)
		}
	}

	// Secure secretKey in enclave
	var encryptedSecretKey *memguard.Enclave
	if viper.GetString("s3storage.secret-key") != "" {
		encryptedSecretKey = memguard.NewEnclave([]byte(viper.GetString("s3storage.secret-key")))
		if encryptedSecretKey == nil {
			return nil, fmt.Errorf(
				"config error in %s. Here's why: %s",
				compName,
				"Error storing secret key securely",
			)
		}
	}

	// now push Options data into an Config
	configForS3Client := Config{
		AuthConfig: s3AuthConfig{
			BucketName: conf.BucketName,
			KeyID:      encryptedKeyID,
			SecretKey:  encryptedSecretKey,
			Region:     conf.Region,
			Profile:    conf.Profile,
			Endpoint:   conf.Endpoint,
		},
		prefixPath:                conf.PrefixPath,
		disableConcurrentDownload: conf.DisableConcurrentDownload,
		partSize:                  conf.PartSizeMb * common.MbToBytes,
		uploadCutoff:              conf.UploadCutoffMb * common.MbToBytes,
		usePathStyle:              conf.UsePathStyle,
		disableUsage:              conf.DisableUsage,
		enableDirMarker:           conf.EnableDirMarker,
		enableChecksum:            conf.EnableChecksum,
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
	storageTestConfigurationParameters.EnableDirMarker = true
	storageTestConfigurationParameters.EnableChecksum = true
	if configuration == "" {
		configuration = fmt.Sprintf(
			"s3storage:\n  bucket-name: %s\n  key-id: %s\n  secret-key: %s\n  endpoint: %s\n  region: %s\n  part-size-mb: %d\n"+
				"  upload-cutoff-mb: %d\n  use-path-style: %t\n  enable-dir-marker: %t\n  enable-checksum: %t\n",
			storageTestConfigurationParameters.BucketName,
			storageTestConfigurationParameters.KeyID,
			storageTestConfigurationParameters.SecretKey,
			storageTestConfigurationParameters.Endpoint,
			storageTestConfigurationParameters.Region,
			storageTestConfigurationParameters.PartSizeMb,
			storageTestConfigurationParameters.UploadCutoffMb,
			storageTestConfigurationParameters.UsePathStyle,
			storageTestConfigurationParameters.EnableDirMarker,
			storageTestConfigurationParameters.EnableChecksum,
		)
	}
	s.config = configuration

	s.assert = assert.New(s.T())

	var err error
	s.client, err = newTestClient(configuration)
	s.awsS3Client = s.client.AwsS3Client
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

func (s *clientTestSuite) TestCredentialsErrorInvalidKeyID() {
	defer s.cleanupTest()
	// setup
	config := fmt.Sprintf(
		"s3storage:\n  bucket-name: %s\n  key-id: %s\n  secret-key: %s\n  endpoint: %s",
		storageTestConfigurationParameters.BucketName,
		"WRONGKEYID",
		storageTestConfigurationParameters.SecretKey,
		storageTestConfigurationParameters.Endpoint,
	)
	// S3 connection creation should fail
	err := s.setupTestHelper(config, false)
	s.assert.Error(err)
}

func (s *clientTestSuite) TestCredentialsErrorInvalidSecretKey() {
	// TODO Fix this test for localstack
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestEnvVarCredentials using LocalStack.")
		return
	}

	defer s.cleanupTest()
	// setup
	config := fmt.Sprintf(
		"s3storage:\n  bucket-name: %s\n  key-id: %s\n  secret-key: %s\n  endpoint: %s",
		storageTestConfigurationParameters.BucketName,
		storageTestConfigurationParameters.KeyID,
		"WRONGSECRETKEY",
		storageTestConfigurationParameters.Endpoint,
	)
	// S3 connection creation should fail
	err := s.setupTestHelper(config, false)
	s.assert.Equal(errInvalidSecretKey, err)
}

func (s *clientTestSuite) TestCredentialsErrorInvalidBucket() {
	// TODO Fix this test for localstack
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestEnvVarCredentials using LocalStack.")
		return
	}

	defer s.cleanupTest()
	// setup
	config := fmt.Sprintf(
		"s3storage:\n  bucket-name: %s\n  key-id: %s\n  secret-key: %s\n  endpoint: %s",
		"WRONGBUCKET",
		storageTestConfigurationParameters.KeyID,
		storageTestConfigurationParameters.SecretKey,
		storageTestConfigurationParameters.Endpoint,
	)
	// S3 connection creation should fail
	err := s.setupTestHelper(config, false)
	s.assert.Error(err)
}

func (s *clientTestSuite) TestCredentialsErrorIncorrectEndpoint() {
	defer s.cleanupTest()
	// setup
	config := fmt.Sprintf(
		"s3storage:\n  bucket-name: %s\n  key-id: %s\n  secret-key: %s\n  endpoint: %s",
		storageTestConfigurationParameters.BucketName,
		storageTestConfigurationParameters.KeyID,
		storageTestConfigurationParameters.SecretKey,
		"https://s3.us-west-1.lyvecloud.seagate.com",
	)
	// S3 connection creation should fail
	err := s.setupTestHelper(config, false)
	s.assert.Equal(errInvalidCredential, err)
}

func (s *clientTestSuite) TestCredentialsErrorInvalidEndpoint() {
	defer s.cleanupTest()
	// setup
	config := fmt.Sprintf(
		"s3storage:\n  bucket-name: %s\n  key-id: %s\n  secret-key: %s\n  endpoint: %s\n  region: %s",
		"WRONGBUCKETNAME",
		"WRONGKEYID",
		"WRONGSECRETKEY",
		"https://google.com",
		"us-east-1",
	)
	// S3 connection creation should fail
	err := s.setupTestHelper(config, false)
	s.assert.Equal(errInvalidEndpoint, err)
}

func (s *clientTestSuite) TestCredentialsErrorInvalidEndpoint2() {
	defer s.cleanupTest()
	// setup
	config := fmt.Sprintf(
		"s3storage:\n  bucket-name: %s\n  key-id: %s\n  secret-key: %s\n  endpoint: %s",
		"WRONGBUCKETNAME",
		"WRONGKEYID",
		"WRONGSECRETKEY",
		"https://invalid.seagate.com",
	)
	// S3 connection creation should fail as this address does not exist
	err := s.setupTestHelper(config, false)
	s.assert.Equal(errInvalidEndpoint, err)
}

func (s *clientTestSuite) TestCredentialsIncorrectRegion() {
	// This test needs to be skipped for LocalStack, does not use region
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestEnvVarCredentials using LocalStack.")
		return
	}

	defer s.cleanupTest()
	// setup
	config := fmt.Sprintf(
		"s3storage:\n  bucket-name: %s\n  key-id: %s\n  secret-key: %s\n  region: %s",
		storageTestConfigurationParameters.BucketName,
		storageTestConfigurationParameters.KeyID,
		storageTestConfigurationParameters.SecretKey,
		"ap-southeast-1",
	)
	// S3 connection creation should fail as this address does not exist
	err := s.setupTestHelper(config, false)
	s.assert.Equal(errInvalidEndpoint, err)
}

func (s *clientTestSuite) TestEnvVarCredentials() {
	// TODO: Fix this test for LocalStack
	// This test needs to be skipped for LocalStack
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestEnvVarCredentials using LocalStack.")
		return
	}

	defer s.cleanupTest()
	// setup
	os.Setenv("AWS_ACCESS_KEY_ID", storageTestConfigurationParameters.KeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", storageTestConfigurationParameters.SecretKey)
	os.Setenv("AWS_REGION", storageTestConfigurationParameters.Region)
	config := fmt.Sprintf(
		"s3storage:\n  bucket-name: %s\n  endpoint: %s",
		storageTestConfigurationParameters.BucketName,
		storageTestConfigurationParameters.Endpoint,
	)
	// S3 connection should find credentials from environment variables
	err := s.setupTestHelper(config, false)
	s.assert.NoError(err)

	os.Unsetenv("AWS_ACCESS_KEY_ID")
	os.Unsetenv("AWS_SECRET_ACCESS_KEY")
	os.Unsetenv("AWS_REGION")
}

func (s *clientTestSuite) TestEnvVarCredentialsErr() {
	// TODO: Fix this test for LocalStack
	// This test needs to be skipped for LocalStack
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestEnvVarCredentials using LocalStack.")
		return
	}

	defer s.cleanupTest()
	// setup
	os.Setenv("AWS_ACCESS_KEY_ID", "WRONGACCESSKEY")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "WRONGSECRETKEY")
	config := fmt.Sprintf(
		"s3storage:\n  bucket-name: %s\n  endpoint: %s",
		storageTestConfigurationParameters.BucketName,
		storageTestConfigurationParameters.Endpoint,
	)
	// S3 connection should find credentials from environment variables
	err := s.setupTestHelper(config, false)
	s.assert.Equal(errInvalidCredential, err)

	os.Unsetenv("AWS_ACCESS_KEY_ID")
	os.Unsetenv("AWS_SECRET_ACCESS_KEY")
}

func (s *clientTestSuite) TestEnvVarCredentialsErrRegion() {
	// TODO: Fix this test for LocalStack
	// This test needs to be skipped for LocalStack
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestEnvVarCredentials using LocalStack.")
		return
	}

	defer s.cleanupTest()
	// setup
	os.Setenv("AWS_ACCESS_KEY_ID", storageTestConfigurationParameters.KeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", storageTestConfigurationParameters.SecretKey)
	// Use wrong, but a valid region
	os.Setenv("AWS_REGION", "ap-southeast-1")
	config := fmt.Sprintf(
		"s3storage:\n  bucket-name: %s\n",
		storageTestConfigurationParameters.BucketName,
	)
	// S3 connection should find credentials from environment variables
	err := s.setupTestHelper(config, false)
	s.assert.Equal(errInvalidEndpoint, err)

	os.Unsetenv("AWS_ACCESS_KEY_ID")
	os.Unsetenv("AWS_SECRET_ACCESS_KEY")
	os.Unsetenv("AWS_REGION")
}

func (s *clientTestSuite) TestDefaultConfig() {
	defer s.cleanupTest()
	// setup
	os.Setenv("AWS_ACCESS_KEY_ID", storageTestConfigurationParameters.KeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", storageTestConfigurationParameters.SecretKey)
	config := fmt.Sprintf("s3storage:\n  bucket-name: %s",
		storageTestConfigurationParameters.BucketName)
	// Test using default region, and default endpoint
	// Ignore error because in unit tests this will fail since some unit tests use localstack
	// so we can't use default endpoint
	_ = s.setupTestHelper(config, false)

	s.assert.Equal(
		"https://s3.us-east-1.sv15.lyve.seagate.com",
		s.client.Config.AuthConfig.Endpoint,
	)
	s.assert.Equal("us-east-1", s.client.Config.AuthConfig.Region)

	os.Unsetenv("AWS_ACCESS_KEY_ID")
	os.Unsetenv("AWS_SECRET_ACCESS_KEY")
}

func (s *clientTestSuite) TestCredentialPrecedenceEnvOverConfig() {
	// TODO Fix this test for localstack
	// This test needs to be skipped for LocalStack as it doesn't use a region
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestEnvVarCredentials using LocalStack.")
		return
	}

	defer s.cleanupTest()
	// setup
	os.Setenv("AWS_ACCESS_KEY_ID", storageTestConfigurationParameters.KeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", storageTestConfigurationParameters.SecretKey)
	config := fmt.Sprintf(
		"s3storage:\n  bucket-name: %s\n  endpoint: %s\n  key-id: %s\n  secret-key: %s",
		storageTestConfigurationParameters.BucketName,
		s.client.Config.AuthConfig.Endpoint,
		storageTestConfigurationParameters.KeyID,
		"WRONGSECRETKEY",
	)
	// Wrong credentials should take precedence, so S3 connection should fail
	err := s.setupTestHelper(config, false)
	s.assert.Error(err)

	os.Unsetenv("AWS_ACCESS_KEY_ID")
	os.Unsetenv("AWS_SECRET_ACCESS_KEY")
}

func (s *clientTestSuite) TestCredentialPrecedenceEnvOverProfile() {
	// TODO Fix this test for localstack
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestEnvVarCredentials using LocalStack.")
		return
	}

	defer s.cleanupTest()
	// setup
	os.Setenv("AWS_ACCESS_KEY_ID", storageTestConfigurationParameters.KeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", storageTestConfigurationParameters.SecretKey)
	config := fmt.Sprintf(
		"s3storage:\n  bucket-name: %s\n  endpoint: %s\n  profile: %s",
		storageTestConfigurationParameters.BucketName,
		s.client.Config.AuthConfig.Endpoint,
		"NoProfile",
	)
	// Invalid profile, but environment variables should take precedence
	err := s.setupTestHelper(config, false)
	s.assert.NoError(err)

	os.Unsetenv("AWS_ACCESS_KEY_ID")
	os.Unsetenv("AWS_SECRET_ACCESS_KEY")
}

func (s *clientTestSuite) TestCredentialPrecedenceConfigOverProfile() {
	// TODO Fix this test for localstack
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestEnvVarCredentials using LocalStack.")
		return
	}

	defer s.cleanupTest()
	// setup
	config := fmt.Sprintf(
		"s3storage:\n  bucket-name: %s\n  endpoint: %s\n  key-id: %s\n  secret-key: %s\n  profile: %s",
		storageTestConfigurationParameters.BucketName,
		storageTestConfigurationParameters.Endpoint,
		storageTestConfigurationParameters.KeyID,
		storageTestConfigurationParameters.SecretKey,
		"NoProfile",
	)
	// Invalid profile, but config should take precedence
	err := s.setupTestHelper(config, false)
	s.assert.NoError(err)
}

func (s *clientTestSuite) TestCredentialPrecedenceRegion() {
	// This test needs to be skipped for LocalStack as it doesn't use a region
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestEnvVarCredentials using LocalStack.")
		return
	}

	defer s.cleanupTest()
	// setup
	os.Setenv("AWS_REGION", storageTestConfigurationParameters.Region)
	config := fmt.Sprintf(
		"s3storage:\n  bucket-name: %s\n  key-id: %s\n  secret-key: %s\n  region: %s",
		storageTestConfigurationParameters.BucketName,
		storageTestConfigurationParameters.KeyID,
		storageTestConfigurationParameters.SecretKey,
		"ap-southeast-1",
	)
	// Wrong region should take precedence, so S3 connection should fail
	err := s.setupTestHelper(config, false)
	s.assert.Error(err)

	os.Unsetenv("AWS_REGION")
}

func (s *clientTestSuite) TestSetEndpointFromRegion() {
	defer s.cleanupTest()
	// setup
	config := fmt.Sprintf(
		"s3storage:\n  bucket-name: %s\n  key-id: %s\n  secret-key: %s\n  region: %s",
		storageTestConfigurationParameters.BucketName,
		storageTestConfigurationParameters.KeyID,
		storageTestConfigurationParameters.SecretKey,
		"us-west-2",
	)
	// Should set endpoint based on lyve cloud if the region is provided and no endpoint is provided
	err := s.setupTestHelper(config, false)
	// Connection should fail since this is a different endpoint
	s.assert.Error(err)
	s.assert.Equal(
		"https://s3.us-west-2.sv15.lyve.seagate.com",
		s.client.Config.AuthConfig.Endpoint,
	)
}

func (s *clientTestSuite) TestSetRegionFromEndpoint() {
	// This test needs to be skipped for LocalStack as endpoint does not have a region
	if storageTestConfigurationParameters.BucketName == "test" {
		fmt.Println("Skipping TestSetRegionFromEndpoint using LocalStack.")
		return
	}

	defer s.cleanupTest()
	// setup
	config := fmt.Sprintf(
		"s3storage:\n  bucket-name: %s\n  key-id: %s\n  secret-key: %s\n  endpoint: %s",
		storageTestConfigurationParameters.BucketName,
		storageTestConfigurationParameters.KeyID,
		storageTestConfigurationParameters.SecretKey,
		storageTestConfigurationParameters.Endpoint,
	)
	// Should set region automatically from endpoint
	err := s.setupTestHelper(config, false)
	s.assert.NoError(err)
	s.assert.NotNil(s.client.Config.AuthConfig.Region)
}

func (s *clientTestSuite) TestGetRegionEndpoint() {
	defer s.cleanupTest()

	region, err := getRegionFromEndpoint("https://s3.us-east-1.lyvecloud.seagate.com")
	s.assert.NoError(err)
	s.assert.Equal("us-east-1", region)

	region, err = getRegionFromEndpoint("https://s3.eu-west-1.sv15.lyve.seagate.com")
	s.assert.NoError(err)
	s.assert.Equal("eu-west-1", region)

	region, err = getRegionFromEndpoint("https://s3.us-east-2.amazonaws.com")
	s.assert.NoError(err)
	s.assert.Equal("us-east-2", region)

	region, err = getRegionFromEndpoint("http://s3.us-east-2.amazonaws.com")
	s.assert.NoError(err)
	s.assert.Equal("us-east-2", region)

	region, err = getRegionFromEndpoint("https://s3.dualstack.us-east-2.amazonaws.com")
	s.assert.NoError(err)
	s.assert.Equal("us-east-2", region)

	region, err = getRegionFromEndpoint("https://s3-fips.us-east-2.amazonaws.com")
	s.assert.NoError(err)
	s.assert.Equal("us-east-2", region)

	region, err = getRegionFromEndpoint("https://s3.us-west-1.wasabisys.com")
	s.assert.NoError(err)
	s.assert.Equal("us-west-1", region)

	region, err = getRegionFromEndpoint("")
	s.assert.Error(err)
	s.assert.Empty(region)
}

func (s *clientTestSuite) TestListBuckets() {
	defer s.cleanupTest()
	// TODO: generalize this test by creating, listing, then destroying a bucket
	buckets, err := s.client.ListBuckets(context.TODO())
	s.assert.NoError(err)
	s.assert.Contains(buckets, storageTestConfigurationParameters.BucketName)
}

func (s *clientTestSuite) TestDefaultBucketName() {
	defer s.cleanupTest()
	// write config with no bucket name
	config := fmt.Sprintf(
		"s3storage:\n  key-id: %s\n  secret-key: %s\n  endpoint: %s\n  region: %s\n  use-path-style: %t\n",
		storageTestConfigurationParameters.KeyID,
		storageTestConfigurationParameters.SecretKey,
		storageTestConfigurationParameters.Endpoint,
		storageTestConfigurationParameters.Region,
		storageTestConfigurationParameters.UsePathStyle,
	)
	err := s.setupTestHelper(config, false)
	s.assert.NoError(err)
	buckets, _ := s.client.ListBuckets(ctx)
	s.assert.Contains(buckets, s.client.Config.AuthConfig.BucketName)
}

func (s *clientTestSuite) TestSetPrefixPath() {
	defer s.cleanupTest()
	// setup
	prefix := generateDirectoryName()
	fileName := generateFileName()

	err := s.client.SetPrefixPath(prefix)
	s.assert.NoError(err)                                    //stub
	err = s.client.CreateFile(ctx, fileName, os.FileMode(0)) // create file uses prefix
	s.assert.NoError(err)

	// object should be at prefix
	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.client.Config.AuthConfig.BucketName),
		Key:          aws.String(path.Join(prefix, fileName)),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
}
func (s *clientTestSuite) TestCreateFile() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()

	err := s.client.CreateFile(ctx, name, os.FileMode(0))
	s.assert.NoError(err)

	// file should be in bucket
	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.client.Config.AuthConfig.BucketName),
		Key:          aws.String(name),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
}
func (s *clientTestSuite) TestCreateDirectory() {
	defer s.cleanupTest()
	// setup
	name := generateDirectoryName()

	err := s.client.CreateDirectory(ctx, name)
	s.assert.NoError(err)
}
func (s *clientTestSuite) TestCreateLink() {
	defer s.cleanupTest()
	// setup
	target := generateFileName()

	_, err := s.awsS3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(target),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})

	s.assert.NoError(err)
	source := generateFileName()

	err = s.client.CreateLink(ctx, source, target, true)
	s.assert.NoError(err)

	source = s.client.getKey(source, true, false)

	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.client.Config.AuthConfig.BucketName),
		Key:          aws.String(source),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)

	// object body should match target file name
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.EqualValues(target, output)

}
func (s *clientTestSuite) TestReadLink() {
	defer s.cleanupTest()
	// setup
	target := generateFileName()

	source := generateFileName()

	err := s.client.CreateLink(ctx, source, target, true)
	s.assert.NoError(err)

	source = s.client.getKey(source, true, false)

	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.client.Config.AuthConfig.BucketName),
		Key:          aws.String(source),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)

	defer result.Body.Close()

	// object body should match target file name
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.Equal(target, string(output))

}

func (s *clientTestSuite) TestDeleteLink() {
	defer s.cleanupTest()
	// setup
	target := generateFileName()

	source := generateFileName()

	err := s.client.CreateLink(ctx, source, target, true)
	s.assert.NoError(err)

	source = s.client.getKey(source, true, false)

	_, err = s.awsS3Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.client.Config.AuthConfig.BucketName),
		Key:    aws.String(source),
	})
	s.assert.NoError(err)

	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.client.Config.AuthConfig.BucketName),
		Key:          aws.String(source),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.Error(err)
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

		err := s.client.CreateLink(ctx, folder+sources[i], targets[i], true)
		s.assert.NoError(err)

		sources[i] = s.client.getKey(sources[i], true, false)

		// make sure the links are there
		result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
			Bucket:       aws.String(s.client.Config.AuthConfig.BucketName),
			Key:          aws.String(folder + sources[i]),
			ChecksumMode: types.ChecksumModeEnabled,
		})
		s.assert.NoError(err)

		// object body should match target file name
		defer result.Body.Close()
		buffer, err := io.ReadAll(result.Body)
		s.assert.NoError(err)

		s.assert.Equal(targets[i], string(buffer))
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
	_, err := s.awsS3Client.DeleteObjects(context.Background(), &s3.DeleteObjectsInput{
		Bucket: aws.String(s.client.Config.AuthConfig.BucketName),
		Delete: &types.Delete{
			Objects: keyList,
			Quiet:   aws.Bool(true),
		},
	})
	s.assert.NoError(err)

	// make sure the links aren't there
	for i := range sources {
		_, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
			Bucket:       aws.String(s.client.Config.AuthConfig.BucketName),
			Key:          aws.String(folder + sources[i]),
			ChecksumMode: types.ChecksumModeEnabled,
		})
		s.assert.Error(err)

	}
}

func (s *clientTestSuite) TestDeleteFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	_, err := s.awsS3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(name),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})
	s.assert.NoError(err)

	err = s.client.DeleteFile(ctx, name)
	s.assert.NoError(err)

	// This is similar to the s3 bucket command, use getobject for now
	//_, err = s.s3.GetAttr(internal.GetAttrOptions{name, false})
	// File should not be in the account
	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.client.Config.AuthConfig.BucketName),
		Key:          aws.String(name),
		ChecksumMode: types.ChecksumModeEnabled,
	})

	s.assert.Error(err)
}
func (s *clientTestSuite) TestDeleteDirectory() {
	defer s.cleanupTest()
	// setup
	dirName := generateDirectoryName()
	fileName := generateFileName() // can't have empty directory
	_, err := s.awsS3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(path.Join(dirName, fileName)),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})
	s.assert.NoError(err)

	err = s.client.DeleteDirectory(ctx, dirName)
	s.assert.NoError(err)

	// file in directory should no longer be there
	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.client.Config.AuthConfig.BucketName),
		Key:          aws.String(path.Join(dirName, fileName)),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.Error(err)
}
func (s *clientTestSuite) TestRenameFile() {
	defer s.cleanupTest()
	// Setup

	src := generateFileName()
	_, err := s.awsS3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(src),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})
	s.assert.NoError(err)
	dst := generateFileName()

	err = s.client.RenameFile(ctx, src, dst, false)
	s.assert.NoError(err)

	// Src should not be in the account
	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.client.Config.AuthConfig.BucketName),
		Key:          aws.String(src),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.Error(err)
	// Dst should be in the account
	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.client.Config.AuthConfig.BucketName),
		Key:          aws.String(dst),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
}
func (s *clientTestSuite) TestRenameFileError() {
	defer s.cleanupTest()
	// Setup

	src := generateFileName()
	dst := generateFileName()

	err := s.client.RenameFile(ctx, src, dst, false)
	s.assert.EqualError(err, syscall.ENOENT.Error())

	// Src should not be in the account
	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.client.Config.AuthConfig.BucketName),
		Key:          aws.String(src),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.Error(err)
	// Dst should not be in the account
	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.client.Config.AuthConfig.BucketName),
		Key:          aws.String(dst),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.Error(err)
}
func (s *clientTestSuite) TestRenameDirectory() {
	defer s.cleanupTest()
	// setup
	srcDir := generateDirectoryName()
	fileName := generateFileName() // can't have empty directory
	_, err := s.awsS3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(path.Join(srcDir, fileName)),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})
	s.assert.NoError(err)

	dstDir := generateDirectoryName()
	err = s.client.RenameDirectory(ctx, srcDir, dstDir)
	s.assert.NoError(err)

	// file in srcDir should no longer be there
	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.client.Config.AuthConfig.BucketName),
		Key:          aws.String(path.Join(srcDir, fileName)),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.Error(err)
	// file in dstDir should be there
	_, err = s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.client.Config.AuthConfig.BucketName),
		Key:          aws.String(path.Join(dstDir, fileName)),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)
}
func (s *clientTestSuite) TestGetAttrDir() {
	defer s.cleanupTest()
	// setup
	dirName := generateDirectoryName()

	_, err := s.awsS3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(dirName + "/"),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})
	s.assert.NoError(err)

	attr, err := s.client.GetAttr(ctx, dirName)
	s.assert.NoError(err)
	s.assert.NotNil(attr)
	s.assert.True(attr.IsDir())
}
func (s *clientTestSuite) TestGetAttrDirWithOnlyFile() {
	defer s.cleanupTest()
	// setup
	dirName := generateDirectoryName()
	filename := dirName + "/" + generateFileName()

	_, err := s.awsS3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(filename),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})
	s.assert.NoError(err)

	attr, err := s.client.GetAttr(ctx, dirName)
	s.assert.NoError(err)
	s.assert.NotNil(attr)
	s.assert.True(attr.IsDir())
}
func (s *clientTestSuite) TestGetAttrFile() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	maxBodyLen := 50
	minBodyLen := 10
	bodyLen := rand.IntN(maxBodyLen-minBodyLen) + minBodyLen
	body := []byte(randomString(bodyLen))
	_, err := s.awsS3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(name),
		Body:              bytes.NewReader(body),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})
	s.assert.NoError(err)

	before, err := s.client.GetAttr(ctx, name)

	// file info
	s.assert.NoError(err)
	s.assert.NotNil(before)
	s.assert.False(before.IsDir())
	s.assert.False(before.IsSymlink())

	// file size
	s.assert.EqualValues(bodyLen, before.Size)

	// file time
	s.assert.NotNil(before.Mtime)

	time.Sleep(1 * time.Second) // Wait and then modify the file again

	_, err = s.awsS3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(name),
		Body:              bytes.NewReader(body),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})
	s.assert.NoError(err)

	after, err := s.client.GetAttr(ctx, name)
	s.assert.NoError(err)
	s.assert.NotNil(after.Mtime)

	s.assert.True(after.Mtime.After(before.Mtime))
}
func (s *clientTestSuite) TestGetAttrError() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()

	// non existent file should throw error
	_, err := s.client.GetAttr(ctx, name)
	s.assert.Error(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}
func (s *clientTestSuite) TestList() {
	defer s.cleanupTest()
	// setup
	base := generateDirectoryName()
	// setup directory hierarchy like setupHierarchy in s3storage_test where 'a' is generated base
	// a/c1/gc1
	gc1 := base + "/c1" + "/gc1"
	_, err := s.awsS3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(gc1),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})
	s.assert.NoError(err)
	ctx := context.Background()
	// a/c2
	c2 := base + "/c2"
	_, err = s.awsS3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(c2),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})
	s.assert.NoError(err)
	// ab/c1
	abc1 := base + "b/c1"
	_, err = s.awsS3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(abc1),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})
	s.assert.NoError(err)
	// ac
	ac := base + "c"
	_, err = s.awsS3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(ac),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})
	s.assert.NoError(err)

	// with trailing "/" should return only the directory c1 and the file c2
	baseTrail := base + "/"
	objects, _, err := s.client.List(ctx, baseTrail, nil, 0)
	s.assert.NoError(err)
	s.assert.NotNil(objects)
	s.assert.Len(objects, 2)
	s.assert.Equal("c1", objects[0].Name)
	s.assert.True(objects[0].IsDir())
	s.assert.Equal("c2", objects[1].Name)
	s.assert.False(objects[1].IsDir())

	// without trailing "/" only get file ac
	// if not including the trailing "/", List will return any files with the given prefix
	// but no directories
	objects, _, err = s.client.List(ctx, base, nil, 0)
	s.assert.NoError(err)
	s.assert.NotNil(objects)
	s.assert.Len(objects, 1)
	s.assert.Equal(objects[0].Name, base+"c")
	s.assert.False(objects[0].IsDir())

	// When listing the root, List should not include the root
	objects, _, err = s.client.List(ctx, "", nil, 0)
	s.assert.NoError(err)
	s.assert.NotNil(objects)
	s.assert.NotEmpty(objects)
	s.assert.NotEmpty(objects[0].Name)
	s.assert.NotEqual("/", objects[0].Name)
	s.assert.NotEqual(".", objects[0].Name)
}
func (s *clientTestSuite) TestReadToFile() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	maxBodyLen := 50
	minBodyLen := 10
	bodyLen := rand.IntN(maxBodyLen-minBodyLen) + minBodyLen
	body := []byte(randomString(bodyLen))
	_, err := s.awsS3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(name),
		Body:              bytes.NewReader(body),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})
	s.assert.NoError(err)

	f, err := os.CreateTemp("", name+".tmp")
	s.assert.NoError(err)
	defer os.Remove(f.Name())

	err = s.client.ReadToFile(ctx, name, 0, 0, f)
	s.assert.NoError(err)

	// file content should match generated body
	output := make([]byte, bodyLen)
	f, err = os.Open(f.Name())
	s.assert.NoError(err)
	outputLen, err := f.Read(output)
	s.assert.NoError(err)
	s.assert.Equal(bodyLen, outputLen)
	s.assert.Equal(body, output)
	f.Close()
}

func (s *clientTestSuite) TestReadToFileRanged() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	maxBodyLen := 50
	minBodyLen := 10
	bodyLen := rand.IntN(maxBodyLen-minBodyLen) + minBodyLen
	body := []byte(randomString(bodyLen))
	_, err := s.awsS3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(name),
		Body:              bytes.NewReader(body),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})
	s.assert.NoError(err)

	f, err := os.CreateTemp("", name+".tmp")
	s.assert.NoError(err)
	defer os.Remove(f.Name())

	err = s.client.ReadToFile(ctx, name, 0, int64(bodyLen), f)
	s.assert.NoError(err)

	// file content should match generated body
	output := make([]byte, bodyLen)
	f, err = os.Open(f.Name())
	s.assert.NoError(err)
	outputLen, err := f.Read(output)
	s.assert.NoError(err)
	s.assert.Equal(bodyLen, outputLen)
	s.assert.Equal(body, output)
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
	bodyLen := rand.IntN(maxBodyLen-minBodyLen) + minBodyLen
	body := []byte(randomString(bodyLen))
	_, err := s.awsS3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(name),
		Body:              bytes.NewReader(body),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})
	s.assert.NoError(err)

	f, err := os.CreateTemp("", name+".tmp")
	s.assert.NoError(err)
	defer os.Remove(f.Name())

	err = s.client.ReadToFile(ctx, name, 0, 0, f)
	s.assert.NoError(err)

	// file content should match generated body
	output := make([]byte, bodyLen)
	f, err = os.Open(f.Name())
	s.assert.NoError(err)
	outputLen, err := f.Read(output)
	s.assert.NoError(err)
	s.assert.Equal(bodyLen, outputLen)
	s.assert.Equal(body, output)
	f.Close()
}

func (s *clientTestSuite) TestReadBuffer() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	maxBodyLen := 50
	minBodyLen := 10
	bodyLen := rand.IntN(maxBodyLen-minBodyLen) + minBodyLen
	body := []byte(randomString(bodyLen))
	_, err := s.awsS3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(name),
		Body:              bytes.NewReader(body),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})
	s.assert.NoError(err)

	result, err := s.client.ReadBuffer(ctx, name, 0, int64(bodyLen), false)

	// result should match generated body
	s.assert.NoError(err)
	s.assert.Equal(body, result)
}
func (s *clientTestSuite) TestReadInBuffer() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	maxBodyLen := 50
	minBodyLen := 10
	bodyLen := rand.IntN(maxBodyLen-minBodyLen) + minBodyLen
	body := []byte(randomString(bodyLen))
	_, err := s.awsS3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(name),
		Body:              bytes.NewReader(body),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})
	s.assert.NoError(err)

	outputLen := rand.IntN(bodyLen-1) + 1 // minimum buffer length of 1
	output := make([]byte, outputLen)
	err = s.client.ReadInBuffer(ctx, name, 0, int64(outputLen), output)

	// read in buffer should match first outputLen characters of generated body
	s.assert.NoError(err)
	s.assert.Equal(body[:outputLen], output)
}
func (s *clientTestSuite) TestWriteFromFile() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	maxBodyLen := 50
	minBodyLen := 10
	bodyLen := rand.IntN(maxBodyLen-minBodyLen) + minBodyLen
	body := []byte(randomString(bodyLen))
	f, err := os.CreateTemp("", name+".tmp")
	s.assert.NoError(err)
	defer os.Remove(f.Name())
	outputLen, err := f.Write(body)
	s.assert.NoError(err)
	s.assert.Equal(bodyLen, outputLen)
	var options internal.WriteFileOptions //stub

	err = s.client.WriteFromFile(ctx, name, options.Metadata, f)
	s.assert.NoError(err)
	f.Close()

	//todo: create another test like this one that does getObject here with and without the .rclonelink suffix
	// this checks the integration between attr cache and s3storage for metadata.make sure the flag passed down is
	// respected.
	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.client.Config.AuthConfig.BucketName),
		Key:          aws.String(name),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)

	// object body should match generated body written to file
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.Equal(body, output)
}
func (s *clientTestSuite) TestWriteFromBuffer() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	maxBodyLen := 50
	minBodyLen := 10
	bodyLen := rand.IntN(maxBodyLen-minBodyLen) + minBodyLen
	body := []byte(randomString(bodyLen))

	var options internal.WriteFileOptions //stub

	err := s.client.WriteFromBuffer(ctx, name, options.Metadata, body)
	s.assert.NoError(err)

	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.client.Config.AuthConfig.BucketName),
		Key:          aws.String(name),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)

	// object body should match generated body
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.Equal(body, output)
}
func (s *clientTestSuite) TestTruncateFile() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	maxBodyLen := 50
	minBodyLen := 10
	bodyLen := rand.IntN(maxBodyLen-minBodyLen) + minBodyLen
	body := []byte(randomString(bodyLen))
	_, err := s.awsS3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(name),
		Body:              bytes.NewReader(body),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})
	s.assert.NoError(err)

	size := rand.IntN(bodyLen-1) + 1 // minimum size of 1
	err = s.client.TruncateFile(ctx, name, int64(size))
	s.assert.NoError(err)

	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.client.Config.AuthConfig.BucketName),
		Key:          aws.String(name),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)

	// object body should match truncated initial body
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.Equal(body[:size], output)
}
func (s *clientTestSuite) TestWrite() {
	defer s.cleanupTest()
	// setup
	name := generateFileName()
	maxBodyLen := 50
	minBodyLen := 10
	bodyLen := rand.IntN(maxBodyLen-minBodyLen) + minBodyLen
	oldBody := []byte(randomString(bodyLen))
	_, err := s.awsS3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:            aws.String(s.client.Config.AuthConfig.BucketName),
		Key:               aws.String(name),
		Body:              bytes.NewReader(oldBody),
		ChecksumAlgorithm: s.client.Config.checksumAlgorithm,
	})
	s.assert.NoError(err)

	offset := rand.IntN(bodyLen-1) + 1 // minimum offset of 1
	newData := []byte(randomString(bodyLen - offset))
	h := handlemap.NewHandle(name)
	err = s.client.Write(
		ctx,
		internal.WriteFileOptions{Handle: h, Offset: int64(offset), Data: newData},
	)
	s.assert.NoError(err)

	result, err := s.awsS3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket:       aws.String(s.client.Config.AuthConfig.BucketName),
		Key:          aws.String(name),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	s.assert.NoError(err)

	// object body should match generated combo of old and new data
	defer result.Body.Close()
	output, err := io.ReadAll(result.Body)
	s.assert.NoError(err)
	s.assert.Equal(oldBody[:offset], output[:offset])
	s.assert.Equal(newData, output[offset:])
}

func (s *clientTestSuite) TestGetCommittedBlockListSmallFile() {
	defer s.cleanupTest()
	name := generateFileName()
	bodyLen := 1024
	body := []byte(randomString(bodyLen))

	err := s.client.WriteFromBuffer(ctx, name, nil, body)
	s.assert.NoError(err)

	blockList, err := s.client.GetCommittedBlockList(ctx, name)

	s.assert.NoError(err)
	s.assert.NotNil(blockList)
	s.assert.Empty(*blockList)
}

func (s *clientTestSuite) TestGetCommittedBlockListMultipartFile() {
	defer s.cleanupTest()
	name := generateFileName()
	partSize := s.client.Config.partSize
	bodyLen := int(partSize*2 + partSize/2)
	body := randomString(bodyLen)

	err := s.client.WriteFromBuffer(ctx, name, nil, []byte(body))
	s.assert.NoError(err)

	blockList, err := s.client.GetCommittedBlockList(ctx, name)

	s.assert.NoError(err)
	s.assert.NotNil(blockList)

	expectedParts := int(bodyLen / int(partSize))
	if bodyLen%int(partSize) != 0 {
		expectedParts++
	}
	s.assert.Len(*blockList, expectedParts)

	var currentOffset int64 = 0
	for i, block := range *blockList {
		s.assert.Equal(currentOffset, block.Offset)

		expectedSize := uint64(partSize)
		if i == expectedParts-1 { // Last part might be smaller
			expectedSize = uint64(bodyLen) - uint64(currentOffset)
		}
		s.assert.Equal(expectedSize, block.Size)

		currentOffset += int64(block.Size)
	}
	s.assert.Equal(int64(bodyLen), currentOffset)
}

func (s *clientTestSuite) TestGetCommittedBlockListNonExistentFile() {
	defer s.cleanupTest()
	name := generateFileName()

	blockList, err := s.client.GetCommittedBlockList(ctx, name)

	s.assert.Error(err)
	s.assert.Equal(syscall.ENOENT, err)
	s.assert.Nil(blockList)
}

func TestClient(t *testing.T) {
	suite.Run(t, new(clientTestSuite))
}
