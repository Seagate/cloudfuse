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

package common

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Seagate/cloudfuse/common/log"

	"github.com/shirou/gopsutil/v4/process"
)

// check whether cloudfuse process is running for the given pid
func CheckProcessStatus(pid string) error {
	if runtime.GOOS == "windows" {
		pid, err := strconv.ParseInt(pid, 10, 32)
		if err != nil {
			log.Err("cpu_mem_monitor::getCpuMemoryUsage : Error parsing process id")
			return err
		}

		// Check if the pid is a current process, if err == nil
		// then the process is running
		_, err = process.NewProcess(int32(pid))
		if err != nil {
			return fmt.Errorf("cloudfuse is not running on pid %v", pid)
		}
		return nil
	}

	// We are running on Linux in this case
	cmd := "ps -ef | grep " + pid
	cliOut, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return err
	}

	processes := strings.Split(string(cliOut), "\n")
	for _, process := range processes {
		l := strings.Fields(process)
		if len(l) >= 2 && l[1] == pid {
			return nil
		}
	}

	return fmt.Errorf("cloudfuse is not running on pid %v", pid)
}

// check cloudfuse pid status at every second
func MonitorPid() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for t := range ticker.C {
		err := CheckProcessStatus(Pid)
		if err != nil {
			log.Err("util::MonitorPid : time = %v, [%v]", t.Format(time.RFC3339), err)
			// wait for 5 seconds for monitor threads to exit
			time.Sleep(5 * time.Second)
			break
		}
	}
}
