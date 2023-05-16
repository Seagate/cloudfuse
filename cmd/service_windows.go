//go:build windows

package cmd

import (
	"errors"
	"fmt"
	"lyvecloudfuse/common"
	"lyvecloudfuse/internal/windowsService"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type serviceOptions struct {
	ConfigFile string
	MountPath  string
}

const SvcName = "lyvecloudfuse"

var servOpts serviceOptions

// Section defining all the command that we have in secure feature
var serviceCmd = &cobra.Command{
	Use:               "service",
	Short:             "Manage lyvecloudfuse as a Windows service. This requires Administrator rights to run.",
	Long:              "Manage lyvecloudfuse as a Windows service. This requires Administrator rights to run.",
	SuggestFor:        []string{"ser", "serv"},
	Example:           "lyvecloudfuse service install",
	Args:              cobra.ExactArgs(1),
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var installCmd = &cobra.Command{
	Use:               "install",
	Short:             "Install as a Windows service",
	Long:              "Install as a Windows service",
	SuggestFor:        []string{"ins", "inst"},
	Example:           "lyvecloudfuse service install",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := installService()
		if err != nil {
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
	Example:           "lyvecloudfuse service uninstall",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := removeService()
		if err != nil {
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
	Example:           "lyvecloudfuse service start",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := startService()
		if err != nil {
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
	Example:           "lyvecloudfuse service stop",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := stopService()
		if err != nil {
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
	Example:           "lyvecloudfuse service mount Z: --config-file=C:\\config.yaml",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		servOpts.MountPath = strings.ReplaceAll(common.ExpandPath(args[0]), "\\", "/")

		err := validateMountOptions()
		if err != nil {
			return fmt.Errorf("failed to validate options [%s]", err.Error())
		}

		// Only allow mounts if the service is running
		running, _ := isServiceRunning()
		if !running {
			return fmt.Errorf("windows service is not running")
		}

		err = mountInstance()
		if err != nil {
			return fmt.Errorf("failed to mount instance [%s]", err.Error())
		}

		err = windowsService.CreateRegistryMount(servOpts.MountPath, servOpts.ConfigFile)
		if err != nil {
			return fmt.Errorf("failed to create registry entry [%s]", err.Error())
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
	Example:           "lyvecloudfuse service unmount --name=Mount1",
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

		err = windowsService.RemoveRegistryMount(servOpts.MountPath)
		if err != nil {
			return fmt.Errorf("failed to remove registry entry [%s]", err.Error())
		}

		err = unmountInstance()
		if err != nil {
			return fmt.Errorf("failed to unmount instance [%s]", err.Error())
		}

		return nil
	},
}

//--------------- command section ends

// installService uninstall the lyvecloudfuse windows service.
func installService() error {
	exepath, err := os.Executable()
	if err != nil {
		return err
	}

	scm, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer scm.Disconnect()

	// Don't install the service if it already exists
	service, err := scm.OpenService(SvcName)
	if err == nil {
		service.Close()
		return fmt.Errorf("%s service already exists", SvcName)
	}

	service, err = scm.CreateService(SvcName, exepath, mgr.Config{DisplayName: "LyveCloudFUSE", StartType: mgr.StartAutomatic})
	if err != nil {
		return err
	}
	defer service.Close()

	// Create the registry for WinFsp
	err = windowsService.CreateWinFspRegistry()
	if err != nil {
		return err
	}

	return nil
}

// removeService uninstall the lyvecloudfuse windows service.
func removeService() error {
	scm, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer scm.Disconnect()

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

// startService starts the windows service.
func startService() error {
	scm, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer scm.Disconnect()

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
	defer scm.Disconnect()

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

func mountInstance() error {
	return windowsService.StartMount(servOpts.MountPath, servOpts.ConfigFile)
}

func unmountInstance() error {
	return windowsService.StopMount(servOpts.MountPath)
}

// isMounted returns if the current mountPath is mounted using lyvecloudfuse.
func isMounted() (bool, error) {
	return windowsService.IsMounted(servOpts.MountPath)
}

// isServiceRunning returns whether the lyvecloudservice is currently running.
func isServiceRunning() (bool, error) {
	scm, err := mgr.Connect()
	if err != nil {
		return false, err
	}
	defer scm.Disconnect()

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
		return errors.New("mmount path not provided")
	}

	if strings.Contains(servOpts.MountPath, "\\") {
		return errors.New("mmount path contains '\\' which is not allowed")
	}

	if _, err := os.Stat(servOpts.MountPath); os.IsExist(err) || err == nil {
		return errors.New("mmount path exists")
	}

	// Config file
	if servOpts.ConfigFile == "" {
		return errors.New("config file not provided")
	}

	if _, err := os.Stat(servOpts.ConfigFile); os.IsNotExist(err) {
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
