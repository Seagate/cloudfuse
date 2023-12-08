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
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"

	"github.com/spf13/cobra"
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
	Short:             "Manage cloudfuse mounts on Windows",
	Long:              "Manage cloudfuse mounts on Windows",
	SuggestFor:        []string{"ser", "serv"},
	Example:           "cloudfuse service install",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("missing command options\n\nDid you mean this?\n\tcloudfuse service mount\n\nRun 'cloudfuse service --help' for usage")
	},
}

var installCmd = &cobra.Command{
	Use:               "install",
	Short:             "Installs the startup process for Cloudfuse",
	Long:              "Installs the startup process for Cloudfuse which remounts any active previously active mounts on startup.",
	SuggestFor:        []string{"ins", "inst"},
	Example:           "cloudfuse service install",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("unable to determine location of cloudfuse binary [%s]", err.Error())
		}
		programPath := filepath.Join(dir, "windows-startup.exe")
		startupPath := filepath.Join(os.Getenv("APPDATA"), "Microsoft\\Windows\\Start Menu\\Programs\\Startup", "CloudfuseStartup.lnk")
		err = makeLink(programPath, startupPath)
		if err != nil {
			return fmt.Errorf("unable to create startup link [%s]", err.Error())
		}
		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:               "uninstall",
	Short:             "Uninstall the startup process for Cloudfuse",
	Long:              "Uninstall the startup process for Cloudfuse",
	SuggestFor:        []string{"uninst", "uninstal"},
	Example:           "cloudfuse service uninstall",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		startupPath := filepath.Join(os.Getenv("APPDATA"), "Microsoft\\Windows\\Start Menu\\Programs\\Startup", "CloudfuseStartup.lnk")
		err := os.Remove(startupPath)
		if err != nil {
			return fmt.Errorf("failed to delete startup process [%s]", err.Error())
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

func makeLink(src, dst string) error {
	ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED|ole.COINIT_SPEED_OVER_MEMORY)
	oleShellObject, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		return err
	}
	defer oleShellObject.Release()
	wshell, err := oleShellObject.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return err
	}
	defer wshell.Release()
	cs, err := oleutil.CallMethod(wshell, "CreateShortcut", dst)
	if err != nil {
		return err
	}
	idispatch := cs.ToIDispatch()
	oleutil.PutProperty(idispatch, "TargetPath", src)
	oleutil.CallMethod(idispatch, "Save")
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
	serviceCmd.AddCommand(mountServiceCmd)
	serviceCmd.AddCommand(unmountServiceCmd)

	mountServiceCmd.Flags().StringVar(&servOpts.ConfigFile, "config-file", "",
		"Configures the path for the file where the account credentials are provided.")
	_ = mountServiceCmd.MarkFlagRequired("config-file")
}
