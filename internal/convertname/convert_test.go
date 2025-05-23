/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates

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

package convertname

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type conversionTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *conversionTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

func (suite *conversionTestSuite) TestCloudToFile() {
	filename := "test\"*:<>?|"
	cloudname := "test＂＊：＜＞？｜"
	result := WindowsCloudToFile(filename)
	suite.assert.Equal(cloudname, result)
}

func (suite *conversionTestSuite) TestFileToCloud() {
	filename := "test＂＊：＜＞？｜"
	cloudname := "test\"*:<>?|"
	result := WindowsFileToCloud(filename)
	suite.assert.Equal(cloudname, result)
}

func TestConversionTestSuite(t *testing.T) {
	suite.Run(t, new(conversionTestSuite))
}
