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

package cloudfuse_stats

import (
	"fmt"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	hmcommon "github.com/Seagate/cloudfuse/tools/health-monitor/common"
	hminternal "github.com/Seagate/cloudfuse/tools/health-monitor/internal"
)

type CloudfuseStats struct {
	name         string
	pollInterval int
	transferPipe string
	pollingPipe  string
}

func (cfs *CloudfuseStats) GetName() string {
	return cfs.name
}

func (cfs *CloudfuseStats) SetName(name string) {
	cfs.name = name
}

func (cfs *CloudfuseStats) Monitor() error {
	err := cfs.Validate()
	if err != nil {
		log.Err("StatsReader::Monitor : [%v]", err)
		return err
	}
	log.Debug("StatsReader::Monitor : started")

	go cfs.statsPoll()

	return cfs.statsReader()
}

func (cfs *CloudfuseStats) ExportStats(timestamp string, st any) {
	se, err := hminternal.NewStatsExporter()
	if err != nil || se == nil {
		log.Err("StatsReader::ExportStats : Error in creating stats exporter instance [%v]", err)
		return
	}

	se.AddMonitorStats(cfs.GetName(), timestamp, st)
}

func (cfs *CloudfuseStats) Validate() error {
	if cfs.pollInterval == 0 {
		return fmt.Errorf("cloudfuse-poll-interval should be non-zero")
	}

	err := hmcommon.CheckProcessStatus(hmcommon.Pid)
	if err != nil {
		return err
	}

	return nil
}

func NewCloudfuseStatsMonitor() hminternal.Monitor {
	cfs := &CloudfuseStats{
		pollInterval: hmcommon.CfsPollInterval,
		transferPipe: common.TransferPipe,
		pollingPipe:  common.PollingPipe,
	}

	cfs.SetName(hmcommon.CloudfuseStats)

	return cfs
}

func init() {
	hminternal.AddMonitor(hmcommon.CloudfuseStats, NewCloudfuseStatsMonitor)
}
