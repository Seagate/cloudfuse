//go:build linux

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

package file_cache

import (
	"bytes"
	"cloudfuse/common/log"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var duPath []string = []string{"/usr/bin/du", "/usr/local/bin/du", "/usr/sbin/du", "/usr/local/sbin/du", "/sbin/du", "/bin/du"}
var selectedDuPath string = ""

// getUsage: The current cache usage in MB
func getUsage(path string) (float64, error) {
	log.Trace("cachePolicy::getCacheUsage : %s", path)

	var currSize float64
	var out bytes.Buffer

	if selectedDuPath == "" {
		selectedDuPath = "-"
		for _, dup := range duPath {
			_, err := os.Stat(dup)
			if err == nil {
				selectedDuPath = dup
				break
			}
		}
	}

	if selectedDuPath == "-" {
		log.Err("cachePolicy::getCacheUsage : error finding du in any configured path")
		return 0, fmt.Errorf("failed to find du")
	}

	// du - estimates file space usage
	// https://man7.org/linux/man-pages/man1/du.1.html
	// Note: We cannot just pass -BM as a parameter here since it will result in less accurate estimates of the size of the path
	// (i.e. du will round up to 1M if the path is smaller than 1M).
	cmd := exec.Command(selectedDuPath, "-sh", path)
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		log.Err("cachePolicy::getCacheUsage : error running du [%s]", err.Error())
		return 0, err
	}

	size := strings.Split(out.String(), "\t")[0]
	if size == "0" {
		return 0, fmt.Errorf("failed to parse du output")
	}

	// some OS's use "," instead of "." that will not work for float parsing - replace it
	size = strings.Replace(size, ",", ".", 1)
	parsed, err := strconv.ParseFloat(size[:len(size)-1], 64)
	if err != nil {
		log.Err("cachePolicy::getCacheUsage : error parsing folder size [%s]", err.Error())
		return 0, fmt.Errorf("failed to parse du output")
	}

	switch size[len(size)-1] {
	case 'K':
		currSize = parsed / float64(1024)
	case 'M':
		currSize = parsed
	case 'G':
		currSize = parsed * 1024
	case 'T':
		currSize = parsed * 1024 * 1024
	}

	log.Debug("cachePolicy::getCacheUsage : current cache usage : %fMB", currSize)
	return currSize, nil
}
