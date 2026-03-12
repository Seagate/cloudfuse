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

package libfuse

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"

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
	suite.assert.Equal(suite.libfuse.dirPermission, uint(common.DefaultDirectoryPermissionBits))
	suite.assert.Equal(suite.libfuse.filePermission, uint(common.DefaultFilePermissionBits))
	suite.assert.Equal(uint32(120), suite.libfuse.entryExpiration)
	suite.assert.Equal(uint32(120), suite.libfuse.attributeExpiration)
	suite.assert.Equal(uint32(120), suite.libfuse.negativeTimeout)
	suite.assert.Equal(uint64(1024*1024*1024), suite.libfuse.displayCapacityMb)
	suite.assert.False(suite.libfuse.disableWritebackCache)
	suite.assert.True(suite.libfuse.ignoreOpenFlags)
	suite.assert.False(suite.libfuse.directIO)
}

func (suite *libfuseTestSuite) TestConfig() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "allow-other: true\nread-only: true\nlibfuse:\n  attribute-expiration-sec: 60\n  entry-expiration-sec: 60\n  negative-entry-expiration-sec: 60\n  fuse-trace: true\n  disable-writeback-cache: true\n  ignore-open-flags: false\n  network-share: true\n  display-capacity-mb: 262144\n"
	suite.setupTestHelper(
		config,
	) // setup a new libfuse with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal("libfuse", suite.libfuse.Name())
	suite.assert.Empty(suite.libfuse.mountPath)
	suite.assert.True(suite.libfuse.readOnly)
	// trace should only be enabled when mounted in foreground otherwise we don't honor the option
	suite.assert.False(suite.libfuse.traceEnable)
	suite.assert.True(suite.libfuse.disableWritebackCache)
	suite.assert.False(suite.libfuse.ignoreOpenFlags)
	suite.assert.True(suite.libfuse.allowOther)
	suite.assert.True(suite.libfuse.networkShare)
	suite.assert.False(suite.libfuse.allowRoot)
	suite.assert.Equal(suite.libfuse.dirPermission, uint(fs.FileMode(0777)))
	suite.assert.Equal(suite.libfuse.filePermission, uint(fs.FileMode(0777)))
	suite.assert.Equal(uint32(60), suite.libfuse.entryExpiration)
	suite.assert.Equal(uint32(60), suite.libfuse.attributeExpiration)
	suite.assert.Equal(uint32(60), suite.libfuse.negativeTimeout)
	suite.assert.Equal(uint64(262144), suite.libfuse.displayCapacityMb)
	suite.assert.False(suite.libfuse.directIO)
}

func (suite *libfuseTestSuite) TestGenConfigDirectIO() {
	defer suite.cleanupTest()
	suite.cleanupTest()

	err := config.ReadConfigFromReader(strings.NewReader("direct-io: true\n"))
	suite.assert.NoError(err)

	gen := suite.libfuse.GenConfig()
	suite.assert.Contains(gen, "attribute-expiration-sec: 0")
	suite.assert.Contains(gen, "entry-expiration-sec: 0")
	suite.assert.Contains(gen, "negative-entry-expiration-sec: 0")
}

func (suite *libfuseTestSuite) TestGenConfigDefault() {
	defer suite.cleanupTest()
	suite.cleanupTest()

	err := config.ReadConfigFromReader(strings.NewReader("direct-io: false\n"))
	suite.assert.NoError(err)

	gen := suite.libfuse.GenConfig()
	suite.assert.Contains(gen, "attribute-expiration-sec: 120")
	suite.assert.Contains(gen, "entry-expiration-sec: 120")
	suite.assert.Contains(gen, "negative-entry-expiration-sec: 120")
}

