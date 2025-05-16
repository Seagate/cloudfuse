package cmd

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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

func (suite *logCollectTestSuite) cleanupTest(dir string) {
	resetCLIFlags(*gatherLogsCmd)
	os.Remove(dir + "/cloudfuse_logs.tar.gz")
}

func (suite *logCollectTestSuite) extractZip(srcPath, destPath string) {
	readCloser, err := zip.OpenReader(srcPath + string(os.PathSeparator) + "cloudfuse_logs.zip")
	suite.assert.NoError(err)
	defer readCloser.Close()

	for _, item := range readCloser.File {
		itemPath := filepath.Join(destPath, item.Name)

		err = os.MkdirAll(filepath.Dir(itemPath), os.ModePerm)
		suite.assert.NoError(err)

		dstFile, err := os.OpenFile(itemPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, item.Mode())
		suite.assert.NoError(err)

		srcFile, err := item.Open()
		suite.assert.NoError(err)

		_, err = io.Copy(dstFile, srcFile)
		suite.assert.NoError(err)
	}
}

// verifyArchive compares original log files with archived log files using a checksum.
func (suite *logCollectTestSuite) verifyArchive(logPath, archivePath string) bool {

	//set up file list and map
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

	if runtime.GOOS == "linux" {
		cmd := exec.Command("tar", "-xvf", archivePath+"/cloudfuse_logs.tar.gz", "-C", tempDir)
		err = cmd.Run()
		suite.assert.NoError(err)
	} else if runtime.GOOS == "windows" {
		suite.extractZip(archivePath, tempDir)
	}

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

// Log collection test where no config file is provided
func (suite *logCollectTestSuite) TestNoConfig() {
	currentDir, err := os.Getwd()
	suite.assert.NoError(err)
	defer suite.cleanupTest(currentDir)

	//create temp files in default directory $HOME/.cloudfuse/cloudfuse.log

	baseDefaultDir := "$HOME/.cloudfuse/"
	baseDefaultDir = common.ExpandPath(baseDefaultDir)
	os.CreateTemp(baseDefaultDir, "cloudfuse*.log")

	//run gatherLogs command
	_, err = executeCommandC(rootCmd, "gatherLogs")
	suite.assert.NoError(err)

	if runtime.GOOS == "linux" {
		suite.assert.FileExists(currentDir + "/cloudfuse_logs.tar.gz")
	} else if runtime.GOOS == "windows" {
		suite.assert.FileExists(currentDir + "/cloudfuse_logs.zip")
	}
	isArcValid := suite.verifyArchive(baseDefaultDir, currentDir)
	suite.assert.True(isArcValid)

}

// Log collection test using 'base' for the logging type in the config.
func (suite *logCollectTestSuite) TestValidBaseConfig() {
	currentDir, err := os.Getwd()
	suite.assert.NoError(err)
	defer suite.cleanupTest(currentDir)

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
	_, err = executeCommandC(rootCmd, "gatherLogs", fmt.Sprintf("--config-file=%s", confFile.Name()))
	suite.assert.NoError(err)

	//verify the archive
	isArcValid := suite.verifyArchive(tempLogDir, currentDir)
	suite.assert.True(isArcValid)

}

// Log collection test using 'base' for the logging type with a nonexisting file path in the config.
func (suite *logCollectTestSuite) TestInvalidBaseConfig() {
	currentDir, err := os.Getwd()
	suite.assert.NoError(err)
	defer suite.cleanupTest(currentDir)

	//set up config file
	invalidBaseConfig := logCollectTestConfig{logType: "base", level: "log_debug", filePath: "/home/fakeUser/cloudfuse.log"}
	config := fmt.Sprintf("logging:\n  type: %s\n  level: %s\n  file-path: %s\n",
		invalidBaseConfig.logType, invalidBaseConfig.level, invalidBaseConfig.filePath)
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	defer os.Remove(confFile.Name())
	_, err = confFile.WriteString(config)
	suite.assert.NoError(err)
	confFile.Close()

	//run the log collector
	_, err = executeCommandC(rootCmd, "gatherLogs", fmt.Sprintf("--config-file=%s", confFile.Name()))
	suite.assert.Error(err)
}

// Log collection test using 'syslog' for the logging type in the config.
func (suite *logCollectTestSuite) TestValidSyslogConfig() {
	currentDir, err := os.Getwd()
	suite.assert.NoError(err)
	defer suite.cleanupTest(currentDir)

	//set up config file
	validSyslogConfig := logCollectTestConfig{logType: "syslog", level: "log_debug"}
	config := fmt.Sprintf("logging:\n  type: %s\n  level: %s\n",
		validSyslogConfig.logType, validSyslogConfig.level)
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	defer os.Remove(confFile.Name())
	_, err = confFile.WriteString(config)
	suite.assert.NoError(err)
	confFile.Close()

	//run the log collector
	_, err = executeCommandC(rootCmd, "gatherLogs", fmt.Sprintf("--config-file=%s", confFile.Name()))
	suite.assert.NoError(err)

	// look for generated archive

	// look for temp cloudfuse.log file generated from syslog
	filteredLogPath := "/tmp/cloudfuseSyslog"

	// use validate archive between those two files.
	isArcValid := suite.verifyArchive(filteredLogPath, currentDir)
	suite.assert.True(isArcValid)
}

// Log collection test using 'invalid' for the logging type and level in the config.
func (suite *logCollectTestSuite) TestInvalidConfig() {
	currentDir, err := os.Getwd()
	suite.assert.NoError(err)
	defer suite.cleanupTest(currentDir)

	//set up config file
	invalidSyslogConfig := logCollectTestConfig{logType: "invalid", level: "invalid"}
	config := fmt.Sprintf("logging:\n  type: %s\n  level: %s\n",
		invalidSyslogConfig.logType, invalidSyslogConfig.level)
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	defer os.Remove(confFile.Name())
	_, err = confFile.WriteString(config)
	suite.assert.NoError(err)
	confFile.Close()

	//set up test log
	baseDefaultDir := "$HOME/.cloudfuse/"
	baseDefaultDir = common.ExpandPath(baseDefaultDir)
	os.CreateTemp(baseDefaultDir, "cloudfuse*.log")

	//run the log collector
	_, err = executeCommandC(rootCmd, "gatherLogs", fmt.Sprintf("--config-file=%s", confFile.Name()))
	suite.assert.NoError(err)

	//check archive
	suite.assert.FileExists(currentDir + "/cloudfuse_logs.tar.gz")
	isArcValid := suite.verifyArchive(baseDefaultDir, currentDir)
	suite.assert.True(isArcValid)
}

// Log collection test using 'silent' for the logging type in the config.
func (suite *logCollectTestSuite) TestSilentConfig() {
	currentDir, err := os.Getwd()
	suite.assert.NoError(err)
	defer suite.cleanupTest(currentDir)

	//set up config file
	silentConfig := logCollectTestConfig{logType: "silent", level: "log_debug"}
	config := fmt.Sprintf("logging:\n  type: %s\n  level: %s\n",
		silentConfig.logType, silentConfig.level)
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	defer os.Remove(confFile.Name())
	_, err = confFile.WriteString(config)
	suite.assert.NoError(err)
	confFile.Close()

	//set up test log
	baseDefaultDir := "$HOME/.cloudfuse/"
	baseDefaultDir = common.ExpandPath(baseDefaultDir)
	os.CreateTemp(baseDefaultDir, "cloudfuse*.log")

	//run the log collector
	_, err = executeCommandC(rootCmd, "gatherLogs", fmt.Sprintf("--config-file=%s", confFile.Name()))
	suite.assert.Error(err)
}

// Log collection test using --output-path flag
func (suite *logCollectTestSuite) TestOutputPath() {

	// create temp folder for output Path
	outputPath := "$HOME/"
	outputPath = common.ExpandPath(outputPath)
	tempDir, err := os.MkdirTemp(outputPath, "tempArcDir")
	suite.assert.NoError(err)
	tempDir, err = filepath.Abs(tempDir)
	suite.assert.NoError(err)
	defer suite.cleanupTest(tempDir)
	defer os.RemoveAll(tempDir)

	// create temp files in default directory $HOME/.cloudfuse/cloudfuse.log
	baseDefaultDir := "$HOME/.cloudfuse/"
	baseDefaultDir = common.ExpandPath(baseDefaultDir)
	os.CreateTemp(baseDefaultDir, "cloudfuse*.log")

	// run gatherLogs command
	_, err = executeCommandC(rootCmd, "gatherLogs", fmt.Sprintf("--output-path=%s", tempDir))
	suite.assert.NoError(err)

	suite.assert.FileExists(tempDir + "/cloudfuse_logs.tar.gz")
	isArcValid := suite.verifyArchive(baseDefaultDir, tempDir)
	suite.assert.True(isArcValid)

}

func TestLogCollectCommand(t *testing.T) {
	suite.Run(t, new(logCollectTestSuite))
}
