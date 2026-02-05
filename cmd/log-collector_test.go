/*
	Licensed under the MIT License <http://opensource.org/licenses/MIT>.

	Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates

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
	SOFTWARE.
*/

package cmd

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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

func (suite *logCollectTestSuite) setupConfig(logInfo logCollectTestConfig) *os.File {
	config := fmt.Sprintf("logging:\n  type: %s\n  level: %s\n  file-path: %s\n",
		logInfo.logType, logInfo.level, logInfo.filePath)
	configFile, err := os.CreateTemp("", "conf*.yaml")
	suite.assert.NoError(err)
	_, err = configFile.WriteString(config)
	suite.assert.NoError(err)
	configFile.Close()
	return configFile
}

func (suite *logCollectTestSuite) extractZip(srcZipPath, destPath string) {
	zipDir := fmt.Sprintf("%s\\cloudfuse_logs.zip", srcZipPath)
	zipDir = filepath.Clean(zipDir)
	readCloser, err := zip.OpenReader(zipDir)
	suite.assert.NoError(err)
	defer readCloser.Close()

	for _, item := range readCloser.File {
		itemPath := filepath.Join(destPath, item.Name)

		if item.FileInfo().IsDir() {
			err = os.MkdirAll(itemPath, item.Mode())
			suite.assert.NoError(err)
			continue
		}

		err = os.MkdirAll(filepath.Dir(itemPath), os.ModePerm)
		suite.assert.NoError(err)

		var dstFile *os.File
		dstFile, err = os.OpenFile(itemPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, item.Mode())
		suite.assert.NoError(err)

		var srcFile io.ReadCloser
		srcFile, err = item.Open()
		suite.assert.NoError(err)
		defer srcFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		suite.assert.NoError(err)
		err = dstFile.Close()
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
		if strings.Contains(item.Name(), "cloudfuse") &&
			regexp.MustCompile(`\.log(?:\.\d)?$`).MatchString(item.Name()) {

			//get file path
			itemPath := filepath.Join(logPath, item.Name())
			itemPath = filepath.Clean(itemPath)

			// generate and store checksum for file
			var file *os.File
			file, err = os.Open(itemPath)
			suite.assert.NoError(err)
			defer file.Close()
			hasher := sha256.New()
			_, err = io.Copy(hasher, file)
			suite.assert.NoError(err)
			hashStr := string(hasher.Sum(nil))
			fileHashMap[item.Name()] = hashStr

		}
	}

	var currentDir string
	currentDir, err = os.Getwd()
	suite.assert.NoError(err)
	var tempDir string
	tempDir, err = os.MkdirTemp(currentDir, "verifyArchive")
	suite.assert.NoError(err)
	tempDir, err = filepath.Abs(tempDir)
	suite.assert.NoError(err)

	defer os.RemoveAll(tempDir)

	switch runtime.GOOS {
	case "linux":
		cmd := exec.Command("tar", "-xvf", archivePath+"/cloudfuse_logs.tar.gz", "-C", tempDir)
		err = cmd.Run()
		suite.assert.NoError(err)
	case "windows":
		suite.extractZip(archivePath, tempDir)
	}

	//verify archive contents (compare with original files that were put into archive)
	var amountLogs int
	err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		var relPath string
		relPath, err = filepath.Rel(tempDir, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			if relPath == "." {
				return nil
			}
			return nil
		}

		if strings.HasPrefix(relPath, "systemprofile") {
			return nil
		}

		if strings.Contains(info.Name(), "cloudfuse") &&
			regexp.MustCompile(`\.log(?:\.\d)?$`).MatchString(info.Name()) {

			// generate and store checksum for file
			var file *os.File
			file, err = os.Open(path)
			suite.assert.NoError(err)
			suite.assert.NotEmpty(fileHashMap[info.Name()])
			hasher := sha256.New()
			_, err = io.Copy(hasher, file)
			suite.assert.NoError(err)
			hashStr := string(hasher.Sum(nil))
			suite.assert.Equal(fileHashMap[info.Name()], hashStr)
			amountLogs++
		}
		return err
	})
	suite.assert.NoError(err)
	suite.assert.Len(
		fileHashMap,
		amountLogs,
	)
	return true
}