func (suite *libfuseTestSuite) TestConfigDirectIO() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "allow-other: true\nread-only: true\nlibfuse:\n  attribute-expiration-sec: 60\n  entry-expiration-sec: 60\n  negative-entry-expiration-sec: 60\n  fuse-trace: true\n  disable-writeback-cache: true\n  ignore-open-flags: false\n  direct-io: true\n  network-share: true\n  display-capacity-mb: 262144\n"
	suite.setupTestHelper(
		config,
	) // setup a new libfuse with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal("libfuse", suite.libfuse.Name())
	suite.assert.Empty(suite.libfuse.mountPath)
	suite.assert.True(suite.libfuse.readOnly)
	suite.assert.False(suite.libfuse.traceEnable)
	suite.assert.True(suite.libfuse.disableWritebackCache)
	suite.assert.False(suite.libfuse.ignoreOpenFlags)
	suite.assert.True(suite.libfuse.allowOther)
	suite.assert.True(suite.libfuse.networkShare)
	suite.assert.False(suite.libfuse.allowRoot)
	suite.assert.Equal(suite.libfuse.dirPermission, uint(fs.FileMode(0777)))
	suite.assert.Equal(suite.libfuse.filePermission, uint(fs.FileMode(0777)))
	suite.assert.Equal(uint32(0), suite.libfuse.entryExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.attributeExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.negativeTimeout)
	suite.assert.Equal(uint64(262144), suite.libfuse.displayCapacityMb)
	suite.assert.True(suite.libfuse.directIO)
}

func (suite *libfuseTestSuite) TestConfigZero() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "read-only: true\nlibfuse:\n  attribute-expiration-sec: 0\n  entry-expiration-sec: 0\n  negative-entry-expiration-sec: 0\n  fuse-trace: true\n  direct-io: false\n  display-capacity-mb: 0\n"
	suite.setupTestHelper(
		config,
	) // setup a new libfuse with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal("libfuse", suite.libfuse.Name())
	suite.assert.Empty(suite.libfuse.mountPath)
	suite.assert.True(suite.libfuse.readOnly)
	// trace should only be enabled when mounted in foreground otherwise we don't honor the option
	suite.assert.False(suite.libfuse.traceEnable)
	suite.assert.False(suite.libfuse.allowOther)
	suite.assert.False(suite.libfuse.networkShare)
	suite.assert.False(suite.libfuse.allowRoot)
	suite.assert.Equal(suite.libfuse.dirPermission, uint(fs.FileMode(0775)))
	suite.assert.Equal(suite.libfuse.filePermission, uint(fs.FileMode(0755)))
	suite.assert.Equal(uint32(0), suite.libfuse.entryExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.attributeExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.negativeTimeout)
	suite.assert.Equal(uint64(1024*1024*1024), suite.libfuse.displayCapacityMb)
	suite.assert.False(suite.libfuse.directIO)
}

func (suite *libfuseTestSuite) TestConfigDefaultPermission() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "read-only: true\nlibfuse:\n  default-permission: 0555\n  attribute-expiration-sec: 0\n  entry-expiration-sec: 0\n  negative-entry-expiration-sec: 0\n  fuse-trace: true\n  direct-io: true\n"
	suite.setupTestHelper(
		config,
	) // setup a new libfuse with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal("libfuse", suite.libfuse.Name())
	suite.assert.Empty(suite.libfuse.mountPath)
	suite.assert.True(suite.libfuse.readOnly)
	// trace should only be enabled when mounted in foreground otherwise we don't honor the option
	suite.assert.False(suite.libfuse.traceEnable)
	suite.assert.False(suite.libfuse.allowOther)
	suite.assert.False(suite.libfuse.networkShare)
	suite.assert.False(suite.libfuse.allowRoot)
	suite.assert.Equal(suite.libfuse.dirPermission, uint(fs.FileMode(0555)))
	suite.assert.Equal(suite.libfuse.filePermission, uint(fs.FileMode(0555)))
	suite.assert.Equal(uint32(0), suite.libfuse.entryExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.attributeExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.negativeTimeout)
	suite.assert.True(suite.libfuse.directIO)
}

func (suite *libfuseTestSuite) TestConfigRootAndThreads() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "allow-root: true\nnonempty: true\nlibfuse:\n  max-fuse-threads: 256\n  umask: 022\n  uid: 1001\n  gid: 1002\n"
	suite.setupTestHelper(config)

	suite.assert.True(suite.libfuse.allowRoot)
	suite.assert.True(suite.libfuse.nonEmptyMount)
	suite.assert.Equal(uint32(256), suite.libfuse.maxFuseThreads)
	suite.assert.Equal(uint32(0o22), suite.libfuse.umask)
	suite.assert.Equal(uint32(1001), suite.libfuse.ownerUID)
	suite.assert.Equal(uint32(1002), suite.libfuse.ownerGID)
}

