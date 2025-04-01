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
	"errors"
	"fmt"
	"path/filepath"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/spf13/cobra"
)

// TODO: rename log command to dumpLogs. followed by a directory to dump logs. ex. cloudfuse dumpLogs /path/to/dir. all logs are gathered by default.
// consider adding a --since flag to specify logs since a given time stamp to be included ex. cloudfuse dumpLogs /path/to/dir --since yyyy/MM/DD:HH:MM:SS
// consider adding a --last flag to specify logs in the last minutes, hours, days. ex. cloudfuse dumpLogs /path/do/dir --last 30 minutes

var since string
var last string
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
		dumpPath, err = filepath.Abs(dumpPath)
		if err != nil {
			return fmt.Errorf("couldn't determine absolute path for dump path [%s]", err.Error())
		}

		configPath = common.ExpandPath(configPath)
		configPath, err := filepath.Abs(configPath)
		if err != nil {
			return fmt.Errorf("couldn't determine absolute path for config file [%s]", err.Error())
		}

		if options.ConfigFile != "" {
			config.SetConfigFile(options.ConfigFile)
		} else if configPath != "" {
			config.SetConfigFile(configPath)
		} else {
			// consider checking everywhere and gathering everything at this point
			return errors.New("config file not provided")
		}

		var logType string
		var logPath string
		if config.IsSet("logging.type") {
			err := config.UnmarshalKey("logging.type", &logType)
			if err != nil {
				return fmt.Errorf("failed to parse logging type from config [%s]", err.Error())
			}
			err = config.UnmarshalKey("logging.file-path", &logPath)
			if err != nil {
				return fmt.Errorf("failed to parse logging file path from config [%s]", err.Error())
			}
			if logType == "base" {
				getBaseLogs(logPath)
			} else if logType == "syslog" {
				getSysLogs("/var/log/syslog")
			}
		} else {
			getSysLogs("/var/log/syslog")
		}

		// are any 'base' logging or syslog filters being used to redirect to a separate file?
		// check for /etc/rsyslog.d and /etc/logrotate.d files

		//once all logs are collected. create archive. OS dependant: what archive format should I use?
		// windows: zip
		// linux: tar

		return err
	},
}

func getBaseLogs(logPath string) error {
	// collect logs
	if since != "" {
		// select only the latest logs in the logPath that are no older than the timestamp provided
	} else if last != "" {
		// select only the latest logs in the logPath that are no older than $last value provided
	}
	var err error
	return err
}

func getSysLogs(logPath string) error {
	// collect logs
	// if time specify flags present. timestamp example in syslog is 'Mar 17 13:41:36'
	//  grep cloudfuse /var/log/syslog > logs
	if since != "" {
		// select only the latest logs in the logPath that are no older than the timestamp provided
	} else if last != "" {
		// select only the latest logs in the logPath that are no older than $last value provided
	}
	var err error
	return err
}

func init() {
	rootCmd.AddCommand(dumpLogsCmd)
	dumpLogsCmd.Flags().StringVar(&configPath, "config-file", "", "Input archive creation path")
	dumpLogsCmd.Flags().StringVar(&since, "since", "", "specify only log data that took place since a given time stamp")
	dumpLogsCmd.Flags().StringVar(&last, "last", "", "specify only log data in the last minutes, hours, or days.")

}
