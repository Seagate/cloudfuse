# Cloudfuse - An S3 and Azure Storage FUSE driver

Cloudfuse provides the ability to mount a cloud bucket in your local filesystem on Linux and Windows with a GUI for easy configuration.
With Cloudfuse you can easily read and write to the cloud, and connect programs on your computer to the cloud even if they're not cloud-aware.
Cloudfuse uses file caching to provide the performance of local storage, or you can use streaming mode to efficiently access small parts of large files (e.g. video playback).
Cloudfuse is a fork of [blobfuse2](https://github.com/Azure/azure-storage-fuse), and adds S3 support, a GUI, and Windows support.
Cloudfuse supports clouds with an S3 or Azure interface.

## Table of Contents  

- [Installation](#installation)
  - [Windows](#windows)
  - [Linux](#linux)
  - [From Tar or Zip files](#from-tar-or-zip-files)
  - [Source Installation](#source-installation)
- [Basic Use](#basic-use)
- [Health Monitor](#health-monitor)
- [Command Line Interface](#command-line-interface)
  - [Linux](#linux-1)
  - [Windows](#windows-1)
  - [Secure options for both Windows and Linux](#secure-options-for-both-windows-and-linux)
- [Limitations](#limitations)
- [License](#license)
- [Support](#support)
- [Contributing](#contributing)

## Installation

### Windows

Download and run the .exe installer from our latest release [here](https://github.com/Seagate/cloudfuse/releases). Uncheck the "Launch Cloudfuse" upon finishing the installation. Run the GUI separately as admin after the install completes.

### Linux

#### Debian /Ubuntu

Download the .deb file from our latest release [here](https://github.com/Seagate/cloudfuse/releases) and run the following command in your terminal:  
`sudo apt-get install ./cloudfuse*.deb`

#### CentOS / RHEL

Download the .rpm file from our latest release [here](https://github.com/Seagate/cloudfuse/releases) and run the following command in your terminal:  
`sudo rpm -i ./cloudfuse*.rpm`

### From Tar or Zip files

In the release tab on GitHub, you can download a tar folder for Linux on x86 and a zip folder for Windows on x86 which bundles

the GUI and the Cloudfuse binary. Then run the `cloudfuseGUI` file on your system to launch the GUI or
call the `cloudfuse` binary file on the command line to use Cloudfuse as a command line tool.

On Windows, you will need to install WinFsp to use Cloudfuse. See [this](https://winfsp.dev/rel/) to install WinFSP.

### Source Installation

Please refer to the [Installation from source](https://github.com/Seagate/cloudfuse/wiki/Installation-From-Source) to
manually install Cloudfuse.

## Basic Use

The quickest way to get started with Cloudfuse is to use the GUI. Open Cloudfuse from the desktop shortcut to launch it.  
If you installed Cloudfuse from an archive, you can run the GUI by running `cloudfuseGUI` from the extracted archive. To run the GUI from source, see instructions [here](https://github.com/Seagate/cloudfuse/wiki/Running-the-GUI-from-source).  

- Choose mount settings
  - Select the desired type of cloud (Azure or S3).
  - Click `config` to open the settings window.
  - Enter the credentials for your cloud storage container  
  (see [here for S3](https://github.com/Seagate/cloudfuse/wiki/S3-Storage-Configuration), or [here for Azure](https://github.com/Seagate/cloudfuse/wiki/Azure-Storage-Configuration) credential requirements).
  - Select file caching or streaming mode (see [File-Cache](https://github.com/Seagate/cloudfuse/wiki/File-Cache) and [Streaming](https://github.com/Seagate/cloudfuse/wiki/Streaming) for details).
  - Close the settings window and save your changes.  

  Cloudfuse will store the config file in `C:\Users\{username}\AppData\Roaming` on Windows and in `/opt/cloudfuse/` on Linux.  
  You can also edit the config file directly (see [guide](https://github.com/Seagate/cloudfuse/wiki/Config-File)).  
- Mount your container
  - Click `Browse` Through the main window in the GUI, browse to the location you want your cloud to be mounted, then select the EMPTY folder you want. You may need to create this folder.
  - Click `Mount`.
  - Watch for status messages below. On success, your files will appear in the mount directory.  
    Note: if mount fails with an error mentioning WinFSP, you may need to install WinFSP (see [installation instructions](#installation)).  

  On Windows, mounted containers will persist across system restarts.
  
- Unmount
  - Make sure the mount directory you want to unmount is listed. If it isn't, click `browse` and select it.
  - Click the `unmount` mutton.
  - Watch for a status message below. On success, the mount directory will become empty.  
    Note: If you enabled the `Persist File Cache` option, the local file cache for the container will be kept and reused when the container is mounted again.  

You can also use the [command line interface](#command-line-interface) to mount and unmount.

## Health Monitor

Cloudfuse also supports a health monitor. It allows customers gain more insight
into how their Cloudfuse instance is behaving with the rest of their machine.
Visit [here](https://github.com/Seagate/cloudfuse/wiki/Health-Monitor) to set it up.

## Command Line Interface

### Linux

The general format of the Cloudfuse Linux commands is `cloudfuse [command] [arguments]
--[flag-name]=[flag-value]`
- `help` - Help about any command
- `mount` - Mounts a cloud storage container as a filesystem. The supported
  containers include:
  - [S3 Bucket Storage](https://aws.amazon.com/s3/)
  - [Azure Blob Storage](https://docs.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction)
  - [Azure Datalake Storage Gen2](https://docs.microsoft.com/en-us/azure/storage/blobs/data-lake-storage-introduction)
  - Example: `cloudfuse mount <mount path> --config-file=<config file>`
- `mount all` - Mounts all the containers in an S3 Account or Azure account supported by mount
  - Example: `cloudfuse mount all <mount path> --config-file=<config file>`
- `mount list` - Lists all Cloudfuse filesystems.
  - Example: `cloudfuse mount list`
- `unmount` - Unmounts the Cloudfuse filesystem.
  - Add `--lazy` (or `-z`) flag to use lazy unmount (prevents busy errors)
  - Example: `cloudfuse unmount --lazy <mount path>`
- `unmount all` - Unmounts all Cloudfuse filesystems.
  - Add `--lazy` (or `-z`) flag to use lazy unmount (prevents busy errors)
  - Example: `cloudfuse unmount all --lazy`

### Windows
The general format of the Cloudfuse Windows commands is:
 `cloudfuse service [command] [arguments] --[flag-name]=[flag-value]`
  - `cloudfuse service install` - Installs the startup process for Cloudfuse
  - `cloudfuse service uninstall` - Uninstall the startup process for Cloudfuse
  - `cloudfuse service mount <mount path>  --config-file=<config file>` - Mount an instance that will persist in Windows when restarted
  - `cloudfuse service unmount <mount path>` - Unmount mount of Cloudfuse running as a Windows service

### Secure options for both Windows and Linux
To use security options the general format for cloudfuse commands is `cloudfuse [command] [arguments] --[flag-name]=[flag-value]`
- `secure decrypt` - Decrypts a config file.
- `secure encrypt` - Encrypts a config file.
- `secure get` - Gets value of a config parameter from an encrypted config file.
- `secure set` - Updates value of a config parameter.

Note - If you do not have admin rights, you can still mount your cloud without Windows Service, however
the process will stay in the foreground. Use `cloudfuse mount <mount path>  --config-file=<config file>` to mount, use Ctrl+C to unmount.

### Find help from your command prompt
To see a list of commands, type `cloudfuse -h`. To
learn about a specific command, just include the name of the command (For
example: `cloudfuse mount -h`).

## Limitations

### NOTICE
- We have seen some customer issues around files getting corrupted when `streaming` is used in write mode. Kindly avoid using this feature for write while we investigate and resolve it.

### Un-Supported File system operations
- mkfifo : fifo creation is not supported by cloudfuse and this will result in
  "function not implemented" error
- chown  : Change of ownership is not supported by Azure Storage hence Cloudfuse
  does not support this.
- Creation of device files or pipes is not supported by Cloudfuse.
- Cloudfuse does not support extended-attributes (x-attrs) operations
- Cloudfuse does not support lseek() operation on directory handles. No error is thrown but it will not work as expected.

### Un-Supported Scenarios
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

### Other Limitations
- In case of Azure BlockBlob accounts, ACLs are not supported by Azure Storage
  so Cloudfuse will by default return success for 'chmod' operation. However it
  will work fine for Gen2 (DataLake) accounts. ACLs are not currently supported
  for S3 accounts.
- When Cloudfuse is mounted on a docker container, SYS_ADMIN privileges are required
  for it to interact with the fuse driver. If container is created without the
  privilege, mount will fail. Sample command to spawn a docker container is  
    `docker run -it --rm --cap-add=SYS_ADMIN --device=/dev/fuse --security-opt
    apparmor:unconfined <environment variables> <docker image>`

### Syslog security warning
By default, Cloudfuse will log to syslog. The default settings will, in some
cases, log relevant file paths to syslog. If this is sensitive information, turn
off logging or set log-level to LOG_ERR.  

## License

The Cloudfuse project is licensed under MIT.

### Third-Party Notices
See [notices](./NOTICE) for third party license notices.

Qt is licensed under the GNU Lesser General Public License version 3, which is available at https://doc.qt.io/qt-6/lgpl.html

WinFSP is licensed under the GPLv3 license with a special exception for Free/Libre and Open Source Software, which is available at https://github.com/winfsp/winfsp/blob/master/License.txt

### Attribution
WinFsp - Windows File System Proxy, Copyright (C) Bill Zissimopoulos https://github.com/winfsp/winfsp

## Support

### Frequently Asked Questions
A list of FAQs can be found [here](https://github.com/Seagate/cloudfuse/wiki/Frequently-Asked-Questions)

### Report Issues and Request Features
We welcome all feedback! Please submit [issues and requests here](https://github.com/Seagate/cloudfuse/issues).

## Contributing
This project welcomes contributions and suggestions.

This project is governed by the [code of conduct](CODE_OF_CONDUCT.md). You are
expected to follow this as you contribute to the project. Please report all
unacceptable behavior to
[opensource@seagate.com](mailto:opensource@seagate.com).
