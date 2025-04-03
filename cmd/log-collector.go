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

		dumpPath := common.ExpandPath(args[0])
		var err error
		if dumpPath, err = filepath.Abs(dumpPath); err != nil {
			return fmt.Errorf("couldn't determine absolute path for dump logs [%s]", err.Error())
		}
		//TODO: make sure dumpPath is empty. if it doesn't exist, create it.

		//the options used here are from the mount options from within the same cmd package.

		// if options.ConfigFile != "" {
		// 	config.SetConfigFile(options.ConfigFile)
		// } else if configPath != "" {
		// 	configPath = common.ExpandPath(configPath)
		// 	configPath, err := filepath.Abs(configPath)
		// 	if err != nil {
		// 		return fmt.Errorf("couldn't determine absolute path for config file [%s]", err.Error())
		// 	}
		// 	config.SetConfigFile(configPath)
		// } else {
		// 	// consider checking everywhere and gathering everything at this point
		// 	return errors.New("config file not provided")
		// }

		if logConfigFile, err = filepath.Abs(logConfigFile); err != nil {
			return fmt.Errorf("couldn't determine absolute path for config file [%s]", err.Error())
		}

		config.SetConfigFile(logConfigFile)

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

		getLogs(logPath)

		// are any 'base' logging or syslog filters being used to redirect to a separate file?
		// check for /etc/rsyslog.d and /etc/logrotate.d files

		//once all logs are collected. create archive. OS dependant: what archive format should I use?
		// windows: zip
		// linux: tar

		return err
	},
}

func getLogs(logPath string) error {

	/*


		2. collect contents from the path
		3. archive contenst to the dumpPath
	*/

	logPath, err := filepath.Abs(logPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %s", err.Error)
	}

	validDir, err := isDirValid(logPath)
	if err != nil {
		return err
	}

	outputFile := fmt.Sprintf("cloudfuse " + time.Now().Format("2006-01-02_15-04-05"))

	if validDir {
		if runtime.GOOS == "windows" {
			outputFile += ".zip"
			err = zipDirectory(logPath, dumpPath+outputFile)
			if err != nil {
				return err
			}

		} else if runtime.GOOS == "linux" {
			outputFile += ".zip"
			err = tarGzDirectory(logPath, dumpPath+outputFile)
			if err != nil {
				return err
			}

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

func tarGzDirectory(srcDir, tarGzFile string) error {
	outFile, err := os.Create(tarGzFile)
	if err != nil {
		return err
	}
	defer outFile.Close()

	gzipWriter := gzip.NewWriter(outFile)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(srcDir, path)
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

func isDirValid(logPath string) (bool, error) {

	info, err := os.Stat(logPath)
	if os.IsNotExist(err) {
		return false, fmt.Errorf("the path, %s, does not exist", logPath)
	} else if !info.IsDir() {
		return false, fmt.Errorf("the path provided is not a directory")
	} else if err != nil {
		return false, fmt.Errorf(err.Error())
	}

	dir, err := os.Open(logPath)
	if err != nil {
		return false, err
	}
	defer dir.Close()

	files, err := dir.Readdirnames(1)
	if err != nil && err != io.EOF {
		return false, err
	}

	return len(files) == 0, nil

}

func init() {
	rootCmd.AddCommand(dumpLogsCmd)
	// dumpLogsCmd.Flags().StringVar(&dumpPath, "dump-path", "", "Input archive creation path")
	// markFlagErrorChk(dumpLogsCmd, "dump-path")
	dumpLogsCmd.Flags().StringVar(&logConfigFile, "config-file", common.DefaultConfigFilePath, "config-file input path")
}
