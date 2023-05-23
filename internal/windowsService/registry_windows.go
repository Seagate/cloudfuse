//go:build windows

package windowsService

import (
	"lyvecloudfuse/common/log"
	"os"

	"golang.org/x/sys/windows/registry"
)

// Windows Registry Paths
const (
	lcfRegistry      = `SOFTWARE\Seagate\LyveCloudFuse\`
	instanceRegistry = lcfRegistry + `Instances\`
	winFspRegistry   = `SOFTWARE\WOW6432Node\WinFsp\Services\`
)

// WinFsp registry contants
const (
	jobControl = 1
	mountCmd   = `mount %1 --config-file=%2`
	security   = `D:P(A;;RPWPLC;;;WD)`
)

func ReadRegistryInstanceEntry(name string) (KeyData, error) {
	registryPath := instanceRegistry + name

	key, err := registry.OpenKey(registry.LOCAL_MACHINE, registryPath, registry.ALL_ACCESS)
	if err != nil {
		log.Err("Unable to read instance names from Windows Registry: %v", err.Error())
		return KeyData{}, err
	}
	defer key.Close()

	var d KeyData
	d.MountPath = name

	d.ConfigFile, _, err = key.GetStringValue("ConfigFile")
	if err != nil {
		log.Err("Unable to read key ConfigFile from instance in Windows Registry: %v", err.Error())
		return KeyData{}, err
	}

	return d, nil
}

// readRegistryEntry reads the lyvecloudfuse registry and returns all the instances to be mounted.
func readRegistryEntry() ([]KeyData, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, instanceRegistry, registry.ALL_ACCESS)
	if err != nil {
		log.Err("Unable to read instance names from Windows Registry: %v", err.Error())
		return nil, err
	}
	defer key.Close()

	keys, err := key.ReadSubKeyNames(-1)
	if err != nil {
		log.Err("Unable to read subkey names from Windows Registry: %v", err.Error())
		return nil, err
	}

	var data []KeyData

	for _, k := range keys {
		d, err := ReadRegistryInstanceEntry(k)
		if err != nil {

		}

		data = append(data, d)
	}

	return data, nil
}

// CreateWinFspRegistry creates an entry in the registry for WinFsp
// so the WinFsp launch tool can launch our mounts.
func CreateWinFspRegistry() error {
	registryPath := winFspRegistry + SvcName
	executablePath, err := os.Executable()
	if err != nil {
		return err
	}

	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, registryPath, registry.ALL_ACCESS)
	if err != nil {
		return err
	}

	err = key.SetStringValue("Executable", executablePath)
	if err != nil {
		return err
	}

	err = key.SetStringValue("CommandLine", mountCmd)
	if err != nil {
		return err
	}
	err = key.SetStringValue("Security", security)
	if err != nil {
		return err
	}
	err = key.SetDWordValue("JobControl", jobControl)
	if err != nil {
		return err
	}

	return nil
}

// RemoveWinFspRegistry removes the entry in the registry for WinFsp.
func RemoveWinFspRegistry() error {
	registryPath := winFspRegistry + SvcName
	err := registry.DeleteKey(registry.LOCAL_MACHINE, registryPath)
	if err != nil {
		return err
	}

	return nil
}

// CreateRegistryMount adds an entry to our registry that
func CreateRegistryMount(mountPath string, configFile string) error {
	registryPath := instanceRegistry + mountPath
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, registryPath, registry.ALL_ACCESS)
	if err != nil {
		return err
	}

	err = key.SetStringValue("ConfigFile", configFile)
	if err != nil {
		return err
	}

	return nil
}

// RemoveRegistryMount removes the entry from our registry
func RemoveRegistryMount(name string) error {
	registryPath := instanceRegistry + name
	err := registry.DeleteKey(registry.LOCAL_MACHINE, registryPath)
	if err != nil {
		return err
	}

	return nil
}

// RemoveRegistryMount removes the entire lyvecloudfuse registry
func RemoveAllRegistryMount() error {
	err := registry.DeleteKey(registry.LOCAL_MACHINE, lcfRegistry)
	if err != nil {
		return err
	}

	return nil
}
