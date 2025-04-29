/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.

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

package file_cache

import (
	"fmt"
	"os"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal/stats_manager"
)

const DefaultEvictTime = 10

type cachePolicyConfig struct {
	tmpPath      string // uses os.Separator (filepath.Join)
	cacheTimeout uint32
	maxEviction  uint32

	maxSizeMB     float64
	highThreshold float64
	lowThreshold  float64

	fileLocks *common.LockMap // uses object name (common.JoinUnixFilepath)

	policyTrace bool
}

type cachePolicy interface {
	StartPolicy() error
	ShutdownPolicy() error

	UpdateConfig(cachePolicyConfig) error

	CacheValid(name string)                            // Mark the file as hit
	CachePurge(name string, flock *common.LockMapItem) // Delete the file from cache

	IsCached(name string) bool // Whether or not the cache policy considers this file cached

	Name() string // The name of the policy
}

// getUsagePercentage:  The current cache usage as a percentage of the maxSize
func getUsagePercentage(path string, maxSize float64) float64 {
	var currSize float64
	var usagePercent float64
	var err error

	if maxSize == 0 {
		currSize, usagePercent, err = common.GetDiskUsageFromStatfs(path)
		if err != nil {
			log.Err(
				"cachePolicy::getUsagePercentage : failed to get disk usage for %s [%v]",
				path,
				err.Error,
			)
		}
	} else {
		// We need to compute % usage of temp directory against configured limit
		currSize, err = common.GetUsage(path)
		if err != nil {
			log.Err("cachePolicy::getUsagePercentage : failed to get directory usage for %s [%v]", path, err.Error)
		}

		usagePercent = (currSize / float64(maxSize)) * 100
	}

	log.Debug("cachePolicy::getUsagePercentage : current cache usage : %f%%", usagePercent)

	fileCacheStatsCollector.UpdateStats(
		stats_manager.Replace,
		cacheUsage,
		fmt.Sprintf("%f MB", currSize),
	)
	fileCacheStatsCollector.UpdateStats(
		stats_manager.Replace,
		usgPer,
		fmt.Sprintf("%f%%", usagePercent),
	)

	return usagePercent
}

// Delete a given file
func deleteFile(name string) error {
	log.Debug("cachePolicy::deleteFile : attempting to delete %s", name)

	err := os.Remove(name)
	if err != nil && os.IsPermission(err) {
		// File is not having delete permissions so change the mode and retry deletion
		log.Warn("cachePolicy::deleteFile : failed to delete %s due to permission", name)

		err = os.Chmod(name, os.FileMode(0666))
		if err != nil {
			log.Err("cachePolicy::deleteFile : %s failed to reset permissions", name)
			return err
		}

		err = os.Remove(name)
	} else if err != nil && os.IsNotExist(err) {
		log.Debug("cachePolicy::deleteFile : %s does not exist in local cache", name)
		return nil
	}

	if err != nil {
		log.Err("cachePolicy::DeleteItem : Failed to delete local file %s [%v]", name, err.Error())
		return err
	}

	return nil
}
