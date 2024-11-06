# Cloudfuse Changelog #

## **1.7.0** ##

November 6th 2024
This version is based on [blobfuse2 2.3.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.2) (upstream).

### Changes ###

- [#358](https://github.com/Seagate/cloudfuse/pull/358) Use Lyve Cloud bucket size by default for StatFs

### Bug Fixes ###

- [#358](https://github.com/Seagate/cloudfuse/pull/358) Fix StatFs used capacity math bug in file cache

## **1.6.1** ##

October 31st 2024
This version is based on [blobfuse2 2.3.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.2) (upstream).

### Bug Fixes ###

- [#354](https://github.com/Seagate/cloudfuse/pull/354) Fix cloudfuse naming in gui to be consistent
- [#352](https://github.com/Seagate/cloudfuse/pull/352) Properly pass settings and default use in get configs when config file does not exist
- [#349](https://github.com/Seagate/cloudfuse/pull/349) Bump pyside6 from 6.8.0 to 6.8.0.2 to fix GUI issues for Ubuntu 24.04
- [#343](https://github.com/Seagate/cloudfuse/pull/343) Change save button behavior in widgets

## **1.6.0** ##

October 22nd 2024
This version is based on [blobfuse2 2.3.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.2) (upstream).

### Changes ###

- [#312](https://github.com/Seagate/cloudfuse/pull/312) Add option to set mount display capacity
- [#342](https://github.com/Seagate/cloudfuse/pull/342) Update default endpoint to LC2 (sv15.lyve) and improve error handling when connecting to Lyve Cloud

### Bug Fixes ###

- [#323](https://github.com/Seagate/cloudfuse/pull/323) Don't evict open files
- [#317](https://github.com/Seagate/cloudfuse/pull/317) When renaming a directory, don't delete the local copy of its contents

## **1.5.0** ##

August 28th 2024
This version is based on [blobfuse2 2.3.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.0) (upstream).

### Changes ###

- [#307](https://github.com/Seagate/cloudfuse/pull/307) Allow all users to write to the mount by default (Windows only)
- [#309](https://github.com/Seagate/cloudfuse/pull/309) Uninstaller now removes WinFSP (Windows only)

## **1.4.0** ##

August 7th 2024
This version is based on [blobfuse2 2.3.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.0) (upstream).

### Changes ###

- [#297](https://github.com/Seagate/cloudfuse/pull/297) Add installation without GUI front-end

## **1.3.2** ##

July 23th 2024
This version is based on [blobfuse2 2.3.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.0) (upstream).

### Bug Fixes ###

- [#294](https://github.com/Seagate/cloudfuse/pull/294) Fix setting system path on Windows install

## **1.3.1** ##

July 11th 2024
This version is based on [blobfuse2 2.3.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.0) (upstream).

### Changes ###

- [#263](https://github.com/Seagate/cloudfuse/pull/263) When mounting to a Windows drive letter with network-share true, set volume name to container name
- [#273](https://github.com/Seagate/cloudfuse/pull/273) Default cloud component to s3storage when config has no components section

## **1.3.0** ##

July 3rd 2024
This version is based on [blobfuse2 2.3.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.0) (upstream).

### Changes ###

- [#219](https://github.com/Seagate/cloudfuse/pull/219) Improve performance with Windows Explorer
- [#212](https://github.com/Seagate/cloudfuse/pull/212) Detect region from endpoint by default
- [#237](https://github.com/Seagate/cloudfuse/pull/237) Allow empty bucket-name and default to the first accessible s3 bucket
- [#238](https://github.com/Seagate/cloudfuse/pull/238) Use base64 encoding for config passphrase

## **1.2.0** ##

May 7th 2024
This version is based on [blobfuse2 2.2.1](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.2.1) (upstream).

### Changes ###

- [#197](https://github.com/Seagate/cloudfuse/pull/197) Disable symlinks by default
- [#188](https://github.com/Seagate/cloudfuse/pull/188) Update file size when writing to a file

### Bug Fixes ###

- [#199](https://github.com/Seagate/cloudfuse/pull/199) Make Cloudfuse CLI run on other flavors and version of Linux - tested on CentOS

## **1.1.3** ##

April 10th 2024
This version is based on [blobfuse2 2.2.1](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.2.1) (upstream).

### Changes ###

- [#159](https://github.com/Seagate/cloudfuse/pull/159) Add instructions to install Cloudfuse as a service on Linux using systemd

### Bug Fixes ###

- [#167](https://github.com/Seagate/cloudfuse/pull/167) GUI: Fix bug where GUI can not find Cloudfuse CLI
- [#181](https://github.com/Seagate/cloudfuse/pull/181) Fix: Renaming a directory leaves behind an empty source directory in file cache

## **1.1.2** ##

March 14th 2024
This version is based on [blobfuse2 2.2.1](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.2.1) (upstream).

### Changes ###

- [#116](https://github.com/Seagate/cloudfuse/pull/116) By default encrypt and decrypt commands now output to the same directory as the supplied config file
- [#122](https://github.com/Seagate/cloudfuse/pull/122) Log file timestamps now include milliseconds
- [#128](https://github.com/Seagate/cloudfuse/pull/128) GUI: Add Cloudfuse version to about page
- cleanup CLI documentation
- update to latest Go dependencies
- update Python dependencies

### Bug Fixes ###

- [#149](https://github.com/Seagate/cloudfuse/pull/149) GUL: Fix dependency-related crash on Linux
- [#150](https://github.com/Seagate/cloudfuse/pull/150) GUL: Fix about page appearance in dark mode
- [#125](https://github.com/Seagate/cloudfuse/pull/125) GUI: Scroll status textbox to latest output
- [#148](https://github.com/Seagate/cloudfuse/pull/148) GUI: Prevent user editing status textbox

## **1.1.1** ##

February 13th 2024
This version is based on [blobfuse2 2.2.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.2.0) (upstream).

### Bug Fixes ###

- fix version output

## **1.1.0** ##

February 12th 2024
This version is based on [blobfuse2 2.2.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.2.0) (upstream).

### Changes ###

- improved performance of directory listing
- merged upstream version 2.2.0

## **1.0.1** ##

January 19th 2024
This version is based on [blobfuse2 2.1.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.1.2) (upstream).

### Bug Fixes ###

- [#102](https://github.com/Seagate/cloudfuse/pull/102) Fix S3 connection error caused by GUI defaulting profile to 'default'
- [#103](https://github.com/Seagate/cloudfuse/pull/103) Improve --dry-run to detect config errors on Windows properly

## **1.0.0** ##

January 18th 2024
This version is based on [blobfuse2 2.1.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.1.2) (upstream).

### Changes ###

- service mount & unmount commands removed (just use mount & unmount)
- mount now runs as a service by default (foreground flag is respected) on Windows
- `mount list` and `unmount all` added to Windows CLI
- GUI now restores most recent mount directory on launch
- sample config files now install to %APPDATA%\Cloudfuse\ on Windows or /usr/share/doc/examples/ on Linux
- config defaults in GUI and samples set for persistent file_cache and improved performance

### Bug Fixes ###

- [#93](https://github.com/Seagate/cloudfuse/pull/93) Respect no-symlinks Flag
- [#97](https://github.com/Seagate/cloudfuse/pull/97) Validate config YAML to prevent GUI issues

## **0.3.0** ##

December 20th 2023
This version is based on [blobfuse2 2.1.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.1.2) (upstream).

### Changes ###

- Windows mount no longer requires admin rights
- Replaced service dedicated to restart mounts on boot up, with a new Windows startup tool that restarts mounts on login.
- Persistent mounts are now stored in AppData on Windows rather than the registry.
- Add --dry-run option
- Bump golang.org/x/crypto from 0.15.0 to 0.17.0

### Bug Fixes ###

- [#58](https://github.com/Seagate/cloudfuse/pull/58) Fix Windows permissions
- [#61](https://github.com/Seagate/cloudfuse/pull/61) Keep window open on failed config write
- [#62](https://github.com/Seagate/cloudfuse/pull/62) Don't delete file cache on unmount when allow-non-empty-temp is set
- [#70](https://github.com/Seagate/cloudfuse/pull/70) Fix window position issue

## **0.2.1** ##

November 30th 2023
This version is based on [blobfuse2 2.1.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.1.2) (upstream).

### Changes ###

- Changed sync-to-flush to true by default.

### Bug Fixes ###

- [#48](https://github.com/Seagate/cloudfuse/pull/48) Prevent "Access Denied" when running as a Windows service

## **0.2.0** ##

November 6th 2023
This version is based on [blobfuse2 2.1.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.1.0) (upstream).

### Features ###

- Include an installer on Windows
- Linux installers now include the GUI and add it to the user applications.
- Unmount now accepts --lazy as an argument to unmount in background

### Bug Fixes ###

- [#29](https://github.com/Seagate/cloudfuse/pull/29) Listing directories on S3 now correctly lists directories up until the maximum configured
- [#28](https://github.com/Seagate/cloudfuse/pull/28)  User-Agent header is now sent on S3 requests

## **0.1.0** ##

October 19th 2023
This release includes all features planned for the 1.0.0 release of cloudfuse, which will be released after additional bug fixes.
This version is based on [blobfuse2 2.1.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.1.0) (upstream).

### Features ###

- Runs on Windows using WinFSP, and runs as a Windows service to provide a persistent mount
- S3 cloud storage is now a supported cloud backend
- GUI for easy configuration on Linux and Windows
