//go:build windows

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.

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
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal/stats_manager"

	"golang.org/x/sys/windows"
)

func (cfs *CloudfuseStats) statsReader() error {
	// Accept incoming connections
	for {
		log.Info("StatsReader::statsReader : In stats reader")
		handle, err := windows.CreateNamedPipe(
			windows.StringToUTF16Ptr(cfs.transferPipe),
			windows.PIPE_ACCESS_DUPLEX,
			windows.PIPE_TYPE_MESSAGE|windows.PIPE_READMODE_MESSAGE|windows.PIPE_WAIT,
			windows.PIPE_UNLIMITED_INSTANCES,
			4096,
			4096,
			0,
			nil,
		)
		if err != nil {
			log.Err("StatsReader::statsReader : unable to create pipe [%v]", err)
			return err
		}

		// This is a blocking call that waits for a client instance to call the CreateFile function and once that
		// happens then we can safely start writing to the named pipe.
		// See https://learn.microsoft.com/en-us/windows/win32/api/namedpipeapi/nf-namedpipeapi-connectnamedpipe
		err = windows.ConnectNamedPipe(handle, nil)
		if err == windows.ERROR_PIPE_CONNECTED {
			log.Err(
				"StatsReader::statsReader : There is a process at other end of pipe %s: retrying... [%v]",
				cfs.transferPipe,
				err,
			)
			windows.Close(handle)
			time.Sleep(1 * time.Second)
		} else if err != nil {
			log.Err(
				"StatsReader::statsReader : unable to connect to named pipe %s: [%v]",
				cfs.transferPipe,
				err,
			)
			windows.Close(handle)
			return err
		}
		log.Info("StatsReader::statsReader : Connected transfer pipe %s", cfs.transferPipe)

		go cfs.handleStatsReader(handle) //nolint
	}
}

func (cfs *CloudfuseStats) handleStatsReader(handle windows.Handle) error {
	defer windows.Close(handle)

	var buf [4096]byte
	var bytesRead uint32
	var messageBuf bytes.Buffer
	var e error

	for {
		// Empty the buffer before reading
		messageBuf.Reset()
		// read the polling message sent by stats monitor
		// we iterate until we have read the entire message
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
			e = err
			break
		}
		cfs.ExportStats(st.Timestamp, st)
	}

	return e
}

func (cfs *CloudfuseStats) statsPoll() {
	var hPipe windows.Handle
	var err error

	// Setup polling pipe by looping to try to create a file (open the pipe).
	// If the server has not been setup yet, then this will fail so we wait
	// and then try again.
	for {
		hPipe, err = windows.CreateFile(
			windows.StringToUTF16Ptr(cfs.pollingPipe),
			windows.GENERIC_WRITE,
			windows.FILE_SHARE_WRITE,
			nil,
			windows.OPEN_EXISTING,
			windows.FILE_ATTRIBUTE_NORMAL,
			windows.InvalidHandle,
		)

		// The pipe was created
		if err == nil {
			break
		}
		windows.Close(hPipe)

		if err == windows.ERROR_FILE_NOT_FOUND {
			log.Err(
				"StatsReader::statsReader : Named pipe %s not found, retrying...",
				cfs.pollingPipe,
			)
			time.Sleep(1 * time.Second)
		} else if err == windows.ERROR_PIPE_BUSY {
			log.Err("StatsReader::statsReader : Pipe instances are busy, retrying...")
			time.Sleep(1 * time.Second)
		} else {
			log.Err(
				"StatsReader::statsReader : Unable to open pipe %s with error [%v]",
				cfs.pollingPipe,
				err,
			)
			return
		}
	}
	defer windows.Close(hPipe)

	log.Info("stats_manager::statsDumper : opened polling pipe file")

	writer := os.NewFile(uintptr(hPipe), cfs.pollingPipe)

	ticker := time.NewTicker(time.Duration(cfs.pollInterval) * time.Second)
	defer ticker.Stop()

	for t := range ticker.C {
		_, err = writer.WriteString(fmt.Sprintf("Poll at %v", t.Format(time.RFC3339)))
		log.Debug(
			"stats_manager::statsDumper : writing to polling pipe file: %s",
			fmt.Sprintf("Poll at %v", t.Format(time.RFC3339)),
		)
		if err != nil {
			log.Err("StatsReader::statsPoll : [%v]", err)
			break
		}
	}
}
