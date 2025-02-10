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
	"html/template"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/Seagate/cloudfuse/common"
	"github.com/spf13/cobra"
)

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
	Use:   				"collect",
	Short: 				"Collect and archive relevant cloudfuse logs",
	Long:  				"Collect and archive relevant cloudfuse logs",
	SuggestFor: 		[]string{"col", "coll"},
	Example:			"cloudfuse log collect",
	FlagErrorHandling: 	cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		
		// require path flag to dump archive

		// check config file for log type

		// if syslog grep cloudfuse /var/log/syslog > logs

		// if base, get log output directory provided.


		//once all logs are collected. create archive. OS dependant: what archive format should I use?  
		// windows: zip 
		// linux: tar 

		return err
	}
}


func init() {
	rootCmd.AddCommand(logCmd)
	logCmd.AddCommand(collect)
	logCmd.Flags().StringVar(&path, "dump-path", "", "Input archive creation path")
	markFlagErrorChk(logCmd, "dump-path")
}