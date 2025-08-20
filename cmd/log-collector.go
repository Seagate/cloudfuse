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
*/

package cmd

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/spf13/cobra"
)

type gatherLogsParams struct {
	outputPath    string
	logConfigFile string
}

var gatherLogOps gatherLogsParams

const (
	windowsArchivePath = "cloudfuse_logs.zip"
	linuxArchivePath   = "cloudfuse_logs.tar.gz"
)

var gatherLogsCmd = &cobra.Command{
	Use:               "gather-logs",
	Short:             "interface to gather and review cloudfuse logs",
	Long:              "interface to gather and review cloudfuse logs",
	SuggestFor:        []string{"gather", "gather-log", "gather-logs"},
	Example:           "cloudfuse gather-logs ",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := checkPath(gatherLogOps.outputPath)
		if err != nil {
			return fmt.Errorf(
				"could not use the output path %s, [%s]",
				gatherLogOps.outputPath,
				err,
			)
		}

		if gatherLogOps.logConfigFile, err = filepath.Abs(gatherLogOps.logConfigFile); err != nil {
			return fmt.Errorf("couldn't determine absolute path for config file [%s]", err.Error())
		}

		logType, logPath, err := getLogInfo(gatherLogOps.logConfigFile)
		if err != nil {
			return fmt.Errorf("cannot use this config file [%s]", err.Error())
		}
		switch logType {
		case "silent":
			return fmt.Errorf("no logs were generated due to log type being silent")
		case "base":
			switch runtime.GOOS {
			case "linux":
				err = createLinuxArchive(logPath)
				if err != nil {
					return fmt.Errorf("unable to create archive: [%s]", err.Error())
				}
			case "windows":
				// set up temporary destination to collect logs
				var dstSysprofPath string
				var dstUserPath string
				dstSysprofPath, dstUserPath, err = setupPreZip()
				if err != nil {
					return fmt.Errorf(
						"could not set up the temporary folder where logs will be collected",
					)
				}
				preArchPath := filepath.Dir(dstUserPath)
				defer os.RemoveAll(preArchPath)

				// get the service logs
				systemRoot := os.Getenv("SystemRoot")
				if systemRoot == "" {
					return errors.New("Could not find system root")
				}
				systemRoot = filepath.Clean(systemRoot)
				srcSrvPath := filepath.Join(
					systemRoot,
					"System32",
					"config",
					"systemprofile",
					".cloudfuse",
				)
				err = copyFiles(srcSrvPath, dstSysprofPath)
				if err != nil {
					return fmt.Errorf(
						"unable to copy files from source path %s to destination %s: [%s]",
						srcSrvPath,
						dstSysprofPath,
						err.Error(),
					)
				}
				err = copyFiles(logPath, dstUserPath)
				if err != nil {
					return fmt.Errorf(
						"unable to copy files from source path %s to destination %s: [%s]",
						logPath,
						dstUserPath,
						err.Error(),
					)
				}

				// archive the two folders.
				err = createWindowsArchive(preArchPath)
				if err != nil {
					return fmt.Errorf("unable to create archive [%s]", err.Error())
				}
			}
		case "syslog":
			switch runtime.GOOS {
			case "linux":
				filteredSyslogPath, err := createFilteredLog(logPath)
				if err != nil {
					return fmt.Errorf(
						"failed to crate a filtered log from the syslog: [%s]",
						err.Error(),
					)
				}
				filteredSyslogPath = filepath.Dir(filteredSyslogPath)
				err = createLinuxArchive(filteredSyslogPath)
				if err != nil {
					return fmt.Errorf("unable to create archive: [%s]", err.Error())
				}
			case "windows":
				fmt.Println("Please refer to the windows event viewer for your cloudfuse logs")
				return fmt.Errorf(
					"no log files to collect. system logging for windows are stored in the event viewer",
				)
			}
		}
		return nil
	},
}

