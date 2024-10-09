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
	"os/user"
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

		// get current dir
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("error: [%s]", err.Error())
		}

		// assumes dir is in cloudfuse repo dir
		serviceData, err := collectServiceData(fmt.Sprintf("%s/setup/cloudfuse.service", dir))
		if err != nil {
			return fmt.Errorf("error collecting data from cloudfuse.service file due to the following error: [%s]", err)
		}

		//TODO: set a default mount and config path when stubbed examples are found in the file.
		mountPath := serviceData["MoutingPoint"]
		configPath := serviceData["ConfigFile"]
		if mountPath == "/path/to/mounting/point" {
			err = modifySericeFile(mountPath, dir)
			if err != nil {
				return fmt.Errorf("error when attempting to write defaults into service file: [%s]", err.Error())
			}
		}
		if configPath == "/path/to/config/file/config.yaml" {
			err = modifySericeFile(configPath, dir)
			if err != nil {
				return fmt.Errorf("error when attempting to write defaults into service file: [%s]", err.Error())
			}
		}

		// 2. retrieve the user account from cloudfuse.service file and create it if it doesn't exist
		user := serviceData["User"]
		usersList, err := os.Open("/etc/passwd")
		if err != nil {
			return fmt.Errorf("failed to open /etc/passwd due to following error: [%s]", err.Error())
		}
		scanner := bufio.NewScanner(usersList)
		var foundUser bool
		defer usersList.Close()

		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, user) {
				foundUser = true
			}
		}
		if !foundUser {
			//create the user
			userAddCmd := exec.Command("useradd", "-m", user)
			err := userAddCmd.Run()
			if err != nil {
				return fmt.Errorf("failed to create user due to following error: [%s]", err.Error())
			}
		}

		// 3. copy the cloudfuse.service file to /etc/systemd/system

		copyFileCmd := exec.Command("cp", "./setup/cloudfuse.service", "/etc/systemd/system")
		err = copyFileCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to copy cloudfuse.service file to /etc/systemd/system due to following error: [%s]", err.Error())
		}

		// 4. run systemctl daemon-reload

		systemctlCmd := exec.Command("systemctl", "daemon-reload")
		err = systemctlCmd.Run()
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
		// 2. handle errors

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
		return nil, err
	}
	return serviceData, nil
}

func modifySericeFile(path string, curDir string) error {

	var defaultMountPath string

	//get current user
	usr, err := user.Current()
	if err != nil {
		return fmt.Errorf("Error: [%s]", err.Error())
	}

	//mountpath or config?
	var oldString string
	var newString string
	var config bool
	var mount bool

	if strings.Contains(path, "config.yaml") {
		oldString = "Environment=ConfigFile=/path/to/config/file/config.yaml"
		newString = fmt.Sprintf("Environment=ConfigFile=%s/config.yaml", curDir)
		config = true
		mount = false
	}
	if strings.Contains(path, "mounting") {
		defaultMountPath = fmt.Sprintf("/home/%s/cloudfuseMount", strings.ToLower(usr.Name))
		fmt.Printf("creating mount folder in %s", defaultMountPath)

		//Create default mount directory
		userAddCmd := exec.Command("mkdir", defaultMountPath)
		err = userAddCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to create default mount folder due to following error: [%s]", err.Error())
		}
		oldString = "Environment=MoutingPoint=/path/to/mounting/point"
		newString = fmt.Sprintf("Environment=MoutingPoint=%s", defaultMountPath)
		config = false
		mount = true
	}

	// open service file for read write
	file, err := os.OpenFile("./setup/cloudfuse.service", os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("Error opening file: [%s]", err.Error())
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string

	// Read the file line by line
	for scanner.Scan() {
		line := scanner.Text()

		// Check if the line contains the search string
		if mount && strings.Contains(line, "MoutingPoint") {
			// Modify the line by replacing the old string with the new string
			line = strings.ReplaceAll(line, oldString, newString)
		}
		if config && strings.Contains(line, "ConfigFile") {
			line = strings.ReplaceAll(line, oldString, newString)
		}

		// Append the (possibly modified) line to the slice
		lines = append(lines, line)
	}
	// Check for errors during file reading
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("Error reading file: [%s]", err.Error())
	}

	// Move the file pointer to the start for overwriting
	file.Seek(0, 0)

	// Create a buffered writer to overwrite the file
	writer := bufio.NewWriter(file)

	// Write the modified lines back to the file
	for _, line := range lines {
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			return fmt.Errorf("Error writing to file: [%s]", err.Error())
		}
	}

	// Truncate the file to the new size in case the modified content is shorter
	err = file.Truncate(int64(writer.Buffered()))
	if err != nil {
		return fmt.Errorf("Error truncating file: [%s]", err.Error())

	}

	// Flush the buffer to write all data to disk
	writer.Flush()

	return nil
}

//TODO: add wrapper function for collecting data, creating user, setting default paths, running commands.

func init() {
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(installCmd)
	serviceCmd.AddCommand(uninstallCmd)
}
