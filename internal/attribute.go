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

package internal

import (
	"os"
	"path"
	"time"

	"github.com/Seagate/cloudfuse/common"
)

// create an object attributes struct
func CreateObjAttr(objectPath string, size int64, lastModified time.Time) (attr *ObjAttr) {
	attr = &ObjAttr{
		Path:   objectPath,
		Name:   path.Base(objectPath),
		Size:   size,
		Mode:   0,
		Mtime:  lastModified,
		Atime:  lastModified,
		Ctime:  lastModified,
		Crtime: lastModified,
		Flags:  NewFileBitMap(),
	}
	// set flags
	attr.Flags.Set(PropFlagModeDefault)
	attr.Metadata = make(map[string]*string)

	return attr
}

// create an object attributes struct for a directory
func CreateObjAttrDir(path string) (attr *ObjAttr) {
	// strip any trailing slash
	path = TruncateDirName(path)
	// For these dirs we get only the name and no other properties so hardcoding time to current time
	currentTime := time.Now()

	attr = CreateObjAttr(path, 4096, currentTime)
	// Change the relevant fields for a directory
	attr.Mode = os.ModeDir
	// set flags
	attr.Flags = NewDirBitMap()
	attr.Flags.Set(PropFlagModeDefault)

	return attr
}

func NewDirBitMap() common.BitMap16 {
	bm := common.BitMap16(0)
	bm.Set(PropFlagIsDir)
	return bm
}

func NewSymlinkBitMap() common.BitMap16 {
	bm := common.BitMap16(0)
	bm.Set(PropFlagSymlink)
	return bm
}

func NewFileBitMap() common.BitMap16 {
	bm := common.BitMap16(0)
	return bm
}

// Flags represented in common.BitMap16 for various properties of the object
const (
	PropFlagUnknown uint16 = iota
	PropFlagNotExists
	PropFlagIsDir
	PropFlagEmptyDir
	PropFlagSymlink
	PropFlagModeDefault // TODO: Does this sound better as ModeDefault or DefaultMode? The getter would be IsModeDefault or IsDefaultMode
)

// ObjAttr : Attributes of any file/directory
type ObjAttr struct {
	Mtime    time.Time       // modified time
	Atime    time.Time       // access time
	Ctime    time.Time       // change time
	Crtime   time.Time       // creation time
	Size     int64           // size of the file/directory
	Mode     os.FileMode     // permissions in 0xxx format
	Flags    common.BitMap16 // flags
	Path     string          // full path
	Name     string          // base name of the path
	MD5      []byte
	Metadata map[string]*string // extra information to preserve
}

// IsDir : Test blob is a directory or not
func (attr *ObjAttr) IsDir() bool {
	return attr.Flags.IsSet(PropFlagIsDir)
}

// IsSymlink : Test blob is a symlink or not
func (attr *ObjAttr) IsSymlink() bool {
	return attr.Flags.IsSet(PropFlagSymlink)
}

// IsModeDefault : Whether or not to use the default mode.
// This is set in any storage service that does not support chmod/chown.
func (attr *ObjAttr) IsModeDefault() bool {
	return attr.Flags.IsSet(PropFlagModeDefault)
}
