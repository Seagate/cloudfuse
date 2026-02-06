//go:build windows

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2026 Seagate Technology LLC and/or its Affiliates

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
	"context"
	"fmt"
	"net"
	"time"

	"github.com/Seagate/cloudfuse/cmd"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal/winservice"
	"golang.org/x/sys/windows/svc"
)

const (
	SvcName         = "CloudfuseServiceStartup"
	maxWaitDuration = 5 * time.Minute
	checkInterval   = 5 * time.Second
	dialTimeout     = 5 * time.Second
)

var networkTargets = []string{"seagate.com:80", "google.com:80", "8.8.8.8:53", "1.1.1.1:53"}

type Cloudfuse struct{}

func (m *Cloudfuse) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	// Notify the Service Control Manager that the service is starting
	changes <- svc.Status{State: svc.StartPending}
	log.Trace("Starting %s service", SvcName)

	log.Trace("Waiting for network")
	ctx, cancel := context.WithTimeout(context.Background(), maxWaitDuration)
	defer cancel()

	err := waitForNetwork(ctx, networkTargets, checkInterval, dialTimeout)
	if err != nil {
		log.Warn("Failed to access network. Attempting to start mounts")
	} else {
		log.Trace("Successfully connected to network.")
	}

	useSystem := true

	// Send request to WinFSP to start the process
	err = winservice.StartMounts(useSystem)
	// If unable to start, then stop the service
	if err != nil {
		changes <- svc.Status{State: svc.StopPending}
		log.Err("Stopping %s service due to error when starting: %v", SvcName, err.Error())
		return
	}

	// Notify the SCM that we are running and these are the commands we will respond to
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	log.Trace("Successfully started %s service", SvcName)

	for { //nolint
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
				time.Sleep(100 * time.Millisecond)
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				log.Trace("Stopping %s service", SvcName)
				changes <- svc.Status{State: svc.StopPending}

				// Tell WinFSP to stop the service
				err := winservice.StopMounts(useSystem)
				if err != nil {
					log.Err("Error stopping %s service: %v", SvcName, err.Error())
				}
				return
			}
		}
	}
}

func waitForNetwork(ctx context.Context, targets []string, interval time.Duration, timeout time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("Timed out waiting for network: %w", ctx.Err())
		case <-ticker.C:
			for _, target := range targets {
				conn, err := net.DialTimeout("tcp", target, timeout)
				if err == nil {
					conn.Close()
					return nil
				}
			}
		}
	}
}

func main() {
	isService, err := svc.IsWindowsService()
	if err != nil || !isService {
		log.Err("Unable to determine if running as Windows service or not running as Windows service: %v", err.Error())
		return
	}

	handler := &Cloudfuse{}
	run := svc.Run
	err = run(cmd.SvcName, handler)
	if err != nil {
		log.Err("Unable to start Windows service: %v", err.Error())
	}
}
