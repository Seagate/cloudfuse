
# Cloudfuse - An S3 and Azure Storage FUSE driver

[![License][license-badge]][license-url]
[![Release][release-badge]][release-url]
[![GitHub Releases Stats of cloudfuse][stats-badge]][stats-url]
[![Go Report Card][go-report-badge]][go-report-url]
[![OpenSSF Scorecard][openssf-badge]][openssf-url]

[license-badge]: https://img.shields.io/github/license/Seagate/cloudfuse
[license-url]: https://github.com/Seagate/cloudfuse/blob/main/LICENSE
[release-badge]: https://img.shields.io/github/release/Seagate/cloudfuse.svg
[release-url]: https://github.com/Seagate/cloudfuse/releases/latest
[stats-badge]: https://img.shields.io/github/downloads/Seagate/cloudfuse/total.svg?logo=github
[stats-url]: https://somsubhra.github.io/github-release-stats/?username=Seagate&repository=cloudfuse
[go-report-badge]: https://goreportcard.com/badge/github.com/Seagate/cloudfuse
[go-report-url]: https://goreportcard.com/report/github.com/Seagate/cloudfuse
[openssf-badge]: https://img.shields.io/ossf-scorecard/github.com/Seagate/cloudfuse?label=openssf%20scorecard
[openssf-url]: https://scorecard.dev/viewer/?uri=github.com/Seagate/cloudfuse

