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

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"lyvecloudfuse/common"
	"lyvecloudfuse/common/log"
	hmcommon "lyvecloudfuse/tools/health-monitor/common"
	hminternal "lyvecloudfuse/tools/health-monitor/internal"
	_ "lyvecloudfuse/tools/health-monitor/monitor"
)

func getMonitors() []hminternal.Monitor {
	compMap := map[string]bool{
		hmcommon.BlobfuseStats:     hmcommon.NoBfsMon,
		hmcommon.CpuMemoryProfiler: (hmcommon.NoCpuProf && hmcommon.NoMemProf),
		hmcommon.NetworkProfiler:   hmcommon.NoNetProf,
		hmcommon.FileCacheMon:      hmcommon.NoFileCacheMon,
	}

	comps := make([]hminternal.Monitor, 0)

	for name, disabled := range compMap {
		if !disabled {
			obj, err := hminternal.GetMonitor(name)
			if err != nil {
				log.Err("main::getMonitors : [%v]", err)
				continue
			}
			comps = append(comps, obj)
		}
	}

	return comps
}

func main() {
	flag.Parse()

	if hmcommon.CheckVersion {
		fmt.Printf("health-monitor version %s\n", hmcommon.BfuseMonitorVersion)
		return
	}

	err := log.SetDefaultLogger("base", common.LogConfig{
		Level:       common.ELogLevel.LOG_DEBUG(),
		FilePath:    common.ExpandPath(hmcommon.DefaultLogFile),
		MaxFileSize: common.DefaultMaxLogFileSize,
		FileCount:   common.DefaultLogFileCount,
		TimeTracker: false,
		Tag:         hmcommon.BfuseMon,
	})

	if err != nil {
		fmt.Printf("Health Monitor: error initializing logger [%v]", err)
		os.Exit(1)
	}

	if len(strings.TrimSpace(hmcommon.Pid)) == 0 {
		fmt.Printf("pid of lyvecloudfuse process not provided\n")
		log.Err("main::main : pid of lyvecloudfuse process not provided")
		time.Sleep(1 * time.Second) // adding 1 second wait for adding to log(base type) before exiting
		os.Exit(1)
	}

	if hmcommon.OutputPath == "" {
		currDir, err := os.Getwd()
		if err != nil {
			fmt.Printf("health-monitor : failed to get current directory [%s]\n", err.Error())
			log.Err("main::main : failed to get current directory [%s]\n", err.Error())
			return
		}
		hmcommon.OutputPath = currDir
	}

	common.TransferPipe += "_" + hmcommon.Pid
	common.PollingPipe += "_" + hmcommon.Pid

	log.Debug("Lyvecloudfuse Pid: %v \n"+
		"Transfer Pipe: %v \n"+
		"Polling Pipe: %v \n"+
		"Lyvecloudfuse Stats poll interval: %v \n"+
		"Health Stats poll interval: %v \n"+
		"Cache Path: %v \n"+
		"Max cache size in MB: %v \n",
		"Output path: %v",
		hmcommon.Pid, common.TransferPipe, common.PollingPipe, hmcommon.BfsPollInterval,
		hmcommon.ProcMonInterval, hmcommon.TempCachePath, hmcommon.MaxCacheSize, hmcommon.OutputPath)

	comps := getMonitors()

	for _, obj := range comps {
		go obj.Monitor() // nolint
	}

	// check if the pid of lyvecloudfuse is active
	if len(comps) > 0 {
		hmcommon.MonitorPid()
	}

	err = hminternal.CloseExporter()
	if err != nil {
		log.Err("main::main : Unable to close exporter [%v]", err)
	}

	log.Debug("Monitoring ended")
}

func init() {
	flag.StringVar(&hmcommon.Pid, "pid", "", "Pid of lyvecloudfuse process")
	flag.IntVar(&hmcommon.BfsPollInterval, "stats-poll-interval-sec", 10, "Lyvecloudfuse stats polling interval in seconds")
	flag.IntVar(&hmcommon.ProcMonInterval, "process-monitor-interval-sec", 30, "CPU, memory and network usage polling interval in seconds")
	flag.StringVar(&hmcommon.OutputPath, "output-path", "", "Path where output files will be created")

	flag.BoolVar(&hmcommon.NoBfsMon, "no-lyvecloudfuse-stats", false, "Disable lyvecloudfuse stats polling")
	flag.BoolVar(&hmcommon.NoCpuProf, "no-cpu-profiler", false, "Disable CPU monitoring on lyvecloudfuse process")
	flag.BoolVar(&hmcommon.NoMemProf, "no-memory-profiler", false, "Disable memory monitoring on lyvecloudfuse process")
	flag.BoolVar(&hmcommon.NoNetProf, "no-network-profiler", false, "Disable network monitoring on lyvecloudfuse process")
	flag.BoolVar(&hmcommon.NoFileCacheMon, "no-file-cache-monitor", false, "Disable file cache directory monitor")

	flag.StringVar(&hmcommon.TempCachePath, "cache-path", "", "path to local disk cache")
	flag.Float64Var(&hmcommon.MaxCacheSize, "max-size-mb", 0, "maximum cache size allowed. Default - 0 (unlimited)")

	flag.BoolVar(&hmcommon.CheckVersion, "version", false, "Print the current version of health-monitor")
}
