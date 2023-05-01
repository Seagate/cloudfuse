//go:build windows

package common

// NotifyMountToParent : Does nothing on Windows
func NotifyMountToParent() error {
	return nil
}
