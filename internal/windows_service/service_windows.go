//go:build windows

package windows_service

import (
	"bytes"
	"encoding/binary"
	"errors"
	"lyvecloudfuse/common/log"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

const (
	SvcName    = "lyvecloudfuse"
	winfspPipe = `\\.\pipe\WinFsp.{14E7137D-22B4-437A-B0C1-D21D1BDF3767}`
	startCmd   = 'S'
	stopCmd    = 'T'
	successCmd = '$'
)

type LyveCloudFuse struct{}

func (m *LyveCloudFuse) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	// Notify the Service Control Manager that the service is starting
	changes <- svc.Status{State: svc.StartPending}
	log.Trace("Starting %s service", SvcName)

	// Send request to WinFSP to start the process
	err := startService()
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

	for {
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
				err := stopService()
				if err != nil {
					log.Err("Error stopping %s service: %v", SvcName, err.Error())
				}
				return
			}
		}
	}
}

// startService starts lyvecloudfuse by instructing WinFsp to launch it.
func startService() error {
	instanceName := "Z:"
	cmd := uint16(startCmd)

	utf16className := windows.StringToUTF16(SvcName)
	utf16instanceName := windows.StringToUTF16(instanceName)

	buf := writeToUtf16(cmd, utf16className, utf16instanceName)

	return winFspCommand(buf)
}

// startService stops lyvecloudfuse by instructing WinFsp to stop it.
func stopService() error {
	instanceName := "Z:"
	cmd := uint16(stopCmd)
	utf16className := windows.StringToUTF16(SvcName)
	utf16instanceName := windows.StringToUTF16(instanceName)

	buf := writeToUtf16(cmd, utf16className, utf16instanceName)

	return winFspCommand(buf)
}

func writeToUtf16(cmd uint16, name []uint16, instance []uint16) []byte {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, cmd)
	for _, w := range name {
		_ = binary.Write(&buf, binary.LittleEndian, w)
	}
	for _, w := range instance {
		_ = binary.Write(&buf, binary.LittleEndian, w)
	}
	return buf.Bytes()
}

// winFspCommand sends an instruciton to WinFsp.
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
	buf := make([]byte, 4096)
	var bytesRead uint32
	err = windows.ReadFile(handle, buf, &bytesRead, &overlapped)
	if err == windows.ERROR_IO_PENDING {
		err = windows.GetOverlappedResult(handle, &overlapped, &bytesRead, true)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// If there are not enough bytes for the return character, then it failed
	if bytesRead < 2 {
		return errors.New("winfsp launchctl tool was unable to start the mount")
	}

	ret := binary.LittleEndian.Uint16(buf)
	if ret != successCmd {
		return errors.New("winfsp launchctl tool was unable to start the mount")
	}

	return nil
}
