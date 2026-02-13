/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2026 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.

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
	"net/http"
	"path"
	"strconv"
	"syscall"
	"testing"

	awsHttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyHttp "github.com/aws/smithy-go/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type utilsTestSuite struct {
	suite.Suite
}

func (s *utilsTestSuite) TestParseS3errGetObjectNoSuchKey() {
	assert := assert.New(s.T())

	errMessage := "No Such Key"
	getObjectS3Err := generateS3Error("GetObject", 404, &types.NoSuchKey{
		Message: &errMessage,
	})
	err := parseS3Err(getObjectS3Err, "test")
	assert.Equal(syscall.ENOENT, err)
}

func (s *utilsTestSuite) TestParseS3errHeadObjectNotFound() {
	assert := assert.New(s.T())

	errMessage := "Not Found"
	apiErrCode := "NotFound"
	getObjectS3Err := generateS3Error("HeadObject", 404, &smithy.GenericAPIError{
		Message: errMessage,
		Code:    apiErrCode,
		Fault:   smithy.FaultClient,
	})
	err := parseS3Err(getObjectS3Err, "test")
	assert.Equal(syscall.ENOENT, err)
}

func (s *utilsTestSuite) TestParseS3errCopyObjectNoSuchKey() {
	assert := assert.New(s.T())

	errMessage := "No Such Key"
	apiErrCode := "NoSuchKey"
	getObjectS3Err := generateS3Error("CopyObject", 404, &smithy.GenericAPIError{
		Message: errMessage,
		Code:    apiErrCode,
		Fault:   smithy.FaultClient,
	})
	err := parseS3Err(getObjectS3Err, "test")
	assert.Equal(syscall.ENOENT, err)
}

func (s *utilsTestSuite) TestParseS3errGetObjectInvalidRange() {
	assert := assert.New(s.T())

	errMessage := "Invalid Range"
	apiErrCode := "InvalidRange"
	getObjectS3Err := generateS3Error("GetObject", 416, &smithy.GenericAPIError{
		Message: errMessage,
		Code:    apiErrCode,
		Fault:   smithy.FaultClient,
	})
	err := parseS3Err(getObjectS3Err, "test")
	// for an error like this, there is no system error
	// so we expect to get the original error back
	assert.Equal(err, getObjectS3Err)
}

func generateS3Error(operation string, httpStatusCode int, apiErr error) *smithy.OperationError {
	return &smithy.OperationError{
		ServiceID:     "S3",
		OperationName: operation,
		Err: &awsHttp.ResponseError{
			RequestID: "",
			ResponseError: &smithyHttp.ResponseError{
				Response: &smithyHttp.Response{
					Response: &http.Response{
						Status:     strconv.Itoa(httpStatusCode),
						StatusCode: httpStatusCode,
					},
				},
				Err: apiErr,
			},
		},
	}
}

func (s *utilsTestSuite) TestContentType() {
	assert := assert.New(s.T())

	val := getContentType("a.tst")
	assert.Equal("application/octet-stream", val, "Content-type mismatch")

	newSet := `{
		".tst": "application/test",
		".dum": "dummy/test"
		}`
	err := populateContentType(newSet)
	assert.NoError(err, "Failed to populate new config")

	val = getContentType("a.tst")
	assert.Equal("application/test", val, "Content-type mismatch")

	// assert mp4 content type would get deserialized correctly
	val = getContentType("file.mp4")
	assert.Equal("video/mp4", val)
}

type contentTypeVal struct {
	val    string
	result string
}

func (s *utilsTestSuite) TestPrefixPathRemoval() {
	assert := assert.New(s.T())

	type PrefixPath struct {
		prefix string
		path   string
		result string
	}

	var inputs = []PrefixPath{
		{prefix: "", path: "abc.txt", result: "abc.txt"},
		{prefix: "", path: "ABC", result: "ABC"},
		{prefix: "", path: "ABC/DEF.txt", result: "ABC/DEF.txt"},
		{prefix: "", path: "ABC/DEF/1.txt", result: "ABC/DEF/1.txt"},

		{prefix: "ABC", path: "ABC/DEF/1.txt", result: "DEF/1.txt"},
		{prefix: "ABC/", path: "ABC/DEF/1.txt", result: "DEF/1.txt"},
		{prefix: "ABC", path: "ABC/DEF", result: "DEF"},
		{prefix: "ABC/", path: "ABC/DEF", result: "DEF"},
		{prefix: "ABC/", path: "ABC/DEF/G/H/1.txt", result: "DEF/G/H/1.txt"},

		{prefix: "ABC/DEF", path: "ABC/DEF/1.txt", result: "1.txt"},
		{prefix: "ABC/DEF/", path: "ABC/DEF/1.txt", result: "1.txt"},
		{prefix: "ABC/DEF", path: "ABC/DEF/A/B/c.txt", result: "A/B/c.txt"},
		{prefix: "ABC/DEF/", path: "ABC/DEF/A/B/c.txt", result: "A/B/c.txt"},

		{prefix: "A/B/C/D/E", path: "A/B/C/D/E/F/G/H/I/j.txt", result: "F/G/H/I/j.txt"},
		{prefix: "A/B/C/D/E/", path: "A/B/C/D/E/F/G/H/I/j.txt", result: "F/G/H/I/j.txt"},
	}

	for _, i := range inputs {
		s.Run(path.Join(i.prefix, i.path), func() {
			output := split(i.prefix, i.path)
			assert.Equal(i.result, output)
		})
	}

}

