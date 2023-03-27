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

package stats_manager

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"lyvecloudfuse/common"
	"lyvecloudfuse/common/log"

	"golang.org/x/sys/windows"
)

type StatsCollector struct {
	channel    chan ChannelMsg
	workerDone sync.WaitGroup
	compIdx    int
}

type PipeMsg struct {
	Timestamp     string                 `json:"timestamp"`
	ComponentName string                 `json:"componentName,omitempty"`
	Operation     string                 `json:"operation,omitempty"`
	Path          string                 `json:"path,omitempty"`
	Value         map[string]interface{} `json:"value,omitempty"`
}

type Events struct {
	Timestamp string
	Operation string
	Path      string
	Value     map[string]interface{}
}

type Stats struct {
	Timestamp string
	Operation string
	Key       string
	Value     interface{}
}

type ChannelMsg struct {
	IsEvent bool
	CompMsg interface{}
}

type statsManagerOpt struct {
	statsList []*PipeMsg
	// map to store the last updated timestamp of component's stats
	// This way a component's stat which was not updated is not pushed to the transfer pipe
	cmpTimeMap  map[string]string
	pollStarted bool
	transferMtx sync.Mutex
	pollMtx     sync.Mutex
	statsMtx    sync.Mutex
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
// 		log.Err("stats_manager::createPipe : unable to create pipe [%v]", err)
// 		return err
// 	}

// 	log.Info("stats_manager::createPipe : Creating named pipe %s", pipe)

// 	// connect to the named pipe
// 	err = windows.ConnectNamedPipe(handle, nil)
// 	if err != nil {
// 		log.Err("stats_manager::createPipe : unable to connect to named pipe %s: [%v]", pipe, err)
// 		windows.CloseHandle(handle)
// 		return err
// 	}

// 	return nil
// }

var stMgrOpt statsManagerOpt

func NewStatsCollector(componentName string) *StatsCollector {
	sc := &StatsCollector{}

	if common.MonitorBfs() {
		sc.channel = make(chan ChannelMsg, 10000)

		stMgrOpt.statsMtx.Lock()

		sc.compIdx = len(stMgrOpt.statsList)
		cmpSt := PipeMsg{
			Timestamp:     time.Now().Format(time.RFC3339),
			ComponentName: componentName,
			Operation:     "",
			Value:         make(map[string]interface{}),
		}
		stMgrOpt.statsList = append(stMgrOpt.statsList, &cmpSt)

		stMgrOpt.cmpTimeMap[componentName] = cmpSt.Timestamp

		stMgrOpt.statsMtx.Unlock()

		sc.Init()
		log.Debug("stats_manager::NewStatsCollector : %v", componentName)
	}

	return sc
}

func (sc *StatsCollector) Init() {
	sc.workerDone.Add(1)
	go sc.statsDumper()

	stMgrOpt.pollMtx.Lock()
	defer stMgrOpt.pollMtx.Unlock()
	if !stMgrOpt.pollStarted {
		stMgrOpt.pollStarted = true
		go statsPolling()
	}
}

func (sc *StatsCollector) Destroy() {
	if common.MonitorBfs() {
		close(sc.channel)
		sc.workerDone.Wait()
	}
}

func (sc *StatsCollector) PushEvents(op string, path string, mp map[string]interface{}) {
	if common.MonitorBfs() {
		event := Events{
			Timestamp: time.Now().Format(time.RFC3339),
			Operation: op,
			Path:      path,
		}

		if mp != nil {
			event.Value = make(map[string]interface{})
			for k, v := range mp {
				event.Value[k] = v
			}
		}

		// check if the channel is full
		if len(sc.channel) == cap(sc.channel) {
			// remove the first element from the channel
			<-sc.channel
		}

		sc.channel <- ChannelMsg{
			IsEvent: true,
			CompMsg: event,
		}
	}
}

func (sc *StatsCollector) UpdateStats(op string, key string, val interface{}) {
	if common.MonitorBfs() {
		st := Stats{
			Timestamp: time.Now().Format(time.RFC3339),
			Operation: op,
			Key:       key,
			Value:     val,
		}

		// check if the channel is full
		if len(sc.channel) == cap(sc.channel) {
			// remove the first element from the channel
			<-sc.channel
		}

		sc.channel <- ChannelMsg{
			IsEvent: false,
			CompMsg: st,
		}
	}
}

func (sc *StatsCollector) statsDumper() {
	defer sc.workerDone.Done()

	var hPipe windows.Handle
	var err error
	for {
		hPipe, err = windows.CreateFile(
			windows.StringToUTF16Ptr(common.WindowsTransferPipe),
			windows.GENERIC_WRITE,
			0,
			nil,
			windows.OPEN_EXISTING,
			windows.FILE_FLAG_OVERLAPPED,
			0,
		)

		if err == nil {
			break
		}

		if err == windows.ERROR_FILE_NOT_FOUND {
			log.Info("stats_manager::statsDumper : Named pipe %s not found, retrying...", common.WindowsTransferPipe)
			time.Sleep(1 * time.Second)
		} else {
			log.Err("stats_manager::statsDumper : unable to open pipe file [%v]", err)
			return
		}
	}

	log.Info("stats_manager::statsDumper : opened transfer pipe file")

	for st := range sc.channel {
		// log.Debug("stats_manager::statsDumper : stats: %v", st)

		idx := sc.compIdx
		if st.IsEvent {
			event := st.CompMsg.(Events)
			pipeMsg := PipeMsg{
				Timestamp:     event.Timestamp,
				ComponentName: stMgrOpt.statsList[idx].ComponentName,
				Operation:     event.Operation,
				Path:          event.Path,
				Value:         event.Value,
			}

			msg, err := json.Marshal(pipeMsg)
			if err != nil {
				log.Err("stats_manager::statsDumper : Unable to marshal [%v]", err)
				continue
			}

			// log.Debug("stats_manager::statsDumper : stats: %v", string(msg))

			stMgrOpt.transferMtx.Lock()
			log.Info("stats_manager::createPipe : Writing message to pipe %s", msg)
			err = windows.WriteFile(hPipe, msg, nil, nil)
			stMgrOpt.transferMtx.Unlock()
			if err != nil {
				log.Err("stats_manager::statsDumper : Unable to write to pipe [%v]", err)
				disableMonitoring()
				break
			}

		} else {
			// accumulate component level stats
			stat := st.CompMsg.(Stats)

			// TODO: check if this lock can be removed
			stMgrOpt.statsMtx.Lock()

			_, isPresent := stMgrOpt.statsList[idx].Value[stat.Key]
			if !isPresent {
				stMgrOpt.statsList[idx].Value[stat.Key] = (int64)(0)
			}

			switch stat.Operation {
			case Increment:
				stMgrOpt.statsList[idx].Value[stat.Key] = stMgrOpt.statsList[idx].Value[stat.Key].(int64) + stat.Value.(int64)

			case Decrement:
				stMgrOpt.statsList[idx].Value[stat.Key] = stMgrOpt.statsList[idx].Value[stat.Key].(int64) - stat.Value.(int64)
				if stMgrOpt.statsList[idx].Value[stat.Key].(int64) < 0 {
					log.Err("stats_manager::statsDumper : Negative value %v after decrement of %v for component %v",
						stMgrOpt.statsList[idx].Value[stat.Key], stat.Key, stMgrOpt.statsList[idx].ComponentName)
				}

			case Replace:
				stMgrOpt.statsList[idx].Value[stat.Key] = stat.Value

			default:
				log.Debug("stats_manager::statsDumper : Incorrect operation for stats collection")
				stMgrOpt.statsMtx.Unlock()
				continue
			}
			stMgrOpt.statsList[idx].Timestamp = stat.Timestamp

			stMgrOpt.statsMtx.Unlock()
		}
	}
}

func statsPolling() {
	// create polling pipe
	handle, err := windows.CreateNamedPipe(
		windows.StringToUTF16Ptr(common.WindowsPollingPipe),
		windows.PIPE_ACCESS_DUPLEX,
		windows.PIPE_TYPE_MESSAGE|windows.PIPE_READMODE_MESSAGE|windows.PIPE_WAIT,
		windows.PIPE_UNLIMITED_INSTANCES,
		4096,
		4096,
		0,
		nil,
	)
	if err != nil && err != windows.ERROR_PIPE_BUSY {
		log.Err("stats_manager::statsPolling : unable to create pipe [%v]", err)
		return
	}

	log.Info("stats_manager::statsPolling : Creating named pipe %s", common.WindowsPollingPipe)

	// connect to the named pipe
	err = windows.ConnectNamedPipe(handle, nil)
	if err != nil {
		log.Err("stats_manager::statsPolling : unable to connect to named pipe %s: [%v]", common.WindowsPollingPipe, err)
		windows.CloseHandle(handle)
		return
	}

	reader := bufio.NewReader(os.NewFile(uintptr(handle), common.WindowsPollingPipe))

	for {
		// read the polling message sent by stats monitor
		line, err := reader.ReadBytes('\n')
		if err != nil {
			log.Err("stats_manager::statsPolling : Unable to read from pipe [%v]", err)
			disableMonitoring()
			break
		}

		// validating poll message
		if !strings.Contains(string(line), "Poll at") {
			continue
		}

		// TODO: check if this lock can be removed
		stMgrOpt.statsMtx.Lock()
		for _, cmpSt := range stMgrOpt.statsList {
			if len(cmpSt.Value) == 0 {
				continue
			}

			if cmpSt.Timestamp == stMgrOpt.cmpTimeMap[cmpSt.ComponentName] {
				log.Debug("stats_manager::statsPolling : Skipping as there is no change in stats collected for %v", cmpSt.ComponentName)
				continue
			}

			msg, err := json.Marshal(cmpSt)
			if err != nil {
				log.Err("stats_manager::statsPolling : Unable to marshal [%v]", err)
				continue
			}

			// log.Debug("stats_manager::statsPolling : stats: %v", string(msg))

			// send the stats collected so far to transfer pipe
			stMgrOpt.transferMtx.Lock()
			var tPipe windows.Handle
			for {
				tPipe, err = windows.CreateFile(
					windows.StringToUTF16Ptr(common.WindowsTransferPipe),
					windows.GENERIC_WRITE,
					0,
					nil,
					windows.OPEN_EXISTING,
					windows.FILE_FLAG_OVERLAPPED,
					0,
				)

				if err == nil {
					break
				}

				if err == windows.ERROR_FILE_NOT_FOUND {
					log.Info("stats_manager::statsDumper : Named pipe %s not found, retrying...", common.WindowsTransferPipe)
					time.Sleep(1 * time.Second)
				} else {
					log.Err("stats_manager::statsDumper: unable to open pipe file [%v]", err)
					return
				}
			}

			file := os.NewFile(uintptr(tPipe), common.WindowsPollingPipe)
			_, err = file.WriteString(fmt.Sprintf("%v\n", string(msg)))
			stMgrOpt.transferMtx.Unlock()
			if err != nil {
				log.Err("stats_manager::statsDumper : Unable to write to pipe [%v]", err)
				disableMonitoring()
				break
			}

			stMgrOpt.cmpTimeMap[cmpSt.ComponentName] = cmpSt.Timestamp
		}
		stMgrOpt.statsMtx.Unlock()
	}
}

func disableMonitoring() {
	common.EnableMonitoring = false
	log.Debug("stats_manager::disableMonitoring : disabling monitoring flag")
}

func init() {
	stMgrOpt = statsManagerOpt{}
	stMgrOpt.pollStarted = false
	stMgrOpt.cmpTimeMap = make(map[string]string)
}
