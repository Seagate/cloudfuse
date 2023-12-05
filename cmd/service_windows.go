//go:build windows

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023 Seagate Technology LLC and/or its Affiliates

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
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal/winservice"

	"github.com/spf13/cobra"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type serviceOptions struct {
	ConfigFile string
	MountPath  string
}

const SvcName = "cloudfuse"

var servOpts serviceOptions

// Section defining all the command that we have in secure feature
var serviceCmd = &cobra.Command{
	Use:               "service",
	Short:             "Manage cloudfuse as a Windows service. This requires Administrator rights to run.",
	Long:              "Manage cloudfuse as a Windows service. This requires Administrator rights to run.",
	SuggestFor:        []string{"ser", "serv"},
	Example:           "cloudfuse service install",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("missing command options\n\nDid you mean this?\n\tcloudfuse service mount\n\nRun 'cloudfuse service --help' for usage")
	},
}

var installCmd = &cobra.Command{
	Use:               "install",
	Short:             "Install as a Windows service",
	Long:              "Install as a Windows service",
	SuggestFor:        []string{"ins", "inst"},
	Example:           "cloudfuse service install",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := installService()
		if err != nil {
			if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
				return errors.New("this action requires admin rights")
			}
			return fmt.Errorf("failed to install as a Windows service [%s]", err.Error())
		}

		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:               "uninstall",
	Short:             "Remove as a Windows service",
	Long:              "Remove as a Windows service",
	SuggestFor:        []string{"uninst", "uninstal"},
	Example:           "cloudfuse service uninstall",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := removeService()
		if err != nil {
			if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
				return errors.New("this action requires admin rights")
			}
			return fmt.Errorf("failed to remove as a Windows service [%s]", err.Error())
		}

		return nil
	},
}

var startCmd = &cobra.Command{
	Use:               "start",
	Short:             "start the Windows service",
	Long:              "start the Windows service",
	SuggestFor:        []string{"sta", "star"},
	Example:           "cloudfuse service start",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := startService()
		if err != nil {
			if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
				return errors.New("this action requires admin rights")
			}
			return fmt.Errorf("failed to start as a Windows service [%s]", err.Error())
		}

		return nil
	},
}

var stopCmd = &cobra.Command{
	Use:               "stop",
	Short:             "stop the Windows service",
	Long:              "stop the Windows service",
	SuggestFor:        []string{"sto"},
	Example:           "cloudfuse service stop",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := stopService()
		if err != nil {
			if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
				return errors.New("this action requires admin rights")
			}
			return fmt.Errorf("failed to stop the Windows service [%s]", err.Error())
		}

		return nil
	},
}

var mountServiceCmd = &cobra.Command{
	Use:               "mount",
	Short:             "mount an instance that will persist in Windows when restarted",
	Long:              "mount an instance that will persist in Windows when restarted",
	SuggestFor:        []string{"mnt", "mout"},
	Args:              cobra.ExactArgs(1),
	Example:           "cloudfuse service mount Z: --config-file=C:\\config.yaml",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		servOpts.MountPath = strings.ReplaceAll(common.ExpandPath(args[0]), "\\", "/")

		err := validateMountOptions()
		if err != nil {
			return fmt.Errorf("failed to validate options [%s]", err.Error())
		}

		err = mountInstance()
		if err != nil {
			return fmt.Errorf("failed to mount instance [%s]", err.Error())
		}

		// Add the mount to the JSON file so it persists on restart.
		err = winservice.AddMountJSON(servOpts.MountPath, servOpts.ConfigFile)
		if err != nil {
			return fmt.Errorf("failed to add entry to json file [%s]", err.Error())
		}

		return nil
	},
}

