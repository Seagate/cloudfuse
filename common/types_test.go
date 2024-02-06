/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2024 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type typesTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *typesTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

func TestGenerateConfig(t *testing.T) {
	suite.Run(t, new(typesTestSuite))
}

func (suite *typesTestSuite) TestBinarySearch() {
	blocksList := []*Block{
		{StartIndex: 0, EndIndex: 4},
		{StartIndex: 4, EndIndex: 7},
		{StartIndex: 7, EndIndex: 12},
	}
	bol := BlockOffsetList{
		BlockList: blocksList,
	}
	found, startingIndex := bol.BinarySearch(5)
	suite.assert.Equal(found, true)
	suite.assert.Equal(startingIndex, 1)

	found, startingIndex = bol.BinarySearch(20)
	suite.assert.Equal(found, false)
	suite.assert.Equal(startingIndex, 3)
}

func (suite *typesTestSuite) TestFindBlocksToModify() {
	blocksList := []*Block{
		{StartIndex: 0, EndIndex: 4},
		{StartIndex: 4, EndIndex: 7},
		{StartIndex: 7, EndIndex: 12},
	}
	bol := BlockOffsetList{
		BlockList: blocksList,
	}
	index, size, largerThanFile, _ := bol.FindBlocksToModify(3, 7)
	suite.assert.Equal(index, 0)
	suite.assert.Equal(size, int64(12))
	suite.assert.Equal(largerThanFile, false)

	index, size, largerThanFile, _ = bol.FindBlocksToModify(8, 10)
	suite.assert.Equal(index, 2)
	suite.assert.Equal(size, int64(5))
	suite.assert.Equal(largerThanFile, true)

	index, size, largerThanFile, appendOnly := bol.FindBlocksToModify(20, 20)
	suite.assert.Equal(size, int64(0))
	suite.assert.Equal(largerThanFile, true)
	suite.assert.Equal(appendOnly, true)
}

func (suite *typesTestSuite) TestDefaultWorkDir() {
	val, err := os.UserHomeDir()
	suite.assert.Nil(err)
	suite.assert.Equal(DefaultWorkDir, JoinUnixFilepath(val, ".cloudfuse"))
	suite.assert.Equal(DefaultLogFilePath, JoinUnixFilepath(val, ".cloudfuse/cloudfuse.log"))
	suite.assert.Equal(StatsConfigFilePath, JoinUnixFilepath(val, ".cloudfuse/stats_monitor.cfg"))
}
