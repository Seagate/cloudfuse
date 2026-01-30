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
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/component/file_cache"
	hmcommon "github.com/Seagate/cloudfuse/tools/health-monitor/common"

	"github.com/spf13/cobra"
)

type monitorOptions struct {
	EnableMon       bool     `config:"enable-monitoring"`
	DisableList     []string `config:"monitor-disable-list"`
	CfsPollInterval int      `config:"stats-poll-interval-sec"`
	ProcMonInterval int      `config:"process-monitor-interval-sec"`
	OutputPath      string   `config:"output-path"`
}

var pid string
var cacheMonitorOptions file_cache.FileCacheOptions
var configFile string

func resetMonitorOptions() {
	options.MonitorOpt = monitorOptions{}
	cacheMonitorOptions = file_cache.FileCacheOptions{}
	cacheMonitorOptions.SyncToFlush = true
}

var healthMonCmd = &cobra.Command{
	Use:        "health-monitor",
	Short:      "Monitor cloudfuse mount",
	Long:       "Monitor cloudfuse mount",
	SuggestFor: []string{"cfusemon", "monitor health"},
	Args:       cobra.ExactArgs(0),
	Hidden:     true,
	RunE: func(_ *cobra.Command, _ []string) error {
		resetMonitorOptions()

		err := validateHMonOptions()
		if err != nil {
			log.Err("health-monitor : failed to validate options [%s]", err.Error())
			return fmt.Errorf("failed to validate options: %w", err)
		}

		options.ConfigFile = configFile
		err = parseConfig()
		if err != nil {
			log.Err("health-monitor : failed to parse config [%s]", err.Error())
			return fmt.Errorf("failed to parse config: %w", err)
		}

		err = config.UnmarshalKey("file_cache", &cacheMonitorOptions)
		if err != nil {
			log.Err(
				"health-monitor : file_cache config error (invalid config attributes) [%s]",
				err.Error(),
			)
			return fmt.Errorf("invalid file_cache config: %w", err)
		}

		err = config.UnmarshalKey("health_monitor", &options.MonitorOpt)
		if err != nil {
			log.Err(
				"health-monitor : health_monitor config error (invalid config attributes) [%s]",
				err.Error(),
			)
			return fmt.Errorf("invalid health_monitor config: %w", err)
		}

		cliParams := buildCliParamForMonitor()
		log.Debug("health-monitor : Options = %v", cliParams)
		log.Debug("health-monitor : Starting health-monitor for cloudfuse pid = %s", pid)

		var hmcmd *exec.Cmd
		if runtime.GOOS == "windows" {
			path, err := filepath.Abs(hmcommon.CfuseMon + ".exe")
			if err != nil {
				return fmt.Errorf("failed to start health monitor: %w", err)
			}
			hmcmd = exec.Command(path, cliParams...)
		} else {
			hmcmd = exec.Command(hmcommon.CfuseMon, cliParams...)
		}
		cliOut, err := hmcmd.Output()
		if len(cliOut) > 0 {
			log.Debug("health-monitor : cliout = %v", string(cliOut))
		}

		if err != nil {
			common.EnableMonitoring = false
			log.Err("health-monitor : failed to start health monitor [%s]", err.Error())
			return fmt.Errorf("failed to start health monitor: %w", err)
		}

		return nil
	},
}

func validateHMonOptions() error {
	pid = strings.TrimSpace(pid)
	configFile = strings.TrimSpace(configFile)
	errMsg := ""

	if len(pid) == 0 {
		errMsg = "pid of cloudfuse process not given. "
	}

	if len(configFile) == 0 {
		errMsg += "config file not given."
	}

	if len(errMsg) != 0 {
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

func buildCliParamForMonitor() []string {
	var cliParams []string

	cliParams = append(cliParams, "--pid="+pid)
	if options.MonitorOpt.CfsPollInterval != 0 {
		cliParams = append(
			cliParams,
			fmt.Sprintf("--stats-poll-interval-sec=%v", options.MonitorOpt.CfsPollInterval),
		)
	}
	if options.MonitorOpt.ProcMonInterval != 0 {
		cliParams = append(
			cliParams,
			fmt.Sprintf("--process-monitor-interval-sec=%v", options.MonitorOpt.ProcMonInterval),
		)
	}

	if options.MonitorOpt.OutputPath != "" {
		cliParams = append(
			cliParams,
			fmt.Sprintf("--output-path=%v", options.MonitorOpt.OutputPath),
		)
	}

	cliParams = append(cliParams, "--cache-path="+common.ExpandPath(cacheMonitorOptions.TmpPath))
	cliParams = append(cliParams, fmt.Sprintf("--max-size-mb=%v", cacheMonitorOptions.MaxSizeMB))

	for _, v := range options.MonitorOpt.DisableList {
		switch v {
		case hmcommon.CloudfuseStats:
			cliParams = append(cliParams, "--no-cloudfuse-stats")
		case hmcommon.CpuProfiler:
			cliParams = append(cliParams, "--no-cpu-profiler")
		case hmcommon.MemoryProfiler:
			cliParams = append(cliParams, "--no-memory-profiler")
		case hmcommon.NetworkProfiler:
			cliParams = append(cliParams, "--no-network-profiler")
		case hmcommon.FileCacheMon:
			cliParams = append(cliParams, "--no-file-cache-monitor")
		default:
			log.Debug(
				"health-monitor::buildCliParamForMonitor: Invalid health monitor option %v",
				v,
			)
		}
	}

	return cliParams
}

func init() {
	rootCmd.AddCommand(healthMonCmd)

	healthMonCmd.Flags().StringVar(&pid, "pid", "", "Pid of cloudfuse process")
	_ = healthMonCmd.MarkFlagRequired("pid")

	healthMonCmd.Flags().StringVar(&configFile, "config-file", "config.yaml",
		"Configures the path for the file where the account credentials are provided. Default is config.yaml")
	_ = healthMonCmd.MarkFlagRequired("config-file")
	_ = healthMonCmd.MarkFlagFilename("config-file", "yaml")
}
