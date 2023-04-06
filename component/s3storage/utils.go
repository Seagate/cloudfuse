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
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"lyvecloudfuse/common/log"
	"lyvecloudfuse/internal"

	awsHttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// TODO: add AWS SDK customization options and helper functions here to write any relevant SDK-specific structures
// TODO: add AWS SDK logging function code here (like getLogOptions)

// ----------- Cloud Storage error code handling ---------------

// This takes an err from an S3 API call, parses the error,
// prints a helpful error message, and returns the corresponding system error code.
func parseS3Err(err error, attemptedAction string) error {
	// guide: https://aws.github.io/aws-sdk-go-v2/docs/handling-errors/
	// reference: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3@v1.30.2/types
	// discussion: https://github.com/aws/aws-sdk-go-v2/issues/1110#issuecomment-1054643716

	// trivial case
	if err == nil {
		noErr := syscall.Errno(0)
		return noErr
	}

	// get the name of the function that called this
	functionName := ""
	pc, _, _, ok := runtime.Caller(1)
	if ok {
		longFuncName := runtime.FuncForPC(pc).Name()
		// the function name returned is long, e.g. "lyvecloudfuse/component/s3storage.(*Client).getObject"
		// split the long function name using the component name
		funcNameParts := strings.Split(longFuncName, compName)
		if len(funcNameParts) > 1 {
			// trim the leading dot, so we get something like "(*Client).getObject"
			functionName = strings.Trim(funcNameParts[1], ".")
		}
	}

	// research print
	fmt.Printf("(Start) Error in fn %s, failed to %s. Err is of type %T.\n", functionName, attemptedAction, err)

	// at the top layer, we have a smithy.OperationError
	// if we unwrap that, we have a awsHttp.ResponseError
	// or
	// we have a smithy.APIError

	// create a list of errors, initially populated with our original error
	errorList := []error{err}

	for i := 0; i < len(errorList); i++ {
		// Ok, let's try to shove the error in all three boxes that it might fit into...
		thisErr := errorList[i]
		fmt.Printf("Index %d: Err is of type %T.\n", i, thisErr)

		// Handle errors of type smithy.OperationError (unwrap these to get the APIError underneath)
		var opErr *smithy.OperationError
		if errors.As(thisErr, &opErr) {
			operation := opErr.Operation()
			unwrappedError := opErr.Unwrap()
			fmt.Printf("Found smithy.OperationError in Err's tree with op %s. Unwrap() returned an %T with contents: %v.\n", operation, unwrappedError, unwrappedError)
			log.Err("failed to call %s with error: %v", operation, unwrappedError)
			fmt.Printf("Adding unwrapped error to errorList at index %d.\n", len(errorList))
			errorList = append(errorList, unwrappedError)
		}

		// Handle errors of type awsHttp.ResponseError
		var httpResponseErr *awsHttp.ResponseError
		if errors.As(thisErr, &httpResponseErr) {
			statusCode := httpResponseErr.HTTPStatusCode()
			unwrappedError := httpResponseErr.Unwrap()
			fmt.Printf("Found awsHttp.ResponseError in Err's tree with status code %d. Unwrap() returned an %T with contents: %v.\n", statusCode, unwrappedError, unwrappedError)
			fmt.Printf("Adding unwrapped error to errorList at index %d.\n", len(errorList))
			errorList = append(errorList, unwrappedError)
		}

		// Handle errors of type smithy.APIError
		var apiErr smithy.APIError
		if errors.As(thisErr, &apiErr) {
			code := apiErr.ErrorCode()
			fmt.Printf("Found smithy.APIError in Err's tree with error code %s. APIErr is of type %T\n", code, apiErr)

			// handle modeled service error responses (those that have dedicated types)
			switch apiErr.(type) {
			case *types.NotFound:
				// Not Found - 404
				fmt.Println("apiErr matched model types.NotFound")
				log.Err("%s : Failed to %s with error %s because key does not exist", functionName, attemptedAction, code)
			case *types.NoSuchKey:
				// No Such Key
				fmt.Println("apiErr matched model types.NoSuchKey")
				log.Err("%s : Failed to %s with error %s because key does not exist", functionName, attemptedAction, code)
			default:
				// the error is either un-modeled, or it was modeled but the model was not one of the cases above
				fmt.Println("apiErr did not match any modeled cases")
				// Invalid range
				if code == "InvalidRange" {
					// InvalidRange is an un-modeled service error response (it does not have a dedicated type)
					fmt.Println("apiErr matched errorCode InvalidRange")
					log.Err("%s : Failed to %s with error %s because range is invalid", functionName, attemptedAction, code)
				}
				if code == "NotFound" {
					// HeadObject's 404 is not modeled (it is a smithy.GenericAPIError)
					fmt.Println("apiErr matched errorCode NotFound")
					log.Err("%s : Failed to %s with error %s because key does not exist", functionName, attemptedAction, code)
				}
				if code == "NoSuchKey" {
					// CopyObject's 404 is not modeled (it is a smithy.GenericAPIError)
					fmt.Println("apiErr matched errorCode NoSuchKey")
					log.Err("%s : Failed to %s with error %s because key does not exist", functionName, attemptedAction, code)
				}
			}
		}

		// what about NSK?
		// No such key (object not in bucket)
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			fmt.Println("Found types.NoSuchKey in Err's tree.")
			log.Err("%s : Failed to %s because key does not exist", functionName, attemptedAction)
		}

		fmt.Printf("End index %d: (Err of type %T).\n", i, thisErr)
	}

	// unrecognized error - parsing failed
	// print and return the original error
	log.Err("%s : Failed to %s. Here's why: %v", functionName, attemptedAction, err)
	fmt.Printf("(End) Error in fn %s, failed to %s. Err is of type %T.\n", functionName, attemptedAction, err)
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

	".gz":  "application/x-gzip",
	".jar": "application/java-archive",
	".rar": "application/vnd.rar",
	".tar": "application/x-tar",
	".zip": "application/x-zip-compressed",
	".7z":  "application/x-7z-compressed",
	".3g2": "video/3gpp2",

	".sh":  "application/x-sh",
	".exe": "application/x-msdownload",
	".dll": "application/x-msdownload",
}

// getContentType : Based on the file extension retrieve the content type to be set
func getContentType(key string) string {
	value, found := ContentTypes[filepath.Ext(key)]
	if found {
		return value
	}
	return "application/octet-stream"
}

func populateContentType(newSet string) error {
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