var unmountServiceCmd = &cobra.Command{
	Use:               "unmount",
	Short:             "unmount an instance and remove entry from Windows service",
	Long:              "unmount an instance and remove entry from Windows service",
	SuggestFor:        []string{"umount", "unmoun"},
	Args:              cobra.ExactArgs(1),
	Example:           "cloudfuse service unmount Z:",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		servOpts.MountPath = strings.ReplaceAll(common.ExpandPath(args[0]), "\\", "/")

		// Check with winfsp to see if this is currently mounted
		ret, err := isMounted()
		if err != nil {
			if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
				return errors.New("this action requires admin rights")
			}
			return fmt.Errorf("failed to validate options [%s]", err.Error())
		} else if !ret {
			return fmt.Errorf("nothing is mounted here")
		}

		// Remove the mount from json file so it does not remount on restart.
		err = winservice.RemoveMountJSON(servOpts.MountPath)
		// If error is not nill then ignore it
		if err != nil {
			log.Err("failed to remove entry from json file [%s]", err.Error())
		}

		err = unmountInstance()
		if err != nil {
			return fmt.Errorf("failed to unmount instance [%s]", err.Error())
		}

		return nil
	},
}

//--------------- command section ends

// installService adds cloudfuse as a windows service.
func installService() error {
	exepath, err := os.Executable()
	if err != nil {
		return err
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

	service, err = scm.CreateService(SvcName, exepath, mgr.Config{DisplayName: "Cloudfuse", StartType: mgr.StartAutomatic})
	if err != nil {
		return err
	}
	defer service.Close()

	// Create the registry for WinFsp
	err = winservice.CreateWinFspRegistry()
	if err != nil {
		return err
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

	// Remove the registry for WinFsp
	// Ignore error if unable to find
	_ = winservice.RemoveWinFspRegistry()

	// Remove all registry entries for cloudfuse
	// Ignore error if registry path does not exist
	_ = winservice.RemoveAllRegistryMount()

	err = service.Delete()
	if err != nil {
		return err
	}
	return nil
}

// startService starts the windows service.
func startService() error {
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

	err = service.Start()
	if err != nil {
		return fmt.Errorf("%s service could not be started: %v", SvcName, err)
	}
	return nil
}

// startService stops the windows service.
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

// mountInstance mounts the given instance.
func mountInstance() error {
	return winservice.StartMount(servOpts.MountPath, servOpts.ConfigFile)
}

// unmountInstance unmounts the given instance.
func unmountInstance() error {
	return winservice.StopMount(servOpts.MountPath)
}

// isMounted returns if the current mountPath is mounted using cloudfuse.
func isMounted() (bool, error) {
	return winservice.IsMounted(servOpts.MountPath)
}

// isServiceRunning returns whether the cloudfuse service is currently running.
func isServiceRunning() (bool, error) {
	scm, err := mgr.Connect()
	if err != nil {
		return false, err
	}
	defer scm.Disconnect() //nolint

	service, err := scm.OpenService(SvcName)
	if err != nil {
		return false, err
	}
	defer service.Close()

	status, err := service.Query()
	if err != nil {
		return false, err
	}

	if status.State == windows.SERVICE_RUNNING {
		return true, nil
	}
	return false, nil
}

// validateMountPath checks whether the mountpath is correct and does not exist.
func validateMountOptions() error {
	// Mount Path
	if servOpts.MountPath == "" {
		return errors.New("mount path not provided")
	}

	if strings.Contains(servOpts.MountPath, "\\") {
		return errors.New("mount path contains '\\' which is not allowed")
	}

	if _, err := os.Stat(servOpts.MountPath); errors.Is(err, fs.ErrExist) || err == nil {
		return errors.New("mount path exists")
	}

	// Config file
	if servOpts.ConfigFile == "" {
		return errors.New("config file not provided")
	}

	// Convert the path into a full path so WinFSP can see the config file
	configPath, err := filepath.Abs(servOpts.ConfigFile)
	if err != nil {
		return errors.New("config file does not exist")
	}
	servOpts.ConfigFile = configPath

	if _, err := os.Stat(servOpts.ConfigFile); errors.Is(err, fs.ErrNotExist) {
		return errors.New("config file does not exist")
	}

	return nil
}

func init() {
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(installCmd)
	serviceCmd.AddCommand(uninstallCmd)
	serviceCmd.AddCommand(startCmd)
	serviceCmd.AddCommand(stopCmd)
	serviceCmd.AddCommand(mountServiceCmd)
	serviceCmd.AddCommand(unmountServiceCmd)

	mountServiceCmd.Flags().StringVar(&servOpts.ConfigFile, "config-file", "",
		"Configures the path for the file where the account credentials are provided.")
	_ = mountServiceCmd.MarkFlagRequired("config-file")
}
