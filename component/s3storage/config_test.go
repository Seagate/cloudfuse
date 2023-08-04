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
	"lyvecloudfuse/common/config"
	"lyvecloudfuse/common/log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type configS3TestSuite struct {
	suite.Suite
}

func (s *configS3TestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
}

func (s *configS3TestSuite) TestEmptyBucketName() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	az := &S3Storage{}
	opt := Options{}

	err := ParseAndValidateConfig(az, opt)
	assert.NotNil(err)
	assert.Contains(err.Error(), "bucket name not provided")
}

func (s *configS3TestSuite) TestPartSize() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	s3 := &S3Storage{}
	opt := Options{}
	opt.BucketName = "Test"
	opt.PartSizeMb = 10

	err := ParseAndValidateConfig(s3, opt)
	assert.Nil(err)
	assert.EqualValues(opt.PartSizeMb*common.MbToBytes, s3.stConfig.partSize)

	opt.PartSizeMb = MaxPartSizeMb + 1
	err = ParseAndValidateConfig(s3, opt)
	assert.Nil(err)
	assert.EqualValues(DefaultPartSize, s3.stConfig.partSize)

	opt.PartSizeMb = 4
	err = ParseAndValidateConfig(s3, opt)
	assert.Nil(err)
	assert.EqualValues(DefaultPartSize, s3.stConfig.partSize)
}

func (s *configS3TestSuite) TestUploadCutoff() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	s3 := &S3Storage{}
	opt := Options{}
	opt.BucketName = "Test"
	opt.UploadCutoffMb = 10

	err := ParseAndValidateConfig(s3, opt)
	assert.Nil(err)
	assert.EqualValues(opt.UploadCutoffMb*common.MbToBytes, s3.stConfig.uploadCutoff)

	opt.UploadCutoffMb = 4
	err = ParseAndValidateConfig(s3, opt)
	assert.Nil(err)
	assert.EqualValues(DefaultUploadCutoff, s3.stConfig.uploadCutoff)
}

func (s *configS3TestSuite) TestConcurrency() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	s3 := &S3Storage{}
	opt := Options{}
	opt.BucketName = "Test"
	opt.Concurrency = 5

	err := ParseAndValidateConfig(s3, opt)
	assert.Nil(err)
	assert.EqualValues(opt.Concurrency, s3.stConfig.concurrency)

	opt.Concurrency = 0
	err = ParseAndValidateConfig(s3, opt)
	assert.Nil(err)
	assert.EqualValues(DefaultConcurrency, s3.stConfig.concurrency)
}

func TestConfigS3TestSuite(t *testing.T) {
	suite.Run(t, new(configS3TestSuite))
}
