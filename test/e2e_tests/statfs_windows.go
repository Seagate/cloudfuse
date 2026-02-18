//go:build windows

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

package e2e_tests

import (
	"golang.org/x/sys/windows"
)

func DiskSize(path string) int {
	var free, total, avail uint64

	// Get path to the cache
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0
	}
	err = windows.GetDiskFreeSpaceEx(pathPtr, &free, &total, &avail)
	if err != nil {
		return 0
	}

	return int(total - free)
}
