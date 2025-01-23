//go:build linux

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
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

package file_cache

import (
	"io/fs"
	"os"
	"syscall"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"golang.org/x/sys/unix"
)

// Creates a new object attribute
func newObjAttr(path string, info fs.FileInfo) *internal.ObjAttr {
	stat := info.Sys().(*syscall.Stat_t)
	attrs := &internal.ObjAttr{
		Path:  path,
		Name:  info.Name(),
		Size:  info.Size(),
		Mode:  info.Mode(),
		Mtime: time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec),
		Atime: time.Unix(stat.Atim.Sec, stat.Atim.Nsec),
		Ctime: time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec),
	}

	if info.Mode()&os.ModeSymlink != 0 {
		attrs.Flags.Set(internal.PropFlagSymlink)
	} else if info.IsDir() {
		attrs.Flags.Set(internal.PropFlagIsDir)
	}

	return attrs
}

// isDownloadRequired: Whether or not the file needs to be downloaded to local cache.
func (fc *FileCache) isDownloadRequired(localPath string, objectPath string, flock *common.LockMapItem) (bool, bool, *internal.ObjAttr, error) {
	cached := false
	downloadRequired := false
	lmt := time.Time{}
	var stat *syscall.Stat_t = nil

	// check if the file exists locally
	finfo, statErr := os.Stat(localPath)
	if statErr == nil {
		// The file does not need to be downloaded as long as it is in the cache policy
		fileInPolicyCache := fc.policy.IsCached(localPath)
		if fileInPolicyCache {
			cached = true
		} else {
			log.Warn("FileCache::isDownloadRequired : %s exists but is not present in local cache policy", localPath)
		}
		// gather stat details
		stat = finfo.Sys().(*syscall.Stat_t)
		lmt = finfo.ModTime()
	} else if os.IsNotExist(statErr) {
		// The file does not exist in the local cache so it needs to be downloaded
		log.Debug("FileCache::isDownloadRequired : %s not present in local cache", localPath)
	} else {
		// Catch all, the file needs to be downloaded
		log.Debug("FileCache::isDownloadRequired : error calling stat %s [%s]", localPath, statErr.Error())
	}

	// check if the file is due for a refresh from cloud storage
	refreshTimerExpired := fc.refreshSec != 0 && time.Since(flock.DownloadTime()).Seconds() > float64(fc.refreshSec)

	// get cloud attributes
	cloudAttr, err := fc.NextComponent().GetAttr(internal.GetAttrOptions{Name: objectPath})
	if err != nil && !os.IsNotExist(err) {
		log.Err("FileCache::isDownloadRequired : Failed to get attr of %s [%s]", objectPath, err.Error())
	}

	if !cached && cloudAttr != nil {
		downloadRequired = true
	}

	if cached && refreshTimerExpired && cloudAttr != nil {
		// File is not expired, but the user has configured a refresh timer, which has expired.
		// Does the cloud have a newer copy?
		cloudHasLatestData := cloudAttr.Mtime.After(lmt) || stat.Size != cloudAttr.Size
		// Is the local file open?
		fileIsOpen := flock.Count() > 0 && !flock.LazyOpen
		if cloudHasLatestData && !fileIsOpen {
			log.Info("FileCache::isDownloadRequired : File is modified in container, so forcing redownload %s [A-%v : L-%v] [A-%v : L-%v]",
				objectPath, cloudAttr.Mtime, lmt, cloudAttr.Size, stat.Size)
			downloadRequired = true
		} else {
			// log why we decided not to refresh
			if !cloudHasLatestData {
				log.Info("FileCache::isDownloadRequired : File in container is not latest, skip redownload %s [A-%v : L-%v]", objectPath, cloudAttr.Mtime, lmt)
			} else if fileIsOpen {
				log.Info("FileCache::isDownloadRequired : Need to re-download %s, but skipping as handle is already open", objectPath)
			}
			// As we have decided to continue using old file, we reset the timer to check again after refresh time interval
			flock.SetDownloadTime()
		}
	}

	return downloadRequired, cached, cloudAttr, err
}

func (fc *FileCache) getAvailableSize() (uint64, error) {
	statfs := &unix.Statfs_t{}
	err := unix.Statfs(fc.tmpPath, statfs)
	if err != nil {
		log.Debug("FileCache::getAvailableSize : statfs err [%s].", err.Error())
		return 0, err
	}

	available := statfs.Bavail * uint64(statfs.Bsize)
	return available, nil
}
