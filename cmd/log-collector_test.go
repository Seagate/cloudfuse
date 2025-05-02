package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"os/exec"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var configValidBaseTest string = `
logging:
  type: base
  level: log_debug
  file-path: $HOME/cloudfuse/logTest/cloudfuse.log
`

var configInvalidBaseTest string = `
logging:
  type: base
  level: log_debug
  file-path: /home/davidhabinsky/cloudfuse/logTest/cloudfuse.log
`

var configValidSyslogTest string = `
logging:
  type: syslog
  level: log_debug
`

// what makes a syslog log type invalid?
var configInvalidSyslogTest string = `
logging:
  type: syslog
  level: log_debug
`

type logCollectTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *logCollectTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	err := log.SetDefaultLogger("base", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set debug logger as default: %v", err))
	}
}

func (suite *logCollectTestSuite) cleanupTest() {
	resetCLIFlags(*gatherLogsCmd)
}

func (suite *logCollectTestSuite) verifyArchive(archivePath string) bool {
	tempDir, err := os.MkdirTemp(currentDir, "tmpLogData")
	suite.assert.NoError(err)

	tempDir, err = filepath.Abs(tempDir)
	suite.assert.NoError(err)

	defer os.RemoveAll(tempDir)

	cmd := exec.Command("tar", "-xvf", archivePath, "-C", tempDir)
	err = cmd.Run()
	suite.assert.NoError(err)

	//verify archive contents (compare with original files that were put into archive)
	return false
}

func (suite *logCollectTestSuite) TestNoConfig() {
	defer suite.cleanupTest()

	//create temp files in default directory $HOME/.cloudfuse/cloudfuse.log

	baseDefaultDir := "$HOME/.cloudfuse/"
	baseDefaultDir = common.ExpandPath(baseDefaultDir)
	os.CreateTemp(baseDefaultDir, "cloudfuse*.log")

	//run gatherLogs command
	curDir, _ := os.Getwd()
	println(curDir)
	_, err := executeCommandC(rootCmd, "gatherLogs")
	suite.assert.NoError(err)

	currentDir, err := os.Getwd()
	suite.assert.NoError(err)

	//check gatherLogs archive (maybe extract it and make sure the files are the same?)
	items, err := os.ReadDir(currentDir)
	suite.assert.NoError(err)

	var foundArchive bool
	var archiveName string
	for _, item := range items {
		if strings.HasPrefix(item.Name(), "cloudfuse_logs") && strings.HasSuffix(item.Name(), "tar.gz") {
			foundArchive = true
			archiveName = item.Name()
		}
		if foundArchive {
			break
		}
	}
	suite.assert.True(foundArchive)
	defer os.Remove(archiveName)

}

func (suite *logCollectTestSuite) TestValidBaseConfig() {

}

func (suite *logCollectTestSuite) TestInvalidBaseConfig() {

}

func (suite *logCollectTestSuite) TestValidSyslogConfig() {

}

func (suite *logCollectTestSuite) TestInvalidSyslogConfig() {

}

func TestLogCollectCommand(t *testing.T) {
	suite.Run(t, new(logCollectTestSuite))
}
