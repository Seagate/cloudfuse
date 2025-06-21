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
	"errors"
	"fmt"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/awnumar/memguard"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var errInvalidConfigField = errors.New("config field is invalid")

type Options struct {
	BucketName                string                  `config:"bucket-name"                   yaml:"bucket-name,omitempty"`
	Region                    string                  `config:"region"                        yaml:"region,omitempty"`
	Profile                   string                  `config:"profile"                       yaml:"region,omitempty"`
	Endpoint                  string                  `config:"endpoint"                      yaml:"endpoint,omitempty"`
	PrefixPath                string                  `config:"subdirectory"                  yaml:"subdirectory,omitempty"`
	RestrictedCharsWin        bool                    `config:"restricted-characters-windows" yaml:"-"`
	PartSizeMb                int64                   `config:"part-size-mb"                  yaml:"part-size-mb,omitempty"`
	UploadCutoffMb            int64                   `config:"upload-cutoff-mb"              yaml:"upload-cutoff-mb,omitempty"`
	Concurrency               int                     `config:"concurrency"                   yaml:"concurrency,omitempty"`
	DisableConcurrentDownload bool                    `config:"disable-concurrent-download"   yaml:"disable-concurrent-download,omitempty"`
	EnableChecksum            bool                    `config:"enable-checksum"               yaml:"enable-checksum,omitempty"`
	ChecksumAlgorithm         types.ChecksumAlgorithm `config:"checksum-algorithm"            yaml:"checksum-algorithm,omitempty"`
	UsePathStyle              bool                    `config:"use-path-style"                yaml:"use-path-style,omitempty"`
	DisableUsage              bool                    `config:"disable-usage"                 yaml:"disable-usage,omitempty"`
	EnableDirMarker           bool                    `config:"enable-dir-marker"             yaml:"enable-dir-marker,omitempty"`
}

type ConfigSecrets struct {
	KeyID     *memguard.Enclave
	SecretKey *memguard.Enclave
}

// ParseAndValidateConfig : Parse and validate config
func ParseAndValidateConfig(s3 *S3Storage, opt Options, secrets ConfigSecrets) error {
	log.Trace("ParseAndValidateConfig : Parsing config")

	// Validate bucket name
	if opt.BucketName == "" {
		log.Warn("ParseAndValidateConfig : bucket name not provided")
	}

	// Set authentication config
	s3.stConfig.authConfig.BucketName = opt.BucketName
	s3.stConfig.authConfig.KeyID = secrets.KeyID
	s3.stConfig.authConfig.SecretKey = secrets.SecretKey
	s3.stConfig.authConfig.Region = opt.Region
	s3.stConfig.authConfig.Profile = opt.Profile
	s3.stConfig.authConfig.Endpoint = opt.Endpoint

	// Set restricted characters
	s3.stConfig.restrictedCharsWin = opt.RestrictedCharsWin
	s3.stConfig.disableConcurrentDownload = opt.DisableConcurrentDownload
	s3.stConfig.usePathStyle = opt.UsePathStyle
	s3.stConfig.disableUsage = opt.DisableUsage
	s3.stConfig.enableDirMarker = opt.EnableDirMarker

	// Part size must be at least 5 MB and smaller than 5GB. Otherwise, set to default.
	if opt.PartSizeMb < 5 || opt.PartSizeMb > MaxPartSizeMb {
		if opt.PartSizeMb != 0 {
			log.Warn(
				"ParseAndValidateConfig : Part size must be between 5MB and 5GB. Defaulting to %dMB.",
				DefaultPartSize/common.MbToBytes,
			)
		}
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

	if opt.Concurrency > 0 {
		s3.stConfig.concurrency = opt.Concurrency
	} else {
		s3.stConfig.concurrency = DefaultConcurrency
	}

	// Part size must be at least 5 MB. Otherwise, set to default of 8 MB.
	if opt.PartSizeMb < 5 {
		s3.stConfig.partSize = DefaultPartSize
	} else {
		s3.stConfig.partSize = opt.PartSizeMb * common.MbToBytes
	}

	// If subdirectory is mounted, take the prefix path
	s3.stConfig.prefixPath = removeLeadingSlashes(opt.PrefixPath)

	s3.stConfig.enableChecksum = opt.EnableChecksum
	if opt.EnableChecksum {
		// Use default CRC32 checksum if user does not provide algorithm
		if opt.ChecksumAlgorithm == "" {
			opt.ChecksumAlgorithm = types.ChecksumAlgorithmCrc32
		}

		if opt.ChecksumAlgorithm != types.ChecksumAlgorithmCrc32 &&
			opt.ChecksumAlgorithm != types.ChecksumAlgorithmCrc32c &&
			opt.ChecksumAlgorithm != types.ChecksumAlgorithmSha1 &&
			opt.ChecksumAlgorithm != types.ChecksumAlgorithmSha256 {
			return fmt.Errorf(
				"%w: checksum is not a valid checksum. valid values are CRC32, CRC32C, SHA1, SHA256",
				errInvalidConfigField,
			)
		}
		s3.stConfig.checksumAlgorithm = opt.ChecksumAlgorithm
	}

	// by default symlink will be disabled
	enableSymlinks := false
	// Borrow enable-symlinks flag from attribute cache
	if config.IsSet("attr_cache.enable-symlinks") {
		err := config.UnmarshalKey("attr_cache.enable-symlinks", &enableSymlinks)
		if err != nil {
			enableSymlinks = false
			log.Err("ParseAndReadDynamicConfig : Failed to unmarshal attr_cache.enable-symlinks")
		}
	}
	s3.stConfig.disableSymlink = !enableSymlinks

	// hardcoded health check interval (for now)
	s3.stConfig.healthCheckInterval = 10 * time.Second

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

	// by default symlink will be disabled
	enableSymlinks := false
	// Borrow enable-symlinks flag from attribute cache
	if config.IsSet("attr_cache.enable-symlinks") {
		err := config.UnmarshalKey("attr_cache.enable-symlinks", &enableSymlinks)
		if err != nil {
			enableSymlinks = false
			log.Err("ParseAndReadDynamicConfig : Failed to unmarshal attr_cache.enable-symlinks")
		}
	}
	s3.stConfig.disableSymlink = !enableSymlinks

	return nil
}

// TODO: write config_test.go with unit tests
// TODO: allow dynamic config changes to affect SDK behavior?
