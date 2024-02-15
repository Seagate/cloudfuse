//go:build linux

/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2024 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.

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
	"container/list"
	"fmt"

	"github.com/Seagate/cloudfuse/common"
	"golang.org/x/sys/unix"
)

// Various flags denoting state of a block
const (
	BlockFlagFresh       uint16 = iota
	BlockFlagDownloading        // Block is being downloaded
	BlockFlagUploading          // Block is being uploaded
	BlockFlagDirty              // Block has been written and data is not persisted yet
	BlockFlagSynced             // Block has been written and data is persisted
	BlockFlagFailed             // Block upload/download has failed
)

// Block is a memory mapped buffer with its state to hold data
type Block struct {
	offset   uint64          // Start offset of the data this block holds
	id       int64           // Id of the block i.e. (offset / block size)
	endIndex uint64          // Length of the data this block holds
	state    chan int        // Channel depicting data has been read for this block or not
	flags    common.BitMap16 // Various states of the block
	data     []byte          // Data read from blob
	node     *list.Element   // node representation of this block in the list inside handle
}

// AllocateBlock creates a new memory mapped buffer for the given size
func AllocateBlock(size uint64) (*Block, error) {
	if size == 0 {
		return nil, fmt.Errorf("invalid size")
	}

	prot, flags := unix.PROT_READ|unix.PROT_WRITE, unix.MAP_ANON|unix.MAP_PRIVATE
	addr, err := unix.Mmap(-1, 0, int(size), prot, flags)

	if err != nil {
		return nil, fmt.Errorf("mmap error: %v", err)
	}

	block := &Block{
		data:  addr,
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

	err := unix.Munmap(b.data)
	b.data = nil
	if err != nil {
		// if we get here, there is likely memory corruption.
		return fmt.Errorf("munmap error: %v", err)
	}

	return nil
}

// ReUse reinits the Block by recreating its channel
func (b *Block) ReUse() {
	b.id = -1
	b.offset = 0
	b.endIndex = 0
	b.flags.Reset()
	b.flags.Set(BlockFlagFresh)
	b.state = make(chan int, 1)
}

// Uploading marks buffer is under upload
func (b *Block) Uploading() {
	b.state = make(chan int, 1)
}

// Ready marks this Block is now ready for reading by its first reader (data download completed)
func (b *Block) Ready() {
	select {
	case b.state <- 1:
		break
	default:
		break
	}
}

// Unblock marks this Block is ready to be read in parllel now
func (b *Block) Unblock() {
	close(b.state)
}

// Mark this block as dirty as it has been modified
func (b *Block) Dirty() {
	b.flags.Set(BlockFlagDirty)
}

// Mark this block as dirty as it has been modified
func (b *Block) NoMoreDirty() {
	b.flags.Clear(BlockFlagDirty)
}

// Check if this block has been modified or not
func (b *Block) IsDirty() bool {
	return b.flags.IsSet(BlockFlagDirty)
}

// Mark this block as failed
func (b *Block) Failed() {
	b.flags.Set(BlockFlagFailed)
}

// Check this block as failed
func (b *Block) IsFailed() bool {
	return b.flags.IsSet(BlockFlagFailed)
}
