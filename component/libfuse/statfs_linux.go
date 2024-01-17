//go:build linux

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2024 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.

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
	"golang.org/x/sys/unix"
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
		statfs := &unix.Statfs_t{}
		err = unix.Statfs("/", statfs)
		if err != nil {
			log.Err("Libfuse::Statfs: Failed to get stats %s [%s]", name, err.Error())
			return -fuse.EIO
		}

		stat.Bsize = uint64(statfs.Bsize)
		stat.Frsize = uint64(statfs.Frsize)
		stat.Blocks = statfs.Blocks
		stat.Bavail = statfs.Bavail
		stat.Bfree = statfs.Bfree
		stat.Files = statfs.Files
		stat.Ffree = statfs.Ffree
		stat.Namemax = uint64(statfs.Namelen)
	}

	return 0
}
