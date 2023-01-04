//go:build windows

package file_cache

import (
	"io/fs"
	"os"
	"syscall"
	"time"

	"lyvecloudfuse/common/log"
	"lyvecloudfuse/internal"
)

// Creates a new object attribute
func newObjAttr(path string, info fs.FileInfo) *internal.ObjAttr {
	stat := info.Sys().(*syscall.Win32FileAttributeData)
	attrs := &internal.ObjAttr{
		Path:  path,
		Name:  info.Name(),
		Size:  info.Size(),
		Mode:  info.Mode(),
		Mtime: time.Unix(0, stat.LastWriteTime.Nanoseconds()),
		Atime: time.Unix(0, stat.LastAccessTime.Nanoseconds()),
		Ctime: time.Unix(0, stat.CreationTime.Nanoseconds()),
	}

	if info.Mode()&os.ModeSymlink != 0 {
		attrs.Flags.Set(internal.PropFlagSymlink)
	} else if info.IsDir() {
		attrs.Flags.Set(internal.PropFlagIsDir)
	}

	return attrs
}

// isDownloadRequired: Whether or not the file needs to be downloaded to local cache.
func (fc *FileCache) isDownloadRequired(localPath string) (bool, bool) {
	fileExists := false
	downloadRequired := false

	// The file is not cached
	if !fc.policy.IsCached(localPath) {
		log.Debug("FileCache::isDownloadRequired : %s not present in local cache policy", localPath)
		downloadRequired = true
	}

	finfo, err := os.Stat(localPath)
	if err == nil {
		// The file exists in local cache
		// The file needs to be downloaded if the cacheTimeout elapsed (check last change time and last modified time)
		fileExists = true
		stat := finfo.Sys().(*syscall.Win32FileAttributeData)

		// Deciding based on last modified time is not correct. Last modified time is based on the file was last written
		// so if file was last written back to container 2 days back then even downloading it now shall represent the same date
		// hence immediately after download it will become invalid. It shall be based on when the file was last downloaded.
		// We can rely on last change time because once file is downloaded we reset its last mod time (represent same time as
		// container on the local disk by resetting last mod time of local disk with utimens)
		// and hence last change time on local disk will then represent the download time.

		if time.Since(finfo.ModTime()).Seconds() > fc.cacheTimeout &&
			time.Since(time.Unix(0, stat.CreationTime.Nanoseconds())).Seconds() > fc.cacheTimeout {
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

	return downloadRequired, fileExists
}
