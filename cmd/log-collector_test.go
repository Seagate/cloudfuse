package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

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
	err := log.SetDefaultLogger("log_debug", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set debug logger as default: %v", err))
	}
}

func (suite *logCollectTestSuite) cleanupTest() {
	resetCLIFlags(*gatherLogsCmd)
}

func (suite *logCollectTestSuite) TestNoConfig() {
	defer suite.cleanupTest()

	mntDir, err := os.MkdirTemp("", "mntdir")
	suite.assert.Nil(err)
	tempDir := filepath.Join(mntDir, "tempdir")
	err = os.MkdirAll(tempDir, 0777)

	//in progress contstructing mount with no config
	op, err := executeCommandC(rootCmd, "mount", tempDir, fmt.Sprintf("--config-file=%s", ""), "--foreground=true")

	//check default log areas have logs generated

	//run gatherLogs command

	//check gatherLogs archvie  (maybe extract it and make sure the files are the same?)
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