// Log collection test where no config file is provided
func (suite *logCollectTestSuite) TestNoConfig() {
	currentDir, err := os.Getwd()
	suite.assert.NoError(err)
	defer suite.cleanupTest(currentDir)

	//create temp files in default directory $HOME/.cloudfuse/cloudfuse.log

	baseDefaultDir := common.GetDefaultWorkDir() + "/.cloudfuse/"
	baseDefaultDir = common.ExpandPath(baseDefaultDir)
	if !common.DirectoryExists(baseDefaultDir) {
		_ = os.Mkdir(baseDefaultDir, os.FileMode(0760))
	}
	var logFile *os.File
	logFile, err = os.CreateTemp(baseDefaultDir, "cloudfuse*.log")
	suite.assert.NoError(err)
	defer os.Remove(logFile.Name())

	//run gatherLogs command
	_, err = executeCommandC(rootCmd, "gather-logs")
	suite.assert.NoError(err)

	switch runtime.GOOS {
	case "linux":
		suite.assert.FileExists(currentDir + "/cloudfuse_logs.tar.gz")
	case "windows":
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
	logPath := common.GetDefaultWorkDir()
	logPath = common.ExpandPath(logPath)
	tempLogDir, err := os.MkdirTemp(logPath, "logTest")
	suite.assert.NoError(err)
	tempLogDir, err = filepath.Abs(tempLogDir)
	suite.assert.NoError(err)

	//put stub log files in test log directory
	tempLog, err := os.CreateTemp(tempLogDir, "cloudfuse*.log")
	suite.assert.NoError(err)
	defer os.RemoveAll(tempLog.Name())

	//set up config file
	validBaseConfig := logCollectTestConfig{
		logType:  "base",
		level:    "log_debug",
		filePath: tempLog.Name(),
	}
	configFile := suite.setupConfig(validBaseConfig)
	defer os.Remove(configFile.Name())

	//run the log collector
	_, err = executeCommandC(
		rootCmd,
		"gather-logs",
		fmt.Sprintf("--config-file=%s", configFile.Name()),
	)
	suite.assert.NoError(err)

	//verify the archive
	isArcValid := suite.verifyArchive(tempLogDir, currentDir)
	suite.assert.True(isArcValid)

}

// Log collection test using 'base' for the logging type with a nonexisting file path in the config.
func (suite *logCollectTestSuite) TestInvalidFilePathBaseConfig() {
	currentDir, err := os.Getwd()
	suite.assert.NoError(err)
	defer suite.cleanupTest(currentDir)

	//set up config file
	invalidBaseConfig := logCollectTestConfig{
		logType:  "base",
		level:    "log_debug",
		filePath: "/home/fakeUser/cloudfuse.log",
	}
	configFile := suite.setupConfig(invalidBaseConfig)

	//run the log collector
	_, err = executeCommandC(
		rootCmd,
		"gather-logs",
		fmt.Sprintf("--config-file=%s", configFile.Name()),
	)
	suite.assert.Error(err)
}

// Log collection test using 'syslog' for the logging type in the config.
func (suite *logCollectTestSuite) TestValidSyslogConfig() {
	currentDir, err := os.Getwd()
	suite.assert.NoError(err)
	defer suite.cleanupTest(currentDir)

	//set up config file
	validSyslogConfig := logCollectTestConfig{logType: "syslog", level: "log_debug", filePath: ""}
	configFile := suite.setupConfig(validSyslogConfig)

	//run the log collector
	_, err = executeCommandC(
		rootCmd,
		"gather-logs",
		fmt.Sprintf("--config-file=%s", configFile.Name()),
	)
	switch runtime.GOOS {
	case "linux":
		suite.assert.NoError(err)

		// look for temp cloudfuse.log file generated from syslog
		filteredLogPath := "/tmp/"

		// use validate archive between those two files.
		isArcValid := suite.verifyArchive(filteredLogPath, currentDir)
		suite.assert.True(isArcValid)
	case "windows":
		suite.assert.Error(err)
	}

}

// Log collection test using 'invalid' for the logging type and level in the config.
func (suite *logCollectTestSuite) TestInvalidConfig() {
	currentDir, err := os.Getwd()
	suite.assert.NoError(err)
	defer suite.cleanupTest(currentDir)

	//set up config file
	invalidSyslogConfig := logCollectTestConfig{
		logType:  "inv$!^alid",
		level:    "log_#@^%$debug",
		filePath: "inv#%^&!alid",
	}
	configFile := suite.setupConfig(invalidSyslogConfig)

	//run the log collector
	_, err = executeCommandC(
		rootCmd,
		"gather-logs",
		fmt.Sprintf("--config-file=%s", configFile.Name()),
	)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "logging type is not valid")
	// test error has "the logging type is not valid"
}

