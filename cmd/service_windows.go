//go:build windows

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2026 Seagate Technology LLC and/or its Affiliates

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
	"os"
	"path/filepath"

	"github.com/Seagate/cloudfuse/internal/winservice"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/spf13/cobra"
)

type serviceOptions struct {
	ConfigFile string
	MountPath  string
}

const (
	SvcName        = "CloudfuseServiceStartup"
	SvcDescription = "Cloudfuse Service to start System Mounts on System Start"
	StartupName    = "CloudfuseStartup.lnk"
)

var servOpts serviceOptions

// Section defining all the command that we have in secure feature
var serviceCmd = &cobra.Command{
	Use:        "service",
	Short:      "Manage cloudfuse startup process on Windows",
	Long:       "Manage cloudfuse startup process on Windows",
	SuggestFor: []string{"ser", "serv"},
	Example:    "cloudfuse service install",
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New(
			"missing command options\n\nDid you mean this?\n\tcloudfuse service install\n\nRun 'cloudfuse service --help' for usage",
		)
	},
}

var installCmd = &cobra.Command{
	Use:        "install",
	Short:      "Installs the startup process and Windows service for Cloudfuse. Requires running as admin.",
	Long:       "Installs the startup process and Windows service for Cloudfuse. Required for remount flags to work. Requires running as admin.",
	SuggestFor: []string{"ins", "inst"},
	Example:    "cloudfuse service install",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create the registry for WinFsp
		err := winservice.CreateWinFspRegistry()
		if err != nil {
			return fmt.Errorf(
				"Failed to add Windows registry for WinFSP support. Here's why: [%v]",
				err,
			)
		}
		// Add our startup process to the registry
		var programPath string
		exepath, err := os.Executable()
		if err != nil {
			// If we can't determine our location, use a standard path
			programFiles := os.Getenv("ProgramFiles")
			if programFiles == "" {
				programPath = filepath.Join(
					"C:",
					"Program Files",
					"Cloudfuse",
					"windows-startup.exe",
				)
			} else {
				programPath = filepath.Join(programFiles, "Cloudfuse", "windows-startup.exe")
			}
		} else {
			programPath = filepath.Join(filepath.Dir(exepath), "windows-startup.exe")
		}

		err = winservice.AddRegistryValue(
			`SOFTWARE\Microsoft\Windows\CurrentVersion\Run`,
			"Cloudfuse",
			programPath,
		)
		if err != nil {
			return fmt.Errorf("Failed to add startup registry value. Here's why: %v", err)
		}

		err = installService()
		if err != nil {
			return fmt.Errorf("Failed to install as a Windows service. Here's why: %v", err)
		}

		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:        "uninstall",
	Short:      "Uninstalls the startup process and Windows service for Cloudfuse. Requires running as admin.",
	Long:       "Uninstalls the startup process and Windows service for Cloudfuse. Requires running as admin.",
	SuggestFor: []string{"uninst", "uninstal"},
	Example:    "cloudfuse service uninstall",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Remove the cloudfuse startup registry entry
		err := winservice.RemoveRegistryValue(
			`SOFTWARE\Microsoft\Windows\CurrentVersion\Run`,
			"Cloudfuse",
		)
		if err != nil {
			return fmt.Errorf(
				"Failed to remove cloudfuse remount service from Windows startup registry. Here's why: %v",
				err,
			)
		}
		// Remove the registry for WinFsp
		err = winservice.RemoveWinFspRegistry()
		if err != nil {
			return fmt.Errorf(
				"Failed to remove cloudfuse entry from WinFSP registry. Here's why: %v",
				err,
			)
		}

		err = stopService()
		if err != nil {
			cmd.PrintErrf(
				"Attempted to stop service but failed, now attempting to remove service. Here's why: %v",
				err,
			)
		}

		err = removeService()
		if err != nil {
			return fmt.Errorf("Failed to remove as a Windows service. Here's why: %v", err)
		}

		err = winservice.DeleteMountJSONFiles()
		if err != nil {
			return fmt.Errorf("Failed to remove mount.json tracker file. Here's why: %v", err)
		}

		return nil
	},
}

var addRegistryCmd = &cobra.Command{
	Use:        "add-registry",
	Short:      "Add registry information for WinFSP to launch cloudfuse. Requires running as admin.",
	Long:       "Add registry information for WinFSP to launch cloudfuse. Requires running as admin.",
	SuggestFor: []string{"add"},
	Example:    "cloudfuse service add-registry",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := winservice.CreateWinFspRegistry()
		if err != nil {
			return fmt.Errorf("error adding Windows registry for WinFSP support [%s]", err.Error())
		}
		return nil
	},
}

var removeRegistryCmd = &cobra.Command{
	Use:        "remove-registry",
	Short:      "Remove registry information for WinFSP to launch cloudfuse. Requires running as admin.",
	Long:       "Remove registry information for WinFSP to launch cloudfuse. Requires running as admin.",
	SuggestFor: []string{"remove"},
	Example:    "cloudfuse service remove-registry",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := winservice.RemoveWinFspRegistry()
		if err != nil {
			return fmt.Errorf("error removing Windows registry from WinFSP [%s]", err.Error())
		}

		return nil
	},
}

//--------------- command section ends

// installService adds cloudfuse as a windows service.
func installService() error {
	// Add our startup process to the registry
	exepath, err := os.Executable()
	if err != nil {
		// If we can't determine our location, use a standard path
		programFiles := os.Getenv("ProgramFiles")
		if programFiles == "" {
			exepath = filepath.Join("C:", "Program Files", "Cloudfuse", "windows-service.exe")
		} else {
			exepath = filepath.Join(programFiles, "Cloudfuse", "windows-service.exe")
		}
	} else {
		exepath = filepath.Join(filepath.Dir(exepath), "windows-service.exe")
	}

	scm, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer scm.Disconnect() //nolint

	// Don't install the service if it already exists
	service, err := scm.OpenService(SvcName)
	if err == nil {
		service.Close()
		return fmt.Errorf("%s service already exists", SvcName)
	}

	config := mgr.Config{
		ServiceType:  windows.SERVICE_WIN32_OWN_PROCESS,
		StartType:    mgr.StartAutomatic,
		ErrorControl: mgr.ErrorNormal,
		DisplayName:  SvcName,
		Description:  SvcDescription,
		Dependencies: []string{"DnsCache", "WinFsp.Launcher"},
	}

	service, err = scm.CreateService(SvcName, exepath, config)
	if err != nil {
		return err
	}
	defer service.Close()

	return nil
}

// stopService stops the windows service.
func stopService() error {
	scm, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer scm.Disconnect() //nolint

	service, err := scm.OpenService(SvcName)
	if err != nil {
		return fmt.Errorf("%s service is not installed", SvcName)
	}
	defer service.Close()

	_, err = service.Control(svc.Stop)
	if err != nil {
		return fmt.Errorf("%s service could not be stopped: %v", SvcName, err)
	}

	return nil
}

// removeService uninstall the cloudfuse windows service.
func removeService() error {
	scm, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer scm.Disconnect() //nolint

	service, err := scm.OpenService(SvcName)
	if err != nil {
		return fmt.Errorf("%s service is not installed", SvcName)
	}
	defer service.Close()

	err = service.Delete()
	if err != nil {
		return err
	}
	return nil
}

func init() {
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(installCmd)
	serviceCmd.AddCommand(uninstallCmd)
	serviceCmd.AddCommand(addRegistryCmd)
	serviceCmd.AddCommand(removeRegistryCmd)
}
