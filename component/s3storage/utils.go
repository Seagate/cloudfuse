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
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"

	"github.com/aws/smithy-go"
)

// TODO: add AWS SDK customization options and helper functions here to write any relevant SDK-specific structures
// TODO: add AWS SDK logging function code here (like getLogOptions)

var UserAgent = func() string {
	// TODO: if we can get the Go version for this, it would be nice.
	return "Seagate-Cloudfuse/" + common.CloudfuseVersion + " (Language=Go)"
}

const (
	DefaultPartSize     = 8 * common.MbToBytes
	DefaultUploadCutoff = 100 * common.MbToBytes
	DefaultConcurrency  = 5
	MaxPartSizeMb       = 5 * 1024
)

// ----------- Cloud Storage error code handling ---------------

// This takes an err from an S3 API call, parses the error,
// prints a helpful error message, and returns the corresponding system error code.
// attemptedAction describes the action that failed with this error.
// Any context that would help in debug should be included in the attemptedAction string.
// This function uses the runtime library to look up the name of the function calling it,
// so there's no need to include that in the attemptedAction.
func parseS3Err(err error, attemptedAction string) error {
	// guide: https://aws.github.io/aws-sdk-go-v2/docs/handling-errors/
	// reference: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3@v1.30.2/types
	// discussion: https://github.com/aws/aws-sdk-go-v2/issues/1110#issuecomment-1054643716

	// trivial case
	if err == nil {
		return nil
	}

	// get the name of the function that called this
	functionName := ""
	pc, _, _, ok := runtime.Caller(1)
	if ok {
		longFuncName := runtime.FuncForPC(pc).Name()
		// the function name returned is long, e.g. "github.com/Seagate/cloudfuse/component/s3storage.(*Client).getObject"
		// split the long function name using the component name
		funcNameParts := strings.Split(longFuncName, compName)
		if len(funcNameParts) > 1 {
			// trim the leading dot, so we get something like "(*Client).getObject"
			functionName = strings.Trim(funcNameParts[1], ".")
		}
	}

	// Every error we've handled thus far follows this structure:
	// *smithy.OperationError
	// | .Operation() returns the API that was called (e.g. "GetObject")
	// | .Unwrap() returns the next error in the tree:
	// └-*s3shared.ResponseError - can be found using errors.As with type *awsHttp.ResponseError
	//   | .HTTPStatusCode() returns the status code (e.g. 404)
	//   | .Unwrap() returns the next error in the tree:
	//   └-*smithy.GenericAPIError | *types.<modeledErrorType> - can be found using errors.As with type *smithy.APIError
	//       .ErrorCode() returns a string identifying the error (e.g. "NoSuchKey", "NotFound", etc.)

	// Any error that comes in should have an APIError somewhere in its tree
	// Find the API error in the error's tree
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		// There is an error modeling system, which allows us to match errors by type:
		//   e.g. *types.NoSuchKey, where types is "github.com/aws/aws-sdk-go-v2/service/s3/types"
		// but sometimes the same errorCode (e.g. "NoSuchKey") will come up but be wrapped in a smithy.GenericAPIError,
		// in which case the errorCode will match (e.g. "NoSuchKey"), but the type will not (*type.NoSuchKey).
		// In testing LC, as of 4/6/2023, this happens with the 404 error response to CopyObject.
		// So to reduce unhelpful complexity, we're just going to match the errorCode.
		errorCode := apiErr.ErrorCode()
		// Invalid range
		if errorCode == "InvalidRange" {
			// the range string sent with getObject is invalid
			// InvalidRange is an un-modeled service error response (it is a *smithy.GenericAPIError)
			log.Err(
				"%s : Failed to %s with error %s because range is invalid",
				functionName,
				attemptedAction,
				errorCode,
			)
			// TODO: identify cases where this may come up in deployment, and identify which syscall.errno() is most appropriate
			// this should not come up in normal operation, so for now we just return it without translating to a system error code
			return err
		}
		if errorCode == "NotFound" || errorCode == "NoSuchKey" {
			// HeadObject's 404 is not modeled (it is a *smithy.GenericAPIError)
			// GetObject's 404 is modeled (it is a *types.NoSuchKey)
			// CopyObject's 404 is not modeled (it is a *smithy.GenericAPIError)
			message := fmt.Sprintf(
				"%s : Failed to %s with error %s because key does not exist",
				functionName,
				attemptedAction,
				errorCode,
			)
			if strings.HasPrefix(attemptedAction, "HeadObject") {
				log.Warn(message)
			} else {
				log.Err(message)
			}
			return syscall.ENOENT
		}
	}

	// unrecognized error - parsing failed
	// print and return the original error
	log.Err("%s : Failed to %s. Here's why: %v", functionName, attemptedAction, err)
	return err
}

