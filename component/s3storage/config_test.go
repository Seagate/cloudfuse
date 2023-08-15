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
	"lyvecloudfuse/common"
	"lyvecloudfuse/common/log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type configTestSuite struct {
	suite.Suite
	assert *assert.Assertions
	s3     *S3Storage
	opt    Options
}

func (s *configTestSuite) SetupTest() {
	// Silent logger
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}

	// Set S3Storage
	s.s3 = &S3Storage{}

	// Set Options
	s.opt = Options{
		BucketName:         "testBucketName",
		KeyID:              "testKeyId",
		SecretKey:          "testSecretKey",
		Region:             "testRegion",
		Endpoint:           "testEndpoint",
		RestrictedCharsWin: true,
		PrefixPath:         "testPrefixPath",
	}

	// Create assertions
	s.assert = assert.New(s.T())
}

func (s *configTestSuite) TestEmptyBucketName() {
	// When
	s.opt.BucketName = ""

	// Then
	err := ParseAndValidateConfig(s.s3, s.opt)
	s.assert.ErrorIs(err, errConfigFieldEmpty)
}

func (s *configTestSuite) TestEmptyKeyID() {
	// When
	s.opt.KeyID = ""

	// Then
	err := ParseAndValidateConfig(s.s3, s.opt)
	s.assert.ErrorIs(err, errConfigFieldEmpty)
}

func (s *configTestSuite) TestEmptySecretKey() {
	// When
	s.opt.SecretKey = ""

	// Then
	err := ParseAndValidateConfig(s.s3, s.opt)
	s.assert.ErrorIs(err, errConfigFieldEmpty)
}

func (s *configTestSuite) TestConfigParse() {
	// When
	err := ParseAndValidateConfig(s.s3, s.opt)

	// Then
	s.assert.Nil(err)
	s.assert.Equal(s.opt.BucketName, s.s3.stConfig.authConfig.BucketName)
	s.assert.Equal(s.opt.KeyID, s.s3.stConfig.authConfig.KeyID)
	s.assert.Equal(s.opt.SecretKey, s.s3.stConfig.authConfig.SecretKey)
	s.assert.Equal(s.opt.Region, s.s3.stConfig.authConfig.Region)
	s.assert.Equal(s.opt.Endpoint, s.s3.stConfig.authConfig.Endpoint)
	s.assert.Equal(s.opt.RestrictedCharsWin, s.s3.stConfig.restrictedCharsWin)
	s.assert.Equal(s.opt.PrefixPath, s.s3.stConfig.prefixPath)
}

func (s *configTestSuite) TestPrefixPath() {
	// When
	s.opt.PrefixPath = "/testPrefixPath"

	// Then
	ParseAndValidateConfig(s.s3, s.opt)
	s.assert.Equal("testPrefixPath", s.s3.stConfig.prefixPath)
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(configTestSuite))
}
