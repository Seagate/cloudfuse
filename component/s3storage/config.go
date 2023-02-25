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
	"errors"
	"fmt"
	"reflect"
	"strings"

	"lyvecloudfuse/common/config"
	"lyvecloudfuse/common/log"

	"github.com/JeffreyRichter/enum/enum"
)

// AuthType Enum
type AuthType int

var EAuthType = AuthType(0).INVALID_AUTH()

func (AuthType) INVALID_AUTH() AuthType {
	return AuthType(0)
}

func (AuthType) KEY() AuthType {
	return AuthType(1)
}

func (AuthType) SAS() AuthType {
	return AuthType(2)
}

func (AuthType) SPN() AuthType {
	return AuthType(3)
}

func (AuthType) MSI() AuthType {
	return AuthType(4)
}

func (a AuthType) String() string {
	return enum.StringInt(a, reflect.TypeOf(a))
}

func (a *AuthType) Parse(s string) error {
	enumVal, err := enum.ParseInt(reflect.TypeOf(a), s, true, false)
	if enumVal != nil {
		*a = enumVal.(AuthType)
	}
	return err
}

// AccountType Enum
type AccountType int

var EAccountType = AccountType(0).INVALID_ACC()

func (AccountType) INVALID_ACC() AccountType {
	return AccountType(0)
}

func (AccountType) BLOCK() AccountType {
	return AccountType(1)
}

func (AccountType) ADLS() AccountType {
	return AccountType(2)
}

func (f AccountType) String() string {
	return enum.StringInt(f, reflect.TypeOf(f))
}

func (a *AccountType) Parse(s string) error {
	enumVal, err := enum.ParseInt(reflect.TypeOf(a), s, true, false)
	if enumVal != nil {
		*a = enumVal.(AccountType)
	}
	return err
}

// Environment variable names
// Here we are not reading MSI_ENDPOINT and MSI_SECRET as they are read by go-sdk directly
// https://github.com/Azure/go-autorest/blob/a46566dfcbdc41e736295f94e9f690ceaf50094a/autorest/adal/token.go#L788
// newServicePrincipalTokenFromMSI : reads them directly from env
const (
	EnvAzStorageAccount            = "AZURE_STORAGE_ACCOUNT"
	EnvAzStorageAccountType        = "AZURE_STORAGE_ACCOUNT_TYPE"
	EnvAzStorageAccessKey          = "AZURE_STORAGE_ACCESS_KEY"
	EnvAzStorageSasToken           = "AZURE_STORAGE_SAS_TOKEN"
	EnvAzStorageIdentityClientId   = "AZURE_STORAGE_IDENTITY_CLIENT_ID"
	EnvAzStorageIdentityResourceId = "AZURE_STORAGE_IDENTITY_RESOURCE_ID"
	EnvAzStorageIdentityObjectId   = "AZURE_STORAGE_IDENTITY_OBJECT_ID"
	EnvAzStorageSpnTenantId        = "AZURE_STORAGE_SPN_TENANT_ID"
	EnvAzStorageSpnClientId        = "AZURE_STORAGE_SPN_CLIENT_ID"
	EnvAzStorageSpnClientSecret    = "AZURE_STORAGE_SPN_CLIENT_SECRET"
	EnvAzStorageAadEndpoint        = "AZURE_STORAGE_AAD_ENDPOINT"
	EnvAzStorageAuthType           = "AZURE_STORAGE_AUTH_TYPE"
	EnvAzStorageBlobEndpoint       = "AZURE_STORAGE_BLOB_ENDPOINT"
	EnvHttpProxy                   = "http_proxy"
	EnvHttpsProxy                  = "https_proxy"
	EnvAzStorageAccountContainer   = "AZURE_STORAGE_ACCOUNT_CONTAINER"
)

type Options struct {
	BucketName string `config:"bucket-name" yaml:"bucket-name,omitempty"`
	AccessKey  string `config:"access-key" yaml:"access-key,omitempty"`
	SecretKey  string `config:"secret-key" yaml:"secret-key,omitempty"`
	Region     string `config:"region" yaml:"region,omitempty"`
	Endpoint   string `config:"endpoint" yaml:"endpoint,omitempty"`
	PrefixPath string `config:"subdirectory" yaml:"subdirectory,omitempty"`
}

