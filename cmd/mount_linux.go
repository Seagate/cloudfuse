//go:build linux

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

package cmd

import (
	"context"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"

	"github.com/sevlyar/go-daemon"
	"golang.org/x/sys/unix"
)

type serviceOptions struct {
	ConfigFile  string
	MountPath   string
	ServiceUser string
}

func createDaemon(
	pipeline *internal.Pipeline,
	ctx context.Context,
	pidFileName string,
	pidFilePerm os.FileMode,
	umask int,
	fname string,
) error {
	dmnCtx := &daemon.Context{
		PidFileName: pidFileName,
		PidFilePerm: pidFilePerm,
		Umask:       umask,
		LogFileName: fname, // this will redirect stderr of child to given file
	}

	// Signal handlers for parent and child to communicate success or failures in mount
	var sigusr2, sigchild chan os.Signal
	if !daemon.WasReborn() { // execute in parent only
		sigusr2 = make(chan os.Signal, 1)
		signal.Notify(sigusr2, unix.SIGUSR2)

		sigchild = make(chan os.Signal, 1)
		signal.Notify(sigchild, unix.SIGCHLD)
	} else { // execute in child only
		daemon.SetSigHandler(sigusrHandler(pipeline, ctx), unix.SIGUSR1, unix.SIGUSR2)
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
		defer func() {
			if err := dmnCtx.Release(); err != nil {
				log.Err("Unable to release pid-file: %s", err.Error())
			}
		}()

		if options.CPUProfile != "" {
			os.Remove(options.CPUProfile)
			f, err := os.Create(options.CPUProfile)
			if err != nil {
				fmt.Printf("Error opening file for cpuprofile [%s]", err.Error())
			}
			defer f.Close()
			if err := pprof.StartCPUProfile(f); err != nil {
				fmt.Printf("Failed to start cpuprofile [%s]", err.Error())
			}
			defer pprof.StopCPUProfile()
		}

		if options.MemProfile != "" {
			os.Remove(options.MemProfile)
			f, err := os.Create(options.MemProfile)
			if err != nil {
				fmt.Printf("Error opening file for memprofile [%s]", err.Error())
			}
			defer f.Close()
			runtime.GC()
			if err = pprof.WriteHeapProfile(f); err != nil {
				fmt.Printf("Error memory profiling [%s]", err.Error())
			}
		}

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

			buff, err := os.ReadFile(dmnCtx.LogFileName)
			if err != nil {
				log.Err("mount: failed to read child [%v] failure logs [%s]", child.Pid, err.Error())
				return Destroy(fmt.Sprintf("failed to mount, please check logs [%s]", err.Error()))
			} else if len(buff) > 0 {
				return Destroy(string(buff))
			} else {
				// Nothing was logged, so mount succeeded
				return nil
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
		if sig == unix.SIGUSR1 {
			log.Crit("Mount::sigusrHandler : SIGUSR1 received")
			config.OnConfigChange()
		}

		return err
	}
}

// stub for compilation
func createMountInstance(bool, bool) error {
	return nil
}

func newService(mountPath string, configPath string, serviceUser string) (string, error) {
	serviceTemplate := `
[Unit]
Description=Cloudfuse is an open source project developed to provide a virtual filesystem backed by S3 or Azure storage.
After=network-online.target
Requires=network-online.target

[Service]
# User service will run as.
User={{.ServiceUser}}

# Under the hood
Type=forking
ExecStart=/usr/bin/cloudfuse mount {{.MountPath}} --config-file={{.ConfigFile}} -o allow_other
ExecStop=/usr/bin/fusermount -u {{.MountPath}} -z

[Install]
WantedBy=multi-user.target
`
	config := serviceOptions{
		ConfigFile:  configPath,
		MountPath:   mountPath,
		ServiceUser: serviceUser,
	}

	tmpl, err := template.New("service").Parse(serviceTemplate)
	if err != nil {
		return "", fmt.Errorf("could not create a new service file: [%s]", err.Error())
	}
	serviceName, serviceFilePath := getService(mountPath)
	err = os.Remove(serviceFilePath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to replace the service file [%s]", err.Error())
	}

	var newFile *os.File
	newFile, err = os.Create(serviceFilePath)
	if err != nil {
		return "", fmt.Errorf("could not create new service file: [%s]", err.Error())
	}

	err = tmpl.Execute(newFile, config)
	if err != nil {
		return "", fmt.Errorf("could not create new service file: [%s]", err.Error())
	}
	return serviceName, nil
}

func setUser(serviceUser string, mountPath string, configPath string) error {
	_, err := user.Lookup(serviceUser)
	if err != nil {
		if strings.Contains(err.Error(), "unknown user") {
			// create the user
			userAddCmd := exec.Command("useradd", "-m", serviceUser)
			err = userAddCmd.Run()
			if err != nil {
				return fmt.Errorf("failed to create user [%s]", err.Error())
			}
			fmt.Println("user " + serviceUser + " has been created")
		}
	}
	// advise on required permissions
	fmt.Println(
		"ensure the user, " + serviceUser + ", has the following access: \n" + mountPath + ": read, write, and execute \n" + configPath + ": read",
	)
	return nil
}

func getService(mountPath string) (string, string) {
	serviceName := strings.ReplaceAll(mountPath, "/", "-")
	serviceFile := "cloudfuse" + serviceName + ".service"
	serviceFilePath := "/etc/systemd/system/" + serviceFile
	return serviceName, serviceFilePath
}

func installRemountService(
	serviceUser string,
	mountPath string,
	configPath string,
) (string, error) {
	//create the new user and set permissions
	mountPath, err := filepath.Abs(mountPath)
	if err != nil {
		return "", fmt.Errorf("installService: Failed to get absolute mount path")
	}

	configPath, err = filepath.Abs(configPath)
	if err != nil {
		return "", fmt.Errorf("installService: Failed to get absolute mount path")
	}

	err = setUser(serviceUser, mountPath, configPath)
	if err != nil {
		fmt.Println("could not set up service user ", err)
		return "", err
	}

	serviceName, err := newService(mountPath, configPath, serviceUser)
	if err != nil {
		return "", fmt.Errorf("unable to create service file: [%s]", err.Error())
	}
	// run systemctl daemon-reload
	systemctlDaemonReloadCmd := exec.Command("systemctl", "daemon-reload")
	err = systemctlDaemonReloadCmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to run 'systemctl daemon-reload' command [%s]", err.Error())
	}
	// Enable the service to start at system boot
	systemctlEnableCmd := exec.Command("systemctl", "enable", serviceName)
	err = systemctlEnableCmd.Run()
	if err != nil {
		return "", fmt.Errorf(
			"failed to run 'systemctl daemon-reload' command due to following [%s]",
			err.Error(),
		)
	}
	return serviceName, nil
}

func startService(serviceName string) error {
	systemctlEnableCmd := exec.Command("systemctl", "start", serviceName)
	err := systemctlEnableCmd.Run()
	if err != nil {
		return fmt.Errorf(
			"failed to run 'systemctl daemon-reload' command due to following [%s]",
			err.Error(),
		)
	}
	return nil
}

// stub for compilation
func readPassphraseFromPipe(pipeName string, timeout time.Duration) (string, error) {
	return "", nil
}
