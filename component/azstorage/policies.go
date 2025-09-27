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
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Seagate/cloudfuse/common"
)

// cloudfuseTelemetryPolicy is a custom pipeline policy to prepend the cloudfuse user agent string to the one coming from SDK.
// This is added in the PerCallPolicies which executes after the SDK's default telemetry policy.
type cloudfuseTelemetryPolicy struct {
	telemetryValue string
}

// newCloudfuseTelemetryPolicy creates an object which prepends the cloudfuse user agent string to the User-Agent request header
func newCloudfuseTelemetryPolicy(telemetryValue string) policy.Policy {
	return &cloudfuseTelemetryPolicy{telemetryValue: telemetryValue}
}

func (p cloudfuseTelemetryPolicy) Do(req *policy.Request) (*http.Response, error) {
	userAgent := p.telemetryValue

	// prepend the cloudfuse user agent string
	if ua := req.Raw().Header.Get(common.UserAgentHeader); ua != "" {
		userAgent = fmt.Sprintf("%s %s", userAgent, ua)
	}
	req.Raw().Header.Set(common.UserAgentHeader, userAgent)
	return req.Next()
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------
// Policy to override the service version if requested by user
type serviceVersionPolicy struct {
	serviceApiVersion string
}

func newServiceVersionPolicy(version string) policy.Policy {
	return &serviceVersionPolicy{
		serviceApiVersion: version,
	}
}

func (r *serviceVersionPolicy) Do(req *policy.Request) (*http.Response, error) {
	req.Raw().Header["x-ms-version"] = []string{r.serviceApiVersion}
	return req.Next()
}
