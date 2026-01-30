/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.

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
	"os/exec"
	"runtime"

	hmcommon "github.com/Seagate/cloudfuse/tools/health-monitor/common"

	"github.com/spf13/cobra"
)

var healthMonStopAll = &cobra.Command{
	Use:        "all",
	Short:      "Stop all health monitor binaries",
	Long:       "Stop all health monitor binaries",
	SuggestFor: []string{"al"},
	Args:       cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := stopAll()
		if err != nil {
			return fmt.Errorf("failed to stop all health monitor binaries: %w", err)
		}
		return nil
	},
}

// Attempts to kill all health monitors
func stopAll() error {
	if runtime.GOOS == "windows" {
		cliOut := exec.Command("taskkill", "/IM", "cfusemon.exe", "/F")
		_, err := cliOut.Output()
		if err != nil {
			return err
		}
		fmt.Println("Successfully stopped all health monitor binaries.")
		return nil
	}
	cliOut := exec.Command("killall", hmcommon.CfuseMon)
	_, err := cliOut.Output()
	if err != nil {
		return err
	}
	fmt.Println("Successfully stopped all health monitor binaries.")
	return nil
}

func init() {
	healthMonStop.AddCommand(healthMonStopAll)
}
