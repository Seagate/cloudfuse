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
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/spf13/cobra"
)

var dumpPath string
var logConfigFile string
var dumpLogsCmd = &cobra.Command{
	Use:               "dumpLogs",
	Short:             "interface to gather and review cloudfuse logs",
	Long:              "interface to gather and review cloudfuse logs",
	SuggestFor:        []string{"dump", "dumpLog", "dumpLogs"},
	Example:           "cloudfuse dumpLogs ",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {

		var err error
		if dumpPath == "" {
			dumpPath, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("couldn't get the current directory [%s]", err.Error())
			}
		} else {
			dumpPathExists := common.DirectoryExists(dumpPath)
			if !dumpPathExists {
				return fmt.Errorf("the output path provided does not exist")
			}

			dumpInfo, err := os.Stat(dumpPath)
			if err != nil {
				return fmt.Errorf("couldn't stat the output path")
			}

			if !dumpInfo.IsDir() {
				return fmt.Errorf("the provided output path needs to be a directory")
			}
		}

		dumpPath, err = filepath.Abs(dumpPath)
		if err != nil {
			return fmt.Errorf("couldn't determine absolute path for logs [%s]", err.Error())
		}

		if logConfigFile, err = filepath.Abs(logConfigFile); err != nil {
			return fmt.Errorf("couldn't determine absolute path for config file [%s]", err.Error())
		}

		_, err = os.Stat(logConfigFile)
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("the config file path provided does not exist")
		}

		config.SetConfigFile(logConfigFile)
		config.ReadFromConfigFile(logConfigFile)

		var logPath string
		var logType string
		if config.IsSet("logging.type") {
			err := config.UnmarshalKey("logging.type", &logType)
			if err != nil {
				return fmt.Errorf("failed to parse logging type from config [%s]", err.Error())
			}
			if logType == "syslog" {
				logPath = "/var/log/syslog"
			} else if logType == "base" {
				err = config.UnmarshalKey("logging.file-path", &logPath)
				if err != nil {
					return fmt.Errorf("failed to parse logging file path from config [%s]", err.Error())
				}
			}
		} else {
			logPath = "$HOME/.cloudfuse/cloudfuse.log" //what does $HOME mean in the context for the end user running the command?
			logType = "base"
		}

		logPath, err = filepath.Abs(logPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: [%s]", err.Error())
		}

		if logType == "base" {
			logPath = filepath.Dir(logPath)
			err = createArchive(logPath)
			if err != nil {
				return fmt.Errorf("unable to create archive: [%s]", err.Error())
			}
		} else if logType == "syslog" && runtime.GOOS == "linux" {

			// call filterLog that outputs a log file. then call createArchive() to put that log into an archive.
			filteredSyslogPath, err := createFilteredLog(logPath) //generate a separate .log file and place it in a folder. output the path of the filtered log file
			if err != nil {
				return fmt.Errorf("failed to crate a filtered log from the syslog: [%s]", err.Error())
			}
			filteredSyslogPath = filepath.Dir(filteredSyslogPath)
			err = createArchive(filteredSyslogPath) //supply the path of the filtered log file here
			if err != nil {
				return fmt.Errorf("unable to create archive: [%s]", err.Error())
			}
		} else if logType == "syslog" && runtime.GOOS == "windows" {
			fmt.Println("Please refer to the windows event viewer for your cloudfuse logs")
			return fmt.Errorf("no log files to collect. system logging for windows are stored in the event viewer: [%s]", err.Error())
		} else if logType == "silent" {
			return fmt.Errorf("no logs were generated due to log type being silent: [%s]", err.Error())
		}

		// are any 'base' logging or syslog filters being used to redirect to a separate file?
		// check for /etc/rsyslog.d and /etc/logrotate.d files

		return nil
	},
}

func createFilteredLog(logFile string) (string, error) {

	//this will take a log file and filter out cloudfuse lines into a separate file
	// this will mostly be used for the linux syslog.

	keyword := "cloudfuse"

	os.MkdirAll("/tmp/cloudfuseSyslog", 0777)

	outPath := "/tmp/cloudfuseSyslog/cloudfuseSyslog.log" //Decide what directory you want to dump this file. an empty folder would be easiest.

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

func createArchive(logPath string) error {
	ArchiveName := fmt.Sprintf("cloudfuse_Logs_" + time.Now().Format("2006-01-02_15-04-05"))

	var err error
	if runtime.GOOS == "linux" {

		outFile, err := os.Create(dumpPath + "/" + ArchiveName + ".tar.gz")
		if err != nil {
			return err
		}
		defer outFile.Close()

		gzWriter := gzip.NewWriter(outFile)
		defer gzWriter.Close()

		tarWriter := tar.NewWriter(gzWriter)
		defer tarWriter.Close()

		items, err := os.ReadDir(logPath)
		if err != nil {
			return err
		}

		for _, item := range items {
			if item.IsDir() {
				continue
			}

			filePath := filepath.Join(logPath, item.Name())
			file, err := os.Open(filePath)
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

		}

	} else if runtime.GOOS == "windows" {

		outFile, err := os.Create(dumpPath + "/" + ArchiveName)
		if err != nil {
			return nil
		}
		defer outFile.Close()

		zipWriter := zip.NewWriter(outFile)
		defer zipWriter.Close()

		items, err := os.ReadDir(logPath)
		if err != nil {
			return err
		}

		for _, item := range items {
			if item.IsDir() {
				continue
			}

			filePath := filepath.Join(logPath, item.Name())
			file, err := os.Open(filePath)
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

		}

	}

	return err
}

func init() {
	rootCmd.AddCommand(dumpLogsCmd)
	dumpLogsCmd.Flags().StringVar(&dumpPath, "output-path", "", "Input archive creation path")
	dumpLogsCmd.Flags().StringVar(&logConfigFile, "config-file", common.DefaultConfigFilePath, "config-file input path")
}
