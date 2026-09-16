//go:build windows

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2026 Seagate Technology LLC and/or its Affiliates

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

package tiered_storage

import (
	"io/fs"
	"os"
	"syscall"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/internal"
	"golang.org/x/sys/windows"
)

func newTieredStorageObjAttr(path string, info fs.FileInfo) *internal.ObjAttr {
	stat := info.Sys().(*syscall.Win32FileAttributeData)
	attrs := &internal.ObjAttr{
		Path:  common.NormalizeObjectName(path),
		Name:  common.NormalizeObjectName(info.Name()),
		Size:  info.Size(),
		Mode:  info.Mode() &^ os.ModePerm,
		Mtime: time.Unix(0, stat.LastWriteTime.Nanoseconds()),
		Atime: time.Unix(0, stat.LastAccessTime.Nanoseconds()),
		Ctime: time.Unix(0, stat.CreationTime.Nanoseconds()),
	}

	attrs.Flags.Set(internal.PropFlagModeDefault)
	if info.Mode()&os.ModeSymlink != 0 {
		attrs.Flags.Set(internal.PropFlagSymlink)
	} else if info.IsDir() {
		attrs.Flags.Set(internal.PropFlagIsDir)
	}
	return attrs
}

func (c *TieredStorage) getAvailableSize() (uint64, error) {
	path, err := windows.UTF16PtrFromString(c.tmpPath)
	if err != nil {
		return 0, err
	}
	var free, total, available uint64
	if err := windows.GetDiskFreeSpaceEx(path, &free, &total, &available); err != nil {
		return 0, err
	}
	return available, nil
}
