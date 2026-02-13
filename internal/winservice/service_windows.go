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

package winservice

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/awnumar/memguard"

	"golang.org/x/sys/windows"
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

// StartMount starts the mount if the name exists in the WinFsp Windows registry.
func StartMount(mountPath string, configFile string, passphrase *memguard.Enclave) error {
	instanceName := strings.ToLower(mountPath)

	// get the current user uid and gid to set file permissions
	userId, groupId, err := common.GetCurrentUser()
	if err != nil {
		log.Err("startMountHelper : GetCurrentUser() failed with error: %v", err)
		return err
	}

	if passphrase != nil {
		buff, err := passphrase.Open()
		if err != nil || buff == nil {
			return errors.New("unable to decrypt passphrase key")
		}

		_, err = winFspCommand(
			writeCommandToUtf16(
				startCmd,
				SvcName,
				instanceName,
				mountPath,
				configFile,
				fmt.Sprint(userId),
				fmt.Sprint(groupId),
				buff.String(),
			),
		)
		defer buff.Destroy()
	} else {
		_, err = winFspCommand(
			writeCommandToUtf16(
				startCmd,
				SvcName,
				instanceName,
				mountPath,
				configFile,
				fmt.Sprint(userId),
				fmt.Sprint(groupId),
				"",
			),
		)
	}
	return err
}

// StopMount stops the mount if the name exists in the WinFsp Windows registry.
func StopMount(mountPath string) error {
	instanceName := strings.ToLower(mountPath)

	buf := writeCommandToUtf16(stopCmd, SvcName, instanceName, mountPath)
	_, err := winFspCommand(buf)
	if err != nil {
		return err
	}
	return nil
}

// IsMounted determines if the given path is mounted.
func IsMounted(mountPath string) (bool, error) {
	instanceName := strings.ToLower(mountPath)
	list, err := getMountList()
	if err != nil {
		return false, err
	}

	for i := 0; i < len(list); i += 2 {
		// Check if the mountpath is associated with our service
		if list[i] == SvcName && list[i+1] == instanceName {
			return true, nil
		}
	}
	return false, nil
}

// startService starts cloudfuse by instructing WinFsp to launch it.
func StartMounts(useSystem bool) error {
	// Read mount file to get names of the mounts we need to start from system
	mounts, err := readMounts(useSystem)
	// If there is nothing in our file to mount then continue
	if err != nil {
		return err
	}

	for _, inst := range mounts.Mounts {
		err := StartMount(inst.MountPath, inst.ConfigFile, nil)
		if err != nil {
			log.Err("Unable to start mount with mountpath: %s", inst.MountPath)
		}
	}

	return nil
}

// StopMounts stops all mounts the mount if the name exists in our Windows registry.
func StopMounts(useSystem bool) error {
	// Read mount file to get names of the mounts we need to start from system
	mounts, err := readMounts(useSystem)
	// If there is nothing in our file to mount then continue
	if err != nil {
		return err
	}

	for _, inst := range mounts.Mounts {
		err := StopMount(inst.MountPath)
		if err != nil {
			log.Err("Unable to start mount with mountpath: %s", inst.MountPath)
		}
	}

	return nil
}

func getMountList() ([]string, error) {
	var emptyList []string
	buf := writeCommandToUtf16(listCmd)
	list, err := winFspCommand(buf)
	if err != nil {
		return emptyList, err
	}

	// Everything in the list is a name of a service using WinFsp, like cloudfuse and then
	// the name of the mount which is the mount path
	if len(list)%2 != 0 {
		return emptyList, errors.New(
			"unable to get list from Winfsp because received odd number of elements",
		)
	}

	return list, nil
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

	_ = binary.Write(&buf, binary.LittleEndian, uint16(0))

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
		windows.GENERIC_READ|windows.FILE_WRITE_DATA|windows.FILE_WRITE_ATTRIBUTES,
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
	overlapped := windows.Overlapped{}
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
