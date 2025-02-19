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

	"github.com/spf13/cobra"
)

var dumpPath string

// Section defining all the command that we have in secure feature
var logCmd = &cobra.Command{
	Use:               "log",
	Short:             "interface to gather and review cloudfuse logs",
	Long:              "interface to gather and review cloudfuse logs",
	SuggestFor:        []string{"log", "logs"},
	Example:           "cloudfuse log collect",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("missing command options\n\nDid you mean this?\n\tcloudfuse log collect\n\nRun 'cloudfuse log --help' for usage")
	},
}

var collectCmd = &cobra.Command{
	Use:               "collect",
	Short:             "Collect and archive relevant cloudfuse logs",
	Long:              "Collect and archive relevant cloudfuse logs",
	SuggestFor:        []string{"col", "coll"},
	Example:           "cloudfuse log collect",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {

		var err error
		// require path flag to dump archive
		dumpPath, err = filepath.Abs(dumpPath)
		if err != nil {
			return fmt.Errorf("couldn't determine absolute path from string [%s]", err.Error())
		}

		foundConfig := false
		configPath = options.ConfigFile
		if configPath == "" {
			fmt.Printf("could not locate config file for log details")
		} else {
			configPath, err = filepath.Abs(configPath)
			if err != nil {
				return fmt.Errorf("couldn't determine absolute path from string [%s]", err.Error())
			}
			foundConfig = true
		}

		logBase := false
		if foundConfig {

			/*
				// check config file for the log type being set to "base" and get the directory where the log files are stored.
				// if "base" is set in config for log, logBase = true

				// if logtype is set to "syslog," then   grep cloudfuse /var/log/syslog > logs

				// if base, get log output from directory provided.
			*/

		} else if !logBase {

			// check everywhere possible for logs

		}

		// are any 'base' logging or syslog filters being used to redirect to a separate file?
		// check for /etc/rsyslog.d and /etc/logrotate.d files

		//once all logs are collected. create archive. OS dependant: what archive format should I use?
		// windows: zip
		// linux: tar

		return err
	},
}

func init() {
	rootCmd.AddCommand(logCmd)
	logCmd.AddCommand(collectCmd)
	logCmd.Flags().StringVar(&dumpPath, "dump-path", "", "Input archive creation path")
	markFlagErrorChk(logCmd, "dump-path")
	logCmd.Flags().StringVar(&configPath, "config-file", "", "Input archive creation path")
}
