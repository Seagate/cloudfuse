/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023 Seagate Technology LLC and/or its Affiliates
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

package blobfuse_stats

import (
	"fmt"

	"lyvecloudfuse/common"
	"lyvecloudfuse/common/log"
	hmcommon "lyvecloudfuse/tools/health-monitor/common"
	hminternal "lyvecloudfuse/tools/health-monitor/internal"
)

type BlobfuseStats struct {
	name         string
	pollInterval int
	transferPipe string
	pollingPipe  string
}

func (bfs *BlobfuseStats) GetName() string {
	return bfs.name
}

func (bfs *BlobfuseStats) SetName(name string) {
	bfs.name = name
}

func (bfs *BlobfuseStats) Monitor() error {
	err := bfs.Validate()
	if err != nil {
		log.Err("StatsReader::Monitor : [%v]", err)
		return err
	}
	log.Debug("StatsReader::Monitor : started")

	go bfs.statsPoll()

	return bfs.statsReader()
}

func (bfs *BlobfuseStats) ExportStats(timestamp string, st interface{}) {
	se, err := hminternal.NewStatsExporter()
	if err != nil || se == nil {
		log.Err("StatsReader::ExportStats : Error in creating stats exporter instance [%v]", err)
		return
	}

	se.AddMonitorStats(bfs.GetName(), timestamp, st)
}

func (bfs *BlobfuseStats) Validate() error {
	if bfs.pollInterval == 0 {
		return fmt.Errorf("blobfuse-poll-interval should be non-zero")
	}

	err := hmcommon.CheckProcessStatus(hmcommon.Pid)
	if err != nil {
		return err
	}

	return nil
}

func NewBlobfuseStatsMonitor() hminternal.Monitor {
	bfs := &BlobfuseStats{
		pollInterval: hmcommon.BfsPollInterval,
		transferPipe: common.TransferPipe,
		pollingPipe:  common.PollingPipe,
	}

	bfs.SetName(hmcommon.BlobfuseStats)

	return bfs
}

func init() {
	hminternal.AddMonitor(hmcommon.BlobfuseStats, NewBlobfuseStatsMonitor)
}
