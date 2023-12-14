# Windows Documentation
## Windows Registry
The project edits the Windows registry to connect to the WinFSP Launcher.

We add a registry instance to SOFTWARE\WOW6432Node\WinFsp\Services\ to WinFsp so that WinFsp can run our service. We
follow the documentation given in https://winfsp.dev/doc/WinFsp-Service-Architecture/. In this registry we give the
location of the Cloudfuse executable and the command that WinFsp will use to start the mount. The command we use is ' -o
uid=%3,gid=%4' where %1, %2, %3, and %4 will represent the four arguments passed to the start command of WinFsp (after
the required class name and instance name).

## Storing Mounts and Launching on Reboot
We store every mount in the users appdata folder in a file called mounts.json to maintain a list of persistent mounts
that will automatically be started during a reboot. The path where the mount is located and the path to the config file
are stored. On startup a startup process that cloudfuse installs will run and will relaunch all mounts in the
mounts.json file.

## WinFsp Commands
When mounting or unmounting using the Windows service ocmmands we send commands to the WinFsp launcher to start or stop
the mount. For each mount or unmount command WinFsp requires an instance name be passed alongside the command. This name
uniquely identifies the mount. For simplicity we use the mount path as the instance name since this uniquely identifies
each mount and it prevents us from needing to remember the instance name.

Commands are sent to WinFsp using named pipes. The WinFsp pipe is located at
\\.\pipe\WinFsp.{14E7137D-22B4-437A-B0C1-D21D1BDF3767}. Each command is sent as a UTF16 formatted string in bytes. Each
command requires a class name to be sent which refers to the name of the registry key in the WinFsp registry that WinFsp
should use when executing the command. In this case it is cloudfuse. Most commands require an instance name to uniquely
identify the running mount. The instance name is simplify the mount path in our architecture.

Each command will write output to the named pipe to indicate success or failure. '$' indicates a successful command and
'!' indicates failure.

### Mount Command
For mount, the command 'S' is sent the to the named pipe along with the class name, instance name, mount path, and
config file path. The mount path and config file path are used when running the listed command for cloudfuse in the
WinFsp registry.

### Unmount Command
For unmount, the command 'T' is sent the to the named pipe along with the class name and instance name.

### List Command
For list, the command 'L' is sent the to the named pipe. This command will list all active mounts managed by WinFsp
which will be written to the named pipe as a list of class name and instance names in the named pipe.
