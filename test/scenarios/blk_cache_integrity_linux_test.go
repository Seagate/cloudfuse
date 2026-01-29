//go:build linux

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
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

package scenarios

import (
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/stretchr/testify/assert"
)

// Test stripe reading with dup.
func TestStripeReadingWithDup(t *testing.T) {
	t.Parallel()
	filename := "testfile_stripe_reading_dup.txt"
	content := []byte("Stripe Reading With Dup Test data")
	tempbuf := make([]byte, len(content))
	offsets := []int64{69, 8*1024*1024 + 69, 16*1024*1024 + 69}
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)
		// Write to the file.
		for _, off := range offsets {
			written, err := file.WriteAt(content, int64(off))
			assert.NoError(t, err)
			assert.Equal(t, len(content), written)
		}
		err = file.Close()
		assert.NoError(t, err)
		// Read from the different offsets using different file descriptions
		file0, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.NoError(t, err)
		fd1, err := unix.Dup(int(file0.Fd()))
		assert.NotEqual(t, int(file.Fd()), fd1)
		assert.NoError(t, err)
		fd2, err := unix.Dup(int(file0.Fd()))
		assert.NotEqual(t, int(file.Fd()), fd1)
		assert.NoError(t, err)

		bytesread, err := file0.ReadAt(tempbuf, offsets[0]) //read at 0MB
		assert.NoError(t, err)
		assert.Equal(t, len(tempbuf), bytesread)
		assert.Equal(t, content, tempbuf)
		bytesread, err = unix.Pread(fd1, tempbuf, offsets[1]) //write at 8MB
		assert.NoError(t, err)
		assert.Equal(t, len(tempbuf), bytesread)
		assert.Equal(t, content, tempbuf)
		bytesread, err = unix.Pread(fd2, tempbuf, offsets[2]) //write at 16MB
		assert.NoError(t, err)
		assert.Equal(t, len(tempbuf), bytesread)
		assert.Equal(t, content, tempbuf)

		err = file0.Close()
		assert.NoError(t, err)
		err = unix.Close(fd1)
		assert.NoError(t, err)
		err = unix.Close(fd2)
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Dup the FD and do parllel flush calls while writing.
func TestParllelFlushCallsByDuping(t *testing.T) {
	filename := "testfile_parallel_flush_calls_using_dup.txt"
	databuffer := make([]byte, 4*1024) // 4KB buffer
	_, err := io.ReadFull(rand.Reader, databuffer)
	assert.NoError(t, err)

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)

		fd1, err := unix.Dup(int(file.Fd()))
		assert.NotEqual(t, int(file.Fd()), fd1)
		assert.NoError(t, err)

		// for each 1MB writes trigger a flush call from another go routine.
		trigger_flush := make(chan struct{}, 1)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				_, ok := <-trigger_flush
				if !ok {
					break
				}
				err := unix.Fdatasync(fd1)
				assert.NoError(t, err)
			}
		}()
		// Write 40M data
		for i := 0; i < 40*1024*1024; i += 4 * 1024 {
			if i%(1*1024*1024) == 0 {
				trigger_flush <- struct{}{}
			}
			byteswritten, err := file.Write(databuffer)
			assert.Equal(t, 4*1024, byteswritten)
			assert.NoError(t, err)
		}
		close(trigger_flush)
		wg.Wait()
		err = file.Close()
		assert.NoError(t, err)
		err = unix.Close(fd1)
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test stripe writing with dup. same as the stripe writing but rather than opening so many files duplicate the file descriptor.
func TestStripeWritingWithDup(t *testing.T) {
	t.Parallel()
	filename := "testfile_stripe_writing_dup.txt"
	content := []byte("Stripe writing with dup test data")
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)
		fd1, err := unix.Dup(int(file.Fd()))
		assert.NotEqual(t, int(file.Fd()), fd1)
		assert.NoError(t, err)

		fd2, err := unix.Dup(int(file.Fd()))
		assert.NotEqual(t, int(file.Fd()), fd1)
		assert.NoError(t, err)

		written, err := file.WriteAt(content, int64(0))
		assert.NoError(t, err)
		assert.Equal(t, len(content), written)
		written, err = unix.Pwrite(fd1, content, int64(8*1024*1024))
		assert.NoError(t, err)
		assert.Equal(t, len(content), written)
		written, err = unix.Pwrite(fd1, content, int64(16*1024*1024))
		assert.NoError(t, err)
		assert.Equal(t, len(content), written)

		err = file.Close()
		assert.NoError(t, err)
		err = unix.Close(fd1)
		assert.NoError(t, err)
		err = unix.Close(fd2)
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}
