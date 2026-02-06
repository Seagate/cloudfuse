//go:build windows

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2026 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.

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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var configMountTest string = `
logging:
  type: syslog
default-working-dir: /tmp/cloudfuse
file_cache:
  path: /tmp/fileCachePath
libfuse:
  attribute-expiration-sec: 120
  entry-expiration-sec: 60
azstorage:
  account-name: myAccountName
  account-key: myAccountKey
  mode: key
  endpoint: myEndpoint
  container: myContainer
  max-retries: 1
components:
  - libfuse
  - file_cache
  - attr_cache
  - azstorage
health_monitor:
  monitor-disable-list:
    - network_profiler
    - cloudfuse_stats
`

var confFileMntTest string

type mountTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *mountTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	options = mountOptions{}
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
}

func (suite *mountTestSuite) cleanupTest() {
	resetCLIFlags(*mountCmd)
	resetCLIFlags(*mountAllCmd)
	viper.Reset()

	common.DefaultWorkDir = "~/.cloudfuse"
	common.DefaultLogFilePath = common.JoinUnixFilepath(common.DefaultWorkDir, "cloudfuse.log")
}

// mount failure test where the mount directory does exist
func (suite *mountTestSuite) TestForegroundMountDirDoesExist() {
	defer suite.cleanupTest()

	mntDir, err := os.MkdirTemp("", "mntdir")
	suite.assert.Nil(err)
	tempDir := filepath.Join(mntDir, "tempdir")
	err = os.MkdirAll(tempDir, 0777)

	op, err := executeCommandC(rootCmd, "mount", tempDir, fmt.Sprintf("--config-file=%s", confFileMntTest), "--foreground=true")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount directory already exists")

	op, err = executeCommandC(rootCmd, "mount", "all", tempDir, fmt.Sprintf("--config-file=%s", confFileMntTest), "--foreground=true")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount directory already exists")
}

// mount failure test where the mount directory is not empty
func (suite *mountTestSuite) TestForegroundMountDirNotEmpty() {
	defer suite.cleanupTest()

	mntDir, err := os.MkdirTemp("", "mntdir")
	suite.assert.Nil(err)
	tempDir := filepath.Join(mntDir, "tempdir")

	err = os.MkdirAll(tempDir, 0777)
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	op, err := executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--config-file=%s", confFileMntTest), "--foreground=true")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount directory already exists")

	op, err = executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--config-file=%s", confFileMntTest), "-o", "nonempty", "--foreground=true")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount directory already exists")
}

// mount failure test where the mount path is not provided
func (suite *mountTestSuite) TestForegroundMountPathNotProvided() {
	defer suite.cleanupTest()

	op, err := executeCommandC(rootCmd, "mount", "", fmt.Sprintf("--config-file=%s", confFileMntTest), "--foreground=true")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount path not provided")

	op, err = executeCommandC(rootCmd, "mount", "all", "", fmt.Sprintf("--config-file=%s", confFileMntTest), "--foreground=true")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount path not provided")
}

// mount failure test where the config file path is empty
func (suite *mountTestSuite) TestForegroundConfigFileEmpty() {
	defer suite.cleanupTest()

	mntDir := "mntdir"

	op, err := executeCommandC(rootCmd, "mount", mntDir, "--config-file=", "--foreground=true")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "config file not provided")
}

// mount failure test where the config file type is unsupported
func (suite *mountTestSuite) TestForegroundConfigFileTypeUnsupported() {
	defer suite.cleanupTest()

	mntDir := "mntdir"

	op, err := executeCommandC(rootCmd, "mount", mntDir, "--config-file=cfgInvalid.yam", "--foreground=true")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "invalid config file")
	suite.assert.Contains(op, "Unsupported Config Type")
}

// mount failure test where the config file is not present
func (suite *mountTestSuite) TestForegroundConfigFileNotFound() {
	defer suite.cleanupTest()

	mntDir := "mntdir"

	op, err := executeCommandC(rootCmd, "mount", mntDir, "--config-file=cfgNotFound.yaml", "--foreground=true")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "invalid config file")
	suite.assert.Contains(op, "cannot find the file specified")

	op, err = executeCommandC(rootCmd, "mount", "all", mntDir, "--config-file=cfgNotFound.yaml", "--foreground=true")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "invalid config file")
	suite.assert.Contains(op, "cannot find the file specified")
}

