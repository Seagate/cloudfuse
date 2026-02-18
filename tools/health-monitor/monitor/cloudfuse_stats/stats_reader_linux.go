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

package cloudfuse_stats

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal/stats_manager"

	"golang.org/x/sys/unix"
)

func (cfs *CloudfuseStats) statsReader() error {
	err := createPipe(cfs.transferPipe)
	if err != nil {
		log.Err("StatsReader::statsReader : [%v]", err)
		return err
	}

	f, err := os.Open(cfs.transferPipe)
	if err != nil {
		log.Err("StatsReader::statsReader : unable to open pipe file [%v]", err)
		return err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	var e error = nil

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			log.Err("StatsReader::statsReader : [%v]", err)
			e = err
			break
		}

		// log.Debug("StatsReader::statsReader : Line: %v", string(line))

		st := stats_manager.PipeMsg{}
		err = json.Unmarshal(line, &st)
		if err != nil {
			log.Err("StatsReader::statsReader : Unable to unmarshal json [%v]", err)
			continue
		}
		cfs.ExportStats(st.Timestamp, st)
	}

	return e
}

func (cfs *CloudfuseStats) statsPoll() {
	err := createPipe(cfs.pollingPipe)
	if err != nil {
		log.Err("StatsReader::statsPoll : [%v]", err)
		return
	}

	pf, err := os.OpenFile(cfs.pollingPipe, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Err("StatsReader::statsPoll : unable to open pipe file [%v]", err)
		return
	}
	defer func() {
		if err := pf.Close(); err != nil {
			log.Err("StatsReader::statsPoll : Error when closing pipe file [%v]", err)
		}
	}()

	ticker := time.NewTicker(time.Duration(cfs.pollInterval) * time.Second)
	defer ticker.Stop()

	for t := range ticker.C {
		_, err = fmt.Fprintf(pf, "Poll at %v\n", t.Format(time.RFC3339))
		if err != nil {
			log.Err("StatsReader::statsPoll : [%v]", err)
			break
		}
	}
}

func createPipe(pipe string) error {
	_, err := os.Stat(pipe)
	if os.IsNotExist(err) {
		err = unix.Mkfifo(pipe, 0666)
		if err != nil {
			log.Err("StatsReader::createPipe : unable to create pipe [%v]", err)
			return err
		}
	} else if err != nil {
		log.Err("StatsReader::createPipe : [%v]", err)
		return err
	}
	return nil
}
