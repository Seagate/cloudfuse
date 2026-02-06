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

package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows"

	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/winservice"
)

// Create dummy function so that mount.go code can compile
// This function is used only on Linux, so it creates an empty context here
func createDaemon(pipeline *internal.Pipeline, ctx context.Context, pidFileName string, pidFilePerm os.FileMode, umask int, fname string) error {
	return nil
}

// Use WinFSP to mount and if successful, add instance to persistent mount list
func createMountInstance(enableRemountUser bool, enableRemountSystem bool) error {
	// Add the mount to the JSON file so it persists on restart.
	if enableRemountUser || enableRemountSystem {
		err := winservice.AddMountJSON(options.MountPath, options.ConfigFile, enableRemountSystem)
		if err != nil {
			return fmt.Errorf("failed to add entry to json file [%s]", err.Error())
		}
	}

	err := winservice.StartMount(options.MountPath, options.ConfigFile, encryptedPassphrase)
	if err != nil {
		return err
	}

	return nil
}

// stub
func installRemountService(string, string, string) (string, error) {
	return "", nil
}

// stub
func startService(string) error {
	return nil
}

// readPassphraseFromPipe connects to a pipe and reads the passphrase.
func readPassphraseFromPipe(pipeName string, timeout time.Duration) (string, error) {
	pipeNameUTF16, err := windows.UTF16PtrFromString(pipeName)
	if err != nil {
		return "", fmt.Errorf("invalid pipe name: %w", err)
	}

	deadline := time.Now().Add(timeout)
	var pipeHandle windows.Handle

	// Loop to connect to the pipe
	for {
		pipeHandle, err = windows.CreateFile(
			pipeNameUTF16,
			windows.GENERIC_READ,
			0,
			nil,
			windows.OPEN_EXISTING,
			windows.FILE_FLAG_OVERLAPPED,
			0,
		)

		if err == nil {
			// Success
			break
		}

		// If the pipe is busy, the named pipe is not available yet
		// We will retry until the timeout expires.
		if err == windows.ERROR_PIPE_BUSY {
			if time.Now().After(deadline) {
				return "", fmt.Errorf("timed out waiting for pipe '%s' to become available", pipeName)
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}

		// Any other error is fatal.
		return "", fmt.Errorf("could not connect to pipe '%s': %w", pipeName, err)
	}
	defer windows.CloseHandle(pipeHandle)

	// Read from the pipe with a timeout using overlapped (asynchronous) I/O.
	overlapped := &windows.Overlapped{}
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return "", fmt.Errorf("ReadFile CreateEvent failed: %w", err)
	}
	defer windows.CloseHandle(event)
	overlapped.HEvent = event

	buffer := make([]byte, 4096)
	var done uint32

	err = windows.ReadFile(pipeHandle, buffer, &done, overlapped)
	if err != nil && err != windows.ERROR_IO_PENDING {
		return "", fmt.Errorf("ReadFile failed: %w", err)
	}

	// Wait for the read operation to complete or timeout.
	readTimeout := time.Until(deadline)
	if readTimeout < 0 {
		readTimeout = 0
	}
	eventState, err := windows.WaitForSingleObject(event, uint32(readTimeout.Milliseconds()))
	if err != nil {
		return "", fmt.Errorf("ReadFile WaitForSingleObject failed: %w", err)
	}

	if eventState == uint32(windows.WAIT_TIMEOUT) {
		return "", fmt.Errorf("timed out waiting for data from pipe '%s'", pipeName)
	}

	// Get the result of the overlapped operation to know how many bytes were read.
	err = windows.GetOverlappedResult(pipeHandle, overlapped, &done, true)
	if err != nil {
		return "", fmt.Errorf("GetOverlappedResult failed: %w", err)
	}

	if done == 0 {
		return "", fmt.Errorf("received empty passphrase from pipe")
	}

	return string(buffer[:done]), nil
}
