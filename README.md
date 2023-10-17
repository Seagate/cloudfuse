# Cloudfuse - An S3 and Azure Storage FUSE driver
## About
Cloudfuse provides the ability to mount a cloud bucket as a folder on Linux and Windows with a GUI for easy configuration. 
Cloudfuse is a fork of the open source project
[blobfuse2](https://github.com/Azure/azure-storage-fuse) from Microsoft which then added support
for S3 storage, a GUI for configuration and mounting, and Windows
support. It provides a virtual filesystem backed by either S3 or Azure Storage.

## SUPPORT
Please submit an issue
[here](https://github.com/Seagate/cloudfuse/issues) for any issues/feature
requests/questions.


## QUICK SETUP

COMING SOON!
Download the provided installation packages for your preferred operating system. 


Please refer to the [Installation from source](https://github.com/Seagate/cloudfuse/wiki/Installation-From-Source) to 
manually install Cloudfuse.

## QUICK CONFIG

COMING SOON!
Open the Cloudfuse GUI provided in the installation package

For now, run the GUI from source, please refer to the [running the GUI from source](https://github.com/Seagate/cloudfuse/wiki/Running-the-GUI-from-source)
to configure the Config file. 

To configure you setup for a specific cloud bucket refer to [Azure Storage Configuration](https://github.com/Seagate/cloudfuse/wiki/Azure-Storage-Configuration) or [S3 Storage Configuration](https://github.com/Seagate/cloudfuse/wiki/S3-Storage-Configuration) wiki's.

## Basic Use


## Health Monitor
Cloudfuse also supports a health monitor. It allows customers gain more insight
into how their Cloudfuse instance is behaving with the rest of their machine.
Visit [here](https://github.com/Seagate/cloudfuse/wiki/Health-Monitor) to set it up.

## Advanced Usage
- Mount with cloudfuse
    * cloudfuse mount \<mount path> --config-file=\<config file>
- Mount all containers in your storage account
    * cloudfuse mount all \<mount path> --config-file=\<config file>
- List all mount instances of cloudfuse
    * cloudfuse mount list
- Unmount cloudfuse on Linux
    * cloudfuse unmount \<mount path>
- Unmount all cloudfuse instances on Linux
    * cloudfuse unmount all
- Install as a Windows service
    * cloudfuse service install
- Uninstall cloudfuse from a Windows service
    * cloudfuse service uninstall
- Start the Windows service
    * cloudfuse service start
- Stop the Windows service
    * cloudfuse service stop
- Mount an instance that will persist in Windows when restarted
    * cloudfuse service mount \<mount path>  --config-file=\<config file>
- Unmount mount of Cloudfuse running as a Windows service
    * cloudfuse service unmount \<mount path>

## Find help from your command prompt
To see a list of commands, type `cloudfuse -h` and then press the ENTER key. To
learn about a specific command, just include the name of the command (For
example: `cloudfuse mount -h`).

## Supported Operations
The general format of the Cloudfuse commands is `cloudfuse [command] [arguments]
--[flag-name]=[flag-value]`
* `help` - Help about any command
* `mount` - Mounts a cloud storage container as a filesystem. The supported
  containers include
  - S3 Bucket
  - Azure Blob Container
  - Azure Datalake Gen2 Container
* `mount all` - Mounts all the containers in an S3 Account or Azure account as a
  filesystem. The supported storage services include
  - [S3 Storage](https://aws.amazon.com/s3/)
  - [Blob Storage](https://docs.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction)
  - [Datalake Storage Gen2](https://docs.microsoft.com/en-us/azure/storage/blobs/data-lake-storage-introduction)
* `mount list` - Lists all Cloudfuse filesystems.
* `secure decrypt` - Decrypts a config file.
* `secure encrypt` - Encrypts a config file.
* `secure get` - Gets value of a config parameter from an encrypted config file.
* `secure set` - Updates value of a config parameter.
* `unmount` - Unmounts the Cloudfuse filesystem.
* `unmount all` - Unmounts all Cloudfuse filesystems.

## NOTICE
- We have seen some customer issues around files getting corrupted when `streaming` is used in write mode. Kindly avoid using this feature for write while we investigate and resolve it.


<!---TODO Add Usage for mount, unmount, etc--->
## CLI parameters
- General options
    * `--config-file=<PATH>`: The path to the config file.
    * `--log-level=<LOG_*>`: The level of logs to capture.
    * `--log-file-path=<PATH>`: The path for the log file.
    * `--foreground=true`: Mounts the system in foreground mode.
    * `--read-only=true`: Mount container in read-only mode.
    * `--default-working-dir`: The default working directory to store log files
      and other cloudfuse related information.
    * `--disable-version-check=true`: Disable the cloudfuse version check.
    * `--secure-config=true` : Config file is encrypted suing 'cloudfuse secure`
      command.
    * `--passphrase=<STRING>` : Passphrase used to encrypt/decrypt config file.
    * `--wait-for-mount=<TIMEOUT IN SECONDS>` : Let parent process wait for
      given timeout before exit to ensure child has started.
- Attribute cache options
    * `--attr-cache-timeout=<TIMEOUT IN SECONDS>`: The timeout for the attribute
      cache entries.
    * `--no-symlinks=true`: To improve performance disable symlink support.
- Storage options
    * `--container-name=<CONTAINER NAME>`: The container to mount.
    * `--cancel-list-on-mount-seconds=<TIMEOUT IN SECONDS>`: Time for which list
      calls will be blocked after mount. (prevent billing charges on mounting)
    * `--virtual-directory=true` : Support virtual directories without existence
      of a special marker blob for block blob account (Azure only).
    * `--subdirectory=<path>` : Subdirectory to mount instead of entire
      container.
    * `--disable-compression:false` : Disable content encoding negotiation with
      server. If objects/blobs have 'content-encoding' set to 'gzip' then turn
      on this flag.
    * `--use-adls=false` : Specify configured storage account is HNS enabled or
      not. This must be turned on when HNS enabled account is mounted.
- File cache options
    * `--file-cache-timeout=<TIMEOUT IN SECONDS>`: Timeout for which file is
      cached on local system.
    * `--tmp-path=<PATH>`: The path to the file cache.
    * `--cache-size-mb=<SIZE IN MB>`: Amount of disk cache that can be used by
      cloudfuse.
    * `--high-disk-threshold=<PERCENTAGE>`: If local cache usage exceeds this,
      start early eviction of files from cache.
    * `--low-disk-threshold=<PERCENTAGE>`: If local cache usage comes below this
      threshold then stop early eviction.
    * `--sync-to-flush=false` : Sync call will force upload a file to storage
      container if this is set to true, otherwise it just evicts file from local
      cache.
- Stream options
    * `--block-size-mb=<SIZE IN MB>`: Size of a block to be downloaded during
      streaming.
- Block-Cache options
    * `--block-cache-block-size=<SIZE IN MB>`: Size of a block to be downloaded
      as a unit.
    * `--block-cache-pool-size=<SIZE IN MB>`: Size of pool to be used for
      caching. This limits total memory used by block-cache.
    * `--block-cache-path=<PATH>`: Path where downloaded blocks will be
      persisted. Not providing this parameter will disable the disk caching.
    * `--block-cache-disk-size=<SIZE IN MB>`: Disk space to be used for caching.
    * `--block-cache-prefetch=<Number of blocks>`: Number of blocks to prefetch
      at max when sequential reads are in progress.
    * `--block-cache-prefetch-on-open=true`: Start prefetching on open system
      call instead of waiting for first read. Enhances perf if file is read
      sequentially from offset 0.
- Fuse options
    * `--attr-timeout=<TIMEOUT IN SECONDS>`: Time the kernel can cache inode
      attributes.
    * `--entry-timeout=<TIMEOUT IN SECONDS>`: Time the kernel can cache
      directory listing.
    * `--negative-timeout=<TIMEOUT IN SECONDS>`: Time the kernel can cache
      non-existence of file or directory.
    * `--allow-other`: Allow other users to have access this mount point.
    * `--disable-writeback-cache=true`: Disallow libfuse to buffer write
      requests if you must strictly open files in O_WRONLY or O_APPEND mode.
    * `--ignore-open-flags=true`: Ignore the append and write only flag since
      O_APPEND and O_WRONLY is not supported with writeback caching.

## Frequently Asked Questions

A list of FAQs can be found [here](https://github.com/Seagate/cloudfuse/wiki/Frequently-Asked-Questions)

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
  [me](https://github.com/Azure/azure-storage-fuse/issues/866) for details on
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

## License
This project is licensed under MIT.

## Contributing
This project welcomes contributions and suggestions.

This project is governed by the [code of conduct](CODE_OF_CONDUCT.md). You are
expected to follow this as you contribute to the project. Please report all
unacceptable behavior to
[opensource@seagate.com](mailto:opensource@seagate.com).
