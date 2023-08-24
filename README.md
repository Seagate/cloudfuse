# Cloudfuse - An S3 and Azure Storage FUSE driver
## About
Cloudfuse is a fork of the open source project [blobfuse2](https://github.com/Azure/azure-storage-fuse) from Microsoft that adds support for S3 storage, a GUI for configuration and mounting, and Windows support. It provides a virtual filesystem backed by either S3 or Azure Storage. It uses the libfuse open source library (fuse) to communicate with the Linux FUSE kernel module and uses WinFSP to support running on Windows. It implements the filesystem operations using the S3 and Azure Storage REST APIs.

<!---TODO Add our github link for issues--->
Cloudfuse is stable, provided that it is used within its limits documented here. Cloudfuse supports both reads and writes however, it does not guarantee continuous sync of data written to storage using other APIs or other mounts of Cloudfuse. For data integrity it is recommended that multiple sources do not modify the same blob/object/file. Please submit an issue [here]() for any issues/feature requests/questions.

## Features
- Mount an S3 bucket or Azure storage container or datalake file system on Linux and Windows.
- Basic file system operations such as mkdir, opendir, readdir, rmdir, open, 
   read, create, write, close, unlink, truncate, stat, rename
- Local caching to improve subsequent access times
- Streaming to support reading AND writing large files 
- Parallel downloads and uploads to improve access time for large files
- Multiple mounts to the same container for read-only workloads

## Health Monitor
Cloudfuse also supports a health monitor. It allows customers gain more insight into how their Cloudfuse instance is behaving with the rest of their machine. Visit [here](tools/health-monitor/README.md) to set it up.

## Features compared to blobfuse2
- Supports any S3 compatable storage
- Adds a GUI to configure and start mounts
- Runs on Windows using WinFSP in foreground or as a Windows service

## Download Cloudfuse
You can install Cloudfuse by cloning this repository. In the workspace execute the build script `./build.sh` to build the binary. 

### Linux
Cloudfuse currently only supports libfuse2. On Linux, you need to install the libfuse2 package, for example on Ubuntu:
    
    sudo apt install libfuse2

### Windows
On Windows, you also need to install the third party utility [WinFsp](https://winfsp.dev/). To download WinFsp, please see
run the WinFsp installer found [here](https://winfsp.dev/rel/).


<!-- ## Find Help
For complete guidance, visit any of these articles
* Blobfuse2 Wiki -->

## Supported Operations
The general format of the Cloudfuse commands is `cloudfuse [command] [arguments] --[flag-name]=[flag-value]`
* `help` - Help about any command
* `mount` - Mounts an Azure container as a filesystem. The supported containers include
  - S3 Bucket
  - Azure Blob Container
  - Azure Datalake Gen2 Container
* `mount all` - Mounts all the containers in an Azure account as a filesystem. The supported storage services include
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

## Find help from your command prompt
To see a list of commands, type `cloudfuse -h` and then press the ENTER key.
To learn about a specific command, just include the name of the command (For example: `cloudfuse mount -h`).

## Usage
- Mount with cloudfuse
    * cloudfuse mount <mount path> --config-file=<config file>
- Mount all containers in your storage account
    * cloudfuse mount all <mount path> --config-file=<config file>
- List all mount instances of cloudfuse
    * cloudfuse mount list
- Unmount cloudfuse on Linux
    * sudo fusermount3 -u <mount path>
- Unmount all cloudfuse instances on Linux
    * cloudfuse unmount all 

<!---TODO Add Usage for mount, unmount, etc--->
## CLI parameters
- Note: Blobfuse2 accepts all CLI parameters that Blobfuse does, but may ignore parameters that are no longer applicable. 
- General options
    * `--config-file=<PATH>`: The path to the config file.
    * `--log-level=<LOG_*>`: The level of logs to capture.
    * `--log-file-path=<PATH>`: The path for the log file.
    * `--foreground=true`: Mounts the system in foreground mode.
    * `--read-only=true`: Mount container in read-only mode.
    * `--default-working-dir`: The default working directory to store log files and other blobfuse2 related information.
    * `--disable-version-check=true`: Disable the blobfuse2 version check.
    * `--secure-config=true` : Config file is encrypted suing 'blobfuse2 secure` command.
    * `--passphrase=<STRING>` : Passphrase used to encrypt/decrypt config file.
    * `--wait-for-mount=<TIMEOUT IN SECONDS>` : Let parent process wait for given timeout before exit to ensure child has started. 
- Attribute cache options
    * `--attr-cache-timeout=<TIMEOUT IN SECONDS>`: The timeout for the attribute cache entries.
    * `--no-symlinks=true`: To improve performance disable symlink support.
- Storage options
    * `--container-name=<CONTAINER NAME>`: The container to mount.
    * `--cancel-list-on-mount-seconds=<TIMEOUT IN SECONDS>`: Time for which list calls will be blocked after mount. ( prevent billing charges on mounting)
    * `--virtual-directory=true` : Support virtual directories without existence of a special marker blob for block blob account.
    * `--subdirectory=<path>` : Subdirectory to mount instead of entire container.
    * `--disable-compression:false` : Disable content encoding negotiation with server. If blobs have 'content-encoding' set to 'gzip' then turn on this flag.
    * `--use-adls=false` : Specify configured storage account is HNS enabled or not. This must be turned on when HNS enabled account is mounted.
- File cache options
    * `--file-cache-timeout=<TIMEOUT IN SECONDS>`: Timeout for which file is cached on local system.
    * `--tmp-path=<PATH>`: The path to the file cache.
    * `--cache-size-mb=<SIZE IN MB>`: Amount of disk cache that can be used by blobfuse.
    * `--high-disk-threshold=<PERCENTAGE>`: If local cache usage exceeds this, start early eviction of files from cache.
    * `--low-disk-threshold=<PERCENTAGE>`: If local cache usage comes below this threshold then stop early eviction.
    * `--sync-to-flush=false` : Sync call will force upload a file to storage container if this is set to true, otherwise it just evicts file from local cache.
- Stream options
    * `--block-size-mb=<SIZE IN MB>`: Size of a block to be downloaded during streaming.
- Fuse options
    * `--attr-timeout=<TIMEOUT IN SECONDS>`: Time the kernel can cache inode attributes.
    * `--entry-timeout=<TIMEOUT IN SECONDS>`: Time the kernel can cache directory listing.
    * `--negative-timeout=<TIMEOUT IN SECONDS>`: Time the kernel can cache non-existance of file or directory.
    * `--allow-other`: Allow other users to have access this mount point.
    * `--disable-writeback-cache=true`: Disallow libfuse to buffer write requests if you must strictly open files in O_WRONLY or O_APPEND mode.
    * `--ignore-open-flags=true`: Ignore the append and write only flag since O_APPEND and O_WRONLY is not supported with writeback caching.


## Environment variables
- General options
    * `AZURE_STORAGE_ACCOUNT`: Specifies the storage account to be connected.
    * `AZURE_STORAGE_ACCOUNT_TYPE`: Specifies the account type 'block' or 'adls'
    * `AZURE_STORAGE_ACCOUNT_CONTAINER`: Specifies the name of the container to be mounted
    * `AZURE_STORAGE_BLOB_ENDPOINT`: Specifies the blob endpoint to use. Defaults to *.blob.core.windows.net, but is useful for targeting storage emulators.
    * `AZURE_STORAGE_AUTH_TYPE`: Overrides the currently specified auth type. Case insensitive. Options: Key, SAS, MSI, SPN
- Account key auth:
    * `AZURE_STORAGE_ACCESS_KEY`: Specifies the storage account key to use for authentication.
- SAS token auth:
    * `AZURE_STORAGE_SAS_TOKEN`: Specifies the SAS token to use for authentication.
- Managed Identity auth:
    * `AZURE_STORAGE_IDENTITY_CLIENT_ID`: Only one of these three parameters are needed if multiple identities are present on the system.
    * `AZURE_STORAGE_IDENTITY_OBJECT_ID`: Only one of these three parameters are needed if multiple identities are present on the system.
    * `AZURE_STORAGE_IDENTITY_RESOURCE_ID`: Only one of these three parameters are needed if multiple identities are present on the system.
    * `MSI_ENDPOINT`: Specifies a custom managed identity endpoint, as IMDS may not be available under some scenarios. Uses the `MSI_SECRET` parameter as the `Secret` header.
    * `MSI_SECRET`: Specifies a custom secret for an alternate managed identity endpoint.
- Service Principal Name auth:
    * `AZURE_STORAGE_SPN_CLIENT_ID`: Specifies the client ID for your application registration
    * `AZURE_STORAGE_SPN_TENANT_ID`: Specifies the tenant ID for your application registration
    * `AZURE_STORAGE_AAD_ENDPOINT`: Specifies a custom AAD endpoint to authenticate against
    * `AZURE_STORAGE_SPN_CLIENT_SECRET`: Specifies the client secret for your application registration.
    * `AZURE_STORAGE_AUTH_RESOURCE` : Scope to be used while requesting for token.
- Proxy Server:
    * `http_proxy`: The proxy server address. Example: `10.1.22.4:8080`.    
    * `https_proxy`: The proxy server address when https is turned off forcing http. Example: `10.1.22.4:8080`.

## Config file
- See [this](./sampleFileCacheConfig.yaml) sample config file.
- See [this](./setup/baseConfig.yaml) config file for a list and description of all possible configurable options in cloudfuse. 

***Please note: do not use quotations `""` for any of the config parameters***

## Frequently Asked Questions
- How do I generate a SAS with permissions for rename?
az cli has a command to generate a sas token. Open a command prompt and make sure you are logged in to az cli. Run the following command and the sas token will be displayed in the command prompt.
az storage container generate-sas --account-name <account name ex:myadlsaccount> --account-key <accountKey> -n <container name> --permissions dlrwac --start <today's date ex: 2021-03-26> --expiry <date greater than the current time ex:2021-03-28>
- Why do I get EINVAL on opening a file with WRONLY or APPEND flags?
To improve performance, Cloudfuse by default enables writeback caching, which can produce unexpected behavior for files opened with WRONLY or APPEND flags, so Cloudfuse returns EINVAL on open of a file with those flags. Either use disable-writeback-caching to turn off writeback caching (can potentially result in degraded performance) or ignore-open-flags (replace WRONLY with RDWR and ignore APPEND) based on your workload. 
- How to mount Cloudfuse inside a container?
Refer to 'docker' folder in this repo. It contains a sample 'Dockerfile'. If you wish to create your own container image, try 'buildandruncontainer.sh' script, it will create a container image and launch the container using current environment variables holding your storage account credentials.
- Why am I not able to see the updated contents of file(s), which were updated through means other than Cloudfuse mount?
If your use-case involves updating/uploading file(s) through other means and you wish to see the updated contents on Cloudfuse mount then you need to disable kernel page-cache. `-o direct_io` CLI parameter is the option you need to use while mounting. Along with this, set `file-cache-timeout=0` and all other libfuse caching parameters should also be set to 0. User shall be aware that disabling kernel cache can result into more calls to S3 or Azure Storage which will have cost and performance implications. 

## Un-Supported File system operations
- mkfifo : fifo creation is not supported by cloudfuse and this will result in "function not implemented" error
- chown  : Change of ownership is not supported by Azure Storage hence Cloudfuse does not support this.
- Creation of device files or pipes is not supported by Cloudfuse.
- Cloudfuse does not support extended-attributes (x-attrs) operations

## Un-Supported Scenarios
- Cloudfuse does not support overlapping mount paths. While running multiple instances of Cloudfuse make sure each instance has a unique and non-overlapping mount point.
- Cloudfuse does not support co-existance with NFS on same mount path. Behaviour in this case is undefined.
- For Azure block blob accounts, where data is uploaded through other means, Cloudfuse expects special directory marker files to exist in container. In absence of this
  few file operations might not work. For e.g. if you have a blob 'A/B/c.txt' then special marker files shall exists for 'A' and 'A/B', otherwise opening of 'A/B/c.txt' will fail.
  Once a 'ls' operation is done on these directories 'A' and 'A/B' you will be able to open 'A/B/c.txt' as well. Possible workaround to resolve this from your container is to either

  create the directory marker files manually through portal or run 'mkdir' command for 'A' and 'A/B' from cloudfuse. Refer [me](https://github.com/Azure/azure-storage-fuse/issues/866) 
  for details on this.

## Limitations
- In case of Azure BlockBlob accounts, ACLs are not supported by Azure Storage so Cloudfuse will by default return success for 'chmod' operation. However it will work fine for Gen2 (DataLake) accounts.
- When Cloudfuse is mounted on a container, SYS_ADMIN privileges are required for it to interact with the fuse driver. If container is created without the privilege, mount will fail. Sample command to spawn a docker container is 

    `docker run -it --rm --cap-add=SYS_ADMIN --device=/dev/fuse --security-opt apparmor:unconfined <environment variables> <docker image>`
        
### Syslog security warning
By default, Cloudfuse will log to syslog. The default settings will, in some cases, log relevant file paths to syslog. 
If this is sensitive information, turn off logging or set log-level to LOG_ERR.  


## License
This project is licensed under MIT.
 
## Contributing
This project welcomes contributions and suggestions.

This project is governed by the [code of conduct](CODE_OF_CONDUCT.md). You are expected to follow this as you contribute to the project. Please report all unacceptable behavior to [opensource@seagate.com](mailto:opensource@seagate.com).