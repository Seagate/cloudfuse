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
	"github.com/Seagate/cloudfuse/common/log"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	serviceBfs "github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/service"
)

// Verify that the Auth implement the correct AzAuth interfaces
var _ azAuth = &azAuthBlobCLI{}
var _ azAuth = &azAuthDatalakeCLI{}

type azAuthCLI struct {
	azAuthBase
}

func (azcli *azAuthCLI) getTokenCredential() (azcore.TokenCredential, error) {
	cred, err := azidentity.NewAzureCLICredential(nil)
	return cred, err
}

type azAuthBlobCLI struct {
	azAuthCLI
}

// getServiceClient : returns service client for blob using azcli as authentication mode
func (azcli *azAuthBlobCLI) getServiceClient(stConfig *AzStorageConfig) (interface{}, error) {
	cred, err := azcli.getTokenCredential()
	if err != nil {
		log.Err(
			"azAuthBlobCLI::getServiceClient : Failed to get token credential from azcli [%s]",
			err.Error(),
		)
		return nil, err
	}

	opts, err := getAzBlobServiceClientOptions(stConfig)
	if err != nil {
		log.Err(
			"azAuthBlobCLI::getServiceClient : Failed to create client options [%s]",
			err.Error(),
		)
		return nil, err
	}

	svcClient, err := service.NewClient(azcli.config.Endpoint, cred, opts)
	if err != nil {
		log.Err(
			"azAuthBlobCLI::getServiceClient : Failed to create service client [%s]",
			err.Error(),
		)
	}

	return svcClient, err
}

type azAuthDatalakeCLI struct {
	azAuthCLI
}

// getServiceClient : returns service client for datalake using azcli as authentication mode
func (azcli *azAuthDatalakeCLI) getServiceClient(stConfig *AzStorageConfig) (interface{}, error) {
	cred, err := azcli.getTokenCredential()
	if err != nil {
		log.Err(
			"azAuthDatalakeCLI::getServiceClient : Failed to get token credential from azcli [%s]",
			err.Error(),
		)
		return nil, err
	}

	opts, err := getAzDatalakeServiceClientOptions(stConfig)
	if err != nil {
		log.Err(
			"azAuthDatalakeCLI::getServiceClient : Failed to create client options [%s]",
			err.Error(),
		)
		return nil, err
	}

	svcClient, err := serviceBfs.NewClient(azcli.config.Endpoint, cred, opts)
	if err != nil {
		log.Err(
			"azAuthDatalakeCLI::getServiceClient : Failed to create service client [%s]",
			err.Error(),
		)
	}

	return svcClient, err
}