// mount failure test where config file is not provided
func (suite *mountTestSuite) TestForegroundConfigFileNotProvided() {
	defer suite.cleanupTest()

	mntDir := "mntdir"

	op, err := executeCommandC(rootCmd, "mount", mntDir, "--foreground=true")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to initialize new pipeline")
}

func (suite *mountTestSuite) TestForegroundDefaultConfigFile() {
	defer suite.cleanupTest()

	mntDir := "mntdir"

	currDir, err := os.Getwd()
	suite.assert.Nil(err)
	defaultCfgPath := filepath.Join(currDir, common.DefaultConfigFilePath)

	// create default config file
	src, err := os.Open(confFileMntTest)
	suite.Equal(nil, err)

	dest, err := os.Create(defaultCfgPath)
	suite.Equal(nil, err)
	defer os.Remove(defaultCfgPath)

	bytesCopied, err := io.Copy(dest, src)
	suite.Equal(nil, err)
	suite.NotEqual(0, bytesCopied)

	err = dest.Close()
	suite.Equal(nil, err)
	err = src.Close()
	suite.Equal(nil, err)

	op, err := executeCommandC(rootCmd, "mount", mntDir, "--foreground=true")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to initialize new pipeline")
}

func (suite *mountTestSuite) TestForegroundInvalidLogLevel() {
	defer suite.cleanupTest()

	mntDir := "mntdir"

	op, err := executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--config-file=%s", confFileMntTest), "--log-level=debug", "--foreground=true")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "invalid log level")
}

// mount failure test where umask value is invalid
func (suite *mountTestSuite) TestForegroundInvalidUmaskValue() {
	defer suite.cleanupTest()

	mntDir := "mntdir"

	// incorrect umask value
	op, err := executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--config-file=%s", confFileMntTest), "--foreground=true",
		"-o allow_other", "-o attr_timeout=120", "-o entry_timeout=120", "-o negative_timeout=120",
		"-o ro", "-o allow_root", "-o default_permissions", "-o umask=abcd")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to parse umask")
}

// fuse option parsing validation
func (suite *mountTestSuite) TestFuseOptions() {
	defer suite.cleanupTest()

	type fuseOpt struct {
		opt    string
		ignore bool
	}

	opts := []fuseOpt{
		{opt: "rw", ignore: true},
		{opt: "dev", ignore: true},
		{opt: "dev", ignore: true},
		{opt: "nodev", ignore: true},
		{opt: "suid", ignore: true},
		{opt: "nosuid", ignore: true},
		{opt: "delay_connect", ignore: true},
		{opt: "auto", ignore: true},
		{opt: "noauto", ignore: true},
		{opt: "user", ignore: true},
		{opt: "nouser", ignore: true},
		{opt: "exec", ignore: true},
		{opt: "noexec", ignore: true},

		{opt: "allow_other", ignore: false},
		{opt: "allow_other=true", ignore: false},
		{opt: "allow_other=false", ignore: false},
		{opt: "nonempty", ignore: false},
		{opt: "attr_timeout=10", ignore: false},
		{opt: "entry_timeout=10", ignore: false},
		{opt: "negative_timeout=10", ignore: false},
		{opt: "ro", ignore: false},
		{opt: "allow_root", ignore: false},
		{opt: "umask=777", ignore: false},
		{opt: "uid=1000", ignore: false},
		{opt: "gid=1000", ignore: false},
		{opt: "direct_io", ignore: false},
	}

	for _, val := range opts {
		ret := ignoreFuseOptions(val.opt)
		suite.assert.Equal(ret, val.ignore)
	}
}

func (suite *mountTestSuite) TestUpdateCliParams() {
	defer suite.cleanupTest()

	cliParams := []string{"cloudfuse", "mount", "~/mntdir/", "--foreground=false"}

	updateCliParams(&cliParams, "tmp-path", "tmpPath1")
	suite.assert.Equal(len(cliParams), 5)
	suite.assert.Equal(cliParams[4], "--tmp-path=tmpPath1")

	updateCliParams(&cliParams, "container-name", "testCnt1")
	suite.assert.Equal(len(cliParams), 6)
	suite.assert.Equal(cliParams[5], "--container-name=testCnt1")

	updateCliParams(&cliParams, "tmp-path", "tmpPath2")
	updateCliParams(&cliParams, "container-name", "testCnt2")
	suite.assert.Equal(len(cliParams), 6)
	suite.assert.Equal(cliParams[4], "--tmp-path=tmpPath2")
	suite.assert.Equal(cliParams[5], "--container-name=testCnt2")
}

