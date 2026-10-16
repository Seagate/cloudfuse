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

package file_cache

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"

	"github.com/stretchr/testify/suite"
)

type cachePolicyTestSuite struct {
	suite.Suite
}

func (suite *cachePolicyTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
}

func (suite *cachePolicyTestSuite) TestGetUsagePercentage() {
	tests := []struct {
		name      string
		usedBytes float64
		maxSizeMB float64
		want      float64
	}{
		{"empty cache", 0, 4, 0},
		{"quarter full", float64(MB), 4, 25},
		{"above capacity", 5 * float64(MB), 4, 125},
		{"zero max size", float64(MB), 0, 0},
		{"negative max size", float64(MB), -1, 0},
	}
	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.Equal(tt.want, getUsagePercentage(tt.usedBytes, tt.maxSizeMB))
		})
	}
}

func (suite *cachePolicyTestSuite) TestDeleteFile() {
	path := filepath.Join(suite.T().TempDir(), "cached")
	suite.Require().NoError(os.WriteFile(path, []byte("data"), 0o644))

	suite.Require().NoError(deleteFile(path))
	_, err := os.Stat(path)
	suite.ErrorIs(err, fs.ErrNotExist)
}

func (suite *cachePolicyTestSuite) TestDeleteFileMissing() {
	suite.NoError(deleteFile(filepath.Join(suite.T().TempDir(), "missing")))
}

func TestCachePolicyTestSuite(t *testing.T) {
	suite.Run(t, new(cachePolicyTestSuite))
}
