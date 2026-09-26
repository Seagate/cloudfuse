//go:build linux

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
	"os"
	"path/filepath"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestChownLocalFilePersistsOnUpload(t *testing.T) {
	ctrl := gomock.NewController(t)
	next := internal.NewMockComponent(ctrl)
	path := "local-chown"
	cachePath := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(cachePath, path), []byte("data"), 0644))

	storage := &TieredStorage{
		tmpPath:   cachePath,
		fileLocks: common.NewLockMap(),
		cacheSize: newCacheSizeTracker(cachePath, 0),
	}
	storage.SetNextComponent(next)
	node := &FileNode{name: path}
	node.size.Store(4)
	storage.fileMap.Store(path, node)

	owner, group := os.Getuid(), os.Getgid()
	require.NoError(t, storage.Chown(internal.ChownOptions{
		Name: path, Owner: owner, Group: group,
	}))
	next.EXPECT().CopyFromFile(gomock.Any()).Return(nil)
	next.EXPECT().Chown(internal.ChownOptions{
		Name: path, Owner: owner, Group: group,
	}).Return(nil)
	require.NoError(t, storage.uploadCachedFile(path))
}