// RegisterEnvVariables : Register environment varilables
func RegisterEnvVariables() {
	config.BindEnv("azstorage.account-name", EnvAzStorageAccount)
	config.BindEnv("azstorage.type", EnvAzStorageAccountType)

	config.BindEnv("azstorage.account-key", EnvAzStorageAccessKey)

	config.BindEnv("azstorage.sas", EnvAzStorageSasToken)

	config.BindEnv("azstorage.appid", EnvAzStorageIdentityClientId)
	config.BindEnv("azstorage.resid", EnvAzStorageIdentityResourceId)

	config.BindEnv("azstorage.tenantid", EnvAzStorageSpnTenantId)
	config.BindEnv("azstorage.clientid", EnvAzStorageSpnClientId)
	config.BindEnv("azstorage.clientsecret", EnvAzStorageSpnClientSecret)
	config.BindEnv("azstorage.objid", EnvAzStorageIdentityObjectId)

	config.BindEnv("azstorage.aadendpoint", EnvAzStorageAadEndpoint)

	config.BindEnv("azstorage.endpoint", EnvAzStorageBlobEndpoint)

	config.BindEnv("azstorage.mode", EnvAzStorageAuthType)

	config.BindEnv("azstorage.http-proxy", EnvHttpProxy)
	config.BindEnv("azstorage.https-proxy", EnvHttpsProxy)

	config.BindEnv("azstorage.container", EnvAzStorageAccountContainer)
}

//    ----------- Config Parsing and Validation  ---------------

// formatEndpointProtocol : add the protocol and missing "/" at the end to the endpoint
func formatEndpointProtocol(endpoint string, http bool) string {
	correctedEndpoint := endpoint

	// If the pvtEndpoint does not have protocol mentioned in front, pvtEndpoint parsing will fail while
	// creating URI also the string shall end with "/"
	if correctedEndpoint != "" {
		if !(strings.HasPrefix(correctedEndpoint, "https://") ||
			strings.HasPrefix(correctedEndpoint, "http://")) {
			if http {
				correctedEndpoint = "http://" + correctedEndpoint
			} else {
				correctedEndpoint = "https://" + correctedEndpoint
			}
		}

		if correctedEndpoint[len(correctedEndpoint)-1] != '/' {
			correctedEndpoint = correctedEndpoint + "/"
		}
	}

	return correctedEndpoint
}