// checkPath makes sure the path for archive creation is valid.
func checkPath(outPath string) error {

	if !common.DirectoryExists(outPath) {
		return fmt.Errorf("the provided output path needs to be a directory")
	}

	var err error
	gatherLogOps.outputPath, err = filepath.Abs(gatherLogOps.outputPath)
	if err != nil {
		return fmt.Errorf("couldn't determine absolute path for logs [%s]", err.Error())
	}
	return nil
}

// getLogInfo returns the logType, and logPath values that are found in the config file.
func getLogInfo(configFile string) (string, string, error) {
	logPath := common.ExpandPath(filepath.Join(common.GetDefaultWorkDir(), ".cloudfuse/"))
	logType := "base"
	var err error
	if _, err = os.Stat(configFile); errors.Is(err, fs.ErrNotExist) {
		fmt.Println("Warning, the config file was not found. Defaults will be used")
		return logType, logPath, nil
	}

	config.SetConfigFile(configFile)
	if err = config.ReadFromConfigFile(configFile); err != nil {
		return "", "", err
	}

	if !config.IsSet("logging") {
		fmt.Printf(
			"Warning, the config file does not have a logging section. Defaults will be used\n",
		)
		return logType, logPath, nil
	}
	if !config.IsSet("logging.type") {
		return "", "", fmt.Errorf("the logging type is not provided")
	}

	if err = config.UnmarshalKey("logging.type", &logType); err != nil {
		return "", "", err
	}
	switch logType {
	case "silent":
		return logType, logPath, nil
	case "syslog":
		logPath = "/var/log/syslog"
		return logType, logPath, nil
	case "base":
		if !config.IsSet("logging.file-path") {
			return logType, logPath, fmt.Errorf("the logging file-path is not provided")
		}
		if err = config.UnmarshalKey("logging.file-path", &logPath); err != nil {
			return "", "", err
		}
		if strings.HasPrefix(logPath, common.GetDefaultWorkDir()) {
			logPath = common.ExpandPath(logPath)
		}
		logPath, err = filepath.Abs(logPath)
		if err != nil {
			return "", "", err
		}

		var leaf os.FileInfo
		leaf, err = os.Stat(logPath)
		if err != nil {
			return "", "", err
		}
		if !leaf.IsDir() {
			logPath = filepath.Dir(logPath)
		}
	default:
		return logType, logPath, fmt.Errorf(
			"the logging type is not valid. Must be 'base', or 'syslog'.",
		)
	}

	return logType, logPath, nil
}

func createLinuxArchive(logPath string) error {
	_, err := os.Stat(logPath)
	if err != nil {
		return err
	}

	outFile, err := os.Create(filepath.Join(gatherLogOps.outputPath, linuxArchivePath))
	if err != nil {
		return err
	}
	defer outFile.Close()

	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()
	var amountLogs int
	items, err := os.ReadDir(logPath)
	if err != nil {
		return err
	}
	for _, item := range items {
		if strings.Contains(item.Name(), "cloudfuse") &&
			regexp.MustCompile(`\.log(?:\.\d)?$`).MatchString(item.Name()) {
			itemPath := filepath.Join(logPath, item.Name())
			itemPath = filepath.Clean(itemPath)
			file, err := os.Open(itemPath)
			if err != nil {
				return err
			}
			defer file.Close()

			info, err := file.Stat()
			if err != nil {
				return err
			}
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			header.Name = item.Name()
			err = tarWriter.WriteHeader(header)
			if err != nil {
				return err
			}
			_, err = io.Copy(tarWriter, file)
			if err != nil {
				return err
			}
			amountLogs++
		} else {
			continue
		}
	}
	if amountLogs == 0 {
		return fmt.Errorf("no log files were found in %s", logPath)
	}
	return nil
}

