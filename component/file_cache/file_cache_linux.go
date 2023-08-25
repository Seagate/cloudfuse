//go:build linux

/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
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

package file_cache

import (
	"io/fs"
	"math"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"cloudfuse/common"
	"cloudfuse/common/log"
	"cloudfuse/internal"
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
func (fc *FileCache) isDownloadRequired(localPath string, blobPath string, flock *common.LockMapItem) (bool, bool, *internal.ObjAttr, error) {
	fileExists := false
	downloadRequired := false
	lmt := time.Time{}
	var stat *syscall.Stat_t = nil

	// The file is not cached then we need to download
	if !fc.policy.IsCached(localPath) {
		log.Debug("FileCache::isDownloadRequired : %s not present in local cache policy", localPath)
		downloadRequired = true
	}

	finfo, err := os.Stat(localPath)
	if err == nil {
		// The file exists in local cache
		// The file needs to be downloaded if the cacheTimeout elapsed (check last change time and last modified time)
		fileExists = true
		stat = finfo.Sys().(*syscall.Stat_t)

		// Deciding based on last modified time is not correct. Last modified time is based on the file was last written
		// so if file was last written back to container 2 days back then even downloading it now shall represent the same date
		// hence immediately after download it will become invalid. It shall be based on when the file was last downloaded.
		// We can rely on last change time because once file is downloaded we reset its last mod time (represent same time as
		// container on the local disk by resetting last mod time of local disk with utimens)
		// and hence last change time on local disk will then represent the download time.

		lmt = finfo.ModTime()
		if time.Since(finfo.ModTime()).Seconds() > fc.cacheTimeout &&
			time.Since(time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)).Seconds() > fc.cacheTimeout {
			log.Debug("FileCache::isDownloadRequired : %s not valid as per time checks", localPath)
			downloadRequired = true
		}
	} else if os.IsNotExist(err) {
		// The file does not exist in the local cache so it needs to be downloaded
		log.Debug("FileCache::isDownloadRequired : %s not present in local cache", localPath)
		downloadRequired = true
	} else {
		// Catch all, the file needs to be downloaded
		log.Debug("FileCache::isDownloadRequired : error calling stat %s [%s]", localPath, err.Error())
		downloadRequired = true
	}

	if fileExists && flock.Count() > 0 {
		// file exists in local cache and there is already an handle open for it
		// In this case we can not redownload the file from container
		log.Info("FileCache::isDownloadRequired : Need to re-download %s, but skipping as handle is already open", blobPath)
		downloadRequired = false
	}

	err = nil // reset err variable
	var attr *internal.ObjAttr = nil
	if downloadRequired ||
		(fc.refreshSec != 0 && time.Since(flock.DownloadTime()).Seconds() > float64(fc.refreshSec)) {
		attr, err = fc.NextComponent().GetAttr(internal.GetAttrOptions{Name: blobPath})
		if err != nil {
			log.Err("FileCache::isDownloadRequired : Failed to get attr of %s [%s]", blobPath, err.Error())
		}
	}

	if fc.refreshSec != 0 && !downloadRequired && attr != nil && stat != nil {
		// We decided that based on lmt of file file-cache-timeout has not expired
		// However, user has configured refresh time then check time has elapsed since last download time of file or not
		// If so, compare the lmt of file in local cache and once in container and redownload only if lmt of container is latest.
		// If time matches but size does not then still we need to redownlaod the file.
		if attr.Mtime.After(lmt) || stat.Size != attr.Size {
			// File has not been modified at storage yet so no point in redownloading the file
			log.Info("FileCache::isDownloadRequired : File is modified in container, so forcing redownload %s [A-%v : L-%v] [A-%v : L-%v]",
				blobPath, attr.Mtime, lmt, attr.Size, stat.Size)
			downloadRequired = true

			// As we have decided to continue using old file, we reset the timer to check again after refresh time interval
			flock.SetDownloadTime()
		} else {
			log.Info("FileCache::isDownloadRequired : File in container is not latest, skip redownload %s [A-%v : L-%v]", blobPath, attr.Mtime, lmt)
		}
	}

	return downloadRequired, fileExists, attr, err
}

func (c *FileCache) StatFs() (*common.Statfs_t, bool, error) {
	// cache_size = f_blocks * f_frsize/1024
	// cache_size - used = f_frsize * f_bavail/1024
	// cache_size - used = vfs.f_bfree * vfs.f_frsize / 1024
	// if cache size is set to 0 then we have the root mount usage
	maxCacheSize := c.maxCacheSize * MB
	if maxCacheSize == 0 {
		return nil, false, nil
	}
	usage, _ := getUsage(c.tmpPath)
	usage *= MB

	available := maxCacheSize - usage
	statfs := &unix.Statfs_t{}
	err := unix.Statfs("/", statfs)
	if err != nil {
		log.Debug("FileCache::StatFs : statfs err [%s].", err.Error())
		return nil, false, err
	}

	stat := common.Statfs_t{
		Blocks:  uint64(maxCacheSize) / uint64(statfs.Frsize),
		Bavail:  uint64(math.Max(0, available)) / uint64(statfs.Frsize),
		Bfree:   statfs.Bavail,
		Bsize:   statfs.Bsize,
		Ffree:   statfs.Ffree,
		Files:   statfs.Files,
		Frsize:  statfs.Frsize,
		Namemax: 255,
	}

	return &stat, true, nil
}
