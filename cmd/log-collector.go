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

var dumpPath string
var logConfigFile string
var gatherLogsCmd = &cobra.Command{
	Use:               "gatherLogs",
	Short:             "interface to gather and review cloudfuse logs",
	Long:              "interface to gather and review cloudfuse logs",
	SuggestFor:        []string{"gather", "gatherLog", "gatherLogs"},
	Example:           "cloudfuse gatherLogs ",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {

		err := checkPath(dumpPath)
		if err != nil {
			return fmt.Errorf("could not use the output path %s, [%s]", dumpPath, err)
		}

		if logConfigFile, err = filepath.Abs(logConfigFile); err != nil {
			return fmt.Errorf("couldn't determine absolute path for config file [%s]", err.Error())
		}

		logType, logPath, err := getLogInfo(logConfigFile)
		if err != nil {
			return fmt.Errorf("cannot use this config file [%s]", err.Error())
		}
		if logType == "silent" {
			return fmt.Errorf("no logs were generated due to log type being silent")
		} else if logType == "base" {
			logPath = filepath.Dir(logPath)
			if runtime.GOOS == "linux" {
				err = createLinuxArchive(logPath)
				if err != nil {
					return fmt.Errorf("unable to create archive: [%s]", err.Error())
				}
			} else if runtime.GOOS == "windows" {

				// set up temporary location to collect logs
				var sysProfDir string
				var userDir string
				sysProfDir, userDir, err = setupPreZip()
				if err != nil {
					return fmt.Errorf("could not set up the temporary folder where logs will be collected")
				}
				preArchPath := filepath.Dir(userDir)
				defer os.RemoveAll(preArchPath)

				// get the service logs regardless of what the config values are
				systemRoot := os.Getenv("SystemRoot")
				if systemRoot == "" {
					return errors.New("Could not find system root")
				}

				systemRoot = filepath.Clean(systemRoot)
				servicePath := filepath.Join(systemRoot, "System32", "config", "systemprofile", ".cloudfuse")

				// copied over the service log files from servicePath -> preArchPath
				err = copyFiles(servicePath, sysProfDir)
				if err != nil {
					return fmt.Errorf("unable to copy files: [%s]", err.Error())
				}

				// fetch provided or default logPath
				logPath = common.ExpandPath(logPath)
				logPath, err = filepath.Abs(logPath)
				if err != nil {
					return fmt.Errorf("failed get absolute path for logs directory: [%s]", err.Error())
				}

				// copied over the user base logs from logPath -> preArchPath
				err = copyFiles(logPath, userDir)
				if err != nil {
					return fmt.Errorf("unable to copy files: [%s]", err.Error())
				}

				// archive the two folders.
				err = createWindowsArchive(preArchPath)
				if err != nil {
					return fmt.Errorf("unable to create archive [%s]", err.Error())
				}
			}
		} else if logType == "syslog" {
			if runtime.GOOS == "linux" {
				filteredSyslogPath, err := createFilteredLog(logPath)
				if err != nil {
					return fmt.Errorf("failed to crate a filtered log from the syslog: [%s]", err.Error())
				}
				filteredSyslogPath = filepath.Dir(filteredSyslogPath)
				err = createLinuxArchive(filteredSyslogPath)
				if err != nil {
					return fmt.Errorf("unable to create archive: [%s]", err.Error())
				}
			} else if runtime.GOOS == "windows" {
				fmt.Println("Please refer to the windows event viewer for your cloudfuse logs")
				return fmt.Errorf("no log files to collect. system logging for windows are stored in the event viewer")
			}
		}
		return nil
	},
}

// checkPath makes sure the path for archive creation is valid.
func checkPath(outPath string) error {
	var err error
	if outPath == "" {
		dumpPath, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		if !common.DirectoryExists(outPath) {
			return err
		}

		dumpInfo, err := os.Stat(outPath)
		if err != nil {
			return err
		}

		if !dumpInfo.IsDir() {
			return fmt.Errorf("the provided output path needs to be a directory")
		}
	}

	dumpPath, err = filepath.Abs(dumpPath)
	if err != nil {
		return fmt.Errorf("couldn't determine absolute path for logs [%s]", err.Error())
	}
	return nil
}

