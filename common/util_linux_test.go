//go:build linux

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

package common

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// func (suite *utilTestSuite) TestSetFrsize() {
// 	st := &unix.Statfs_t{}
// 	var val uint64 = 4096
// 	SetFrsize(st, val)
// 	suite.assert.Equal(int64(val), st.Frsize)
// }

func (suite *utilTestSuite) TestGetAvailableMemoryBytes() {
	availableMemory, err := GetAvailableMemoryBytes()
	suite.assert.NoError(err)

	meminfoMemory, err := readAvailableMemoryBytesFromProcMeminfo()
	suite.assert.NoError(err)

	var difference uint64
	if availableMemory > meminfoMemory {
		difference = availableMemory - meminfoMemory
	} else {
		difference = meminfoMemory - availableMemory
	}

	tolerance := meminfoMemory / 100
	suite.assert.LessOrEqual(difference, tolerance)
}

func readAvailableMemoryBytesFromProcMeminfo() (uint64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}

	var memFree uint64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, err
		}

		value *= 1024
		switch fields[0] {
		case "MemAvailable:":
			if value > 0 {
				return value, nil
			}
		case "MemFree:":
			memFree = value
		}
	}

	if memFree > 0 {
		return memFree, nil
	}

	return 0, fmt.Errorf("neither MemAvailable nor MemFree found in /proc/meminfo")
}
