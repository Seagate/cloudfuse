/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.

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

package libfuse

import (
	"io/fs"
	"testing"

	"github.com/Seagate/cloudfuse/common"

	"github.com/stretchr/testify/suite"
)

// Tests the default configuration of libfuse
func (suite *libfuseTestSuite) TestDefault() {
	defer suite.cleanupTest()
	suite.assert.Equal("libfuse", suite.libfuse.Name())
	suite.assert.Empty(suite.libfuse.mountPath)
	suite.assert.False(suite.libfuse.readOnly)
	suite.assert.False(suite.libfuse.traceEnable)
	suite.assert.False(suite.libfuse.allowOther)
	suite.assert.False(suite.libfuse.networkShare)
	suite.assert.False(suite.libfuse.allowRoot)
	suite.assert.Equal(common.DefaultDirectoryPermissionBits, suite.libfuse.dirPermission)
	suite.assert.Equal(common.DefaultFilePermissionBits, suite.libfuse.filePermission)
	suite.assert.Equal(uint32(120), suite.libfuse.entryExpiration)
	suite.assert.Equal(uint32(120), suite.libfuse.attributeExpiration)
	suite.assert.Equal(uint32(120), suite.libfuse.negativeTimeout)
	suite.assert.Equal(uint64(1024*1024), suite.libfuse.displayCapacityMb)
	suite.assert.False(suite.libfuse.disableWritebackCache)
	suite.assert.True(suite.libfuse.ignoreOpenFlags)
	suite.assert.False(suite.libfuse.directIO)
}

func (suite *libfuseTestSuite) TestConfig() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "allow-other: true\nread-only: true\nlibfuse:\n  attribute-expiration-sec: 60\n  entry-expiration-sec: 60\n  negative-entry-expiration-sec: 60\n  fuse-trace: true\n  disable-writeback-cache: true\n  ignore-open-flags: false\n  direct-io: true\n  network-share: true\n  display-capacity-mb: 262144\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal("libfuse", suite.libfuse.Name())
	suite.assert.Empty(suite.libfuse.mountPath)
	suite.assert.True(suite.libfuse.readOnly)
	suite.assert.True(suite.libfuse.traceEnable)
	suite.assert.True(suite.libfuse.disableWritebackCache)
	suite.assert.False(suite.libfuse.ignoreOpenFlags)
	suite.assert.True(suite.libfuse.allowOther)
	suite.assert.True(suite.libfuse.networkShare)
	suite.assert.False(suite.libfuse.allowRoot)
	suite.assert.Equal(fs.FileMode(0o777), suite.libfuse.dirPermission)
	suite.assert.Equal(fs.FileMode(0o777), suite.libfuse.filePermission)
	suite.assert.Equal(uint32(60), suite.libfuse.entryExpiration)
	suite.assert.Equal(uint32(60), suite.libfuse.attributeExpiration)
	suite.assert.Equal(uint32(60), suite.libfuse.negativeTimeout)
	suite.assert.Equal(uint64(262144), suite.libfuse.displayCapacityMb)
	suite.assert.True(suite.libfuse.directIO)
}

func (suite *libfuseTestSuite) TestConfigZero() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "read-only: true\nlibfuse:\n  attribute-expiration-sec: 0\n  entry-expiration-sec: 0\n  negative-entry-expiration-sec: 0\n  fuse-trace: true\n  direct-io: false\n  display-capacity-mb: 0\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal("libfuse", suite.libfuse.Name())
	suite.assert.Empty(suite.libfuse.mountPath)
	suite.assert.True(suite.libfuse.readOnly)
	suite.assert.True(suite.libfuse.traceEnable)
	suite.assert.False(suite.libfuse.allowOther)
	suite.assert.False(suite.libfuse.networkShare)
	suite.assert.False(suite.libfuse.allowRoot)
	suite.assert.Equal(fs.FileMode(0o775), suite.libfuse.dirPermission)
	suite.assert.Equal(fs.FileMode(0o755), suite.libfuse.filePermission)
	suite.assert.Equal(uint32(0), suite.libfuse.entryExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.attributeExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.negativeTimeout)
	suite.assert.Equal(uint64(1024*1024), suite.libfuse.displayCapacityMb)
	suite.assert.False(suite.libfuse.directIO)
}

func (suite *libfuseTestSuite) TestConfigDefaultPermission() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "read-only: true\nlibfuse:\n  default-permission: 0555\n  attribute-expiration-sec: 0\n  entry-expiration-sec: 0\n  negative-entry-expiration-sec: 0\n  fuse-trace: true\n  direct-io: true\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal("libfuse", suite.libfuse.Name())
	suite.assert.Empty(suite.libfuse.mountPath)
	suite.assert.True(suite.libfuse.readOnly)
	suite.assert.True(suite.libfuse.traceEnable)
	suite.assert.False(suite.libfuse.allowOther)
	suite.assert.False(suite.libfuse.networkShare)
	suite.assert.False(suite.libfuse.allowRoot)
	suite.assert.Equal(fs.FileMode(0o555), suite.libfuse.dirPermission)
	suite.assert.Equal(fs.FileMode(0o555), suite.libfuse.filePermission)
	suite.assert.Equal(uint32(0), suite.libfuse.entryExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.attributeExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.negativeTimeout)
	suite.assert.True(suite.libfuse.directIO)
}

