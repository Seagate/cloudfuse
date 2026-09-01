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
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/Seagate/cloudfuse/common"
)

const (
	tieredStorageSnapshotPath    = ".tieredStorageSnapshot.gob"
	tieredStorageSnapshotVersion = 1
)

type persistedFileState struct {
	Size        int64
	Mtime       int64
	CloudBacked bool
	Dirty       bool
}

type tieredStorageSnapshot struct {
	Version  uint32
	Files    map[string]persistedFileState
	LRUOrder []string
}

type recoveredFile struct {
	name  string
	info  fs.FileInfo
	state persistedFileState
}

func (c *TieredStorage) writeSnapshot() error {
	snapshot := tieredStorageSnapshot{
		Version: tieredStorageSnapshotVersion,
		Files:   make(map[string]persistedFileState),
	}

	c.fileMap.Range(func(key, value any) bool {
		name := key.(string)
		localPath, err := c.localPath(name)
		if err != nil {
			return true
		}
		info, err := os.Stat(localPath)
		if err != nil || !info.Mode().IsRegular() {
			return true
		}
		node := value.(*FileNode)
		snapshot.Files[name] = persistedFileState{
			Size:        info.Size(),
			Mtime:       info.ModTime().UnixNano(),
			CloudBacked: node.cloudBacked.Load(),
			Dirty:       node.isDirty.Load(),
		}
		return true
	})

	c.policy.mu.Lock()
	for node := c.policy.head; node != nil; node = node.next {
		snapshot.LRUOrder = append(snapshot.LRUOrder, node.name)
	}
	c.policy.mu.Unlock()

	var data bytes.Buffer
	if err := gob.NewEncoder(&data).Encode(snapshot); err != nil {
		return fmt.Errorf("encode state snapshot: %w", err)
	}

	path := filepath.Join(c.tmpPath, tieredStorageSnapshotPath)
	tmpPath := path + ".tmp"
	file, err := common.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("create state snapshot: %w", err)
	}
	if _, err = file.Write(data.Bytes()); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write state snapshot: %w", err)
	}
	if err := replaceFile(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("install state snapshot: %w", err)
	}
	return nil
}

func (c *TieredStorage) readSnapshot() (*tieredStorageSnapshot, error) {
	path := filepath.Join(c.tmpPath, tieredStorageSnapshotPath)
	_ = os.Remove(path + ".tmp")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer os.Remove(path)

	var snapshot tieredStorageSnapshot
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("decode state snapshot: %w", err)
	}
	if snapshot.Version != tieredStorageSnapshotVersion {
		return nil, fmt.Errorf("unsupported state snapshot version %d", snapshot.Version)
	}
	return &snapshot, nil
}

func (c *TieredStorage) recoverLocalState(snapshot *tieredStorageSnapshot) error {
	var recovered []recoveredFile
	err := filepath.WalkDir(c.tmpPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		if entry.Name() == tieredStorageSnapshotPath ||
			entry.Name() == tieredStorageSnapshotPath+".tmp" {
			return nil
		}

		rel, err := filepath.Rel(c.tmpPath, path)
		if err != nil {
			return err
		}
		name := common.NormalizeObjectName(filepath.ToSlash(rel))
		info, err := entry.Info()
		if err != nil {
			return err
		}

		state := persistedFileState{
			Size:  info.Size(),
			Mtime: info.ModTime().UnixNano(),
			Dirty: true,
		}
		if snapshot != nil {
			if saved, found := snapshot.Files[name]; found &&
				saved.Size == info.Size() && saved.Mtime == info.ModTime().UnixNano() {
				state = saved
			}
		}

		if state.CloudBacked && !state.Dirty {
			if err := os.Remove(path); err != nil {
				return err
			}
			c.cacheSize.Add(-info.Size())
			return nil
		}
		if !state.CloudBacked {
			state.Dirty = true
		}
		recovered = append(recovered, recoveredFile{name: name, info: info, state: state})
		return nil
	})
	if err != nil {
		return err
	}

	byName := make(map[string]recoveredFile, len(recovered))
	for _, file := range recovered {
		byName[file.name] = file
		node := &FileNode{name: file.name}
		node.size.Store(file.info.Size())
		node.cloudBacked.Store(file.state.CloudBacked)
		node.isDirty.Store(file.state.Dirty)
		c.fileMap.Store(file.name, node)
	}

	var order []string
	queued := make(map[string]struct{}, len(recovered))
	if snapshot != nil {
		for _, name := range snapshot.LRUOrder {
			if _, found := byName[name]; found {
				order = append(order, name)
				queued[name] = struct{}{}
			}
		}
	}

	var unmatched []recoveredFile
	for _, file := range recovered {
		if _, found := queued[file.name]; !found {
			unmatched = append(unmatched, file)
		}
	}
	slices.SortFunc(unmatched, func(a, b recoveredFile) int {
		return b.info.ModTime().Compare(a.info.ModTime())
	})
	for _, file := range unmatched {
		order = append(order, file.name)
	}

	for _, name := range slices.Backward(order) {
		c.policy.Enqueue(name)
	}
	return nil
}
