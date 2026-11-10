//go:build windows

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2026 Seagate Technology LLC and/or its Affiliates

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

package common

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/mem"
	"golang.org/x/sys/windows"
)

// NotifyMountToParent : Does nothing on Windows
func NotifyMountToParent() error {
	return nil
}

// totalSectors walks through all files in the path and gives an estimate of the total number of sectors
// that are being used. Based on https://stackoverflow.com/questions/32482673/how-to-get-directory-total-size
func totalSectors(path string) (int64, error) {
	//bytes per sector is hard coded to 4096 bytes since syscall to windows and BytesPerSector for the drive in question is an estimate.
	// https://devblogs.microsoft.com/oldnewthing/20160427-00/?p=93365

	var totalSectors int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSectors += (info.Size() / SectorSize)
			if info.Size()%SectorSize != 0 {
				totalSectors++
			}
		}
		return err
	})

	if err != nil {
		return totalSectors, err
	}

	return totalSectors, nil

}

// GetUsage: The current disk usage in bytes
func GetUsage(path string) (float64, error) {
	sectors, err := totalSectors(path)
	if err != nil {
		return 0, fmt.Errorf("failed to measure %s: %w", path, err)
	}

	totalBytes := float64(sectors * SectorSize)

	return totalBytes, nil
}

// GetDiskUsageFromStatfs: Current disk usage of temp path
func GetDiskUsageFromStatfs(path string) (float64, float64, error) {
	// We need to compute the disk usage percentage for the temp path
	var free, total, avail uint64

	// Get path to the cache
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		panic(err)
	}
	err = windows.GetDiskFreeSpaceEx(pathPtr, &free, &total, &avail)
	if err != nil {
		return 0, 0, err
	}

	usedSpace := float64(total - avail)
	return usedSpace, float64(usedSpace) / float64(total) * 100, nil
}

// GetAvailFree returns the bytes available to the caller, honoring per-user
// quotas, and the total free bytes on the volume containing path.
func GetAvailFree(path string) (avail, free uint64, err error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var total uint64
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &avail, &total, &free); err != nil {
		return 0, 0, err
	}
	return avail, free, nil
}

// GetFileTimes returns the last access and creation times of a local file.
// The modification time is available portably from info.ModTime.
func GetFileTimes(info fs.FileInfo) (atime, ctime time.Time) {
	stat, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return info.ModTime(), info.ModTime()
	}
	return time.Unix(0, stat.LastAccessTime.Nanoseconds()),
		time.Unix(0, stat.CreationTime.Nanoseconds())
}

// GetFileOwner reports no owner: Windows files carry no POSIX uid or gid.
func GetFileOwner(info fs.FileInfo) (uid, gid int, ok bool) {
	return 0, 0, false
}

// IsProtectedSystemFile reports whether info is marked both hidden and
// system, which Explorer treats as a protected operating system file.
func IsProtectedSystemFile(info fs.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return false
	}
	const protected = windows.FILE_ATTRIBUTE_HIDDEN | windows.FILE_ATTRIBUTE_SYSTEM
	return stat.FileAttributes&protected == protected
}

// GetFreeRam: Available ram
func GetFreeRam() (uint64, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	return v.Available, nil
}

// List all mount points which were mounted using cloudfuse
func ListMountPoints() ([]string, error) {
	out, err := exec.Command(`C:\Program Files (x86)\WinFsp\bin\launchctl-x64.exe`, "list").Output()
	if err != nil {
		fmt.Printf("Is WinFSP installed? 'launchctl-x64.exe list' failed with error: %v\n", err)
		return nil, err
	}
	var mntList []string
	outList := strings.SplitSeq(string(out), "\n")
	for item := range outList {
		if strings.HasPrefix(item, "cloudfuse") {
			// Extract the mount path from this line
			mntPath := strings.Split(item, " ")[1]
			mntList = append(mntList, mntPath)
		}
	}
	return mntList, nil
}
