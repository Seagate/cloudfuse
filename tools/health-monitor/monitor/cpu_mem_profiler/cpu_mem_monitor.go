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

package cpu_mem_profiler

import (
	"fmt"
	"math"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Seagate/cloudfuse/common/log"
	hmcommon "github.com/Seagate/cloudfuse/tools/health-monitor/common"
	hminternal "github.com/Seagate/cloudfuse/tools/health-monitor/internal"

	"github.com/shirou/gopsutil/v4/process"
)

type CpuMemProfiler struct {
	name         string
	pid          string
	pollInterval int
}

func (cm *CpuMemProfiler) GetName() string {
	return cm.name
}

func (cm *CpuMemProfiler) SetName(name string) {
	cm.name = name
}

func (cm *CpuMemProfiler) Monitor() error {
	err := cm.Validate()
	if err != nil {
		log.Err("cpu_mem_monitor::Monitor : [%v]", err)
		return err
	}
	log.Debug("cpu_mem_monitor::Monitor : started")

	ticker := time.NewTicker(time.Duration(cm.pollInterval) * time.Second)
	defer ticker.Stop()

	for t := range ticker.C {
		c, err := cm.getCpuMemoryUsage()
		if err != nil {
			log.Err("cpu_mem_monitor::Monitor : [%v]", err)
			return err
		}

		if !hmcommon.NoCpuProf {
			cm.ExportStats(t.Format(time.RFC3339), c.CpuUsage)
		}
		if !hmcommon.NoMemProf {
			cm.ExportStats(t.Format(time.RFC3339), c.MemUsage)
		}
	}

	return nil
}

func (cm *CpuMemProfiler) ExportStats(timestamp string, st interface{}) {
	se, err := hminternal.NewStatsExporter()
	if err != nil || se == nil {
		log.Err(
			"cpu_mem_monitor::ExportStats : Error in creating stats exporter instance [%v]",
			err,
		)
		return
	}

	s := st.(string)
	if s[len(s)-1] == '%' {
		se.AddMonitorStats(hmcommon.CpuProfiler, timestamp, st)
	} else {
		se.AddMonitorStats(hmcommon.MemoryProfiler, timestamp, st)
	}
}

func (cm *CpuMemProfiler) Validate() error {
	if len(cm.pid) == 0 {
		return fmt.Errorf("pid of cloudfuse is not given")
	}

	if cm.pollInterval == 0 {
		return fmt.Errorf("process-monitor-interval-sec should be non-zero")
	}

	return nil
}

func (cm *CpuMemProfiler) getCpuMemoryUsage() (*hmcommon.CpuMemStat, error) {
	// If on windows we use an external library to get CPU and memory usage
	if runtime.GOOS == "windows" {
		pid, err := strconv.ParseInt(cm.pid, 10, 32)
		if err != nil {
			log.Err("cpu_mem_monitor::getCpuMemoryUsage : Error parsing process id")
			return nil, err
		}

		proc, err := process.NewProcess(int32(pid))
		if err != nil {
			log.Err("cpu_mem_monitor::getCpuMemoryUsage : Error getting process")
			return nil, err
		}

		cpuPercent, err := proc.CPUPercent()
		if err != nil {
			log.Err("cpu_mem_monitor::getCpuMemoryUsage : Error getting cpu percentage")
			cpuPercent = 0
		}

		memInfo, err := proc.MemoryInfo()
		var memUsage uint64
		if err != nil {
			log.Err("cpu_mem_monitor::getCpuMemoryUsage : Error getting memory usage")
		} else {
			memUsage = memInfo.RSS
		}

		// Get CPU as a percentage and the number of kilobytes of memory
		cpuMemStat := &hmcommon.CpuMemStat{
			CpuUsage: fmt.Sprintf("%.2f", cpuPercent),
			MemUsage: fmt.Sprintf("%dk", memUsage/1024),
		}
		cpuMemStat.CpuUsage += "%"
		return cpuMemStat, nil
	}

	topCmd := "top -b -n 1 -d 0.2 -p " + cm.pid + " | tail -2"

	cliOut, err := exec.Command("bash", "-c", topCmd).Output()
	if err != nil {
		log.Err(
			"cpu_mem_monitor::getCpuMemoryUsage : Cloudfuse is not running on pid %v [%v]",
			cm.pid,
			err,
		)
		return nil, err
	}

	processes := strings.Split(strings.Trim(string(cliOut), "\n"), "\n")
	if len(processes) < 2 {
		log.Err("cpu_mem_monitor::getCpuMemoryUsage : Cloudfuse is not running on pid %v", cm.pid)
		return nil, fmt.Errorf("cloudfuse is not running on pid %v", cm.pid)
	}

	cpuIndex, memIndex := getCpuMemIndex(processes[0])
	stats := strings.Fields(processes[1])
	if cpuIndex == -1 || memIndex == -1 ||
		len(stats) <= int(math.Max(float64(cpuIndex), float64(memIndex))) ||
		len(stats[cpuIndex]) == 0 ||
		len(stats[memIndex]) == 0 {
		log.Debug("cpu_mem_monitor::getCpuMemoryUsage : %v", processes)
		log.Err("cpu_mem_monitor::getCpuMemoryUsage : Cloudfuse is not running on pid %v", cm.pid)
		return nil, fmt.Errorf("cloudfuse is not running on pid %v", cm.pid)
	}

	cpuMemStat := &hmcommon.CpuMemStat{
		CpuUsage: stats[cpuIndex],
		MemUsage: stats[memIndex],
	}
	cpuMemStat.CpuUsage += "%"
	if cpuMemStat.MemUsage[len(cpuMemStat.MemUsage)-1] >= '0' &&
		cpuMemStat.MemUsage[len(cpuMemStat.MemUsage)-1] <= '9' {
		cpuMemStat.MemUsage += "k"
	}

	return cpuMemStat, nil
}

func getCpuMemIndex(process string) (int, int) {
	cols := strings.Fields(process)
	var cpuIndex, memIndex = -1, -1
	for i, col := range cols {
		switch col {
		case "%CPU":
			cpuIndex = i
		case "VIRT":
			memIndex = i
		}
	}
	return cpuIndex, memIndex
}

func NewCpuMemoryMonitor() hminternal.Monitor {
	cm := &CpuMemProfiler{
		pid:          hmcommon.Pid,
		pollInterval: hmcommon.ProcMonInterval,
	}

	cm.SetName(hmcommon.CpuMemoryProfiler)

	return cm
}

func init() {
	hminternal.AddMonitor(hmcommon.CpuMemoryProfiler, NewCpuMemoryMonitor)
}
