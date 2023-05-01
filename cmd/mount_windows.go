//go:build windows

package cmd

import (
	"context"
	"lyvecloudfuse/internal"
	"os"
)

// Create dummy function so that mount.go code can compile
// This function is used only on Linux, so it creates an empty context here
func createDaemon(pipeline *internal.Pipeline, ctx context.Context, pidFileName string, pidFilePerm os.FileMode, umask int, fname string) error {
	return nil
}
