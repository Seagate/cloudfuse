/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2026 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.

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

package tiered_storage

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type lruPolicyTestSuite struct {
	suite.Suite
	assert *assert.Assertions
	policy *lruQueue
}

var cache_path = filepath.Join(home_dir, "file_cache"+randomString(8))

func (suite *lruPolicyTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
	suite.assert = assert.New(suite.T())

	err = os.Mkdir(cache_path, fs.FileMode(0777))
	suite.assert.NoError(err)

	suite.setupTestHelper(cache_path, 1024, 0.8, 0.6, 2)
}

func (suite *lruPolicyTestSuite) TeardownTest() {
	err := suite.policy.StopPolicy()
	suite.assert.NoError(err)

	err = os.RemoveAll(cache_path)
	suite.assert.NoError(err)
}

// setupTestHelper creates and starts an lruQueue for testing.
// cachePath: where cached files live
// maxCacheMB: max cache size in MB (converted to bytes internally)
// threshold: eviction triggers above this ratio (e.g. 0.8 = 80% full)
// targetRatio: evict down to this ratio (e.g. 0.6 = 60% full)
// numWorkers: number of upload worker goroutines
func (suite *lruPolicyTestSuite) setupTestHelper(
	cachePath string, maxCacheMB float64, threshold float64, targetRatio float64, numWorkers int,
) {
	suite.policy = &lruQueue{
		cachePath:    cachePath,
		maxCacheSize: maxCacheMB * 1024 * 1024, // convert MB to bytes
		threshold:    threshold,
		targetRatio:  targetRatio,
		numWorkers:   numWorkers,

		// Stub: tests don't upload to a real backend
		upload: func(name string) error {
			return nil
		},
		// Stub: assume no file handles are open during tests
		FileHasOpenFileHandle: func(name string) bool {
			return false
		},
	}

	err := suite.policy.StartPolicy()
	suite.assert.NoError(err)
}
