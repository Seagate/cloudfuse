//go:build windows

package windowsService

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
	listCmd    = 'L'
	successCmd = '$'
	failCmd    = '!'
)

type LyveCloudFuse struct{}

type KeyData struct {
	MountPath  string
	ConfigFile string
}

func (m *LyveCloudFuse) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
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
	cmd := uint16(startCmd)

	utf16className := windows.StringToUTF16(SvcName)
	utf16driveName := windows.StringToUTF16(mountPath)
	utf16instanceName := utf16driveName
	utf16configFile := windows.StringToUTF16(configFile)

	buf := writeToUtf16(cmd, utf16className, utf16instanceName, utf16driveName, utf16configFile)
	_, err := winFspCommand(buf)
	if err != nil {
		return err
	}
	return nil
}

// StopMount stops the mount if the name exists in our Windows registry.
func StopMount(mountPath string) error {
	cmd := uint16(stopCmd)

	utf16className := windows.StringToUTF16(SvcName)
	utf16driveName := windows.StringToUTF16(mountPath)
	utf16instanceName := utf16driveName

	buf := writeToUtf16(cmd, utf16className, utf16instanceName, utf16driveName)
	_, err := winFspCommand(buf)
	if err != nil {
		return err
	}
	return nil
}

func IsMounted(mountPath string) (bool, error) {
	cmd := uint16(listCmd)

	buf := writeToUtf16(cmd)
	list, err := winFspCommand(buf)
	if err != nil {
		return false, err
	}

	// Everything in the list is a name of a service using WinFsp, like lyvecloudfuse and then
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

// startService starts lyvecloudfuse by instructing WinFsp to launch it.
func startServices() error {
	// Read registry to get names of the instances we need to start
	instances, err := readRegistryEntry()
	if err != nil {
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

// stopServicess stops lyvecloudfuse by instructing WinFsp to stop it.
func stopServices() error {
	// Read registry to get names of the instances we need to stop
	instances, err := readRegistryEntry()
	if err != nil {
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

func writeToUtf16(cmd uint16, args ...[]uint16) []byte {
	var buf bytes.Buffer

	// Write the command we are sending to WinFsp
	_ = binary.Write(&buf, binary.LittleEndian, cmd)

	// Write the arguments
	for _, arg := range args {
		for _, w := range arg {
			_ = binary.Write(&buf, binary.LittleEndian, w)
		}
	}

	return buf.Bytes()
}

// winFspCommand sends an instruciton to WinFsp.
func winFspCommand(command []byte) ([]string, error) {
	var retStrings []string
	winPipe, err := windows.UTF16PtrFromString(winfspPipe)
	if err != nil {
		return retStrings, err
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
		return retStrings, err
	}
	defer windows.CloseHandle(handle)

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

	// If there is more to read then we are using a command with return data,
	// so let's try to read it
	if bytesRead > 2 {
		var start int
		buffer := ubuf[1 : bytesRead/2]
		for i, v := range buffer {
			if v == 0 {
				if start != i {
					retStrings = append(retStrings, windows.UTF16ToString(buffer[start:i]))
				}
				start = i + 1
			}
		}
	}

	return retStrings, nil
}

func bytesToUint16(buf []byte) []uint16 {
	var ubuf []uint16
	for i := 0; i < len(buf); i += 2 {
		ubuf = append(ubuf, binary.LittleEndian.Uint16(buf[i:i+2]))
	}
	return ubuf
}
