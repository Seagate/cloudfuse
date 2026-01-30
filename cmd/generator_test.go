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

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type generatorTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *generatorTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

func (suite *generatorTestSuite) cleanupTest() {
	resetCLIFlags(*generateCmd)
}

func TestGeneratorCommand(t *testing.T) {
	suite.Run(t, new(generatorTestSuite))
}

// TestGeneratorRequiresArg tests that generate command requires exactly one argument
func (suite *generatorTestSuite) TestGeneratorRequiresArg() {
	defer suite.cleanupTest()

	output, _ := executeCommandC(rootCmd, "generate")
	suite.assert.Contains(output, "accepts 1 arg(s)")
}

// TestGeneratorIsHidden tests that the generate command is hidden
func (suite *generatorTestSuite) TestGeneratorIsHidden() {
	defer suite.cleanupTest()

	suite.assert.True(generateCmd.Hidden, "generate command should be hidden")
}

// TestGeneratorHelp tests that help is displayed correctly
func (suite *generatorTestSuite) TestGeneratorHelp() {
	defer suite.cleanupTest()

	output, _ := executeCommandC(rootCmd, "generate", "--help")
	suite.assert.Contains(output, "Generate a new cloudfuse component")
	suite.assert.Contains(output, "cloudfuse generate mycomponent")
}
