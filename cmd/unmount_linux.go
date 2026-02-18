//go:build linux

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2026 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.

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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// stub
func unmountCloudfuseWindows(string, bool, bool) error {
	return nil
}

func uninstallService(mountPath string) error {
	mountPath, err := filepath.Abs(mountPath)
	if err != nil {
		return fmt.Errorf("couldn't determine absolute path from string [%s]", err.Error())
	}
	serviceName, serviceFilePath := getService(mountPath)
	if _, err := os.Stat(serviceFilePath); err == nil {
		removeFileCmd := exec.Command("rm", serviceFilePath)
		err := removeFileCmd.Run()
		if err != nil {
			return fmt.Errorf(
				"failed to delete "+serviceName+" file from /etc/systemd/system [%s]",
				err.Error(),
			)
		}
	} else if os.IsNotExist(err) {
		return fmt.Errorf(
			"failed to delete "+serviceName+" file from /etc/systemd/system [%s]",
			err.Error(),
		)
	}
	// reload daemon
	systemctlDaemonReloadCmd := exec.Command("systemctl", "daemon-reload")
	err = systemctlDaemonReloadCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run 'systemctl daemon-reload' command [%s]", err.Error())
	}
	return nil
}
