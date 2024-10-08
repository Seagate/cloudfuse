//go:build linux

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2024 Seagate Technology LLC and/or its Affiliates

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
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

type serviceOptions struct {
	ConfigFile string
	MountPath  string
}

var servOpts serviceOptions

// Section defining all the command that we have in secure feature
var serviceCmd = &cobra.Command{
	Use:               "service",
	Short:             "Manage cloudfuse startup process on Linux",
	Long:              "Manage cloudfuse startup process on Linux",
	SuggestFor:        []string{"ser", "serv"},
	Example:           "cloudfuse service install",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("missing command options\n\nDid you mean this?\n\tcloudfuse service install\n\nRun 'cloudfuse service --help' for usage")
	},
}

var installCmd = &cobra.Command{
	Use:               "install",
	Short:             "Installs the startup process for Cloudfuse. Requires elevated permissions.",
	Long:              "Installs the startup process for Cloudfuse which remounts any active previously active mounts on startup. elevated permissions.",
	SuggestFor:        []string{"ins", "inst"},
	Example:           "cloudfuse service install",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {

		// 1. get the cloudfuse.service file from the setup folder and collect relevant data (user, mount, config)
		serviceFile, err := os.Open("./setup/cloudfuse.service")

		if err != nil {
			fmt.Println("Error opening file:", err)
			return err
		}

		defer serviceFile.Close()

		scanner := bufio.NewScanner(serviceFile)
		serviceData := make(map[string]string)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "Environment=") {
				parts := strings.SplitN(line, "=", 3)
				key := strings.TrimSpace(parts[1])
				value := strings.TrimSpace(parts[2])
				serviceData[key] = value
			}
			if strings.Contains(line, "User=") {
				parts := strings.SplitN(line, "=", 2)
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				serviceData[key] = value
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading file:", err)
			return err
		}

		//check the 'User' key. compare to the the /etc/passwd list for the value and create it if it doesn't exist.

		value := serviceData["User"]
		usersList, err := os.Open("/etc/passwd")
		if err != nil {
			return fmt.Errorf("failed to open /etc/passwd due to following error: [%s]", err.Error())
		}
		scanner = bufio.NewScanner(usersList)
		var foundUser bool
		defer usersList.Close()

		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, value) {
				foundUser = true
			}
		}
		if !foundUser {
			//create the user
			cmd := exec.Command("useradd", "-m", value)
			err := cmd.Run()
			if err != nil {
				return fmt.Errorf("failed to create user due to following error: [%s]", err.Error())
			}

		}

		// 2. retrieve the user account from cloudfuse.service file and make it if it doesn't exist
		// 3. copy the cloudfuse.service file to /etc/systemd/system
		// 4. run systemctl daemon-reload

		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:               "uninstall",
	Short:             "Uninstall the startup process for Cloudfuse. Requires running as admin.",
	Long:              "Uninstall the startup process for Cloudfuse. Requires running as admin.",
	SuggestFor:        []string{"uninst", "uninstal"},
	Example:           "cloudfuse service uninstall",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {

		// 1. find and remove cloudfuse.service from /etc/systemd/system and run systemctl daemon-reload
		// 2. handle errors

		return nil
	},
}

//--------------- command section ends

func init() {
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(installCmd)
	serviceCmd.AddCommand(uninstallCmd)
}
