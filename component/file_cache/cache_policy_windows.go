//go:build windows

package file_cache

import (
	"os"
	"path/filepath"
)

// totalSectors walks through all files in the path and gives an estimate of the total number of sectors
// that are being used. Based on https://stackoverflow.com/questions/32482673/how-to-get-directory-total-size
func totalSectors(path string) int64 {
	//bytes per sector is hard coded to 4096 bytes since syscall to windows and BytesPerSector for the drive in question is an estimate.
	// https://devblogs.microsoft.com/oldnewthing/20160427-00/?p=93365

	var totalSectors int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSectors += (info.Size() / sectorSize)
			if info.Size()%sectorSize != 0 {
				totalSectors++
			}
		}
		return err
	})

	// TODO: Handle this error properly
	if err != nil {
		return totalSectors
	}

	return totalSectors

}

// getUsage providse an estimate of the size on disk in MB for provided directory path string
func getUsage(path string) float64 {
	totalSectors := totalSectors(path)

	totalBytes := float64(totalSectors * sectorSize)
	totalBytes = totalBytes / MB

	return totalBytes
}
