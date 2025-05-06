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
		return errors.New(
			"missing command options\n\nDid you mean this?\n\tcloudfuse service install\n\nRun 'cloudfuse service --help' for usage",
		)
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
	Example:           "cloudfuse service install --mount-path=<path/to/mount/point> --config-file=<path/to/config/file>",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {

		mountPath, err := filepath.Abs(mountPath)
		if err != nil {
			return fmt.Errorf(
				"couldn't couldn't determine absolute path from string [%s]",
				err.Error(),
			)
		}
		configPath, err := filepath.Abs(configPath)
		if err != nil {
			return fmt.Errorf(
				"couldn't couldn't determine absolute path from string [%s]",
				err.Error(),
			)
		}

		mountExists := common.DirectoryExists(mountPath)
		if !mountExists {
			return fmt.Errorf("the mount path provided does not exist")
		}
		_, err = os.Stat(configPath)
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("the config file path provided does not exist")
		}
		//create the new user and set permissions
		err = setUser(serviceUser, mountPath, configPath)
		if err != nil {
			fmt.Println("could not set up service user ", err)
			return err
		}

		serviceName, err := newService(mountPath, configPath, serviceUser)
		if err != nil {
			return fmt.Errorf("unable to create service file: [%s]", err.Error())
		}
		// run systemctl daemon-reload
		systemctlDaemonReloadCmd := exec.Command("systemctl", "daemon-reload")
		err = systemctlDaemonReloadCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to run 'systemctl daemon-reload' command [%s]", err.Error())
		}
		// Enable the service to start at system boot
		systemctlEnableCmd := exec.Command("systemctl", "enable", serviceName)
		err = systemctlEnableCmd.Run()
		if err != nil {
			return fmt.Errorf(
				"failed to run 'systemctl daemon-reload' command due to following [%s]",
				err.Error(),
			)
		}
		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:               "uninstall",
	Short:             "Uninstall a startup process for Cloudfuse.",
	Long:              "Uninstall a startup process for Cloudfuse.",
	SuggestFor:        []string{"uninst", "uninstal"},
	Example:           "cloudfuse service uninstall --mount-path=<path/to/mount/path>",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		// get absolute path of provided relative mount path

		mountPath, err := filepath.Abs(mountPath)
		if err != nil {
			return fmt.Errorf("couldn't determine absolute path from string [%s]", err.Error())
		}
		serviceName, serviceFilePath := getService(mountPath)
		if _, err := os.Stat(serviceFilePath); err == nil {
			removeFileCmd := exec.Command("rm", serviceFilePath)
			err := removeFileCmd.Run()
			if err != nil {
				return fmt.Errorf(
					"failed to delete "+serviceName+" file from /etc/systemd/system [%s]",
					err.Error(),
				)
			}
		} else if os.IsNotExist(err) {
			return fmt.Errorf("failed to delete "+serviceName+" file from /etc/systemd/system [%s]", err.Error())
		}
		// reload daemon
		systemctlDaemonReloadCmd := exec.Command("systemctl", "daemon-reload")
		err = systemctlDaemonReloadCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to run 'systemctl daemon-reload' command [%s]", err.Error())
		}
		return nil
	},
}

//--------------- command section ends

func newService(mountPath string, configPath string, serviceUser string) (string, error) {
	serviceTemplate := `
[Unit]
Description=Cloudfuse is an open source project developed to provide a virtual filesystem backed by S3 or Azure storage.
After=network-online.target
Requires=network-online.target

[Service]
# User service will run as.
User={{.ServiceUser}}

# Under the hood
Type=forking
ExecStart=/usr/bin/cloudfuse mount {{.MountPath}} --config-file={{.ConfigFile}} -o allow_other
ExecStop=/usr/bin/fusermount -u {{.MountPath}} -z

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
		return "", fmt.Errorf("could not create a new service file: [%s]", err.Error())
	}
	serviceName, serviceFilePath := getService(mountPath)
	err = os.Remove(serviceFilePath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to replace the service file [%s]", err.Error())
	}

	var newFile *os.File
	newFile, err = os.Create(serviceFilePath)
	if err != nil {
		return "", fmt.Errorf("could not create new service file: [%s]", err.Error())
	}

	err = tmpl.Execute(newFile, config)
	if err != nil {
		return "", fmt.Errorf("could not create new service file: [%s]", err.Error())
	}
	return serviceName, nil
}

func setUser(serviceUser string, mountPath string, configPath string) error {
	_, err := user.Lookup(serviceUser)
	if err != nil {
		if strings.Contains(err.Error(), "unknown user") {
			// create the user
			userAddCmd := exec.Command("useradd", "-m", serviceUser)
			err = userAddCmd.Run()
			if err != nil {
				return fmt.Errorf("failed to create user [%s]", err.Error())
			}
			fmt.Println("user " + serviceUser + " has been created")
		}
	}
	// advise on required permissions
	fmt.Println(
		"ensure the user, " + serviceUser + ", has the following access: \n" + mountPath + ": read, write, and execute \n" + configPath + ": read",
	)
	return nil
}

func getService(mountPath string) (string, string) {
	serviceName := strings.ReplaceAll(mountPath, "/", "-")
	serviceFile := "cloudfuse" + serviceName + ".service"
	serviceFilePath := "/etc/systemd/system/" + serviceFile
	return serviceName, serviceFilePath
}

func markFlagErrorChk(cmd *cobra.Command, flagName string) {
	if err := cmd.MarkFlagRequired(flagName); err != nil {
		panic(fmt.Sprintf("Failed to mark flag as required: %v", err))
	}
}

func init() {
	rootCmd.AddCommand(serviceCmd)
	rootCmd.SilenceUsage = false
	serviceCmd.AddCommand(installCmd)
	installCmd.Flags().StringVar(&mountPath, "mount-path", "", "Input mount path")
	installCmd.Flags().StringVar(&configPath, "config-file", "", "Input config file")
	installCmd.Flags().StringVar(&serviceUser, "user", "cloudfuse", "Input service user")
	markFlagErrorChk(installCmd, "mount-path")
	markFlagErrorChk(installCmd, "config-file")
	serviceCmd.AddCommand(uninstallCmd)
	uninstallCmd.Flags().StringVar(&mountPath, "mount-path", "", "Input mount path")
	markFlagErrorChk(uninstallCmd, "mount-path")
}