func (suite *libfuseTestSuite) TestConfigDisableKernelCache() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "read-only: true\ndisable-kernel-cache: true\n\n"
	suite.setupTestHelper(
		config,
	) // setup a new libfuse with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal("libfuse", suite.libfuse.Name())
	suite.assert.Empty(suite.libfuse.mountPath)
	suite.assert.Equal(uint32(0), suite.libfuse.entryExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.attributeExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.negativeTimeout)
	suite.assert.True(suite.libfuse.directIO)
}

func (suite *libfuseTestSuite) TestConfigFuseTraceEnable() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "foreground: true\nlibfuse:\n  fuse-trace: true\n"

	// Foreground mount option is global config option which is exported to others using a global variable.
	// Hence setting the option before starting the test.
	common.ForegroundMount = true
	suite.setupTestHelper(
		config,
	) // setup a new libfuse with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal("libfuse", suite.libfuse.Name())
	suite.assert.Empty(suite.libfuse.mountPath)
	// Fuse trace should work as we are mounting using foreground option.
	suite.assert.True(suite.libfuse.traceEnable)
	common.ForegroundMount = false
}

func (suite *libfuseTestSuite) TestDisableWritebackCache() {
	defer suite.cleanupTest()
	suite.assert.False(suite.libfuse.disableWritebackCache)

	suite.cleanupTest() // clean up the default libfuse generated
	config := "libfuse:\n  disable-writeback-cache: true\n"
	suite.setupTestHelper(
		config,
	) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.True(suite.libfuse.disableWritebackCache)

	suite.cleanupTest() // clean up the default libfuse generated
	config = "libfuse:\n  disable-writeback-cache: false\n"
	suite.setupTestHelper(
		config,
	) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.False(suite.libfuse.disableWritebackCache)
}

func (suite *libfuseTestSuite) TestIgnoreAppendFlag() {
	defer suite.cleanupTest()
	suite.assert.True(suite.libfuse.ignoreOpenFlags)

	suite.cleanupTest() // clean up the default libfuse generated
	config := "libfuse:\n  ignore-open-flags: false\n"
	suite.setupTestHelper(
		config,
	) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.False(suite.libfuse.ignoreOpenFlags)

	suite.cleanupTest() // clean up the default libfuse generated
	config = "libfuse:\n  ignore-open-flags: true\n"
	suite.setupTestHelper(
		config,
	) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.True(suite.libfuse.ignoreOpenFlags)
}

func (suite *libfuseTestSuite) TestTrimFusePath() {
	testTrimFusePath(suite)
}

func (suite *libfuseTestSuite) TestNewCgofuseFS() {
	testNewCgofuseFS(suite)
}

func (suite *libfuseTestSuite) TestGetAttrRoot() {
	testGetAttrRoot(suite)
}

func (suite *libfuseTestSuite) TestGetAttrIgnoredFile() {
	testGetAttrIgnoredFile(suite)
}

func (suite *libfuseTestSuite) TestGetAttrErrors() {
	testGetAttrErrors(suite)
}

func (suite *libfuseTestSuite) TestFuseErrnoFromError() {
	testFuseErrnoFromError(suite)
}

// getattr

func (suite *libfuseTestSuite) TestMkDir() {
	testMkDir(suite)
}

func (suite *libfuseTestSuite) TestMkDirError() {
	testMkDirError(suite)
}

func (suite *libfuseTestSuite) TestMkDirErrorPermission() {
	testMkDirErrorPermission(suite)
}

func (suite *libfuseTestSuite) TestMkDirErrorExist() {
	testMkDirErrorExist(suite)
}

func (suite *libfuseTestSuite) TestMkDirErrorAttrExist() {
	testMkDirErrorAttrExist(suite)
}

// readdir

func (suite *libfuseTestSuite) TestReaddirMissingHandle() {
	testReaddirMissingHandle(suite)
}

func (suite *libfuseTestSuite) TestReaddirMissingCache() {
	testReaddirMissingCache(suite)
}

