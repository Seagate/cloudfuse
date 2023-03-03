//go:build linux

package libfuse

import (
	"lyvecloudfuse/common"
	"lyvecloudfuse/common/log"

	"github.com/winfsp/cgofuse/fuse"
	"golang.org/x/sys/unix"
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
		statfs := &unix.Statfs_t{}
		err = unix.Statfs("/", statfs)
		if err != nil {
			log.Err("Libfuse::Statfs: Failed to get stats %s [%s]", name, err.Error())
			return -fuse.EIO
		}

		stat.Bsize = uint64(statfs.Bsize)
		stat.Frsize = uint64(statfs.Frsize)
		stat.Blocks = statfs.Blocks
		stat.Bavail = statfs.Bavail
		stat.Bfree = statfs.Bfree
		stat.Files = statfs.Files
		stat.Ffree = statfs.Ffree
		stat.Namemax = uint64(statfs.Namelen)
	}

	return 0
}
