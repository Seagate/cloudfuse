//go:build linux

package common

import (
	"fmt"
	"syscall"
)

// NotifyMountToParent : Send a signal to parent process about successful mount
func NotifyMountToParent() error {
	if !ForegroundMount {
		ppid := syscall.Getppid()
		if ppid > 1 {
			if err := syscall.Kill(ppid, syscall.SIGUSR2); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("failed to get parent pid, received : %v", ppid)
		}
	}

	return nil
}
