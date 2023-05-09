//go:build windows

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const SvcName = "lyvecloudfuse"

// Section defining all the command that we have in secure feature
var serviceCmd = &cobra.Command{
	Use:               "service",
	Short:             "Manage lyvecloudfuse as a Windows service",
	Long:              "Manage lyvecloudfuse as a Windows service",
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

var removeCmd = &cobra.Command{
	Use:               "remove",
	Short:             "Remove as a Windows service",
	Long:              "Remove as a Windows service",
	SuggestFor:        []string{"ins", "inst"},
	Example:           "lyvecloudfuse service remove",
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
	err = createRegistryEntry()
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

// createRegistryEntry creates an entry in the registry for WinFsp
// so the WinFsp launch tool can launch our service.
func createRegistryEntry() error {
	const registryPath = `SOFTWARE\WOW6432Node\WinFsp\Services\` + SvcName
	executablePath, err := os.Executable()
	if err != nil {
		return err
	}
	executableDir := filepath.Dir(executablePath)

	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, registryPath, registry.ALL_ACCESS)
	if err != nil {
		return err
	}

	err = key.SetStringValue("Executable", executablePath)
	if err != nil {
		return err
	}
	// TODO: Add ability to pass in mounth path and config file path
	err = key.SetStringValue("CommandLine", `mount Z: --config-file=`+filepath.Join(executableDir, "config.yaml"))
	if err != nil {
		return err
	}
	err = key.SetStringValue("Security", "D:P(A;;RPWPLC;;;WD)")
	if err != nil {
		return err
	}
	err = key.SetDWordValue("JobControl", 1)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(installCmd)
	serviceCmd.AddCommand(removeCmd)
	serviceCmd.AddCommand(startCmd)
	serviceCmd.AddCommand(stopCmd)
}
