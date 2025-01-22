/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates

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

package size_tracker

import (
	"encoding/binary"
	"os"
	"sync"

	"github.com/Seagate/cloudfuse/common"
)

var journalFile string

type MountSize struct {
	size uint64
	file *os.File
	mu   sync.Mutex
}

func CreateSizeJournal(filename string) (*MountSize, error) {
	journalFile = common.JoinUnixFilepath(common.DefaultWorkDir, filename)
	f, err := os.OpenFile(journalFile, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fileInfo, err := f.Stat()
	if err != nil {
		return nil, err
	}

	var size uint64

	if fileInfo.Size() >= 8 {
		buf := make([]byte, 8)
		if _, err := f.ReadAt(buf, 0); err != nil {
			return nil, err
		}

		size = binary.BigEndian.Uint64(buf)
	}

	return &MountSize{size: size, file: f, mu: sync.Mutex{}}, nil
}

func (s *MountSize) GetSize() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.size
}

func (s *MountSize) Add(delta uint64) uint64 {
	return s.updateSize(delta)
}

func (s *MountSize) Subtract(delta uint64) uint64 {
	return s.updateSize(-delta)
}

func (s *MountSize) CloseFile() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.file.Close()
}

func (s *MountSize) updateSize(delta uint64) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.size += delta
	_ = s.writeSizeToFile()
	return s.size
}

func (s *MountSize) writeSizeToFile() error {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, s.size)

	if _, err := s.file.WriteAt(buf, 0); err != nil {
		return err
	}

	return s.file.Sync()
}
