/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

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
	"regexp"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var lyvecloudfusePid string

var healthMonStop = &cobra.Command{
	Use:               "stop",
	Short:             "Stops the health monitor binary associated with a given Lyvecloudfuse pid",
	Long:              "Stops the health monitor binary associated with a given Lyvecloudfuse pid",
	SuggestFor:        []string{"stp", "st"},
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		lyvecloudfusePid = strings.TrimSpace(lyvecloudfusePid)

		if len(lyvecloudfusePid) == 0 {
			return fmt.Errorf("pid of lyvecloudfuse process not given")
		}

		pid, err := getPid(lyvecloudfusePid)
		if err != nil {
			return fmt.Errorf("failed to get health monitor pid")
		}

		err = stop(pid)
		if err != nil {
			return fmt.Errorf("failed to stop health monitor")
		}

		return nil
	},
}

// Attempts to get pid of the health monitor
func getPid(lyvecloudfusePid string) (string, error) {
	if runtime.GOOS == "windows" {
		cliOut := exec.Command("wmic", "process", "where", fmt.Sprintf("ParentProcessId=%s", lyvecloudfusePid), "get", "ProcessId")
		output, err := cliOut.Output()
		if err != nil {
			return "", err
		}
		strOutput := string(output)

		if strings.Contains(strOutput, "No Instance") {
			return "", fmt.Errorf("failed to process PID from %s", lyvecloudfusePid)
		}
		lines := strings.Split(strings.TrimSpace(strOutput), "\n")[1:]

		if len(lines) == 0 {
			return "", fmt.Errorf("failed to process PID from %s", lyvecloudfusePid)
		}
		pid := strings.TrimSpace(lines[0])
		return pid, nil
	}

	psAux := exec.Command("ps", "aux")
	out, err := psAux.Output()
	if err != nil {
		return "", err
	}
	processes := strings.Split(string(out), "\n")
	for _, process := range processes {
		if strings.Contains(process, "bfusemon") && strings.Contains(process, fmt.Sprintf("--pid=%s", lyvecloudfusePid)) {
			re := regexp.MustCompile(`[-]?\d[\d,]*[\.]?[\d{2}]*`)
			pids := re.FindAllString(process, 1)
			if pids == nil {
				return "", fmt.Errorf("failed to process PID from %s", process)
			}
			pid := pids[0]
			fmt.Printf("Successfully got health monitor PID %s.\n", pid)
			return pid, nil
		}
	}
	return "", fmt.Errorf("no corresponding health monitor PID found")

}

// Attempts to kill all health monitors
func stop(pid string) error {
	if runtime.GOOS == "windows" {
		cliOut := exec.Command("taskkill", "/PID", pid, "/F")
		_, err := cliOut.Output()
		if err != nil {
			return err
		}
		fmt.Println("Successfully stopped health monitor binary.")
		return nil
	}
	cliOut := exec.Command("kill", "-9", pid)
	_, err := cliOut.Output()
	if err != nil {
		return err
	} else {
		fmt.Println("Successfully stopped health monitor binary.")
		return nil
	}
}

func init() {
	healthMonCmd.AddCommand(healthMonStop)
	healthMonStop.AddCommand(healthMonStopAll)

	healthMonStop.Flags().StringVar(&lyvecloudfusePid, "pid", "", "Lyvecloudfuse PID associated with the health monitor that should be stopped")
	_ = healthMonStop.MarkFlagRequired("pid")
}