func (suite *libfuseTestSuite) TestReaddirEmptyPageToken() {
	testReaddirEmptyPageToken(suite)
}

func (suite *libfuseTestSuite) TestReleasedirMissingHandle() {
	testReleasedirMissingHandle(suite)
}

func (suite *libfuseTestSuite) TestReaddirPermissionError() {
	testReaddirPermissionError(suite)
}

func (suite *libfuseTestSuite) TestPopulateDirChildCacheReplaceCache() {
	testPopulateDirChildCacheReplaceCache(suite)
}

func (suite *libfuseTestSuite) TestPopulateDirChildCacheLastPage() {
	testPopulateDirChildCacheLastPage(suite)
}

func (suite *libfuseTestSuite) TestPopulateDirChildCacheNotFound() {
	testPopulateDirChildCacheNotFound(suite)
}

func (suite *libfuseTestSuite) TestPopulateDirChildCacheAppend() {
	testPopulateDirChildCacheAppend(suite)
}

func (suite *libfuseTestSuite) TestCreateFuseOptionsFlags() {
	testCreateFuseOptionsFlags(suite)
}

func (suite *libfuseTestSuite) TestCreateFuseOptionsDirectIO() {
	testCreateFuseOptionsDirectIO(suite)
}

func (suite *libfuseTestSuite) TestFillStatModes() {
	testFillStatModes(suite)
}

func (suite *libfuseTestSuite) TestFillStatModeDefault() {
	testFillStatModeDefault(suite)
}

func (suite *libfuseTestSuite) TestOpendirAndReleasedir() {
	testOpendirAndReleasedir(suite)
}

func (suite *libfuseTestSuite) TestServeCachedEntries() {
	testServeCachedEntries(suite)
}

func (suite *libfuseTestSuite) TestServeCachedEntriesStopEarly() {
	testServeCachedEntriesStopEarly(suite)
}

func (suite *libfuseTestSuite) TestRmDir() {
	testRmDir(suite)
}

func (suite *libfuseTestSuite) TestRmDirNotEmpty() {
	testRmDirNotEmpty(suite)
}

func (suite *libfuseTestSuite) TestRmDirError() {
	testRmDirError(suite)
}

func (suite *libfuseTestSuite) TestRmDirNotExists() {
	testRmDirNotExists(suite)
}

func (suite *libfuseTestSuite) TestRmDirPermission() {
	testRmDirPermission(suite)
}

func (suite *libfuseTestSuite) TestRmDirRaceNotEmpty() {
	testRmDirRaceNotEmpty(suite)
}

func (suite *libfuseTestSuite) TestCreate() {
	testCreate(suite)
}

func (suite *libfuseTestSuite) TestCreateError() {
	testCreateError(suite)
}

func (suite *libfuseTestSuite) TestCreateErrorExists() {
	testCreateErrorExists(suite)
}

