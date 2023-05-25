//go:build linux

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

package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"lyvecloudfuse/common/config"
	"lyvecloudfuse/common/log"
	"lyvecloudfuse/internal"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sevlyar/go-daemon"
)

func createDaemon(pipeline *internal.Pipeline, ctx context.Context, pidFileName string, pidFilePerm os.FileMode, umask int, fname string) error {
	dmnCtx := &daemon.Context{
		PidFileName: pidFileName,
		PidFilePerm: 0644,
		Umask:       022,
		LogFileName: fname, // this will redirect stderr of child to given file
	}

	// Signal handlers for parent and child to communicate success or failures in mount
	var sigusr2, sigchild chan os.Signal
	if !daemon.WasReborn() { // execute in parent only
		sigusr2 = make(chan os.Signal, 1)
		signal.Notify(sigusr2, syscall.SIGUSR2)

		sigchild = make(chan os.Signal, 1)
		signal.Notify(sigchild, syscall.SIGCHLD)
	} else { // execute in child only
		daemon.SetSigHandler(sigusrHandler(pipeline, ctx), syscall.SIGUSR1, syscall.SIGUSR2)
		go func() {
			_ = daemon.ServeSignals()
		}()
	}

	child, err := dmnCtx.Reborn()
	if err != nil {
		log.Err("mount : failed to daemonize application [%v]", err)
		return Destroy(fmt.Sprintf("failed to daemonize application [%s]", err.Error()))
	}

	log.Debug("mount: foreground disabled, child = %v", daemon.WasReborn())
	if child == nil { // execute in child only
		defer dmnCtx.Release() // nolint
		setGOConfig()
		go startDynamicProfiler()

		// In case of failure stderr will have the error emitted by child and parent will read
		// those logs from the file set in daemon context
		return runPipeline(pipeline, ctx)
	} else { // execute in parent only
		defer os.Remove(fname)

		select {
		case <-sigusr2:
			log.Info("mount: Child [%v] mounted successfully at %s", child.Pid, options.MountPath)

		case <-sigchild:
			// Get error string from the child, stderr or child was redirected to a file
			log.Info("mount: Child [%v] terminated from %s", child.Pid, options.MountPath)

			buff, err := ioutil.ReadFile(dmnCtx.LogFileName)
			if err != nil {
				log.Err("mount: failed to read child [%v] failure logs [%s]", child.Pid, err.Error())
				return Destroy(fmt.Sprintf("failed to mount, please check logs [%s]", err.Error()))
			} else {
				return Destroy(string(buff))
			}

		case <-time.After(options.WaitForMount):
			log.Info("mount: Child [%v : %s] status check timeout", child.Pid, options.MountPath)
		}

		_ = log.Destroy()
	}
	return nil
}

func sigusrHandler(pipeline *internal.Pipeline, ctx context.Context) daemon.SignalHandlerFunc {
	return func(sig os.Signal) error {
		log.Crit("Mount::sigusrHandler : Signal %d received", sig)

		var err error
		if sig == syscall.SIGUSR1 {
			log.Crit("Mount::sigusrHandler : SIGUSR1 received")
			config.OnConfigChange()
		}

		return err
	}
}