func (suite *libfuseTestSuite) TestDisableWritebackCache() {
	defer suite.cleanupTest()
	suite.assert.False(suite.libfuse.disableWritebackCache)

	suite.cleanupTest() // clean up the default libfuse generated
	config := "libfuse:\n  disable-writeback-cache: true\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.True(suite.libfuse.disableWritebackCache)

	suite.cleanupTest() // clean up the default libfuse generated
	config = "libfuse:\n  disable-writeback-cache: false\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.False(suite.libfuse.disableWritebackCache)
}

func (suite *libfuseTestSuite) TestIgnoreAppendFlag() {
	defer suite.cleanupTest()
	suite.assert.True(suite.libfuse.ignoreOpenFlags)

	suite.cleanupTest() // clean up the default libfuse generated
	config := "libfuse:\n  ignore-open-flags: false\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.False(suite.libfuse.ignoreOpenFlags)

	suite.cleanupTest() // clean up the default libfuse generated
	config = "libfuse:\n  ignore-open-flags: true\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.True(suite.libfuse.ignoreOpenFlags)
}

// getattr

func (suite *libfuseTestSuite) TestMkDir() {
	testMkDir(suite)
}

func (suite *libfuseTestSuite) TestMkDirError() {
	testMkDirError(suite)
}

func (suite *libfuseTestSuite) TestMkDirErrorAttrExist() {
	testMkDirErrorAttrExist(suite)
}

// readdir

func (suite *libfuseTestSuite) TestRmDir() {
	testRmDir(suite)
}

func (suite *libfuseTestSuite) TestRmDirNotEmpty() {
	testRmDirNotEmpty(suite)
}

func (suite *libfuseTestSuite) TestRmDirError() {
	testRmDirError(suite)
}

func (suite *libfuseTestSuite) TestCreate() {
	testCreate(suite)
}

func (suite *libfuseTestSuite) TestCreateError() {
	testCreateError(suite)
}

func (suite *libfuseTestSuite) TestOpen() {
	testOpen(suite)
}

func (suite *libfuseTestSuite) TestOpenAppendFlagDefault() {
	testOpenAppendFlagDefault(suite)
}

func (suite *libfuseTestSuite) TestOpenAppendFlagDisableWritebackCache() {
	testOpenAppendFlagDisableWritebackCache(suite)
}

func (suite *libfuseTestSuite) TestOpenAppendFlagIgnoreAppendFlag() {
	testOpenAppendFlagIgnoreAppendFlag(suite)
}

func (suite *libfuseTestSuite) TestOpenNotExists() {
	testOpenNotExists(suite)
}

func (suite *libfuseTestSuite) TestOpenError() {
	testOpenError(suite)
}

// read

// write

// flush

func (suite *libfuseTestSuite) TestTruncate() {
	testTruncate(suite)
}

func (suite *libfuseTestSuite) TestTruncateError() {
	testTruncateError(suite)
}

// release

func (suite *libfuseTestSuite) TestUnlink() {
	testUnlink(suite)
}

func (suite *libfuseTestSuite) TestUnlinkNotExists() {
	testUnlinkNotExists(suite)
}

func (suite *libfuseTestSuite) TestUnlinkError() {
	testUnlinkError(suite)
}

// rename

func (suite *libfuseTestSuite) TestSymlink() {
	testSymlink(suite)
}

func (suite *libfuseTestSuite) TestSymlinkError() {
	testSymlinkError(suite)
}

func (suite *libfuseTestSuite) TestReadLink() {
	testReadLink(suite)
}

func (suite *libfuseTestSuite) TestReadLinkNotExists() {
	testReadLinkNotExists(suite)
}

func (suite *libfuseTestSuite) TestReadLinkError() {
	testReadLinkError(suite)
}

func (suite *libfuseTestSuite) TestFsync() {
	testFsync(suite)
}

func (suite *libfuseTestSuite) TestFsyncHandleError() {
	testFsyncHandleError(suite)
}

func (suite *libfuseTestSuite) TestFsyncError() {
	testFsyncError(suite)
}

func (suite *libfuseTestSuite) TestFsyncDir() {
	testFsyncDir(suite)
}

func (suite *libfuseTestSuite) TestFsyncDirError() {
	testFsyncDirError(suite)
}

func (suite *libfuseTestSuite) TestChmod() {
	testChmod(suite)
}

func (suite *libfuseTestSuite) TestChmodNotExists() {
	testChmodNotExists(suite)
}

func (suite *libfuseTestSuite) TestStatFs() {
	testStatFs(suite)
}

func (suite *libfuseTestSuite) TestStatFsNotPopulated() {
	testStatFsNotPopulated(suite)
}

func (suite *libfuseTestSuite) TestStatFsError() {
	testStatFsError(suite)
}

func (suite *libfuseTestSuite) TestChmodError() {
	testChmodError(suite)
}

func (suite *libfuseTestSuite) TestChown() {
	testChown(suite)
}

func (suite *libfuseTestSuite) TestUtimens() {
	testUtimens(suite)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestLibfuseTestSuite(t *testing.T) {
	suite.Run(t, new(libfuseTestSuite))
}
