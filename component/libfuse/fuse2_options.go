//go:build !fuse3

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

package libfuse

import (
	"fmt"

	"github.com/winfsp/cgofuse/fuse"
)

func createFuseOptions(
	host *fuse.FileSystemHost,
	allowOther bool,
	allowRoot bool,
	readOnly bool,
	nonEmptyMount bool,
	maxFuseThreads uint32,
	umask uint32,
) string {
	var options string
	// While reading a file let kernel do readahead for better perf
	options += fmt.Sprintf(",max_readahead=%d", 4*1024*1024)

	// Max background thread on the fuse layer for high parallelism
	options += fmt.Sprintf(",max_background=%d", maxFuseThreads)

	if allowOther {
		options += ",allow_other"
	}
	if allowRoot {
		options += ",allow_root"
	}
	if readOnly {
		options += ",ro"
	}
	if nonEmptyMount {
		options += ",nonempty"
	}

	// force the fuse library to always pass O_TRUNC flag on open call
	// Not checking the options since we don't allow user to configure this flag.
	// This is the default behaviour for the fuse3 hence we don't pass this flag there.
	// ref: https://github.com/libfuse/libfuse/blob/7f86f3be7148b15b71b63357813c66dd32177cf6/lib/fuse_lowlevel.c#L2161C2-L2161C16
	options += ",atomic_o_trunc"

	if umask != 0 {
		options += fmt.Sprintf(",umask=%04d", umask)
	}

	// direct_io option is used to bypass the kernel cache. It disables the use of
	// page cache (file content cache) in the kernel for the filesystem.
	if fuseFS.directIO {
		options += ",direct_io"
	} else {
		options += ",kernel_cache"
	}
	return options
}