// setupPreZip will create a temporary folder that will contain the logs and be the source path for creating the archive
// This will only run on windows.
func setupPreZip() (string, string, error) {
	preArchPath, err := os.MkdirTemp(gatherLogOps.outputPath, "tmpPreZip*")
	if err != nil {
		return "", "", fmt.Errorf(
			"could not create temporary path, %s, to extract data",
			preArchPath,
		)
	}

	// create a sub folder for the service logs
	sysProfDir := fmt.Sprintf("%s%csystemprofile", preArchPath, os.PathSeparator)
	err = os.Mkdir(sysProfDir, 0760)
	if err != nil {
		return "", "", fmt.Errorf("unable to create folder, %s: [%s]", sysProfDir, err.Error())
	}

	// create a sub folder for the user logs
	userDir := fmt.Sprintf("%s%cuser", preArchPath, os.PathSeparator)
	err = os.Mkdir(userDir, 0760)
	if err != nil {
		return "", "", fmt.Errorf("unable to create folder, %s: [%s]", userDir, err.Error())
	}

	return sysProfDir, userDir, nil
}

func copyFiles(srcPath, dstPath string) error {
	var items []os.DirEntry
	items, err := os.ReadDir(srcPath)
	if err != nil {
		return err
	}

	for _, item := range items {
		if item.IsDir() {
			continue
		}

		srcFilePath := filepath.Join(srcPath, item.Name())
		dstFilePath := filepath.Join(dstPath, item.Name())

		var srcFile *os.File
		srcFile, err = os.Open(srcFilePath)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		var dstFile *os.File
		dstFile, err = os.Create(dstFilePath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			return err
		}

	}
	return nil
}

func createWindowsArchive(archPath string) error {
	outFile, err := os.Create(filepath.Join(gatherLogOps.outputPath, linuxArchivePath))
	if err != nil {
		return err
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()
	var amountLogs int
	err = filepath.Walk(archPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		var relPath string
		relPath, err = filepath.Rel(archPath, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if relPath == "." {
				return nil
			}
			_, err := zipWriter.Create(relPath + "/")
			if err != nil {
				return err
			}
			return nil
		}
		if strings.Contains(relPath, "cloudfuse") &&
			regexp.MustCompile(`\.log(?:\.\d)?$`).MatchString(relPath) {
			var file *os.File
			file, err = os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			var zipEntry io.Writer
			zipEntry, err = zipWriter.Create(relPath)
			if err != nil {
				return err
			}
			_, err = io.Copy(zipEntry, file)
			if err != nil {
				return err
			}
			amountLogs++
		}
		return err
	})
	if amountLogs == 0 {
		return fmt.Errorf("no cloudfuse log file were found in %s", archPath)
	}
	return err
}

// createFilteredLog creates a new log file containing only cloudfuse logs from the logFile input.
// It only runs for linux when the logging type is set to "syslog" in the config
func createFilteredLog(logFile string) (string, error) {
	keyword := "cloudfuse"
	outFile, err := os.CreateTemp("", "cloudfuseSyslog*.log")
	if err != nil {
		return "", err
	}
	var inFile *os.File
	inFile, err = os.Open(logFile)
	if err != nil {
		return "", err
	}
	defer inFile.Close()
	defer outFile.Close()

	scanner := bufio.NewScanner(inFile)
	writer := bufio.NewWriter(outFile)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, keyword) {
			_, err := writer.WriteString(line + "\n")
			if err != nil {
				return "", err
			}
		}
	}
	writer.Flush()
	return outFile.Name(), scanner.Err()
}

func init() {

	rootCmd.AddCommand(gatherLogsCmd)
	curDir, _ := os.Getwd()
	gatherLogsCmd.Flags().
		StringVar(&gatherLogOps.outputPath, "output-path", curDir, "Input archive creation path")
	gatherLogsCmd.Flags().
		StringVar(&gatherLogOps.logConfigFile, "config-file", common.DefaultConfigFilePath, "config-file input path")
}
