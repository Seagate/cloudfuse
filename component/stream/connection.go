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

package stream

import (
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/handlemap"
)

type StreamConnection interface {
	RenameDirectory(options internal.RenameDirOptions) error
	DeleteDirectory(options internal.DeleteDirOptions) error
	RenameFile(options internal.RenameFileOptions) error
	DeleteFile(options internal.DeleteFileOptions) error
	CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) //TODO TEST THIS
	Configure(cfg StreamOptions) error
	ReadInBuffer(internal.ReadInBufferOptions) (int, error)
	OpenFile(internal.OpenFileOptions) (*handlemap.Handle, error)
	WriteFile(options internal.WriteFileOptions) (int, error)
	TruncateFile(internal.TruncateFileOptions) error
	FlushFile(internal.FlushFileOptions) error
	GetAttr(internal.GetAttrOptions) (*internal.ObjAttr, error)
	CloseFile(options internal.CloseFileOptions) error
	SyncFile(options internal.SyncFileOptions) error
	Stop() error
}

// NewAzStorageConnection : Based on account type create respective AzConnection Object
func NewStreamConnection(cfg StreamOptions, stream *Stream) StreamConnection {
	if cfg.readOnly {
		r := ReadCache{}
		r.Stream = stream
		_ = r.Configure(cfg)
		return &r
	}
	if cfg.FileCaching {
		rw := ReadWriteFilenameCache{}
		rw.Stream = stream
		_ = rw.Configure(cfg)
		return &rw
	}
	rw := ReadWriteCache{}
	rw.Stream = stream
	_ = rw.Configure(cfg)
	return &rw
}
