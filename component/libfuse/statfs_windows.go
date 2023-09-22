//go:build windows

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

package libfuse

import (
	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"

	"github.com/winfsp/cgofuse/fuse"
	"golang.org/x/sys/windows"
)

// Statfs sets file system statistics. It returns 0 if successful.
func (cf *CgofuseFS) Statfs(path string, stat *fuse.Statfs_t) int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_statfs : %s", name)

	attr, populated, err := fuseFS.NextComponent().StatFs()
	if err != nil {
		log.Err("Libfuse::Statfs: Failed to get stats %s [%s]", name, err.Error())
		return -fuse.EIO
	}

	// if populated then we need to overwrite root attributes
	if populated {
		stat.Bsize = uint64(attr.Bsize)
		stat.Frsize = uint64(attr.Frsize)
		stat.Blocks = attr.Blocks
		stat.Bavail = attr.Bavail
		stat.Bfree = attr.Bfree
		stat.Files = attr.Files
		stat.Ffree = attr.Ffree
		stat.Namemax = attr.Namemax
	} else {
		var free, total, avail uint64

		// Get path to the cache
		pathPtr, err := windows.UTF16PtrFromString("/")
		if err != nil {
			log.Err("Libfuse::Statfs: Failed to get stats %s [%s]", name, err.Error())
			return -fuse.EIO
		}
		err = windows.GetDiskFreeSpaceEx(pathPtr, &free, &total, &avail)
		if err != nil {
			log.Err("Libfuse::Statfs: Failed to get stats %s [%s]", name, err.Error())
			return -fuse.EIO
		}

		const blockSize = 4096

		stat.Bsize = blockSize
		stat.Frsize = blockSize
		stat.Blocks = total / blockSize
		stat.Bavail = avail / blockSize
		stat.Bfree = free / blockSize
		stat.Files = 1e9
		stat.Ffree = 1e9
		stat.Namemax = 255
	}

	return 0
}