// ParseAndValidateConfig : Parse and validate config
func ParseAndValidateConfig(s3 *S3Storage, opt Options) error {
	log.Trace("ParseAndValidateConfig : Parsing config")

	// Validate account name is present or not
	if opt.BucketName == "" {
		return errors.New("bucket name not provided")
	}
	s3.stConfig.authConfig.BucketName = opt.BucketName
	s3.stConfig.authConfig.AccessKey = opt.AccessKey
	s3.stConfig.authConfig.SecretKey = opt.SecretKey
	s3.stConfig.authConfig.Region = opt.Region

	// Validate container name is present or not
	// TODO: Need to fix for buckets
	// err := config.UnmarshalKey("mount-all-containers", &s3.stConfig.mountAllContainers)
	// if err != nil {
	// 	log.Err("ParseAndValidateConfig : Failed to detect mount-all-container")
	// }

	// Validate endpoint
	if opt.Endpoint == "" {
		log.Warn("ParseAndValidateConfig : account endpoint not provided, assuming the default .lyvecloud.seagate.com style endpoint")
		opt.Endpoint = fmt.Sprintf("s3.%s.lyvecloud.seagate.com", opt.Region)
	}
	s3.stConfig.authConfig.Endpoint = opt.Endpoint

	// If subdirectory is mounted, take the prefix path
	s3.stConfig.prefixPath = opt.PrefixPath

	// Block list call on mount for given amount of time
	// TODO: Add cancellation timeout
	// s3.stConfig.cancelListForSeconds = opt.CancelListForSeconds

	// TODO: Enable sdk trace in aws-sdk-go-v2
	// s3.stConfig.sdkTrace = opt.SdkTrace
	// log.Info("ParseAndValidateConfig : sdk logging from the config file: %t", s3.stConfig.sdkTrace)

	// err = ParseAndReadDynamicConfig(s3, opt, false)
	// if err != nil {
	// 	return err
	// }

	// Retry policy configuration
	// A user provided value of 0 doesn't make sense for MaxRetries, MaxTimeout, BackoffTime, or MaxRetryDelay.

	// s3.stConfig.maxRetries = 5     // Max number of retry to be done  (default 4) (v1 : 0)
	// s3.stConfig.maxTimeout = 900   // Max timeout for any single retry (default 1 min) (v1 : 60)
	// s3.stConfig.backoffTime = 4    // Delay before any retry (exponential increase) (default 4 sec)
	// s3.stConfig.maxRetryDelay = 60 // Maximum allowed delay before retry (default 120 sec) (v1 : 1.2)

	// TODO: Add this
	// if opt.MaxRetries != 0 {
	// 	s3.stConfig.maxRetries = opt.MaxRetries
	// }
	// if opt.MaxTimeout != 0 {
	// 	s3.stConfig.maxTimeout = opt.MaxTimeout
	// }
	// if opt.BackoffTime != 0 {
	// 	s3.stConfig.backoffTime = opt.BackoffTime
	// }
	// if opt.MaxRetryDelay != 0 {
	// 	s3.stConfig.maxRetryDelay = opt.MaxRetryDelay
	// }

	if config.IsSet(compName + ".set-content-type") {
		log.Warn("unsupported v1 CLI parameter: set-content-type is always true in lyvecloudfuse.")
	}
	if config.IsSet(compName + ".ca-cert-file") {
		log.Warn("unsupported v1 CLI parameter: ca-cert-file is not supported in lyvecloudfuse. Use the default ca cert path for your environment.")
	}
	if config.IsSet(compName + ".debug-libcurl") {
		log.Warn("unsupported v1 CLI parameter: debug-libcurl is not applicable in lyvecloudfuse.")
	}

	// log.Info("ParseAndValidateConfig : Bucket: %s, Container: %s, Prefix: %s, Endpoint: %s, ListBlock: %d, MD5 : %v %v, Virtual Directory: %v",
	// 	s3.stConfig.authConfig.BucketName, s3.stConfig.container,
	// 	s3.stConfig.prefixPath, s3.stConfig.authConfig.Endpoint, s3.stConfig.cancelListForSeconds, s3.stConfig.validateMD5, s3.stConfig.updateMD5, s3.stConfig.virtualDirectory)

	// log.Info("ParseAndValidateConfig : Retry Config: Retry count %d, Max Timeout %d, BackOff Time %d, Max Delay %d",
	// 	s3.stConfig.maxRetries, s3.stConfig.maxTimeout, s3.stConfig.backoffTime, s3.stConfig.maxRetryDelay)

	return nil
}

// ParseAndReadDynamicConfig : On config change read only the required config
// func ParseAndReadDynamicConfig(s3 *AzStorage, opt Options, reload bool) error {
// 	log.Trace("ParseAndReadDynamicConfig : Reparsing config")

// 	// If block size and max concurrency is configured use those
// 	// A user provided value of 0 doesn't make sense for BlockSize, or MaxConcurrency.
// 	if opt.BlockSize != 0 {
// 		s3.stConfig.blockSize = opt.BlockSize * 1024 * 1024
// 	}

// 	if opt.MaxConcurrency != 0 {
// 		s3.stConfig.maxConcurrency = opt.MaxConcurrency
// 	}

// 	// Populate default tier
// 	if opt.DefaultTier != "" {
// 		s3.stConfig.defaultTier = getAccessTierType(opt.DefaultTier)
// 	}

// 	s3.stConfig.ignoreAccessModifiers = !opt.FailUnsupportedOp
// 	s3.stConfig.validateMD5 = opt.ValidateMD5
// 	s3.stConfig.updateMD5 = opt.UpdateMD5

// 	s3.stConfig.virtualDirectory = opt.VirtualDirectory

// 	// Auth related reconfig
// 	switch opt.AuthMode {
// 	case "sas":
// 		s3.stConfig.authConfig.AuthMode = EAuthType.SAS()
// 		if opt.SaSKey == "" {
// 			return errors.New("SAS key not provided")
// 		}

// 		oldSas := s3.stConfig.authConfig.SASKey
// 		s3.stConfig.authConfig.SASKey = sanitizeSASKey(opt.SaSKey)

// 		if reload {
// 			log.Info("ParseAndReadDynamicConfig : SAS Key updated")

// 			if err := s3.storage.NewCredentialKey("saskey", s3.stConfig.authConfig.SASKey); err != nil {
// 				s3.stConfig.authConfig.SASKey = oldSas
// 				_ = s3.storage.NewCredentialKey("saskey", s3.stConfig.authConfig.SASKey)
// 				return errors.New("SAS key update failure")
// 			}
// 		}
// 	}

// 	return nil
// }
