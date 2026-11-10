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

package file_cache

import (
	"os"
	"syscall"

	"github.com/Seagate/cloudfuse/common/log"
	"golang.org/x/sys/unix"
)

func (fc *FileCache) syncFile(f *os.File, path string) error {
	// Flush all data to disk that has been buffered by the kernel.
	// We cannot close the incoming handle since the user called flush, note close and flush can be called on the same handle multiple times.
	// To ensure the data is flushed to disk before writing to storage, we duplicate the handle and close that handle.
	// f.fsync() is another option but dup+close does it quickly compared to sync
	dupFd, err := unix.Dup(int(f.Fd()))
	if err != nil {
		log.Err("FileCache::FlushFile : error [couldn't duplicate the fd] %s", path)
		return syscall.EIO
	}

	err = unix.Close(dupFd)
	if err != nil {
		log.Err("FileCache::FlushFile : error [unable to close duplicate fd] %s", path)
		return syscall.EIO
	}
	return nil
}