func (s *utilsTestSuite) TestGetContentType() {
	assert := assert.New(s.T())
	var inputs = []contentTypeVal{
		{val: "a.css", result: "text/css"},
		{val: "a.pdf", result: "application/pdf"},
		{val: "a.xml", result: "text/xml"},
		{val: "a.csv", result: "text/csv"},
		{val: "a.json", result: "application/json"},
		{val: "a.rtf", result: "application/rtf"},
		{val: "a.txt", result: "text/plain"},
		{val: "a.java", result: "text/plain"},
		{val: "a.dat", result: "text/plain"},
		{val: "a.htm", result: "text/html"},
		{val: "a.html", result: "text/html"},
		{val: "a.gif", result: "image/gif"},
		{val: "a.jpeg", result: "image/jpeg"},
		{val: "a.jpg", result: "image/jpeg"},
		{val: "a.png", result: "image/png"},
		{val: "a.bmp", result: "image/bmp"},
		{val: "a.js", result: "application/javascript"},
		{val: "a.mjs", result: "application/javascript"},
		{val: "a.svg", result: "image/svg+xml"},
		{val: "a.wasm", result: "application/wasm"},
		{val: "a.webp", result: "image/webp"},
		{val: "a.wav", result: "audio/wav"},
		{val: "a.mp3", result: "audio/mpeg"},
		{val: "a.mpeg", result: "video/mpeg"},
		{val: "a.aac", result: "audio/aac"},
		{val: "a.avi", result: "video/x-msvideo"},
		{val: "a.m3u8", result: "application/x-mpegURL"},
		{val: "a.ts", result: "video/MP2T"},
		{val: "a.mid", result: "audio/midiaudio/x-midi"},
		{val: "a.3gp", result: "video/3gpp"},
		{val: "a.mp4", result: "video/mp4"},
		{val: "a.doc", result: "application/msword"},
		{
			val:    "a.docx",
			result: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		},
		{val: "a.ppt", result: "application/vnd.ms-powerpoint"},
		{
			val:    "a.pptx",
			result: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		},
		{val: "a.xls", result: "application/vnd.ms-excel"},
		{
			val:    "a.xlsx",
			result: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		},
		{val: "a.gz", result: "application/x-gzip"},
		{val: "a.jar", result: "application/java-archive"},
		{val: "a.rar", result: "application/vnd.rar"},
		{val: "a.tar", result: "application/x-tar"},
		{val: "a.zip", result: "application/x-zip-compressed"},
		{val: "a.7z", result: "application/x-7z-compressed"},
		{val: "a.3g2", result: "video/3gpp2"},
		{val: "a.sh", result: "application/x-sh"},
		{val: "a.exe", result: "application/x-msdownload"},
		{val: "a.dll", result: "application/x-msdownload"},
		{val: "a.cSS", result: "text/css"},
		{val: "a.Mp4", result: "video/mp4"},
		{val: "a.JPG", result: "image/jpeg"},
		{val: "a.usdz", result: "application/zip"},
	}
	for _, i := range inputs {
		s.Run(i.val, func() {
			output := getContentType(i.val)
			assert.Equal(i.result, output)
		})
	}
}

func (s *utilsTestSuite) TestRemoveLeadingSlashes() {
	assert := assert.New(s.T())
	var inputs = []struct {
		subdirectory string
		result       string
	}{
		{subdirectory: "/abc/def", result: "abc/def"},
		{subdirectory: "////abc/def/", result: "abc/def/"},
		{subdirectory: "abc/def/", result: "abc/def/"},
		{subdirectory: "", result: ""},
	}

	for _, i := range inputs {
		assert.Equal(i.result, removeLeadingSlashes(i.subdirectory))
	}
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(utilsTestSuite))
}