// TODO: handle AWS S3 storage tiers here
// TODO: write utils_test.go with unit tests

//    ----------- Content-type handling  ---------------

// ContentTypeMap : Store file extension to content-type mapping
var ContentTypes = map[string]string{
	".css":  "text/css",
	".pdf":  "application/pdf",
	".xml":  "text/xml",
	".csv":  "text/csv",
	".json": "application/json",
	".rtf":  "application/rtf",
	".txt":  "text/plain",
	".java": "text/plain",
	".dat":  "text/plain",

	".htm":  "text/html",
	".html": "text/html",

	".gif":  "image/gif",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".png":  "image/png",
	".bmp":  "image/bmp",

	".js":   "application/javascript",
	".mjs":  "application/javascript",
	".svg":  "image/svg+xml",
	".wasm": "application/wasm",
	".webp": "image/webp",

	".wav":  "audio/wav",
	".mp3":  "audio/mpeg",
	".mpeg": "video/mpeg",
	".aac":  "audio/aac",
	".avi":  "video/x-msvideo",
	".m3u8": "application/x-mpegURL",
	".ts":   "video/MP2T",
	".mid":  "audio/midiaudio/x-midi",
	".3gp":  "video/3gpp",
	".mp4":  "video/mp4",

	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".ppt":  "application/vnd.ms-powerpoint",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",

	".gz":   "application/x-gzip",
	".jar":  "application/java-archive",
	".rar":  "application/vnd.rar",
	".tar":  "application/x-tar",
	".zip":  "application/x-zip-compressed",
	".7z":   "application/x-7z-compressed",
	".3g2":  "video/3gpp2",
	".usdz": "application/zip",

	".sh":  "application/x-sh",
	".exe": "application/x-msdownload",
	".dll": "application/x-msdownload",
}

// getContentType : Based on the file extension retrieve the content type to be set
func getContentType(key string) string {
	value, found := ContentTypes[strings.ToLower(filepath.Ext(key))]
	if found {
		return value
	}
	return "application/octet-stream"
}

func populateContentType(newSet string) error { //nolint
	var data map[string]string
	if err := json.Unmarshal([]byte(newSet), &data); err != nil {
		log.Err("Failed to parse config file : %s [%s]", newSet, err.Error())
		return err
	}

	// We can simply append the new data to end of the map
	// however there may be conflicting keys and hence we need to merge manually
	//ContentTypeMap = append(ContentTypeMap, data)
	for k, v := range data {
		ContentTypes[k] = v
	}
	return nil
}

// TODO: implement ACL permissions and file mode conversions here

// Strips the prefixPath from the path and returns the joined string
func split(prefixPath string, path string) string {
	if prefixPath == "" {
		return path
	}

	// remove prefix's trailing slash too
	prefixPath = internal.ExtendDirName(prefixPath)
	if strings.HasPrefix(path, prefixPath) {
		return strings.Replace(path, prefixPath, "", 1)
	}

	// prefix not found - return the path unaltered
	return path
}

func removeLeadingSlashes(s string) string {
	for strings.HasPrefix(s, "/") {
		s = strings.TrimLeft(s, "/")
	}
	return s
}
