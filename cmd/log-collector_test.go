package cmd

import (
	"crypto/sha256"
	"fmt"
	"io"
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

type logCollectTestConfig struct {
	logType  string
	level    string
	filePath string
}

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

func (suite *logCollectTestSuite) verifyArchive(logPath, archivePath string) bool {

	//store file name and hash in a map
	fileHashMap := make(map[string]string)
	items, err := os.ReadDir(logPath)
	suite.assert.NoError(err)

	//collect file and hash values into the map
	for _, item := range items {
		if strings.HasPrefix(item.Name(), "cloudfuse") && strings.HasSuffix(item.Name(), ".log") {

			//get file path
			itemPath := filepath.Join(logPath, item.Name())
			itemPath = filepath.Clean(itemPath)

			// generate and store checksum for file
			file, err := os.Open(itemPath)
			suite.assert.NoError(err)
			hasher := sha256.New()
			_, err = io.Copy(hasher, file)
			suite.assert.NoError(err)
			hashStr := string(hasher.Sum(nil))
			fileHashMap[item.Name()] = hashStr

		}
	}

	currentDir, err := os.Getwd()
	suite.assert.NoError(err)
	tempDir, err := os.MkdirTemp(currentDir, "tmpLogData")
	suite.assert.NoError(err)

	tempDir, err = filepath.Abs(tempDir)
	suite.assert.NoError(err)

	defer os.RemoveAll(tempDir)

	cmd := exec.Command("tar", "-xvf", archivePath+"/cloudfuse_logs.tar.gz", "-C", tempDir)
	err = cmd.Run()
	suite.assert.NoError(err)

	//verify archive contents (compare with original files that were put into archive)

	items, err = os.ReadDir(tempDir)
	suite.assert.NoError(err)
	for _, archivedItem := range items {
		if strings.HasPrefix(archivedItem.Name(), "cloudfuse") && strings.HasSuffix(archivedItem.Name(), ".log") {

			//get file path
			itemPath := filepath.Join(tempDir, archivedItem.Name())
			itemPath = filepath.Clean(itemPath)

			// generate and store checksum for file
			file, err := os.Open(itemPath)
			suite.assert.NoError(err)

			suite.assert.True(fileHashMap[archivedItem.Name()] != "")
			hasher := sha256.New()
			_, err = io.Copy(hasher, file)
			suite.assert.NoError(err)
			hashStr := string(hasher.Sum(nil))
			suite.assert.Equal(fileHashMap[archivedItem.Name()], hashStr)

		} else {
			return false //found a non cloudfuse log file
		}
	}
	return true
}

func (suite *logCollectTestSuite) TestNoConfig() {
	defer suite.cleanupTest()

	//create temp files in default directory $HOME/.cloudfuse/cloudfuse.log

	baseDefaultDir := "$HOME/.cloudfuse/"
	baseDefaultDir = common.ExpandPath(baseDefaultDir)
	os.CreateTemp(baseDefaultDir, "cloudfuse*.log")

	//run gatherLogs command
	_, err := executeCommandC(rootCmd, "gatherLogs")
	suite.assert.NoError(err)

	currentDir, err := os.Getwd()
	suite.assert.NoError(err)
	suite.assert.FileExists(currentDir + "/cloudfuse_logs.tar.gz")
	isArcValid := suite.verifyArchive(baseDefaultDir, currentDir)
	suite.assert.True(isArcValid)

}

func (suite *logCollectTestSuite) TestValidBaseConfig() {
	defer suite.cleanupTest()

	//set up test log directory
	logPath := "$HOME"
	logPath = common.ExpandPath(logPath)
	tempLogDir, err := os.MkdirTemp(logPath, "logTest")
	suite.assert.NoError(err)
	tempLogDir, err = filepath.Abs(tempLogDir)
	suite.assert.NoError(err)
	defer os.RemoveAll(tempLogDir)

	//set up config file
	validBaseConfig := logCollectTestConfig{logType: "base", level: "log_debug", filePath: tempLogDir + "/cloudfuse.log"}
	config := fmt.Sprintf("logging:\n  type: %s\n  level: %s\n  file-path: %s\n",
		validBaseConfig.logType, validBaseConfig.level, validBaseConfig.filePath)
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	defer os.Remove(confFile.Name())
	_, err = confFile.WriteString(config)
	suite.assert.NoError(err)
	confFile.Close()

	//put stub log files in test log directory
	tempLog, err := os.CreateTemp(tempLogDir, "cloudfuse*.log")
	suite.assert.NoError(err)
	defer os.RemoveAll(tempLog.Name())

	//run the log collector
	_, err = executeCommandSecure(rootCmd, "gatherLogs", fmt.Sprintf("--config-file=%s", confFile.Name()))
	suite.assert.NoError(err)

	//verify the archive
	currentDir, err := os.Getwd()
	suite.assert.NoError(err)
	isArcValid := suite.verifyArchive(tempLogDir, currentDir)
	suite.assert.True(isArcValid)

}

func (suite *logCollectTestSuite) TestInvalidBaseConfig() {
	defer suite.cleanupTest()

	//set up config file
	invalidBaseConfig := logCollectTestConfig{logType: "base", level: "log_debug", filePath: "/home/fakeUser/cloudfuse.log"}
	config := fmt.Sprintf("logging:\n  type: %s\n  level: %s\n  file-path: %s\n",
		invalidBaseConfig.logType, invalidBaseConfig.level, invalidBaseConfig.filePath)
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	defer os.Remove(confFile.Name())
	_, err := confFile.WriteString(config)
	suite.assert.NoError(err)
	confFile.Close()

	//run the log collector
	_, err = executeCommandSecure(rootCmd, "gatherLogs", fmt.Sprintf("--config-file=%s", confFile.Name()))
	suite.assert.Error(err)
}

func (suite *logCollectTestSuite) TestValidSyslogConfig() {
	defer suite.cleanupTest()

	//set up config file
	validSyslogConfig := logCollectTestConfig{logType: "syslog", level: "log_debug"}
	config := fmt.Sprintf("logging:\n  type: %s\n  level: %s\n",
		validSyslogConfig.logType, validSyslogConfig.level)
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	defer os.Remove(confFile.Name())
	_, err := confFile.WriteString(config)
	suite.assert.NoError(err)
	confFile.Close()

	//run the log collector
	_, err = executeCommandSecure(rootCmd, "gatherLogs", fmt.Sprintf("--config-file=%s", confFile.Name()))
	suite.assert.NoError(err)

	// look for generated archive
	currentDir, err := os.Getwd()
	suite.assert.NoError(err)
	// look for temp cloudfuse.log file generated from syslog
	filteredLogPath := "/tmp/cloudfuseSyslog"

	// use validate archive between those two files.
	isArcValid := suite.verifyArchive(filteredLogPath, currentDir)
	suite.assert.True(isArcValid)
}

func (suite *logCollectTestSuite) TestInvalidSyslogConfig() {

}

func TestLogCollectCommand(t *testing.T) {
	suite.Run(t, new(logCollectTestSuite))
}
