/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2026 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.

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
	"fmt"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/awnumar/memguard"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type configTestSuite struct {
	suite.Suite
	assert  *assert.Assertions
	s3      *S3Storage
	opt     Options
	secrets ConfigSecrets
}

func (s *configTestSuite) SetupTest() {
	// Silent logger
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}

	// Set S3Storage
	s.s3 = &S3Storage{}

	// Set Options
	s.opt = Options{
		BucketName:         "testBucketName",
		Region:             "testRegion",
		Profile:            "testProfile",
		Endpoint:           "testEndpoint",
		RestrictedCharsWin: true,
		PrefixPath:         "testPrefixPath",
	}

	encryptedKeyID := memguard.NewEnclave([]byte("testKeyId"))
	encryptedSecretKey := memguard.NewEnclave([]byte("testKeyId"))

	s.secrets = ConfigSecrets{
		KeyID:     encryptedKeyID,
		SecretKey: encryptedSecretKey,
	}

	// Create assertions
	s.assert = assert.New(s.T())
}

func (s *configTestSuite) TestEmptyBucketName() {
	// When
	s.opt.BucketName = ""

	// Then
	err := ParseAndValidateConfig(s.s3, s.opt, s.secrets)
	s.assert.NoError(err)
}

// TODO: make errors from the default aws credentials provider visible to the user somehow

func (s *configTestSuite) TestConfigParse() {
	// When
	err := ParseAndValidateConfig(s.s3, s.opt, s.secrets)

	// Then
	s.assert.NoError(err)
	s.assert.Equal(s.opt.BucketName, s.s3.stConfig.AuthConfig.BucketName)
	s.assert.Equal(s.opt.Region, s.s3.stConfig.AuthConfig.Region)
	s.assert.Equal(s.opt.Profile, s.s3.stConfig.AuthConfig.Profile)
	s.assert.Equal(s.opt.Endpoint, s.s3.stConfig.AuthConfig.Endpoint)
	s.assert.Equal(s.opt.RestrictedCharsWin, s.s3.stConfig.restrictedCharsWin)
	s.assert.Equal(s.opt.PrefixPath, s.s3.stConfig.prefixPath)
}

func (s *configTestSuite) TestPrefixPath() {
	// When
	s.opt.PrefixPath = "/testPrefixPath"

	// Then
	err := ParseAndValidateConfig(s.s3, s.opt, s.secrets)
	s.assert.NoError(err)
	s.assert.Equal("testPrefixPath", s.s3.stConfig.prefixPath)
}

func (s *configTestSuite) TestValidChecksum() {
	// When
	s.opt.EnableChecksum = true

	// Then
	// Default should be CRC32 if user does not provide checksum algorithm
	err := ParseAndValidateConfig(s.s3, s.opt, s.secrets)
	s.assert.NoError(err)
	s.assert.True(s.s3.stConfig.enableChecksum)
	s.assert.Equal(types.ChecksumAlgorithm("CRC32"), s.s3.stConfig.checksumAlgorithm)

	// When
	s.opt.EnableChecksum = true
	s.opt.ChecksumAlgorithm = "SHA1"

	// Then
	err = ParseAndValidateConfig(s.s3, s.opt, s.secrets)
	s.assert.NoError(err)
	s.assert.True(s.s3.stConfig.enableChecksum)
	s.assert.Equal(types.ChecksumAlgorithm("SHA1"), s.s3.stConfig.checksumAlgorithm)

	// When
	s.opt.ChecksumAlgorithm = "SHA256"

	// Then
	err = ParseAndValidateConfig(s.s3, s.opt, s.secrets)
	s.assert.NoError(err)
	s.assert.Equal(types.ChecksumAlgorithm("SHA256"), s.s3.stConfig.checksumAlgorithm)

	// When
	s.opt.ChecksumAlgorithm = "CRC32"

	// Then
	err = ParseAndValidateConfig(s.s3, s.opt, s.secrets)
	s.assert.NoError(err)
	s.assert.Equal(types.ChecksumAlgorithm("CRC32"), s.s3.stConfig.checksumAlgorithm)

	// When
	s.opt.ChecksumAlgorithm = "CRC32C"

	// Then
	err = ParseAndValidateConfig(s.s3, s.opt, s.secrets)
	s.assert.NoError(err)
	s.assert.Equal(types.ChecksumAlgorithm("CRC32C"), s.s3.stConfig.checksumAlgorithm)
}

func (s *configTestSuite) TestInvalidChecksum() {
	// When
	s.opt.EnableChecksum = true
	s.opt.ChecksumAlgorithm = "invalid"

	// Then
	err := ParseAndValidateConfig(s.s3, s.opt, s.secrets)
	s.assert.Error(err)
	s.assert.ErrorIs(err, errInvalidConfigField)
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(configTestSuite))
}