func (suite *libfuseTestSuite) TestCreateErrorPermission() {
	testCreateErrorPermission(suite)
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

func (suite *libfuseTestSuite) TestOpenPermissionError() {
	testOpenPermissionError(suite)
}

// read

func (suite *libfuseTestSuite) TestReadMissingHandle() {
	testReadMissingHandle(suite)
}

func (suite *libfuseTestSuite) TestReadCachedHandle() {
	testReadCachedHandle(suite)
}

func (suite *libfuseTestSuite) TestReadCachedHandleEOF() {
	testReadCachedHandleEOF(suite)
}

func (suite *libfuseTestSuite) TestReadFromComponent() {
	testReadFromComponent(suite)
}

func (suite *libfuseTestSuite) TestReadAccessDenied() {
	testReadAccessDenied(suite)
}

func (suite *libfuseTestSuite) TestReadError() {
	testReadError(suite)
}

// write

func (suite *libfuseTestSuite) TestWriteMissingHandle() {
	testWriteMissingHandle(suite)
}

func (suite *libfuseTestSuite) TestWriteSuccess() {
	testWriteSuccess(suite)
}

func (suite *libfuseTestSuite) TestWriteError() {
	testWriteError(suite)
}

func (suite *libfuseTestSuite) TestWriteAccessDenied() {
	testWriteAccessDenied(suite)
}

// flush

func (suite *libfuseTestSuite) TestFlushNotDirty() {
	testFlushNotDirty(suite)
}

func (suite *libfuseTestSuite) TestFlushMissingHandle() {
	testFlushMissingHandle(suite)
}

func (suite *libfuseTestSuite) TestFlushErrors() {
	testFlushErrors(suite)
}

func (suite *libfuseTestSuite) TestTruncate() {
	testTruncate(suite)
}

func (suite *libfuseTestSuite) TestTruncateError() {
	testTruncateError(suite)
}

func (suite *libfuseTestSuite) TestTruncatePermission() {
	testTruncatePermission(suite)
}

func (suite *libfuseTestSuite) TestFTruncate() {
	testFTruncate(suite)
}

func (suite *libfuseTestSuite) TestFTruncateError() {
	testFTruncateError(suite)
}

// release

func (suite *libfuseTestSuite) TestReleaseMissingHandle() {
	testReleaseMissingHandle(suite)
}

func (suite *libfuseTestSuite) TestReleaseError() {
	testReleaseError(suite)
}

func (suite *libfuseTestSuite) TestReleaseErrorAccess() {
	testReleaseErrorAccess(suite)
}

func (suite *libfuseTestSuite) TestUnlink() {
	testUnlink(suite)
}

func (suite *libfuseTestSuite) TestUnlinkNotExists() {
	testUnlinkNotExists(suite)
}

func (suite *libfuseTestSuite) TestUnlinkPermission() {
	testUnlinkPermission(suite)
}

func (suite *libfuseTestSuite) TestUnlinkError() {
	testUnlinkError(suite)
}

func (suite *libfuseTestSuite) TestRenameFileFastPathSuccess() {
	testRenameFileFastPathSuccess(suite)
}

func (suite *libfuseTestSuite) TestRenameFileFastPathDstDirOnError() {
	testRenameFileFastPathDstDirOnError(suite)
}

func (suite *libfuseTestSuite) TestRenameFileFastPathError() {
	testRenameFileFastPathError(suite)
}

func (suite *libfuseTestSuite) TestRenameDirNotEmpty() {
	testRenameDirNotEmpty(suite)
}

func (suite *libfuseTestSuite) TestRenameDirDstNotDir() {
	testRenameDirDstNotDir(suite)
}

func (suite *libfuseTestSuite) TestRenameDirPermission() {
	testRenameDirPermission(suite)
}

func (suite *libfuseTestSuite) TestRenameDirDstGetAttrPermission() {
	testRenameDirDstGetAttrPermission(suite)
}

func (suite *libfuseTestSuite) TestRenameSrcGetAttrPermission() {
	testRenameSrcGetAttrPermission(suite)
}

func (suite *libfuseTestSuite) TestRenameSrcGetAttrError() {
	testRenameSrcGetAttrError(suite)
}

func (suite *libfuseTestSuite) TestSymlink() {
	testSymlink(suite)
}

func (suite *libfuseTestSuite) TestSymlinkError() {
	testSymlinkError(suite)
}

func (suite *libfuseTestSuite) TestSymlinkPermission() {
	testSymlinkPermission(suite)
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

func (suite *libfuseTestSuite) TestReadLinkPermission() {
	testReadLinkPermission(suite)
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

func (suite *libfuseTestSuite) TestFsyncPermission() {
	testFsyncPermission(suite)
}

func (suite *libfuseTestSuite) TestFsyncDir() {
	testFsyncDir(suite)
}

func (suite *libfuseTestSuite) TestFsyncDirError() {
	testFsyncDirError(suite)
}

func (suite *libfuseTestSuite) TestFsyncDirPermission() {
	testFsyncDirPermission(suite)
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

func (suite *libfuseTestSuite) TestStatFsCloudStorageCapacity() {
	testStatFsCloudStorageCapacity(suite)
}

func (suite *libfuseTestSuite) TestStatFsCloudStorageCapacityUsedExceedsDisplay() {
	testStatFsCloudStorageCapacityUsedExceedsDisplay(suite)
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

func (suite *libfuseTestSuite) TestUnsupportedOps() {
	testUnsupportedOps(suite)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestLibfuseTestSuite(t *testing.T) {
	suite.Run(t, new(libfuseTestSuite))
}
