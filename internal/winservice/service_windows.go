//go:build windows

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023 Seagate Technology LLC and/or its Affiliates

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

package winservice

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
)

const (
	SvcName    = "cloudfuse"
	winfspPipe = `\\.\pipe\WinFsp.{14E7137D-22B4-437A-B0C1-D21D1BDF3767}`
	startCmd   = 'S'
	stopCmd    = 'T'
	listCmd    = 'L'
	successCmd = '$'
	failCmd    = '!'
)

type Cloudfuse struct{}

func (m *Cloudfuse) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	// Notify the Service Control Manager that the service is starting
	changes <- svc.Status{State: svc.StartPending}
	log.Trace("Starting %s service", SvcName)

	// Send request to WinFSP to start the process
	err := startServices()
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
				err := stopServices()
				if err != nil {
					log.Err("Error stopping %s service: %v", SvcName, err.Error())
				}
				return
			}
		}
	}
}

// StartMount starts the mount if the name exists in our Windows registry.
func StartMount(mountPath string, configFile string) error {
	// get the current user uid and gid to set file permissions
	userId, groupId, err := common.GetCurrentUser()
	if err != nil {
		log.Err("StartMount : GetCurrentUser() failed with error: %v", err)
		return err
	}

	instanceName := mountPath

	buf := writeCommandToUtf16(startCmd, SvcName, instanceName, mountPath, configFile, userId, groupId)
	_, err = winFspCommand(buf)
	if err != nil {
		return err
	}
	return nil
}

// StopMount stops the mount if the name exists in our Windows registry.
func StopMount(mountPath string) error {
	instanceName := mountPath

	buf := writeCommandToUtf16(stopCmd, SvcName, instanceName, mountPath)
	_, err := winFspCommand(buf)
	if err != nil {
		return err
	}
	return nil
}

// IsMounted determines if the given path is mounted.
func IsMounted(mountPath string) (bool, error) {
	buf := writeCommandToUtf16(listCmd)
	list, err := winFspCommand(buf)
	if err != nil {
		return false, err
	}

	// Everything in the list is a name of a service using WinFsp, like cloudfuse and then
	// the name of the mount which is the mount path
	if len(list)%2 != 0 {
		return false, errors.New("unable to get list from Winfsp because received odd number of elements")
	}

	for i := 0; i < len(list); i += 2 {
		// Check if the mountpath is associated with our service
		if list[i] == SvcName && list[i+1] == mountPath {
			return true, nil
		}
	}
	return false, nil
}

// startService starts cloudfuse by instructing WinFsp to launch it.
func startServices() error {
	// Read registry to get names of the instances we need to start
	instances, err := readRegistryEntry()
	// If there is nothing in our registry to mount then continue
	if err == registry.ErrNotExist {
		return nil
	} else if err != nil {
		return err
	}

	for _, inst := range instances {
		err := StartMount(inst.MountPath, inst.ConfigFile)
		if err != nil {
			log.Err("Unable to start mount with mountpath: ", inst.MountPath)
		}
	}

	return nil
}

// stopServicess stops cloudfuse by instructing WinFsp to stop it.
func stopServices() error {
	// Read registry to get names of the instances we need to stop
	instances, err := readRegistryEntry()
	// If there is nothing in our registry to mount then continue
	if err == registry.ErrNotExist {
		return nil
	} else if err != nil {
		return err
	}

	for _, inst := range instances {
		err := StopMount(inst.MountPath)
		if err != nil {
			log.Err("Unable to stop mount with mountpath: ", inst.MountPath)
		}
	}

	return nil
}

// writeCommandToUtf16 writes a given cmd and arguments as a byte array in UTF16.
func writeCommandToUtf16(cmd uint16, args ...string) []byte {
	var buf bytes.Buffer

	// Write the command we are sending to WinFsp
	_ = binary.Write(&buf, binary.LittleEndian, cmd)

	// Write the arguments
	for _, arg := range args {
		uStr, err := windows.UTF16FromString(arg)
		if err != nil {
			return nil
		}
		for _, w := range uStr {
			_ = binary.Write(&buf, binary.LittleEndian, w)
		}
	}

	return buf.Bytes()
}

// winFspCommand sends an instruction to WinFsp.
func winFspCommand(command []byte) ([]string, error) {
	var retStrings []string
	winPipe, err := windows.UTF16PtrFromString(winfspPipe)
	if err != nil {
		return retStrings, err
	}

	// Open the named pipe for WinFSP
	handle, err := windows.CreateFile(
		winPipe,
		windows.GENERIC_WRITE|windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OVERLAPPED,
		windows.InvalidHandle,
	)
	if err != nil {
		return retStrings, err
	}
	defer windows.CloseHandle(handle) //nolint

	// Send the command to WinFSP
	var overlapped windows.Overlapped
	err = windows.WriteFile(handle, command, nil, &overlapped)
	if err == windows.ERROR_IO_PENDING {
		err = windows.GetOverlappedResult(handle, &overlapped, nil, true)
		if err != nil {
			return retStrings, err
		}
	} else if err != nil {
		return retStrings, err
	}

	// Get the response from WinFSP
	overlapped = windows.Overlapped{}
	buf := make([]byte, 4096)
	var bytesRead uint32
	err = windows.ReadFile(handle, buf, &bytesRead, &overlapped)
	if err == windows.ERROR_IO_PENDING {
		err = windows.GetOverlappedResult(handle, &overlapped, &bytesRead, true)
		if err != nil {
			return retStrings, err
		}
	} else if err != nil {
		return retStrings, err
	}

	// If there are not enough bytes for the return character, then it failed
	if bytesRead < 2 {
		return retStrings, errors.New("winfsp launchctl tool failed with non standard return")
	}

	ubuf := bytesToUint16(buf)
	if ubuf[0] == failCmd {
		return retStrings, errors.New("winfsp launchctl tool was not successful")
	} else if ubuf[0] != successCmd {
		return retStrings, errors.New("winfsp launchctl tool failed with non standard return")
	}

	// If there is more to read then we are using a WinFSP command that returns more data such as
	// list, so let's try to read it.
	if bytesRead > 2 {
		buffer := ubuf[1 : bytesRead/2]
		retStrings = winfspBytesToString(buffer)
	}

	return retStrings, nil
}

// winfspBytesToString takes in a utf16 formatted slices from WinFSP and returns a slice of strings.
func winfspBytesToString(buf []uint16) []string {
	var start int
	var retStrings []string
	for i, v := range buf {
		// 0 indicates the end of a null-terminated string
		if v == 0 {
			if start != i {
				retStrings = append(retStrings, windows.UTF16ToString(buf[start:i]))
			}
			start = i + 1
		}
	}
	return retStrings
}

// bytesToUint16 converts the byte slice to a uint16 slice.
func bytesToUint16(buf []byte) []uint16 {
	var ubuf []uint16
	for i := 0; i < len(buf); i += 2 {
		ubuf = append(ubuf, binary.LittleEndian.Uint16(buf[i:i+2]))
	}
	return ubuf
}
