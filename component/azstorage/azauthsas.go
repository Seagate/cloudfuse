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

package azstorage

import (
	"errors"
	"strings"

	"github.com/Seagate/cloudfuse/common/log"
	"github.com/awnumar/memguard"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	serviceBfs "github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/service"
)

// Verify that the Auth implement the correct AzAuth interfaces
var _ azAuth = &azAuthBlobSAS{}
var _ azAuth = &azAuthDatalakeSAS{}

type azAuthSAS struct {
	azAuthBase
}

// SetOption : Sets the sas key information for the SAS auth.
func (azsas *azAuthSAS) setOption(key, value string) {
	if key == "saskey" {
		azsas.config.SASKey = memguard.NewEnclave([]byte(value))
	}
}

// GetEndpoint : Gets the SAS endpoint
func (azsas *azAuthSAS) getEndpoint() string {
	if azsas.config.SASKey != nil {
		buff, err := azsas.config.SASKey.Open()
		if err != nil || buff == nil {
			return ""
		}
		defer buff.Destroy()
		endpoint := azsas.config.Endpoint + "?" + strings.TrimLeft(buff.String(), "?")
		return endpoint
	}

	return ""
}

type azAuthBlobSAS struct {
	azAuthSAS
}

// getServiceClient : returns SAS based service client for blob
func (azsas *azAuthBlobSAS) getServiceClient(stConfig *AzStorageConfig) (interface{}, error) {
	if azsas.config.SASKey == nil {
		log.Err(
			"azAuthBlobSAS::getServiceClient : SAS key for account is empty, cannot authenticate user",
		)
		return nil, errors.New("sas key for account is empty, cannot authenticate user")
	}

	opts, err := getAzBlobServiceClientOptions(stConfig)
	if err != nil {
		log.Err(
			"azAuthBlobSAS::getServiceClient : Failed to create client options [%s]",
			err.Error(),
		)
		return nil, err
	}

	svcClient, err := service.NewClientWithNoCredential(azsas.getEndpoint(), opts)
	if err != nil {
		log.Err(
			"azAuthBlobSAS::getServiceClient : Failed to create service client [%s]",
			err.Error(),
		)
	}

	return svcClient, err
}

type azAuthDatalakeSAS struct {
	azAuthSAS
}

// getServiceClient : returns SAS based service client for datalake
func (azsas *azAuthDatalakeSAS) getServiceClient(stConfig *AzStorageConfig) (interface{}, error) {
	if azsas.config.SASKey == nil {
		log.Err(
			"azAuthDatalakeSAS::getServiceClient : SAS key for account is empty, cannot authenticate user",
		)
		return nil, errors.New("sas key for account is empty, cannot authenticate user")
	}

	opts, err := getAzDatalakeServiceClientOptions(stConfig)
	if err != nil {
		log.Err(
			"azAuthDatalakeSAS::getServiceClient : Failed to create client options [%s]",
			err.Error(),
		)
		return nil, err
	}

	svcClient, err := serviceBfs.NewClientWithNoCredential(azsas.getEndpoint(), opts)
	if err != nil {
		log.Err(
			"azAuthDatalakeSAS::getServiceClient : Failed to create service client [%s]",
			err.Error(),
		)
	}

	return svcClient, err
}
