//go:build windows

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
   SOFTWARE
*/

package winservice

import (
	"os"

	"github.com/Seagate/cloudfuse/common/log"

	"golang.org/x/sys/windows/registry"
)

// Windows Registry Paths
const (
	cfRegistry       = `SOFTWARE\Seagate\Cloudfuse\`
	instanceRegistry = cfRegistry + `Instances\`
	winFspRegistry   = `SOFTWARE\WOW6432Node\WinFsp\Services\`
)

// WinFsp registry constants. JobControl is specified to be 1 by default in WinFsp. The security
// string is a Windows SDDL string which specifies access control and this is the default given by WinFsp.
// Here is syntax for SDDL
// https://web.archive.org/web/20160320065614/http://www.netid.washington.edu/documentation/domains/sddl.aspx
const (
	jobControl = 1
	security   = `D:P(A;;RPWPLC;;;WD)`
)

type KeyData struct {
	MountPath  string
	ConfigFile string
}

// Specific mount command used in cloudfuse. This is the command that is executed when WinFsp launches our service.
// %1-%4 are strings that are added when mounting where:
// %1 is the mount directory
// %2 is the location of the config file
// %3 is the current user's Windows user ID
// %4 is the current user's Windows group ID
const mountCmd = `mount %1 --config-file=%2 -o uid=%3,gid=%4 --foreground=true`

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

// readRegistryEntry reads the cloudfuse registry and returns all the instances to be mounted.
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
		d, _ := ReadRegistryInstanceEntry(k)
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

// RemoveRegistryMount removes the entire cloudfuse registry
func RemoveAllRegistryMount() error {
	err := registry.DeleteKey(registry.LOCAL_MACHINE, cfRegistry)
	if err != nil {
		return err
	}

	return nil
}