// getLogInfo populates the logType, and logPath values found in the config file.
func getLogInfo(configFile string) (string, string, error) {
	logPath := "$HOME/.cloudfuse/cloudfuse.log"
	logPath = common.ExpandPath(logPath)
	logType := "base"
	_, err := os.Stat(configFile)
	if errors.Is(err, fs.ErrNotExist) {
		fmt.Printf("Warning, the config file was not found. Defaults will be used ")
	} else {
		config.SetConfigFile(configFile)
		config.ReadFromConfigFile(configFile)
		if config.IsSet("logging.type") {
			err := config.UnmarshalKey("logging.type", &logType)
			if err != nil {
				return "", "", err
			}
			if logType == "silent" {
				return logType, logPath, nil
			} else if logType == "syslog" {
				logPath = "/var/log/syslog"
			} else if logType == "base" {
				if config.IsSet("logging.file-path") {
					err = config.UnmarshalKey("logging.file-path", &logPath)
					if err != nil {
						return "", "", err
					}
					if strings.HasPrefix(logPath, "$HOME") {
						logPath = common.ExpandPath(logPath)
					}
					logPath, err = filepath.Abs(logPath)
					if err != nil {
						return "", "", err
					}
					_, err = os.Stat(logPath)
					if err != nil {
						return logType, logPath, fmt.Errorf("the file path, %s, cannot be found: [%s]", logPath, err.Error())
					}
				} else {
					return logType, logPath, fmt.Errorf("the logging file-path is not provided")
				}
			} else { // TODO: this should be a failure
				return logType, logPath, fmt.Errorf("the logging type is not valid. Must be 'base', or 'syslog'.")
			}
		} else {
			return "", "", fmt.Errorf("the logging type is not provided")
		}
	}
	return logType, logPath, nil
}

func createLinuxArchive(logPath string) error {
	//first check logPath is valid
	_, err := os.Stat(logPath)
	if err != nil {
		return err
	}

	//setup tar.gz file
	outFile, err := os.Create(dumpPath + "/cloudfuse_logs.tar.gz")
	if err != nil {
		return err
	}
	defer outFile.Close()

	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	//populate tar.gz file
	var amountLogs int
	items, err := os.ReadDir(logPath)
	if err != nil {
		return err
	}
	for _, item := range items {
		if strings.HasPrefix(item.Name(), "cloudfuse") && strings.HasSuffix(item.Name(), ".log") {
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
		return fmt.Errorf("no cloudfuse log file were found in %s", logPath)
	}

	return nil
}

func setupPreZip() (string, string, error) {
	preArchPath, err := os.MkdirTemp(dumpPath, "tmpPreZip*")
	if err != nil {
		return "", "", fmt.Errorf("could not create temporary path, %s,  to extract data", preArchPath)
	}

	// create a sub folder for the service logs
	sysProfDir := fmt.Sprintf("%s/systemprofile", preArchPath)
	err = os.Mkdir(sysProfDir, 0760)
	if err != nil {
		return "", "", fmt.Errorf("unable to create folder, %s: [%s]", sysProfDir, err.Error())
	}

	// create a sub folder for the user logs
	userDir := fmt.Sprintf("%s/user", preArchPath)
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
		return fmt.Errorf("failed read the service path directory: [%s]", err.Error())
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
			return fmt.Errorf("failed to open file in service path to copy: [%s]", err.Error())
		}
		defer srcFile.Close()

		var dstFile *os.File
		dstFile, err = os.Create(dstFilePath)
		if err != nil {
			return fmt.Errorf("failed to create file to copy service file: [%s]", err.Error())
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			return fmt.Errorf("failed to copy file %s: [%s]", item.Name(), err.Error())
		}

	}
	//question: when is it noisy to nest error messages in other error messages?
	return err
}

func createWindowsArchive(archPath string) error {
	outFile, err := os.Create(dumpPath + "/cloudfuse_logs.zip")
	if err != nil {
		return nil
	}

	defer outFile.Sync()
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
				return fmt.Errorf("failed to create directory in zip %w", err)
			}
			return nil
		}

		if strings.Contains(relPath, "cloudfuse") && regexp.MustCompile(`\.log(?:\.\d)?$`).MatchString(relPath) {
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

// createFilteredLog creates a new log file containing only the cloudfuse logs from the input log file.
// It only runs for linux when the configured logging type is set to "syslog"
func createFilteredLog(logFile string) (string, error) {
	keyword := "cloudfuse"
	os.Mkdir("/tmp/cloudfuseSyslog", 0760)
	outPath := "/tmp/cloudfuseSyslog/cloudfuseSyslog.log"
	inFile, err := os.Open(logFile)
	if err != nil {
		return "", err
	}
	defer inFile.Close()

	outFile, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
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

	return outPath, scanner.Err()
}

func init() {
	rootCmd.AddCommand(gatherLogsCmd)
	gatherLogsCmd.Flags().StringVar(&dumpPath, "output-path", "", "Input archive creation path")
	gatherLogsCmd.Flags().StringVar(&logConfigFile, "config-file", common.DefaultConfigFilePath, "config-file input path")
}
