//go:build windows

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
   SOFTWARE
*/

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/winservice"
)

// Create dummy function so that mount.go code can compile
// This function is used only on Linux, so it creates an empty context here
func createDaemon(pipeline *internal.Pipeline, ctx context.Context, pidFileName string, pidFilePerm os.FileMode, umask int, fname string) error {
	return nil
}

// Use WinFSP to mount and if successful, add instance to persistent mount list
func createMountInstance(enableRemountUser bool, enableRemountSystem bool) error {
	err := winservice.StartMount(options.MountPath, options.ConfigFile, encryptedPassphrase)
	if err != nil {
		return err
	}
	// Add the mount to the JSON file so it persists on restart.
	if enableRemountUser || enableRemountSystem {
		err = winservice.AddMountJSON(options.MountPath, options.ConfigFile, enableRemountSystem)
		if err != nil {
			return fmt.Errorf("failed to add entry to json file [%s]", err.Error())
		}
	}
	return nil
}

// stub
func installRemountService(string, string, string) (string, error) {
	return "", nil
}

// stub
func startService(string) error {
	return nil
}
