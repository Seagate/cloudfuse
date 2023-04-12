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

	"lyvecloudfuse/common/log"
	"lyvecloudfuse/internal/stats_manager"

	"golang.org/x/sys/windows"
)

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

		// This is a blocking call that waits for a client instance to call the CreateFile function and once that
		// happens then we can safely start writing to the named pipe.
		// See https://learn.microsoft.com/en-us/windows/win32/api/namedpipeapi/nf-namedpipeapi-connectnamedpipe
		err = windows.ConnectNamedPipe(handle, nil)
		if err != nil {
			log.Err("StatsReader::statsReader : unable to connect to named pipe %s: [%v]", bfs.transferPipe, err)
			windows.CloseHandle(handle)
			time.Sleep(1 * time.Second)
		}
		log.Info("StatsReader::statsReader : Connected transfer pipe %s", bfs.transferPipe)

		go bfs.handleStatsReader(handle)
	}
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
