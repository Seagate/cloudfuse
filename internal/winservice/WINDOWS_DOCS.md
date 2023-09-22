# Windows Documentation
## Running as a Windows Service

Cloudfuse supports running as a Windows service by using the service command on the command line. See [cloudfuse
service](../../doc/cloudfuse_service.md) for more information on the specific commands.

For examples of running a Windows service in Go refer to the following example in the Go source code
https://pkg.go.dev/golang.org/x/sys/windows/svc/example

## Windows Registry
The project utilizes the Windows registry in two ways.

1) We store every mount in the Windows registry to maintain a list of current mounts that will automatically be started
during a reboot. These are stored in SOFTWARE\Seagate\Cloudfuse\Instances. Each key in the instance folder will be a
mount path with all back slashes replaced with fordward slashes (since backward slashes are not allowed in registry key
names). Each of these will store a ConfigFile where the entry is the full path to the relevant config file for the
mount.

2) We add a registry instance to SOFTWARE\WOW6432Node\WinFsp\Services\ to WinFsp so that WinFsp can run our service. We
follow the documentation given in https://winfsp.dev/doc/WinFsp-Service-Architecture/. In this registry we give the
location of the Cloudfuse executable and the command that WinFsp will use to start the mount. The command we use is
'mount %1 --config-file=%2' where %1 and %2 will represent the first and second arguments passed to the start command of
WinFsp (after the required class name and instance name).

## WinFsp Commands
When mounting or unmounting as a Windows service we send commands to the WinFsp launcher to start or stop the mount. For
each mount or unmount command WinFsp requires an instance name be passed alongside the command. This name uniquely
identifies the mount. For simplicity we use the mount path as the instance name since this uniquely identifies each
mount and it prevents us from needing to remember the instance name.

Commands are sent to WinFsp using named pipes. The WinFsp pipe is located at
\\.\pipe\WinFsp.{14E7137D-22B4-437A-B0C1-D21D1BDF3767}. Each command is sent as a UTF16 formatted string in bytes. Each
command requires a class name to be sent which refers to the name of the registry key in the WinFsp registry that WinFsp
should use when executing the command. In this case it is cloudfuse. Most commands require an instance name to
uniquely identify the running mount. The instance name is simplify the mount path in our architecture.

Each command will write output to the named pipe to indicate success or failure. '$' indicates a successful command and
'!' indicates failure.

### Mount Command
For mount, the command 'S' is sent the to the named pipe along with the class name, instance name, mount path, and
config file path. The mount path and config file path are used when running the listed command for cloudfuse in the
WinFsp registry.

### Unmount Command
For unmount, the command 'T' is sent the to the named pipe along with the class name and instance name.

### List Command
For list, the command 'L' is sent the to the named pipe. This command will list all active mounts managed by WinFsp which
will be written to the named pipe as a list of class name and instance names in the named pipe.