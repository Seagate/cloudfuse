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
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/Seagate/cloudfuse/common"
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
	Use:               "install [mount path] [config path]",
	Short:             "Installs a service file for a single mount with Cloudfuse. Requires elevated permissions.",
	Long:              "Installs a service file for a single mount with Cloudfuse which remounts any active previously active mounts on startup. elevated permissions.",
	SuggestFor:        []string{"ins", "inst"},
	Example:           "cloudfuse service install",
	FlagErrorHandling: cobra.ExitOnError,
	Args:              cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {

		// 1. get the cloudfuse.service file from the setup folder and collect relevant data (user, mount, config)

		// get current dir
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("error: [%s]", err.Error())
		}

		// TODO: get arguments of cloudfuse service install and pass that to be values written to the service file's config file and mount point paths.
		mountPath := args[0]
		configPath := args[1]

		mountExists := common.DirectoryExists(mountPath)
		if !mountExists {
			return fmt.Errorf("error, the mount path provided does not exist")
			// TODO: add useage output upon failure with input
		}
		isDirEmpty := common.IsDirectoryEmpty(mountPath)
		if !isDirEmpty {
			return fmt.Errorf("error, the mount path provided is not empty")
			// TODO: add useage output upon failure with input
		}

		_, err = os.Stat(configPath)
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("error, the configfile path provided does not exist") // TODO: add useage output upon failure with input
		}

		serviceFile, err := newSericeFile(mountPath, configPath, dir)
		if err != nil {
			return fmt.Errorf("error when attempting to create service file: [%s]", err.Error())
		}

		// 2. retrieve the newUser account from cloudfuse.service file and create it if it doesn't exist

		// assumes dir is in cloudfuse repo dir
		serviceData, err := collectServiceData(fmt.Sprintf("%s/setup/cloudfuse.service", dir))
		if err != nil {
			return fmt.Errorf("error collecting data from cloudfuse.service file due to the following error: [%s]", err)
		}
		serviceUser := serviceData["User"]
		setUser(serviceUser)

		// 4. run systemctl daemon-reload
		systemctlDaemonReloadCmd := exec.Command("sudo", "systemctl", "daemon-reload")
		err = systemctlDaemonReloadCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to run 'systemctl daemon-reload' command due to following error: [%s]", err.Error())
		}

		// 5. Enable the service to start at system boot
		systemctlEnableCmd := exec.Command("sudo", "systemctl", "enable", serviceFile)
		err = systemctlEnableCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to run 'systemctl daemon-reload' command due to following error: [%s]", err.Error())
		}
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
		removeFileCmd := exec.Command("sudo", "rm", "/etc/systemd/system/cloudfuse.service")
		err := removeFileCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to delete cloudfuse.service file from /etc/systemd/system due to following error: [%s]", err.Error())
		}

		systemctlDaemonReloadCmd := exec.Command("sudo", "systemctl", "daemon-reload")
		err = systemctlDaemonReloadCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to run 'systemctl daemon-reload' command due to following error: [%s]", err.Error())
		}

		return nil
	},
}

//--------------- command section ends

func collectServiceData(serviceFilePath string) (map[string]string, error) {
	serviceFile, err := os.Open("./setup/cloudfuse.service")

	if err != nil {
		fmt.Println("Error opening file:", err)
		return nil, err
	}

	defer serviceFile.Close()

	scanner := bufio.NewScanner(serviceFile)
	serviceData := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "User=") {
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			serviceData[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading file:", err)
		return nil, err
	}
	return serviceData, nil
}

func newSericeFile(mountPath string, configPath string, dir string) (string, error) {

	//mountpath or config?
	var oldString string
	var newString string

	oldConfigStr := "Environment=ConfigFile=/path/to/config/file/config.yaml"
	newConfigStr := fmt.Sprintf("Environment=ConfigFile=%s", common.JoinUnixFilepath(dir, configPath))

	oldMountStr := "Environment=MoutingPoint=/path/to/mounting/point"
	newMountStr := fmt.Sprintf("Environment=MoutingPoint=%s", common.JoinUnixFilepath(dir, mountPath))

	// open service defaultFile for read write
	defaultFile, err := os.OpenFile("./setup/cloudfuse.service", os.O_RDWR, 0644)
	if err != nil {
		return "", fmt.Errorf("error opening file: [%s]", err.Error())
	}
	defer defaultFile.Close()

	scanner := bufio.NewScanner(defaultFile)
	var lines []string

	// Read the file line by line
	for scanner.Scan() {
		line := scanner.Text()

		// Check if the line contains the search string
		if strings.Contains(line, "MoutingPoint") {
			// Modify the line by replacing the old string with the new string
			line = strings.ReplaceAll(line, oldMountStr, newMountStr)
		}
		if strings.Contains(line, "ConfigFile") {
			line = strings.ReplaceAll(line, oldConfigStr, newConfigStr)
		}

		// add the -o default_permissions if not present
		if strings.Contains(line, "ExecStart") && !strings.Contains(line, "-o allow_other") {
			oldString = "ExecStart=/usr/bin/cloudfuse mount ${MoutingPoint} --config-file=${ConfigFile}"
			newString = "ExecStart=/usr/bin/cloudfuse mount ${MoutingPoint} --config-file=${ConfigFile} -o allow_other"
			line = strings.ReplaceAll(line, oldString, newString)
		}

		// Append the (possibly modified) line to the slice
		lines = append(lines, line)
	}
	// Check for errors during file reading
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file: [%s]", err.Error())
	}

	folderList := strings.Split(mountPath, "/")
	newFile, err := os.Create("/etc/systemd/system/" + folderList[len(folderList)-1] + ".service")
	if err != nil {
		return "", fmt.Errorf("error creating new service file: [%s]", err.Error())
	}

	// Open a new file and write all lines to the new file.

	// Create a buffered writer to overwrite the file
	writer := bufio.NewWriter(newFile)

	// Write the modified lines back to the file
	for _, line := range lines {
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			return "", fmt.Errorf("error writing to file: [%s]", err.Error())
		}
	}

	// Flush the buffer to write all data to disk
	writer.Flush()

	return serviceName, nil
}

func setUser(serviceUser string) error {
	usersList, err := os.Open("/etc/passwd")
	if err != nil {
		return fmt.Errorf("failed to open /etc/passwd due to following error: [%s]", err.Error())
	}
	scanner := bufio.NewScanner(usersList)
	var foundUser bool
	defer usersList.Close()

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, serviceUser) {
			foundUser = true
		}
	}
	if !foundUser {
		//create the user
		userAddCmd := exec.Command("sudo", "useradd", "-m", serviceUser)
		err = userAddCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to create user due to following error: [%s]", err.Error())
		}

		// use the current user for reference on permissions
		usr, err := user.Current()
		if err != nil {
			return fmt.Errorf("error getting looking up current user: [%s]", err.Error())
		}

		// get a list of group from reference user
		groups, err := usr.GroupIds()
		if err != nil {
			return fmt.Errorf("error getting group id list from current user: [%s]", err.Error())
		}

		// add the list of groups to the CloudfuseUser
		for _, group := range groups {
			usermodCmd := exec.Command("sudo", "usermod", "-aG", group, serviceUser)
			err = usermodCmd.Run()
			if err != nil {
				return fmt.Errorf("failed to add group to user due to following error: [%s]", err.Error())
			}
		}
	}
	return nil
}

//TODO: add wrapper function for collecting data, creating user, setting default paths, running commands.

func init() {
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(installCmd)
	serviceCmd.AddCommand(uninstallCmd)
}
