//go:build windows

/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
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
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"lyvecloudfuse/common"
	"lyvecloudfuse/common/log"
	"lyvecloudfuse/internal/stats_manager"
	hmcommon "lyvecloudfuse/tools/health-monitor/common"
	hminternal "lyvecloudfuse/tools/health-monitor/internal"

	"golang.org/x/sys/windows"
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
		log.Err("stats_reader::ExportStats : Error in creating stats exporter instance [%v]", err)
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

func (bfs *BlobfuseStats) handleStatsReader(handle windows.Handle) error {
	defer windows.CloseHandle(handle)

	var buf [4096]byte
	var bytesRead uint32
	var messageBuf bytes.Buffer
	var e error

	for {
		// Empty the buffer before reading
		messageBuf.Reset()
		// read the polling message sent by stats monitor
		for {
			err := windows.ReadFile(handle, buf[:], &bytesRead, nil)

			if err != nil && err != windows.ERROR_MORE_DATA {
				log.Err("StatsReader::statsReader : Unable to read from pipe [%v]", err)
				return err
			}

			messageBuf.Write(buf[:bytesRead])

			if err != windows.ERROR_MORE_DATA {
				break
			}
		}

		message := messageBuf.String()

		log.Debug("StatsReader::statsReader : Message: %v", message)

		st := stats_manager.PipeMsg{}
		err := json.Unmarshal([]byte(message), &st)
		if err != nil {
			log.Err("StatsReader::statsReader : Unable to unmarshal json [%v]", err)
			continue
		}
		bfs.ExportStats(st.Timestamp, st)
	}

	return e
}

func (bfs *BlobfuseStats) statsReader() error {
	// Accept incoming connections
	for {
		log.Info("StatsReader::statsReader : In stats reader")
		handle, err := windows.CreateNamedPipe(
			windows.StringToUTF16Ptr(bfs.transferPipe),
			windows.PIPE_ACCESS_DUPLEX,
			windows.PIPE_TYPE_MESSAGE|windows.PIPE_READMODE_MESSAGE|windows.PIPE_WAIT,
			windows.PIPE_UNLIMITED_INSTANCES,
			4096,
			4096,
			0,
			nil,
		)
		if err != nil && err != windows.ERROR_PIPE_BUSY {
			log.Err("StatsReader::statsReader : unable to create pipe [%v]", err)
			return err
		}

		// connect to the named pipe
		err = windows.ConnectNamedPipe(handle, nil)
		if err != nil {
			log.Err("StatsReader::statsReader : unable to connect to named pipe %s: [%v]", bfs.transferPipe, err)
			windows.CloseHandle(handle)
			time.Sleep(1 * time.Second)
		}
		log.Info("StatsReader::statsReader : Connected transfer pipe %s", bfs.transferPipe)

		go bfs.handleStatsReader(handle)
	}

	// reader := bufio.NewReader(os.NewFile(uintptr(handle), bfs.transferPipe))
	// var e error

	// for {
	// 	line, err := reader.ReadBytes('\n')
	// 	log.Info(string(line))
	// 	if err != nil {
	// 		log.Err("StatsReader::statsReader : [%v]", err)
	// 		e = err
	// 		break
	// 	}

	// 	log.Debug("StatsReader::statsReader : Line: %v", string(line))

	// 	st := stats_manager.PipeMsg{}
	// 	err = json.Unmarshal(line, &st)
	// 	if err != nil {
	// 		log.Err("StatsReader::statsReader : Unable to unmarshal json [%v]", err)
	// 		continue
	// 	}
	// 	bfs.ExportStats(st.Timestamp, st)
	// }

	// return e
}

func (bfs *BlobfuseStats) statsPoll() {
	var hPipe windows.Handle
	var err error
	for {
		hPipe, err = windows.CreateFile(
			windows.StringToUTF16Ptr(bfs.pollingPipe),
			windows.GENERIC_WRITE,
			0,
			nil,
			windows.OPEN_EXISTING,
			0,
			0,
		)

		if err == nil {
			break
		}

		if err == windows.ERROR_FILE_NOT_FOUND {
			log.Info("StatsReader::statsReader : Named pipe %s not found, retrying...", bfs.pollingPipe)
			time.Sleep(1 * time.Second)
		} else {
			log.Err("StatsReader::statsReader : unable to open pipe file [%v]", err)
			time.Sleep(1 * time.Second)
			// return
		}
	}
	//defer windows.CloseHandle(hPipe)

	log.Info("stats_manager::statsDumper : opened polling pipe file")

	writer := os.NewFile(uintptr(hPipe), bfs.pollingPipe)

	ticker := time.NewTicker(time.Duration(bfs.pollInterval) * time.Second)
	defer ticker.Stop()

	for t := range ticker.C {
		_, err = writer.WriteString(fmt.Sprintf("Poll at %v\n", t.Format(time.RFC3339)))
		log.Info("stats_manager::statsDumper : writing to polling pipe file:", fmt.Sprintf("Poll at %v\n", t.Format(time.RFC3339)))
		if err != nil {
			log.Err("StatsReader::statsPoll : [%v]", err)
			break
		}
	}
}

func NewBlobfuseStatsMonitor() hminternal.Monitor {
	bfs := &BlobfuseStats{
		pollInterval: hmcommon.BfsPollInterval,
		transferPipe: common.WindowsTransferPipe,
		pollingPipe:  common.WindowsPollingPipe,
	}

	bfs.SetName(hmcommon.BlobfuseStats)

	return bfs
}

// func createPipe(pipe string) error {
// 	handle, err := windows.CreateNamedPipe(
// 		windows.StringToUTF16Ptr(pipe),
// 		windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_OVERLAPPED,
// 		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT,
// 		windows.PIPE_UNLIMITED_INSTANCES,
// 		4096,
// 		4096,
// 		0,
// 		nil,
// 	)
// 	if err != nil && err != windows.ERROR_PIPE_BUSY {
// 		log.Err("StatsReader::createPipe : unable to create pipe [%v]", err)
// 		return err
// 	}

// 	log.Info("StatsReader::createPipe : Creating named pipe %s", pipe)

// 	// connect to the named pipe
// 	err = windows.ConnectNamedPipe(handle, nil)
// 	if err != nil {
// 		log.Err("StatsReader::createPipe : unable to connect to named pipe %s: [%v]", pipe, err)
// 		windows.CloseHandle(handle)
// 		return err
// 	}

// 	return nil
// }

func init() {
	hminternal.AddMonitor(hmcommon.BlobfuseStats, NewBlobfuseStatsMonitor)
}
