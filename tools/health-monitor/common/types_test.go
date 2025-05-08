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

package common

import (
	"os"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type typesHMTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *typesHMTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

func TestGenerateConfig(t *testing.T) {
	suite.Run(t, new(typesHMTestSuite))
}

func (suite *typesHMTestSuite) TestDefaultWorkDir() {
	val, err := os.UserHomeDir()
	suite.assert.NoError(err)
	suite.assert.Equal(DefaultWorkDir, common.JoinUnixFilepath(val, ".cloudfuse"))
	suite.assert.Equal(DefaultLogFile, common.JoinUnixFilepath(val, ".cloudfuse/CfuseMonitor.log"))
}