func (suite *mountTestSuite) TestOptionsValidate() {
	defer suite.cleanupTest()
	opts := &mountOptions{}

	err := opts.validate(true)
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "mount path not provided")

	// Mount directory must not already exist for Windows
	opts.MountPath = filepath.Join("tmp", "mntdir")
	defer os.RemoveAll(opts.MountPath)

	err = opts.validate(true)
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "invalid log level")

	opts.Logging.LogLevel = "log_junk"
	err = opts.validate(true)
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "invalid log level")

	opts.Logging.LogLevel = "log_debug"
	err = opts.validate(true)
	suite.assert.Nil(err)
	suite.assert.Empty(opts.Logging.LogFilePath)

	opts.DefaultWorkingDir, _ = os.UserHomeDir()
	opts.DefaultWorkingDir = common.JoinUnixFilepath(opts.DefaultWorkingDir)
	err = opts.validate(true)
	suite.assert.Nil(err)
	suite.assert.Empty(opts.Logging.LogFilePath)
	suite.assert.Equal(common.DefaultWorkDir, opts.DefaultWorkingDir)

	opts.Logging.LogFilePath = common.DefaultLogFilePath
	err = opts.validate(true)
	suite.assert.Nil(err)
	suite.assert.Contains(opts.Logging.LogFilePath, opts.DefaultWorkingDir)
	suite.assert.Equal(common.DefaultWorkDir, opts.DefaultWorkingDir)
	suite.assert.Equal(common.DefaultLogFilePath, opts.Logging.LogFilePath)
}

func (suite *mountTestSuite) TestBackgroundMissingArgs() {
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "mount")
	suite.assert.NotNil(err)
}

func (suite *mountTestSuite) TestBackgroundMountPathEmpty() {
	defer suite.cleanupTest()

	// Create config file
	confFile, err := os.CreateTemp("", "conf*.yaml")
	suite.assert.Nil(err)
	cfgFile := confFile.Name()
	defer os.Remove(cfgFile)

	mntPath := ""

	op, err := executeCommandC(rootCmd, "mount", mntPath, fmt.Sprintf("--config-file=%s", cfgFile))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount path not provided")
}

func (suite *mountTestSuite) TestBackgroundConfigFileEmpty() {
	defer suite.cleanupTest()

	mntPath := "mntdir" + randomString(8)
	cfgFile := ""

	op, err := executeCommandC(rootCmd, "mount", mntPath, fmt.Sprintf("--config-file=%s", cfgFile))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "config file not provided")
}

func (suite *mountTestSuite) TestBackgroundMountDirExist() {
	defer suite.cleanupTest()

	// Create Mount Directory
	mntPath, err := os.MkdirTemp("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntPath)

	// Create config file
	confFile, err := os.CreateTemp("", "conf*.yaml")
	suite.assert.Nil(err)
	cfgFile := confFile.Name()
	defer os.Remove(cfgFile)

	op, err := executeCommandC(rootCmd, "mount", mntPath, fmt.Sprintf("--config-file=%s", cfgFile))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount path exists")
}

func (suite *mountTestSuite) TestBackgroundConfigFileNotExist() {
	defer suite.cleanupTest()

	mntPath := "mntdir" + randomString(8)
	cfgFile := "cfgNotFound.yaml"

	op, err := executeCommandC(rootCmd, "mount", mntPath, fmt.Sprintf("--config-file=%s", cfgFile))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "config file")
}

func TestMountCommand(t *testing.T) {
	confFile, err := os.CreateTemp("", "conf*.yaml")
	if err != nil {
		t.Error("Failed to create config file")
	}
	confFileMntTest = confFile.Name()
	defer os.Remove(confFileMntTest)

	_, err = confFile.WriteString(configMountTest)
	if err != nil {
		t.Error("Failed to write to config file")
	}
	confFile.Close()

	suite.Run(t, new(mountTestSuite))
}
