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
   SOFTWARE
*/

package winservice

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

// Windows Registry Paths
const (
	winFspRegistry = `SOFTWARE\WOW6432Node\WinFsp\Services\`
)

// WinFsp registry constants. JobControl is specified to be 1 by default in WinFsp. The security
// string is a Windows SDDL string which specifies access control and this is the default given by WinFsp.
// Here is syntax for SDDL
// https://web.archive.org/web/20160320065614/http://www.netid.washington.edu/documentation/domains/sddl.aspx
const (
	jobControl = 1
	security   = `D:P(A;;RPWPLC;;;WD)`
)

// Specific mount command used in cloudfuse. This is the command that is executed when WinFsp launches our service.
// %1-%5 are strings that are added when mounting where:
// %1 is the mount directory
// %2 is the location of the config file
// %3 is the current user's Windows user ID
// %4 is the current user's Windows group ID
// %5 is the passphrase if the config file is encrypted
const mountCmd = `mount %1 --config-file=%2 -o uid=%3,gid=%4 --passphrase=%5 --foreground=true`

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
	defer key.Close()

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

func AddRegistryValue(keyName string, valueName string, value string) error {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, keyName, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	err = key.SetStringValue(valueName, value)
	if err != nil {
		return err
	}

	return nil
}

func RemoveRegistryValue(keyName string, valueName string) error {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, keyName, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	// Check if the value exists before trying to delete it.
	_, _, err = key.GetStringValue(valueName)
	if err != nil {
		// the entry already doesn't exist - no need to report an error
		return nil
	}

	// Delete the registry value
	err = key.DeleteValue(valueName)
	if err != nil {
		return err
	}

	return nil
}