func (suite *logCollectTestSuite) TestNoLogTypeConfig() {
	currentDir, err := os.Getwd()
	suite.assert.NoError(err)
	defer suite.cleanupTest(currentDir)

	//set up config file
	TestNoLogTypeConfig := logCollectTestConfig{
		logType:  "",
		level:    "log_debug",
		filePath: "/home/fakeUser/cloudfuse.log",
	}
	configFile := suite.setupConfig(TestNoLogTypeConfig)

	//run the log collector
	_, err = executeCommandC(
		rootCmd,
		"gather-logs",
		fmt.Sprintf("--config-file=%s", configFile.Name()),
	)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "logging type is not provided")

}

func (suite *logCollectTestSuite) TestNoLogPathConfig() {
	currentDir, err := os.Getwd()
	suite.assert.NoError(err)
	defer suite.cleanupTest(currentDir)

	//set up config file
	TestNoLogTypeConfig := logCollectTestConfig{logType: "base", level: "log_debug", filePath: ""}
	configFile := suite.setupConfig(TestNoLogTypeConfig)

	//run the log collector
	_, err = executeCommandC(
		rootCmd,
		"gather-logs",
		fmt.Sprintf("--config-file=%s", configFile.Name()),
	)

	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "file-path is not provided")

}

// Log collection test using 'silent' for the logging type in the config.
func (suite *logCollectTestSuite) TestSilentConfig() {
	currentDir, err := os.Getwd()
	suite.assert.NoError(err)
	defer suite.cleanupTest(currentDir)

	//set up config file
	silentConfig := logCollectTestConfig{logType: "silent", level: "log_debug", filePath: ""}
	configFile := suite.setupConfig(silentConfig)

	//run the log collector
	_, err = executeCommandC(
		rootCmd,
		"gather-logs",
		fmt.Sprintf("--config-file=%s", configFile.Name()),
	)
	suite.assert.Error(err)
}

// Log collection test using --output-path flag
func (suite *logCollectTestSuite) TestArchivePath() {
	currentDir, err := os.Getwd()
	suite.assert.NoError(err)
	defer suite.cleanupTest(currentDir)

	// create temp folder for output Path
	outputPath := common.GetDefaultWorkDir()
	outputPath = common.ExpandPath(outputPath)
	tempDir, err := os.MkdirTemp(outputPath, "tempArcDir")
	suite.assert.NoError(err)
	tempDir, err = filepath.Abs(tempDir)
	suite.assert.NoError(err)
	defer suite.cleanupTest(tempDir)
	defer os.RemoveAll(tempDir)

	// create temp files in default directory $HOME/.cloudfuse/cloudfuse.log
	baseDefaultDir := common.GetDefaultWorkDir() + "/.cloudfuse/"
	baseDefaultDir = common.ExpandPath(baseDefaultDir)
	if !common.DirectoryExists(baseDefaultDir) {
		_ = os.Mkdir(baseDefaultDir, os.FileMode(0760))
	}
	var logFile *os.File
	logFile, err = os.CreateTemp(baseDefaultDir, "cloudfuse*.log")
	suite.assert.NoError(err)
	defer os.Remove(logFile.Name())

	// run gatherLogs command
	_, err = executeCommandC(rootCmd, "gather-logs", fmt.Sprintf("--output-path=%s", tempDir))
	suite.assert.NoError(err)

	switch runtime.GOOS {
	case "linux":
		suite.assert.FileExists(tempDir + "/cloudfuse_logs.tar.gz")
	case "windows":
		suite.assert.FileExists(tempDir + "/cloudfuse_logs.zip")
	}
	isArcValid := suite.verifyArchive(baseDefaultDir, tempDir)
	suite.assert.True(isArcValid)
}

// TestGatherLogsHelp tests the help output
func (suite *logCollectTestSuite) TestGatherLogsHelp() {
	currentDir, err := os.Getwd()
	suite.assert.NoError(err)
	defer suite.cleanupTest(currentDir)

	op, err := executeCommandC(rootCmd, "gather-logs", "--help")
	suite.assert.NoError(err)
	suite.assert.Contains(op, "gather-logs")
	suite.assert.Contains(op, "output-path")
}

func TestLogCollectCommand(t *testing.T) {
	suite.Run(t, new(logCollectTestSuite))
}
