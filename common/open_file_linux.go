//go:build linux

package common

import (
	"os"
)

// OpenFile on linux passes the call to the os. This file is needed so we can
// easily compile the code on both windows and linux.
func OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}
