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

package custom

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type customTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *customTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	// err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	// suite.assert.NoError(err)
}

// This test builds a custom component and then tries to load the .so file
// Though both .so file and tests are being built using same go packages, it still gives an error in loading .so file
//
//	"plugin was built with a different version of package"
//
// Hence, this test is disabled for now.
// Same .so file loads fine when cloudfuse is run from CLI.
// If you wish to debug cloudfuse with a custom component then always build .so file with "-gcflags=all=-N -l" option
//
//	e.g. go build -buildmode=plugin -gcflags=all=-N -l -o x.so <path to source>
//
// This flag disables all optimizations and inline replacements and then .so will load in debug mode as well.
// However same .so will not work with cli mount and there you need to build .so without these flags.
func (suite *customTestSuite) _TestInitializePluginsValidPath() {
	// Direct paths to the Go plugin source files
	source1 := "../../test/sample_custom_component1/main.go"
	source2 := "../../test/sample_custom_component2/main.go"

	// Paths to the compiled .so files in the current directory
	plugin1 := "./sample_custom_component1.so"
	plugin2 := "./sample_custom_component2.so"

	// Compile the Go plugin source files into .so files
	cmd := exec.Command(
		"go",
		"build",
		"-buildmode=plugin",
		"-gcflags=all=-N -l",
		"-o",
		plugin1,
		source1,
	)
	err := cmd.Run()
	suite.assert.NoError(err)
	cmd = exec.Command(
		"go",
		"build",
		"-buildmode=plugin",
		"-gcflags=all=-N -l",
		"-o",
		plugin2,
		source2,
	)
	err = cmd.Run()
	suite.assert.NoError(err)

	os.Setenv("CLOUDFUSE_PLUGIN_PATH", plugin1+":"+plugin2)

	err = initializePlugins()
	suite.assert.NoError(err)

	// Clean up the generated .so files
	os.Remove(plugin1)
	os.Remove(plugin2)
}

func (suite *customTestSuite) TestInitializePluginsInvalidPath() {
	dummyPath := "/invalid/path/plugin1.so"
	os.Setenv("CLOUDFUSE_PLUGIN_PATH", dummyPath)

	err := initializePlugins()
	suite.assert.Error(err)
}

func (suite *customTestSuite) TestInitializePluginsEmptyPath() {
	os.Setenv("CLOUDFUSE_PLUGIN_PATH", "")

	err := initializePlugins()
	suite.assert.NoError(err)
}

func TestCustomSuite(t *testing.T) {
	suite.Run(t, new(customTestSuite))
}
