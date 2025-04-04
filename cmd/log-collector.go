//go:build linux

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
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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
	Args:              cobra.ExactArgs(1),
	Example:           "cloudfuse dumpLogs ",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {

		var err error
		dumpPath = args[0]
		dumpPath, err = filepath.Abs(dumpPath)
		if err != nil {
			return fmt.Errorf("couldn't determine absolute path for dump logs [%s]", err.Error())
		}

		dumpInfo, err := os.Stat(dumpPath)
		if err != nil {
			return fmt.Errorf("couldn't stat dump Path")
		}

		if !dumpInfo.IsDir() {
			return fmt.Errorf("dumpPath provided needs to be a directory")
		}

		println(logConfigFile)
		if logConfigFile, err = filepath.Abs(logConfigFile); err != nil {
			return fmt.Errorf("couldn't determine absolute path for config file [%s]", err.Error())
		}

		println(logConfigFile)

		config.SetConfigFile(logConfigFile)
		config.ReadFromConfigFile(logConfigFile)

		var logPath string
		if config.IsSet("logging.type") {
			var logType string
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
			logPath = "/var/log/syslog"
		}

		err = getLogs(logPath)
		if err != nil {
			println(err.Error())
		}

		// are any 'base' logging or syslog filters being used to redirect to a separate file?
		// check for /etc/rsyslog.d and /etc/logrotate.d files

		//once all logs are collected. create archive. OS dependant: what archive format should I use?
		// windows: zip
		// linux: tar

		return err
	},
}

func getLogs(logPath string) error {
	//TODO: add logType support and provide syslog gather path
	logPath, err := filepath.Abs(logPath)
	logPath = filepath.Dir(logPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: [%s]", err.Error())
	}

	outputFile := fmt.Sprintf("cloudfuse " + time.Now().Format("2006-01-02_15-04-05"))

	if runtime.GOOS == "windows" {
		outputFile += ".zip"
		err = zipDirectory(logPath, dumpPath+"/"+outputFile)
		if err != nil {
			return err
		}

	} else if runtime.GOOS == "linux" {
		outputFile += ".tar.gz"
		println(dumpPath + "/" + outputFile)
		err = tarGzDirectory(logPath, dumpPath+"/"+outputFile)
		if err != nil {
			return err
		}
	}

	return nil
}

func zipDirectory(logPath, outputFile string) error {
	outFile, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	filepath.Walk(logPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(logPath, path)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		zipEntry, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		_, err = io.Copy(zipEntry, file)
		return err
	})

	return nil
}

func tarGzDirectory(logPath, outputFile string) error {
	outFile, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer outFile.Close()

	gzipWriter := gzip.NewWriter(outFile)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	err = filepath.Walk(logPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(logPath, path)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		header, err := tar.FileInfoHeader(info, relPath)
		if err != nil {
			return err
		}
		header.Name = relPath // Ensure relative path is used

		err = tarWriter.WriteHeader(header)
		if err != nil {
			return err
		}

		_, err = io.Copy(tarWriter, file)
		return err
	})

	return err
}

func init() {
	rootCmd.AddCommand(dumpLogsCmd)
	// dumpLogsCmd.Flags().StringVar(&dumpPath, "dump-path", "", "Input archive creation path")
	// markFlagErrorChk(dumpLogsCmd, "dump-path")
	dumpLogsCmd.Flags().StringVar(&logConfigFile, "config-file", common.DefaultConfigFilePath, "config-file input path")
}
