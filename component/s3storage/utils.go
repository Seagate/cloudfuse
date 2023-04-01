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
	"path/filepath"
	"strings"

	"lyvecloudfuse/common/log"
	"lyvecloudfuse/internal"
)

// TODO: add AWS SDK customization options and helper functions here to write any relevant SDK-specific structures
// TODO: add AWS SDK logging function code here (like getLogOptions)
// TODO: define any SDK-specific error code enumerations here

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
