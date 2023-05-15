//go:build windows

package cmd

import (
	"fmt"
	"os"
	"testing"

	"lyvecloudfuse/common"
	"lyvecloudfuse/common/log"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type serviceTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *serviceTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	servOpts = serviceOptions{}
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
}

func (suite *serviceTestSuite) cleanupTest() {
	resetCLIFlags(*serviceCmd)
	resetCLIFlags(*mountServiceCmd)
	viper.Reset()
}

func (suite *serviceTestSuite) TestHelp() {
	defer suite.cleanupTest()
	_, err := executeCommandC(rootCmd, "service", "-h")
	suite.assert.Nil(err)
}

// Mount Tests

func (suite *serviceTestSuite) TestMountMissingArgs() {
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "service", "mount")
	suite.assert.NotNil(err)
}

func (suite *serviceTestSuite) TestMountPathEmpty() {
	defer suite.cleanupTest()

	mntPath := ""
	cfgFile := "cfgNotFound.yaml"

	op, err := executeCommandC(rootCmd, "service", "mount", mntPath, fmt.Sprintf("--config-file=%s", cfgFile))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mmount path not provided]")
}

func (suite *serviceTestSuite) TestConfigFileEmpty() {
	defer suite.cleanupTest()

	mntPath := randomString(8)
	cfgFile := ""

	op, err := executeCommandC(rootCmd, "service", "mount", mntPath, fmt.Sprintf("--config-file=%s", cfgFile))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "config file not provided")
}

func (suite *serviceTestSuite) TestMountDirExist() {
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

	op, err := executeCommandC(rootCmd, "service", "mount", mntPath, fmt.Sprintf("--config-file=%s", cfgFile))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mmount path exists")
}

func (suite *serviceTestSuite) TestConfigFileNotExist() {
	defer suite.cleanupTest()

	mntPath := randomString(8)
	cfgFile := "cfgNotFound.yaml"

	op, err := executeCommandC(rootCmd, "service", "mount", mntPath, fmt.Sprintf("--config-file=%s", cfgFile))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "config file does not exist")
}

// Unmount Tests

func (suite *serviceTestSuite) TestUnountMountPathEmpty() {
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "service", "unmount")
	suite.assert.NotNil(err)
}

func TestServiceCommand(t *testing.T) {
	suite.Run(t, new(serviceTestSuite))
}
