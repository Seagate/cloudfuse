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
	"errors"

	"lyvecloudfuse/common"
	"lyvecloudfuse/common/log"
)

type Options struct {
	BucketName                string `config:"bucket-name" yaml:"bucket-name,omitempty"`
	KeyID                     string `config:"key-id" yaml:"key-id,omitempty"`
	SecretKey                 string `config:"secret-key" yaml:"secret-key,omitempty"`
	Region                    string `config:"region" yaml:"region,omitempty"`
	Endpoint                  string `config:"endpoint" yaml:"endpoint,omitempty"`
	PrefixPath                string `config:"subdirectory" yaml:"subdirectory,omitempty"`
	RestrictedCharsWin        bool   `config:"restricted-characters-windows" yaml:"-"`
	PartSizeMb                int64  `config:"part-size-mb" yaml:"part-size-mb,omitempty"`
	UploadCutoffMb            int64  `config:"upload-cutoff-mb" yaml:"upload-cutoff-mb,omitempty"`
	Concurrency               int    `config:"concurrency" yaml:"concurrency,omitempty"`
	DisableConcurrentDownload bool   `config:"disable-concurrent-download" yaml:"disable-concurrent-download,omitempty"`
}

// ParseAndValidateConfig : Parse and validate config
func ParseAndValidateConfig(s3 *S3Storage, opt Options) error {
	log.Trace("ParseAndValidateConfig : Parsing config")

	// Validate account name is present or not
	if opt.BucketName == "" {
		return errors.New("bucket name not provided")
	}
	s3.stConfig.authConfig.BucketName = opt.BucketName
	s3.stConfig.authConfig.KeyID = opt.KeyID
	s3.stConfig.authConfig.SecretKey = opt.SecretKey
	s3.stConfig.authConfig.Region = opt.Region
	s3.stConfig.authConfig.Endpoint = opt.Endpoint

	s3.stConfig.restrictedCharsWin = opt.RestrictedCharsWin
	s3.stConfig.disableConcurrentDownload = opt.DisableConcurrentDownload

	// Part size must be at least 5 MB and smaller than 5GB. Otherwise, set to default.
	if opt.PartSizeMb < 5 || opt.PartSizeMb > MaxPartSizeMb {
		log.Warn("ParseAndValidateConfig : Part size must be between 5MB and 5GB. Default to default size")
		s3.stConfig.partSize = DefaultPartSize
	} else {
		s3.stConfig.partSize = opt.PartSizeMb * common.MbToBytes
	}

	// Cutoff size must not be less than 5 MB. Otherwise, set to default.
	if opt.UploadCutoffMb < 5 {
		s3.stConfig.uploadCutoff = DefaultUploadCutoff
	} else {
		s3.stConfig.uploadCutoff = opt.UploadCutoffMb * common.MbToBytes
	}

	if opt.Concurrency != 0 {
		s3.stConfig.concurrency = opt.Concurrency
	} else {
		s3.stConfig.concurrency = DefaultConcurrency
	}

	// If subdirectory is mounted, take the prefix path
	s3.stConfig.prefixPath = removeLeadingSlashes(opt.PrefixPath)
	// TODO: add more config options to customize AWS SDK behavior and import them here

	return nil
}

// ParseAndReadDynamicConfig : On config change read only the required config
func ParseAndReadDynamicConfig(s3 *S3Storage, opt Options, reload bool) error {
	log.Trace("ParseAndReadDynamicConfig : Reparsing config")

	// Part size must be at least 5 MB. Otherwise, set to default of 8 MB.
	if opt.PartSizeMb < 5 {
		s3.stConfig.partSize = DefaultPartSize
	} else {
		s3.stConfig.partSize = opt.PartSizeMb * common.MbToBytes
	}

	// Cutoff size must not be less than 5 MB. Otherwise, set to default of 200 MB.
	if opt.UploadCutoffMb < 5 {
		s3.stConfig.uploadCutoff = DefaultUploadCutoff
	} else {
		s3.stConfig.uploadCutoff = opt.UploadCutoffMb * common.MbToBytes
	}
	s3.stConfig.concurrency = opt.Concurrency

	return nil
}

// TODO: write config_test.go with unit tests
