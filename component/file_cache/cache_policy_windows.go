//go:build windows

package file_cache

import (
	"os"
	"path/filepath"
)

/*
description:

	walk through all the files in the directorhy and sub directories and accumilate / calculate the total number of sectors being used.
	this block of code is based on the solution provided in the following stack overflow link:
		https://stackoverflow.com/questions/32482673/how-to-get-directory-total-size

input:

	string directory path

output:

	two values are returned:
		1. int64 representing number of sectors used in the directory path.
		2. int64 representing number of bytes per sector
*/
func totalSectors(path string) (int64, int64) {

	/*
		bytes per sector is hard coded to 4096 bytes since syscall to windows and BytesPerSector for the drive in question is an estimate.
		source: https://devblogs.microsoft.com/oldnewthing/20160427-00/?p=93365
	*/

	var totalSectors int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSectors += (info.Size() / sectorSize)
			if info.Size()%sectorSize != 0 {
				totalSectors += 1
			}
		}
		return err
	})
	return totalSectors

}

/*
description:

	provide an estimate size on disk in MB for provided directory path string

input:

	string directory path

output:

	float64 value representing the size on disk in MB.
*/
func getUsage(path string) float64 {

	totalSectors := totalSectors(path)

	totalBytes := float64(totalSectors * sectorSize)
	totalBytes = totalBytes / MB

	return totalBytes

}
