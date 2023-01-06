//go:build linux

package file_cache

import (
	"io/fs"
	"math"
	"os"
	"time"

	"golang.org/x/sys/unix"

	"lyvecloudfuse/common"
	"lyvecloudfuse/common/log"
	"lyvecloudfuse/internal"
)

// Creates a new object attribute
func newObjAttr(path string, info fs.FileInfo) *internal.ObjAttr {
	stat := info.Sys().(*unix.Stat_t)
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
		stat := finfo.Sys().(*unix.Stat_t)

		// Deciding based on last modified time is not correct. Last modified time is based on the file was last written
		// so if file was last written back to container 2 days back then even downloading it now shall represent the same date
		// hence immediately after download it will become invalid. It shall be based on when the file was last downloaded.
		// We can rely on last change time because once file is downloaded we reset its last mod time (represent same time as
		// container on the local disk by resetting last mod time of local disk with utimens)
		// and hence last change time on local disk will then represent the download time.

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

	return downloadRequired, fileExists
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
	usage := getUsage(c.tmpPath) * MB

	available := maxCacheSize - usage
	statfs := &unix.Statfs_t{}
	err := unix.Statfs("/", statfs)
	if err != nil {
		log.Debug("FileCache::StatFs : statfs err [%s].", err.Error())
		return nil, false, err
	}

	stat := common.Statfs_t{
		Blocks: uint64(maxCacheSize) / uint64(statfs.Frsize),
		Bavail: uint64(math.Max(0, available)) / uint64(statfs.Frsize),
		Bfree:  statfs.Bavail,
		Bsize:  statfs.Bsize,
		Ffree:  statfs.Ffree,
		Files:  statfs.Files,
		Flags:  statfs.Flags,
		Frsize: statfs.Frsize,
	}

	return &stat, true, nil
}
