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
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Seagate/cloudfuse/common"
	"github.com/spf13/cobra"
)

type serviceOptions struct {
	ConfigFile  string
	MountPath   string
	ServiceUser string
}

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

var mountPath string
var configPath string
var serviceUser string

var installCmd = &cobra.Command{
	Use:               "install",
	Short:             "Installs a service file for a single mount with Cloudfuse. Requires elevated permissions.",
	Long:              "Installs a service file for a single mount with Cloudfuse. Requires elevated permissions.",
	SuggestFor:        []string{"ins", "inst"},
	Example:           "cloudfuse service install --mount-path=<path/to/mount/point> --config-file=<path/to/config/file> --user=<username>",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {

		var err error
		mountPath, err = getAbsPath(mountPath)
		if err != nil {
			return err
		}
		configPath, err = getAbsPath(configPath)
		if err != nil {
			return err
		}

		mountExists := common.DirectoryExists(mountPath)
		if !mountExists {
			return fmt.Errorf("the mount path provided does not exist")
			// TODO: add useage output upon failure with input
		}
		// TODO: consider logging a warning if the mount path is empty

		_, err = os.Stat(configPath)
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("error, the configfile path provided does not exist")
		}

		//create the new user and set permissions
		err = setUser(serviceUser, mountPath, configPath)
		if err != nil {
			fmt.Println("Error setting permissions for user:", err)
			return err
		}

		serviceFile, err := newServiceFile(mountPath, configPath, serviceUser)
		if err != nil {
			return fmt.Errorf("error when attempting to create service file: [%s]", err.Error())
		}

		// run systemctl daemon-reload
		systemctlDaemonReloadCmd := exec.Command("sudo", "systemctl", "daemon-reload")
		err = systemctlDaemonReloadCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to run 'systemctl daemon-reload' command due to following error: [%s]", err.Error())
		}

		// Enable the service to start at system boot
		systemctlEnableCmd := exec.Command("sudo", "systemctl", "enable", serviceFile)
		err = systemctlEnableCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to run 'systemctl daemon-reload' command due to following error: [%s]", err.Error())
		}
		return nil
	},
}

var serviceName string
var uninstallCmd = &cobra.Command{
	Use:               "uninstall",
	Short:             "Uninstall a startup process for Cloudfuse.",
	Long:              "Uninstall a startup process for Cloudfuse.",
	SuggestFor:        []string{"uninst", "uninstal"},
	Example:           "cloudfuse service uninstall --mount-path=<path/to/mount/path>",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {

		// get absolute path of provided relative mount path
		var err error
		mountPath, err = getAbsPath(serviceName)
		if err != nil {
			return err
		}

		folderList := strings.Split(mountPath, "/")
		serviceFile := "cloudfuse-" + folderList[len(folderList)-1] + ".service"
		serviceFilePath := "/etc/systemd/system/" + serviceFile
		if _, err := os.Stat(serviceFilePath); err == nil {
			removeFileCmd := exec.Command("sudo", "rm", serviceFilePath)
			err := removeFileCmd.Run()
			if err != nil {
				return fmt.Errorf("failed to delete "+serviceName+" file from /etc/systemd/system due to following error: [%s]", err.Error())
			}
		} else if os.IsNotExist(err) {
			return fmt.Errorf("failed to delete "+serviceName+" file from /etc/systemd/system due to following error: [%s]", err.Error())
		}

		// reload daemon
		systemctlDaemonReloadCmd := exec.Command("sudo", "systemctl", "daemon-reload")
		err = systemctlDaemonReloadCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to run 'systemctl daemon-reload' command due to following error: [%s]", err.Error())
		}
		return nil
	},
}

//--------------- command section ends

func newServiceFile(mountPath string, configPath string, serviceUser string) (string, error) {
	serviceTemplate := `[Unit]
	Description=Cloudfuse is an open source project developed to provide a virtual filesystem backed by S3 or Azure storage.
	After=network-online.target
	Requires=network-online.target

	[Service]
	# User service will run as.
	User={{.ServiceUser}}
	# Path to the location Cloudfuse will mount to. Note this folder must currently exist.
	MountingPoint={{.MountPath}}
	# Path to the configuration file.
	ConfigFile={{.ConfigFile}}

	# Under the hood
	Type=forking
	ExecStart=/usr/bin/cloudfuse mount ${MountingPoint} --config-file=${ConfigFile} -o allow_other
	ExecStop=/usr/bin/fusermount -u ${MountingPoint} -z

	[Install]
	WantedBy=multi-user.target
	`

	config := serviceOptions{
		ConfigFile:  configPath,
		MountPath:   mountPath,
		ServiceUser: serviceUser,
	}

	tmpl, err := template.New("service").Parse(serviceTemplate)
	if err != nil {
		fmt.Errorf("error creating new service file: [%s]", err.Error())
	}

	folderList := strings.Split(mountPath, "/")
	serviceName := "cloudfuse-" + folderList[len(folderList)-1] + ".service"
	newFile, err := os.Create("/etc/systemd/system/" + serviceName)
	if err != nil {
		return "", fmt.Errorf("error creating new service file: [%s]", err.Error())
	}

	err = tmpl.Execute(newFile, config)
	if err != nil {
		return "", fmt.Errorf("error creating new service file: [%s]", err.Error())
	}

	return serviceName, nil
}

