//go:build linux

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

package file_cache

import (
	"io/fs"
	"os"
	"syscall"
	"time"

	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"golang.org/x/sys/unix"
)

// Creates a new object attribute
func newObjAttr(path string, info fs.FileInfo) *internal.ObjAttr {
	stat := info.Sys().(*syscall.Stat_t)
	attrs := &internal.ObjAttr{
		Path:  path,
		Name:  info.Name(),
		Size:  info.Size(),
		Mode:  info.Mode(),
		Mtime: time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec),
		Atime: time.Unix(stat.Atim.Sec, stat.Atim.Nsec),
		Ctime: time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec),
	}

	if info.Mode()&os.ModeSymlink != 0 {
		attrs.Flags.Set(internal.PropFlagSymlink)
	} else if info.IsDir() {
		attrs.Flags.Set(internal.PropFlagIsDir)
	}

	return attrs
}

func (fc *FileCache) getAvailableSize() (uint64, error) {
	statfs := &unix.Statfs_t{}
	err := unix.Statfs(fc.tmpPath, statfs)
	if err != nil {
		log.Debug("FileCache::getAvailableSize : statfs err [%s].", err.Error())
		return 0, err
	}

	available := statfs.Bavail * uint64(statfs.Bsize)
	return available, nil
}
