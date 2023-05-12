//go:build windows

package windowsService

import (
	"lyvecloudfuse/common/log"
	"os"

	"golang.org/x/sys/windows/registry"
)

// Windows Registry Paths
const (
	lcfRegistry    = `SOFTWARE\Seagate\LyveCloudFuse\Instances\`
	winFspRegistry = `SOFTWARE\WOW6432Node\WinFsp\Services\`
)

// WinFsp registry contants
const (
	jobControl = 1
	mountCmd   = `mount %1 --config-file=%2`
	security   = `D:P(A;;RPWPLC;;;WD)`
)

func ReadRegistryInstanceEntry(name string) (KeyData, error) {
	registryPath := lcfRegistry + name

	key, err := registry.OpenKey(registry.LOCAL_MACHINE, registryPath, registry.ALL_ACCESS)
	if err != nil {
		log.Err("Unable to read instance names from Windows Registry: %v", err.Error())
		return KeyData{}, err
	}
	defer key.Close()

	var d KeyData
	d.InstanceName = name

	d.ConfigFile, _, err = key.GetStringValue("ConfigFile")
	if err != nil {
		log.Err("Unable to read key ConfigFile from instance in Windows Registry: %v", err.Error())
		return KeyData{}, err
	}

	d.MountDir, _, err = key.GetStringValue("MountDir")
	if err != nil {
		log.Err("Unable to read key MountDir from instance in Windows Registry: %v", err.Error())
		return KeyData{}, err
	}

	return d, nil
}

func readRegistryEntry() ([]KeyData, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, lcfRegistry, registry.ALL_ACCESS)
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

// createRegistryEntry creates an entry in the registry for WinFsp
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

// CreateRegistryMount adds an entry to our registry that
func CreateRegistryMount(name string, mountDir string, configFile string) error {
	registryPath := lcfRegistry + name
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, registryPath, registry.ALL_ACCESS)
	if err != nil {
		return err
	}

	err = key.SetStringValue("MountDir", mountDir)
	if err != nil {
		return err
	}

	err = key.SetStringValue("ConfigFile", configFile)
	if err != nil {
		return err
	}

	return nil
}

func RemoveRegistryMount(name string) error {
	registryPath := lcfRegistry + name
	err := registry.DeleteKey(registry.LOCAL_MACHINE, registryPath)
	if err != nil {
		return err
	}

	return nil
}
