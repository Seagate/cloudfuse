//go:build windows

package windows_service

import (
	"bytes"
	"encoding/binary"
	"errors"
	"lyvecloudfuse/common/log"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
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

type KeyData struct {
	InstanceName string
	MountDir     string
	ConfigFile   string
}

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
	// Read registry to get names of the instances we need to start
	instances, err := readRegistryEntry()
	if err != nil {
		return err
	}

	cmd := uint16(startCmd)

	for _, inst := range instances {
		utf16className := windows.StringToUTF16(SvcName)
		utf16instanceName := windows.StringToUTF16(inst.InstanceName)
		utf16driveName := windows.StringToUTF16(inst.MountDir)
		utf16configFile := windows.StringToUTF16(inst.ConfigFile)

		buf := writeToUtf16(cmd, utf16className, utf16instanceName, utf16driveName, utf16configFile)
		err = winFspCommand(buf)
		if err != nil {
			return err
		}
	}

	return nil
}

// startService stops lyvecloudfuse by instructing WinFsp to stop it.
func stopService() error {
	// Read registry to get names of the instances we need to stop
	instances, err := readRegistryEntry()
	if err != nil {
		return err
	}

	cmd := uint16(stopCmd)

	for _, inst := range instances {
		utf16className := windows.StringToUTF16(SvcName)
		utf16instanceName := windows.StringToUTF16(inst.InstanceName)
		utf16driveName := windows.StringToUTF16(inst.MountDir)

		buf := writeToUtf16(cmd, utf16className, utf16instanceName, utf16driveName)
		err = winFspCommand(buf)
		if err != nil {
			return err
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

func readRegistryEntry() ([]KeyData, error) {
	registryPath := `SOFTWARE\Seagate\LyveCloudFuse\Instances\`

	key, err := registry.OpenKey(registry.LOCAL_MACHINE, registryPath, registry.ALL_ACCESS)
	if err != nil {
		log.Err("Unable to read instance names from Windows Registry: %v", err.Error())
		return nil, err
	}
	defer key.Close()

	keys, err := key.ReadSubKeyNames(-1)
	if err != nil {
		log.Err("Unable to read subkey names from Windows Registry: %v", err.Error())
		return nil, err
	}

	var data []KeyData

	for _, k := range keys {
		subkey, err := registry.OpenKey(key, k, registry.QUERY_VALUE)
		if err != nil {
			log.Err("Unable to open subkey from Windows Registry: %v", err.Error())
			return nil, err
		}

		var d KeyData

		d.InstanceName, _, err = subkey.GetStringValue("InstanceName")
		if err != nil {
			log.Err("Unable to read key InstanceName from instance in Windows Registry: %v", err.Error())
			return nil, err
		}

		d.ConfigFile, _, err = subkey.GetStringValue("ConfigFile")
		if err != nil {
			log.Err("Unable to read key ConfigFile from instance in Windows Registry: %v", err.Error())
			return nil, err
		}

		d.MountDir, _, err = subkey.GetStringValue("MountDir")
		if err != nil {
			log.Err("Unable to read key MountDir from instance in Windows Registry: %v", err.Error())
			return nil, err
		}

		data = append(data, d)
		_ = subkey.Close()
	}

	return data, nil
}
