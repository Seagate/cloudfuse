package cmd

import (
	"context"
	"lyvecloudfuse/internal"
	"os"

	"github.com/sevlyar/go-daemon"
)

// Create dummy function so that mount.go code can compile
// This function is used only on Linux, so it creates an empty context here
func createDaemon(pipeline *internal.Pipeline, ctx context.Context, pidFileName string, pidFilePerm os.FileMode, umask int) *daemon.Context {
	dmnCtx := &daemon.Context{}
	return dmnCtx
}