Cloudfuse provides the ability to mount a cloud bucket in your local filesystem on Linux and Windows.
With Cloudfuse you can easily read and write to the cloud, and connect programs on your computer to the cloud even if they're not cloud-aware.
Cloudfuse uses file caching to provide the performance of local storage, or you can use streaming mode to efficiently access small parts of large files (e.g. video playback).
Cloudfuse is a fork of [blobfuse2](https://github.com/Azure/azure-storage-fuse), and adds S3 support and Windows support.
Cloudfuse supports clouds with an S3 or Azure interface.

## Table of Contents

- [Installation](#installation)
  - [Windows](#windows)
  - [Linux](#linux)
  - [From Archive](#from-archive)
  - [From Source](#from-source)
- [Basic Use](#basic-use)
- [Health Monitor](#health-monitor)
- [Command Line Interface](#command-line-interface)
- [Limitations](#limitations)
- [License](#license)
- [Support](#support)
- [Contributing](#contributing)

## Installation

### Windows

Download and run the .exe installer from our latest release [here](https://github.com/Seagate/cloudfuse/releases). Uncheck the "Launch Cloudfuse" upon finishing the installation. Run the CLI after the install completes.

### Linux

#### Debian /Ubuntu

Download the .deb file from our latest release [here](https://github.com/Seagate/cloudfuse/releases) and run the following command in your terminal:

`sudo apt-get install ./cloudfuse*.deb`

#### CentOS / RHEL

Download the .rpm file from our latest release [here](https://github.com/Seagate/cloudfuse/releases) and run the following command in your terminal:

`sudo rpm -i ./cloudfuse*.rpm`

#### Enable Running With Systemd

To enable Cloudfuse to run using systemd, see [Setup for systemd instructions](setup/readme.md)

### From Archive

Download the archive for your platform and architecture from the latest release [here](https://github.com/Seagate/cloudfuse/releases).
On Windows, you will need to install WinFsp to use Cloudfuse. See [this](https://winfsp.dev/rel/) to install WinFSP.

### From Source

Please refer to the [Installation from source](https://github.com/Seagate/cloudfuse/wiki/Installation-From-Source) to
manually install Cloudfuse.

## Basic Use

## Health Monitor

Cloudfuse also supports a health monitor.
The health monitor allows customers gain more insight into how their Cloudfuse instance is behaving with the rest of their machine.
Visit [here](https://github.com/Seagate/cloudfuse/wiki/Health-Monitor) to set it up.

## Command Line Interface

The general format of the Cloudfuse Linux commands is:

`cloudfuse [command] [arguments] --[flag-name]=[flag-value]`

Available commands:

- `help [command]` - Displays general help, or help for the specified command
- `mount` - Mounts a cloud storage container as a filesystem
  Example: `cloudfuse mount <mount path> --config-file=<config file>`
  Supported container types:
  - [S3 Bucket Storage](https://aws.amazon.com/s3/)
  - [Azure Blob Storage](https://docs.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction)
  - [Azure Datalake Storage Gen2](https://docs.microsoft.com/en-us/azure/storage/blobs/data-lake-storage-introduction)
- `mount all` - Mounts all the containers in an S3 Account or Azure account supported by mount
  Example: `cloudfuse mount all <mount path> --config-file=<config file>`
  On Windows, mounted containers will be remounted on login after a restart.
- `mount list` - Lists all Cloudfuse filesystems
  Example: `cloudfuse mount list`
- `unmount` - Unmounts the Cloudfuse filesystem
  Add `--lazy` (or `-z`) flag to use lazy unmount (prevents busy errors).
  Example: `cloudfuse unmount --lazy <mount path>`
  On Windows, unmounting a container also stops it from being automatically remounted at login.
- `unmount all` - Unmounts all Cloudfuse filesystems
  Add `--lazy` (or `-z`) flag to use lazy unmount (prevents busy errors) - Linux only.
  Example: `cloudfuse unmount all --lazy`

### Remount on Startup (Windows Only)

- `cloudfuse service install` - Installs the startup process for Cloudfuse which remounts containers on login after a restart.
- `cloudfuse service uninstall` - Uninstalls the startup process for Cloudfuse and prevents containers from being remounted on login.

### Secure Options

To use security options the general format for cloudfuse commands is:

`cloudfuse [command] [arguments] --[flag-name]=[flag-value]`

- `secure decrypt` - Decrypts a config file
- `secure encrypt` - Encrypts a config file
- `secure get` - Gets value of a config parameter from an encrypted config file
- `secure set` - Updates value of a config parameter

### Find help from your command prompt

To see a list of commands, type `cloudfuse -h`.
To learn about a specific command, just include the name of the command (For example: `cloudfuse mount -h`).

### Verifying Authenticity

Cloudfuse releases are signed keylessly using Cosign. To verify that the checksum file is genuine, use the following command where {tag} is the version of cloudfuse.

```bash
cosign verify-blob ./checksums_sha256.txt --bundle ./checksums_sha256.txt.bundle --certificate-identity https://github.com/Seagate/cloudfuse/.github/workflows/publish-release.yml@refs/tags/{tag} --certificate-oidc-is
suer https://token.actions.githubusercontent.com
```

This command should then print out "Verified OK" is the checksum file is valid. You can then use these checksums to verify that the cloudfuse version downloaded matches the expected checksum.

## Limitations

### NOTICE

- We have seen some customer issues around files getting corrupted when `streaming` is used in write mode.
Kindly avoid using this feature for write while we investigate and resolve it.

### Un-Supported File system operations

- mkfifo : fifo creation is not supported by cloudfuse and this will result in
  "function not implemented" error
- chown  : Change of ownership is not supported by Azure Storage hence Cloudfuse
  does not support this.
- Creation of device files or pipes is not supported by Cloudfuse.
- Cloudfuse does not support extended-attributes (x-attrs) operations
- Cloudfuse does not support lseek() operation on directory handles.
  No error is thrown but it will not work as expected.

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

## License

The Cloudfuse project is licensed under MIT.

### Third-Party Notices

See [notices](./NOTICE) for third party license notices.

WinFSP is licensed under the GPLv3 license with a special exception for Free/Libre and Open Source Software,
which is available [here](https://github.com/winfsp/winfsp/blob/master/License.txt).

### Attribution

WinFsp - Windows File System Proxy, Copyright (C) Bill Zissimopoulos - [link](https://github.com/winfsp/winfsp)

## Support

### Contact Us

We welcome your questions and feedback!
Email us: [cloudfuse@seagate.com](mailto:cloudfuse@seagate.com).

### Frequently Asked Questions

A list of FAQs can be found [here](https://github.com/Seagate/cloudfuse/wiki/Frequently-Asked-Questions).

### Report Issues and Request Features

Please submit [issues and requests here](https://github.com/Seagate/cloudfuse/issues).

## Contributing

This project welcomes contributions and suggestions.

This project is governed by the [code of conduct](CODE_OF_CONDUCT.md).
You are expected to follow this as you contribute to the project.
Please report all unacceptable behavior to [opensource@seagate.com](mailto:opensource@seagate.com)
