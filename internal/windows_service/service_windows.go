//go:build windows

package windows_service

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

const (
	SvcName    = "lyvecloudfuse"
	winfspPipe = `\\.\pipe\WinFsp.{14E7137D-22B4-437A-B0C1-D21D1BDF3767}`
)

type LyveCloudFuse struct{}

func (m *LyveCloudFuse) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	// Notify the Service Control Manager that the service is starting
	changes <- svc.Status{State: svc.StartPending}

	// Send request to WinFSP to start the process
	err := startService()
	// If unable to start, then stop the service
	if err != nil {
		changes <- svc.Status{State: svc.StopPending}
		return
	}

	// Notify the SCM that we are running and these are the commands we will respond to
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}

				// Tell WinFSP to stop the service
				_ = stopService()
				return
			default:

			}
		}
	}
}

func startService() error {
	instanceName := "Z:"
	cmd := uint16('S')

	utf16className := windows.StringToUTF16(SvcName)
	utf16instanceName := windows.StringToUTF16(instanceName)

	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, cmd)
	for _, w := range utf16className {
		binary.Write(&buf, binary.LittleEndian, w)
	}
	for _, w := range utf16instanceName {
		binary.Write(&buf, binary.LittleEndian, w)
	}
	return winFspCommand(buf.Bytes())
}

func stopService() error {
	instanceName := "Z:"
	cmd := uint16('T')

	utf16className := windows.StringToUTF16(SvcName)
	utf16instanceName := windows.StringToUTF16(instanceName)

	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, cmd)
	for _, w := range utf16className {
		binary.Write(&buf, binary.LittleEndian, w)
	}
	for _, w := range utf16instanceName {
		binary.Write(&buf, binary.LittleEndian, w)
	}
	return winFspCommand(buf.Bytes())
}

func winFspCommand(command []byte) error {
	winPipe, err := windows.UTF16PtrFromString(winfspPipe)
	if err != nil {
		return err
	}

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
		return err
	}
	defer windows.CloseHandle(handle)

	var overlapped windows.Overlapped
	err = windows.WriteFile(handle, command, nil, &overlapped)
	if err == windows.ERROR_IO_PENDING {
		err = windows.GetOverlappedResult(handle, &overlapped, nil, true)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	overlapped = windows.Overlapped{}
	buff := make([]byte, 4096)
	var bytesRead uint32
	err = windows.ReadFile(handle, buff, &bytesRead, &overlapped)
	if err == windows.ERROR_IO_PENDING {
		err = windows.GetOverlappedResult(handle, &overlapped, &bytesRead, true)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	ret := strings.TrimSpace(string(buff[:bytesRead]))
	if ret != "$" {
		return errors.New("winfsp launchctl tool was unable to start the mount")
	}

	return nil
}