func setUser(serviceUser string, mountPath string, configPath string) error {

	configFileInfo, err := os.Stat(configPath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %v", err)
	}

	// Get file's group ID
	stat, ok := configFileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("failed to get file system stats")
	}
	configGroupID := stat.Gid

	// Get configFileGroup name
	configFileGroup, err := user.LookupGroupId(fmt.Sprint(configGroupID))
	if err != nil {
		return fmt.Errorf("failed to lookup group: %v", err)
	}

	mountPathInfo, err := os.Stat(mountPath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %v", err)
	}

	// Get file's group ID
	stat, ok = mountPathInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("failed to get file system stats")
	}
	mountGroupID := stat.Gid

	// Get configFileGroup name
	mountPathGroup, err := user.LookupGroupId(fmt.Sprint(mountGroupID))
	if err != nil {
		return fmt.Errorf("failed to lookup group: %v", err)
	}

	_, err = user.Lookup(serviceUser)
	if err != nil {
		if strings.Contains(err.Error(), "unknown user") {
			//create the user
			userAddCmd := exec.Command("sudo", "useradd", "-r", serviceUser)
			err = userAddCmd.Run()
			if err != nil {
				return fmt.Errorf("failed to create user due to following error: [%s]", err.Error())
			}

			fmt.Println("user " + serviceUser + " has been created")

			//suggest usermod -aG commands here to the end user.
			fmt.Println("groups: " + configFileGroup.Name + " and " + mountPathGroup.Name + " need to be added to the user, " + serviceUser)

			// suggest the chmod 770 command
			fmt.Println("please ensure the " + mountPathGroup.Name + "has read and write permissions for " + mountPath)

		} else {
			fmt.Printf("An error occurred: %v\n", err)
		}
	} else {

		// TODO: use configFileGroup and mountPathGroup to check if service user has these groups. complain / warn if it doesn't
		usermodCmd := exec.Command("sudo", "usermod", "-aG", configFileGroup.Name, serviceUser)
		err = usermodCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to create user due to following error: [%s]", err.Error())
		}
		usermodCmd = exec.Command("sudo", "usermod", "-aG", mountPathGroup.Name, serviceUser)
		err = usermodCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to create user due to following error: [%s]", err.Error())
		}

		//set set folder permission on the mount path
		chmodCmd := exec.Command("sudo", "chmod", "770", mountPath)
		err = chmodCmd.Run()
		if err != nil {
			return fmt.Errorf("failed set permisions on mount path due to following error: [%s]", err.Error())
		}

	}

	return nil
}

//TODO: add wrapper function for collecting data, creating user, setting default paths, running commands.

// takes a file or folder name and returns its absolute path
func getAbsPath(leaf string) (string, error) {
	var absPath string
	var err error
	if !filepath.IsAbs(leaf) {
		absPath, err = filepath.Abs(leaf)
		if err != nil {
			return "", fmt.Errorf("couldn't format the path string", err.Error())
		}
	}
	return absPath, err
}

func init() {
	rootCmd.AddCommand(serviceCmd)
	rootCmd.SilenceUsage = false
	serviceCmd.AddCommand(installCmd)
	installCmd.Flags().StringVar(&mountPath, "mount-path", "", "Input mount path")
	installCmd.Flags().StringVar(&configPath, "config-file", "", "Input config file")
	installCmd.Flags().StringVar(&serviceUser, "user", "cloudfuse", "Input service user")
	installCmd.MarkFlagRequired("mount-path")
	installCmd.MarkFlagRequired("config-file")
	serviceCmd.AddCommand(uninstallCmd)
	uninstallCmd.Flags().StringVar(&serviceName, "mount-path", "", "Input mount path")
	uninstallCmd.MarkFlagRequired("mount-path")
}
