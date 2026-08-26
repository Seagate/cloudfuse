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
	"errors"
	"fmt"
	"os"
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
}

var healthMonCmd = &cobra.Command{
	Use:        "health-monitor",
	Short:      "Monitor cloudfuse mount",
	Long:       "Monitor a cloudfuse mount point for health and performance.\nThis command is typically spawned by the mount command when health monitoring is enabled.",
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

		// Default output path to the cloudfuse working directory when not set
		// via config. This keeps JSON stats files alongside cloudfuse logs.
		if options.MonitorOpt.OutputPath == "" {
			options.MonitorOpt.OutputPath = hmcommon.DefaultWorkDir
		}

		cliParams := buildCliParamForMonitor()
		log.Debug("health-monitor : Options = %v", cliParams)
		log.Debug("health-monitor : Starting health-monitor for cloudfuse pid = %s", pid)

		// Ensure the output directory exists before spawning the monitor subprocess.
		// This is important when running as a service where the child process may
		// have more restrictive permissions than the parent.
		if options.MonitorOpt.OutputPath != "" {
			err := os.MkdirAll(options.MonitorOpt.OutputPath, 0755)
			if err != nil {
				common.EnableMonitoring = false
				log.Err(
					"health-monitor : failed to create output directory [%s] [%s]",
					options.MonitorOpt.OutputPath,
					err.Error(),
				)
				return fmt.Errorf("failed to create output directory: %w", err)
			}
		}

		var hmcmd *exec.Cmd
		hmBinary := hmcommon.CfuseMon
		if runtime.GOOS == "windows" {
			hmBinary += ".exe"
		}

		monPath, err := findHMBinary(hmBinary)
		if err == nil {
			//nolint:gosec // G204: executable path is resolved via LookPath; args are not interpreted by a shell.
			hmcmd = exec.Command(monPath, cliParams...)
			var cliOut []byte
			cliOut, err = hmcmd.Output()
			if len(cliOut) > 0 {
				log.Debug("health-monitor : cliout = %v", string(cliOut))
			}
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

// findHMBinary locates the health-monitor binary.
// It first checks the directory of the running cloudfuse executable, then falls back to PATH.
func findHMBinary(hmBinary string) (string, error) {
	// search PATH
	foundPath, err := exec.LookPath(hmBinary)
	if errors.Is(err, exec.ErrDot) {
		return filepath.Abs(foundPath)
	}
	// fallback to a sibling of the current executable
	if exePath, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(exePath), hmBinary)
		if _, err := os.Stat(sibling); err == nil {
			return sibling, nil
		}
	}

	return "", fmt.Errorf("health-monitor binary %s not found", hmBinary)
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
