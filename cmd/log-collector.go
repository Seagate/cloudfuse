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

		err := checkOutputPath(dumpPath)
		if err != nil {
			return fmt.Errorf("could not use the output path %s, [%s]", dumpPath, err)
		}

		if logConfigFile, err = filepath.Abs(logConfigFile); err != nil {
			return fmt.Errorf("couldn't determine absolute path for config file [%s]", err.Error())
		}

		logType, logPath, err := getLogInfo(logConfigFile)
		if err != nil {
			fmt.Errorf("failed to parse config file [%s]", err.Error())
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
				//add the system app data system32 thing if there is no filepath in the config.
				if strings.HasPrefix(logPath, ".cloudfuse") {
					err = createWindowsArchive("path/to/cloudfuse/app/data/system32/thing")
				} else {
					err = createWindowsArchive(logPath)
				}
				if err != nil {
					return fmt.Errorf("unable to create archive: [%s]", err.Error())
				}
			}

		} else if logType == "syslog" {
			if runtime.GOOS == "linux" {
				// call filterLog that outputs a log file. then call createArchive() to put that log into an archive.
				filteredSyslogPath, err := createFilteredLog(logPath) //generate a separate .log file and place it in a folder. output the path of the filtered log file
				if err != nil {
					return fmt.Errorf("failed to crate a filtered log from the syslog: [%s]", err.Error())
				}
				filteredSyslogPath = filepath.Dir(filteredSyslogPath)
				err = createLinuxArchive(filteredSyslogPath) //supply the path of the filtered log file here
				if err != nil {
					return fmt.Errorf("unable to create archive: [%s]", err.Error())
				}
			} else if runtime.GOOS == "windows" {
				fmt.Println("Please refer to the windows event viewer for your cloudfuse logs")
				return fmt.Errorf("no log files to collect. system logging for windows are stored in the event viewer: [%s]", err.Error())
			}
		}
		// TODO: check if any 'base' logging or syslog filters are being used to redirect to a separate file. do this by checking for /etc/rsyslog.d and /etc/logrotate.d files

		return nil
	},
}

func checkOutputPath(outPath string) error {
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
				} else {
					fmt.Printf("Warning, file path for base log not found. using default.")
				}
			} else {
				fmt.Printf("Warning, logging type not found. using default.")
				logType = "base"
			}
		}
		if err != nil {
			return "", "", fmt.Errorf("failed to get absolute path for the log path: [%s]", err.Error())
		}
	}
	return logType, logPath, nil
}

func createFilteredLog(logFile string) (string, error) {

	//this will take a log file and filter out cloudfuse lines into a separate file
	// this will mostly be used for the linux syslog.

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

func createLinuxArchive(logPath string) error {

	//first check logPath is valid
	items, err := os.ReadDir(logPath)
	if err != nil {
		return err
	}

	//setup tar.gz file
	ArchiveName := fmt.Sprintf("cloudfuse_logs")
	outFile, err := os.Create(dumpPath + "/" + ArchiveName + ".tar.gz")
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

func createWindowsArchive(logPath string) error {

	ArchiveName := fmt.Sprintf("cloudfuse_logs")

	outFile, err := os.Create(dumpPath + "/" + ArchiveName)
	if err != nil {
		return nil
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	// Replace os.ReadDir() with more logic that can determine if the file is appropriate to collect.
	items, err := os.ReadDir(logPath)
	if err != nil {
		return err
	}

	var amountLogs int
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

			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}

			header.Name = item.Name()

			zipEntry, err := zipWriter.Create(item.Name())
			if err != nil {
				return err
			}

			_, err = io.Copy(zipEntry, file)
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

func init() {
	rootCmd.AddCommand(gatherLogsCmd)
	gatherLogsCmd.Flags().StringVar(&dumpPath, "output-path", "", "Input archive creation path")
	gatherLogsCmd.Flags().StringVar(&logConfigFile, "config-file", common.DefaultConfigFilePath, "config-file input path")
}
