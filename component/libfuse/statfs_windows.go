//go:build windows

package libfuse

import (
	"lyvecloudfuse/common"
	"lyvecloudfuse/common/log"

	"github.com/winfsp/cgofuse/fuse"
	"golang.org/x/sys/windows"
)

// Statfs sets file system statics. It returns 0 if successful.
func (cf *CgofuseFS) Statfs(path string, stat *fuse.Statfs_t) int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_statfs : %s", name)

	attr, populated, err := fuseFS.NextComponent().StatFs()
	if err != nil {
		log.Err("Libfuse::Statfs: Failed to get stats %s [%s]", name, err.Error())
		return -fuse.EIO
	}

	// if populated then we need to overwrite root attributes
	if populated {
		stat.Bsize = uint64(attr.Bsize)
		stat.Frsize = uint64(attr.Frsize)
		stat.Blocks = attr.Blocks
		stat.Bavail = attr.Bavail
		stat.Bfree = attr.Bfree
		stat.Files = attr.Files
		stat.Ffree = attr.Ffree
		stat.Namemax = attr.Namemax
	} else {
		var free, total, avail uint64

		// Get path to the cache
		pathPtr, err := windows.UTF16PtrFromString("/")
		if err != nil {
			log.Err("Libfuse::Statfs: Failed to get stats %s [%s]", name, err.Error())
			return -fuse.EIO
		}
		err = windows.GetDiskFreeSpaceEx(pathPtr, &free, &total, &avail)
		if err != nil {
			log.Err("Libfuse::Statfs: Failed to get stats %s [%s]", name, err.Error())
			return -fuse.EIO
		}

		const blockSize = 4096

		stat.Bsize = blockSize
		stat.Frsize = blockSize
		stat.Blocks = free / blockSize
		stat.Bavail = avail / blockSize
		stat.Bfree = free
		stat.Files = 1e9
		stat.Ffree = 1e9
		stat.Namemax = 255
	}

	return 0
}
