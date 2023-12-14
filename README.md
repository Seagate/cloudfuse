# Cloudfuse - An S3 and Azure Storage FUSE driver
## About
Cloudfuse provides the ability to mount a cloud bucket in your local filesystem on Linux and Windows with a GUI for easy configuration.
With Cloudfuse you can easily read and write to the cloud, and connect programs on your computer to the cloud even if they're not cloud-aware.
Cloudfuse uses file caching to provide the performance of local storage, or you can use streaming mode to efficiently access small parts of large files (e.g. video playback).
Cloudfuse is a fork of [blobfuse2](https://github.com/Azure/azure-storage-fuse), and adds S3 support, a GUI, and Windows support.
Cloudfuse supports clouds with an S3 or Azure interface.

## Installation
Download the provided installation packages for your preferred operating system.  -- COMING SOON!

### Windows

Download and the .exe installer from our latest release [here](https://github.com/Seagate/cloudfuse/releases).

Cloudfuse will store the config file in `	C:\Users\{username}\AppData\Roaming`.

### From Tar or Zip files
In the release tab on GitHub, you can download a tar folder for Linux on x86 and a zip folder for Windows on x86 which bundles

the GUI and the Cloudfuse binary. Then run the `cloudfuseGUI` file on your system to launch the GUI or
call the `cloudfuse` binary file on the command line to use Cloudfuse as a command line tool.

On Windows, you will need to install WinFsp to use Cloudfuse. See [this](https://winfsp.dev/rel/) to install WinFSP.

### Source Installation
Please refer to the [Installation from source](https://github.com/Seagate/cloudfuse/wiki/Installation-From-Source) to 
manually install Cloudfuse.

## Config
The GUI can help you quickly configure Cloudfuse.

* There are three ways to run the GUI:
  - Open the Cloudfuse GUI provided in the installation package -- COMING SOON!
  - Extract either the tar or zip files to your computer and launch the `cloudfuseGUI`.
  - Run the GUI from source, see instructions [here](https://github.com/Seagate/cloudfuse/wiki/Running-the-GUI-from-source)
* Select the desired type of cloud (Azure or S3).
* Click `config`, which opens a new window for settings.
* Enter the credentials for your cloud:
  - Go to [S3 Storage Configuration](https://github.com/Seagate/cloudfuse/wiki/S3-Storage-Configuration) to find S3 credential requirements.
  - Go to [Azure Storage Configuration](https://github.com/Seagate/cloudfuse/wiki/Azure-Storage-Configuration) to find Azure credential requirements.
* Select you desired pipeline mode:
  - File Caching: Browse to a folder you wish to have you temp file caching.
  - Streaming: If 0MB are selected, streaming will not cache any blocks.
  - Go to [File-Cache](https://github.com/Seagate/cloudfuse/wiki/File-Cache) and [Streaming](https://github.com/Seagate/cloudfuse/wiki/Streaming) for details.
* Save your changes for the GUI to write your new config file.

You can also manually configure the config file without the GUI, you can find instructions [here](https://github.com/Seagate/cloudfuse/wiki/Config-File).

## Basic Use
### Linux
Launch the provided GUI and use the [Config instructions above](#Config) to configure the settings.
* `To mount`
  - Through the main window in the GUI, browse to the location you want your cloud to be mounted, then select the EMPTY folder you want.
    You may need to create this folder.
  - In the drop down menu, select the type of cloud you have, S3 or Azure, then click `mount`.
  - You should now see your data in the folder you selected.
* `To unmount`
  - Through the main window in the GUI, browse to your mounted folder and select it.
  - Select the type of cloud you have, S3 or Azure.
  - Click the unmount mutton.
  - The mount folder should now be empty.

### Windows
Note: Make sure WinSFP is installed, found in the [install instructions](#Installation).
* `To Mount`
  - Run the provided GUI.
  - To attach your cloud to a folder, browse to the location you want your cloud to mount and make sure that directory is empty.
  - Select the type of cloud you want, either S3 or Azure.
  - Click mount.

* `To Unmount`
  - Launch the provided GUI.
  - Through the main window of the GUI, browse to the folder you want unmounted and select it.
  - Select the type of cloud you have mounted, either S3 or Azure.
  - Click unmount.

You can also use the [command line interface](#Command-Line-Interface) to mount and unmount.

## Health Monitor
Cloudfuse also supports a health monitor. It allows customers gain more insight
into how their Cloudfuse instance is behaving with the rest of their machine.
Visit [here](https://github.com/Seagate/cloudfuse/wiki/Health-Monitor) to set it up.

## Command Line Interface

### Linux:
The general format of the Cloudfuse Linux commands is `cloudfuse [command] [arguments]
--[flag-name]=[flag-value]`
* `help` - Help about any command
* `mount` - Mounts a cloud storage container as a filesystem. The supported
  containers include:
  - [S3 Bucket Storage](https://aws.amazon.com/s3/)
  - [Azure Blob Storage](https://docs.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction)
  - [Azure Datalake Storage Gen2](https://docs.microsoft.com/en-us/azure/storage/blobs/data-lake-storage-introduction)
  - Example: `cloudfuse mount <mount path> --config-file=<config file>`
* `mount all` - Mounts all the containers in an S3 Account or Azure account supported by mount
  - Example: `cloudfuse mount all <mount path> --config-file=<config file>`
* `mount list` - Lists all Cloudfuse filesystems.
  - Example: `cloudfuse mount list`
* `unmount` - Unmounts the Cloudfuse filesystem.
  - Add `--lazy` (or `-z`) flag to use lazy unmount (prevents busy errors)
  - Example: `cloudfuse unmount --lazy <mount path>`
* `unmount all` - Unmounts all Cloudfuse filesystems.
  - Add `--lazy` (or `-z`) flag to use lazy unmount (prevents busy errors)
  - Example: `cloudfuse unmount all --lazy`

### Windows:

The general format of the Cloudfuse Windows commands is:
 `cloudfuse service [command] [arguments] --[flag-name]=[flag-value]`
  * `cloudfuse service install` - Installs the startup process for Cloudfuse
  * `cloudfuse service uninstall` - Uninstall the startup process for Cloudfuse
  * `cloudfuse service mount <mount path>  --config-file=<config file>` - Mount an instance that will persist in Windows when restarted
  * `cloudfuse service unmount <mount path>` - Unmount mount of Cloudfuse running as a Windows service

### Secure options for both Windows and Linux
To use security options the general format for cloudfuse commands is `cloudfuse [command] [arguments] --[flag-name]=[flag-value]`
* `secure decrypt` - Decrypts a config file.
* `secure encrypt` - Encrypts a config file.
* `secure get` - Gets value of a config parameter from an encrypted config file.
* `secure set` - Updates value of a config parameter.

## Find help from your command prompt
To see a list of commands, type `cloudfuse -h`. To
learn about a specific command, just include the name of the command (For
example: `cloudfuse mount -h`).


## NOTICE
- We have seen some customer issues around files getting corrupted when `streaming` is used in write mode. Kindly avoid using this feature for write while we investigate and resolve it.

## Un-Supported File system operations
- mkfifo : fifo creation is not supported by cloudfuse and this will result in
  "function not implemented" error
- chown  : Change of ownership is not supported by Azure Storage hence Cloudfuse
  does not support this.
- Creation of device files or pipes is not supported by Cloudfuse.
- Cloudfuse does not support extended-attributes (x-attrs) operations
- Cloudfuse does not support lseek() operation on directory handles. No error is thrown but it will not work as expected.

## Un-Supported Scenarios
- Cloudfuse does not support overlapping mount paths. While running multiple
  instances of Cloudfuse make sure each instance has a unique and
  non-overlapping mount point.
- Cloudfuse does not support co-existence with NFS on same mount path. Behavior
  in this case is undefined.
- For Azure block blob accounts, where data is uploaded through other means,
  Cloudfuse expects special directory marker files to exist in container. In
  absence of this few file operations might not work. For e.g. if you have a
  blob 'A/B/c.txt' then special marker files shall exists for 'A' and 'A/B',
  otherwise opening of 'A/B/c.txt' will fail. Once a 'ls' operation is done on
  these directories 'A' and 'A/B' you will be able to open 'A/B/c.txt' as well.
  Possible workaround to resolve this from your container is to either

  create the directory marker files manually through portal or run 'mkdir'
  command for 'A' and 'A/B' from cloudfuse. Refer
  [here](https://github.com/Azure/azure-storage-fuse/issues/866) for details on
  this.

## Limitations
- In case of Azure BlockBlob accounts, ACLs are not supported by Azure Storage
  so Cloudfuse will by default return success for 'chmod' operation. However it
  will work fine for Gen2 (DataLake) accounts. ACLs are not currently supported
  for S3 accounts.
- When Cloudfuse is mounted on a container, SYS_ADMIN privileges are required
  for it to interact with the fuse driver. If container is created without the
  privilege, mount will fail. Sample command to spawn a docker container is

    `docker run -it --rm --cap-add=SYS_ADMIN --device=/dev/fuse --security-opt
    apparmor:unconfined <environment variables> <docker image>`

### Syslog security warning
By default, Cloudfuse will log to syslog. The default settings will, in some
cases, log relevant file paths to syslog. If this is sensitive information, turn
off logging or set log-level to LOG_ERR.  

## Third-Party Notices
See [notices](./NOTICE) for third party license notices.

Qt is licensed under the GNU Lesser General Public License version 3, which is available at https://doc.qt.io/qt-6/lgpl.html

WinFSP is licensed under the GPLv3 license with a special exception for Free/Libre and Open Source Software, which is available at https://github.com/winfsp/winfsp/blob/master/License.txt

## Attribution
WinFsp - Windows File System Proxy, Copyright (C) Bill Zissimopoulos https://github.com/winfsp/winfsp

## License
The Cloudfuse project is licensed under MIT.

## Support
Please submit an issue
[here](https://github.com/Seagate/cloudfuse/issues) for any issues/feature
requests/questions.

## Frequently Asked Questions
A list of FAQs can be found [here](https://github.com/Seagate/cloudfuse/wiki/Frequently-Asked-Questions)

## Contributing
This project welcomes contributions and suggestions.

This project is governed by the [code of conduct](CODE_OF_CONDUCT.md). You are
expected to follow this as you contribute to the project. Please report all
unacceptable behavior to
[opensource@seagate.com](mailto:opensource@seagate.com).
