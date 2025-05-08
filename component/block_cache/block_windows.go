//go:build windows

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2024 Seagate Technology LLC and/or its Affiliates
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

package block_cache

import (
	"fmt"
	"unsafe"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"golang.org/x/sys/windows"
)

// AllocateBlock creates a new memory mapped buffer for the given size
func AllocateBlock(size uint64) (*Block, error) {
	if size == 0 {
		return nil, fmt.Errorf("invalid size")
	}

	freeRam, err := common.GetFreeRam()
	if err != nil {
		log.Warn("could not get free RAM: %v", err)
	} else {
		if freeRam < size {
			// Not enough free RAM to allocate the requested size
			return nil, fmt.Errorf("insufficient memory available: requested %d bytes, available %d bytes", size, freeRam)
		}
	}

	// https://learn.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-virtualalloc
	ptr, err := windows.VirtualAlloc(
		0,
		uintptr(size),
		windows.MEM_COMMIT|windows.MEM_RESERVE,
		windows.PAGE_READWRITE,
	)
	if err != nil || ptr == 0 {
		return nil, fmt.Errorf("create virtual mapping error: %v", err)
	}

	data := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), size)

	block := &Block{
		data:  data,
		state: nil,
		id:    -1,
		node:  nil,
	}

	// we do not create channel here, as that will be created when buffer is retrieved
	// reinit will always be called before use and that will create the channel as well.
	block.flags.Reset()
	block.flags.Set(BlockFlagFresh)
	return block, nil
}

// Delete cleans up the memory mapped buffer
func (b *Block) Delete() error {
	if b.data == nil {
		return fmt.Errorf("invalid buffer")
	}

	addr := uintptr(unsafe.Pointer(unsafe.SliceData(b.data)))

	err := windows.VirtualFree(
		uintptr(addr),
		0,
		windows.MEM_RELEASE,
	)
	b.data = nil

	if err != nil {
		return fmt.Errorf("cannot unmap memory mapped file: %w", err)
	}

	return nil
}
