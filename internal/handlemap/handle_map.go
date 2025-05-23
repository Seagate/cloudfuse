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

package handlemap

import (
	"container/list"
	"os"
	"sync"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/cache_policy"

	"go.uber.org/atomic"
)

type HandleID uint64

const InvalidHandleID HandleID = 0

// Flags represented in BitMap for various flags in the handle
const (
	HandleFlagUnknown  uint16 = iota
	HandleFlagDirty           // File has been modified with write operation or is a new file
	HandleFlagFSynced         // User has called fsync on the file explicitly
	HandleFlagCached          // File is cached in the local system by cloudfuse
	HandleOpenedAppend        // File is opened for Append
)

// Structure to hold in memory cache for streaming layer
type Cache struct {
	sync.RWMutex
	*cache_policy.LRUCache
	*common.BlockOffsetList
	StreamOnly  bool
	HandleCount int64
}

type Buffers struct {
	Cooked  *list.List
	Cooking *list.List
}

type Handle struct {
	sync.RWMutex
	FObj     *os.File // File object being represented by this handle
	CacheObj *Cache   // Streaming layer cache for this handle
	Buffers  *Buffers
	ID       HandleID // Cloudfuse assigned unique ID to this handle
	Size     int64    // Size of the file being handled here
	Mtime    time.Time
	UnixFD   uint64                 // Unix FD created by create/open syscall
	OptCnt   uint64                 // Number of operations done on this file
	Flags    common.BitMap16        // Various states of the file
	Path     string                 // Always holds path relative to mount dir, same as object name (uses common.JoinUnixFilepath)
	values   map[string]interface{} // Map to hold other info if application wants to store
}

func NewHandle(path string) *Handle {
	return &Handle{
		ID:       InvalidHandleID,
		Path:     path,
		Size:     0,
		Flags:    0,
		OptCnt:   0,
		values:   make(map[string]interface{}),
		CacheObj: nil,
		FObj:     nil,
		Buffers:  nil,
	}
}

// Dirty : Handle is dirty or not
func (handle *Handle) Dirty() bool {
	return handle.Flags.IsSet(HandleFlagDirty)
}

// Fsynced : Handle is Fsynced or not
func (handle *Handle) Fsynced() bool {
	return handle.Flags.IsSet(HandleFlagFSynced)
}

// Cached : File is cached on local disk or not
func (handle *Handle) Cached() bool {
	return handle.Flags.IsSet(HandleFlagCached)
}

// GetFileObject : Get the OS.File handle stored within
func (handle *Handle) GetFileObject() *os.File {
	return handle.FObj
}

// SetFileObject : Store the OS.File handle
func (handle *Handle) SetFileObject(f *os.File) {
	handle.FObj = f
}

// FD : Get Unix file descriptor
func (handle *Handle) FD() int {
	return int(handle.UnixFD)
}

// SetValue : Store user defined parameter inside handle
func (handle *Handle) SetValue(key string, value interface{}) {
	handle.values[key] = value
}

// GetValue : Retrieve user defined parameter from handle
func (handle *Handle) GetValue(key string) (interface{}, bool) {
	val, ok := handle.values[key]
	return val, ok
}

// GetValue : Retrieve user defined parameter from handle
func (handle *Handle) RemoveValue(key string) (interface{}, bool) {
	val, ok := handle.values[key]
	delete(handle.values, key)
	return val, ok
}

// Cleanup : Delete all user defined parameter from handle
func (handle *Handle) Cleanup() {
	clear(handle.values)
}

// defaultHandleMap holds a synchronized map[ HandleID ]*Handle
var defaultHandleMap sync.Map
var nextHandleID = *atomic.NewUint64(uint64(0))

// Add : Add the newly created handle to map and allocate a handle id
func Add(handle *Handle) HandleID {
	var ok = true
	var key HandleID
	for ok {
		key = HandleID(nextHandleID.Inc())
		_, ok = defaultHandleMap.LoadOrStore(key, handle)
	}
	handle.ID = key
	return key
}

// Delete : Remove handle object from map
func Delete(key HandleID) {
	defaultHandleMap.Delete(key)
}

// Delete : Remove handle object from map, and return the entry (if any)
func LoadAndDelete(key HandleID) (*Handle, bool) {
	val, found := defaultHandleMap.LoadAndDelete(key)
	if !found {
		return nil, false
	}
	return val.(*Handle), true
}

func CreateCacheObject(capacity int64, handle *Handle) {
	handle.CacheObj = &Cache{
		sync.RWMutex{},
		cache_policy.NewLRUCache(capacity),
		&common.BlockOffsetList{},
		false,
		0,
	}
}

// GetHandles : Get map of handles stored
func GetHandles() *sync.Map {
	return &defaultHandleMap
}

// Load : Search the handle object based on its id
func Load(key HandleID) (*Handle, bool) {
	handleIF, ok := defaultHandleMap.Load(key)
	if !ok {
		return nil, false
	}
	handle := handleIF.(*Handle)
	return handle, true
}

// Store function must not be used in production application.
// This is a utility function present only for test scenarios.
func Store(key HandleID, path string, _ uintptr) *Handle {
	handle := &Handle{
		ID:   key,
		Path: path,
	}
	defaultHandleMap.Store(key, handle)
	return handle
}
