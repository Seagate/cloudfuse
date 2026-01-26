# Cloudfuse Changelog #

## **2.0.3** ##

January 23rd 2026
This version is based on [blobfuse2 2.5.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.5.0) (upstream).

### Bug Fixes ###

- [#801](https://github.com/Seagate/cloudfuse/pull/801) Fix libfuse3 dependency for Debian Trixie

## **2.0.2** ##

December 16th 2025
This version is based on [blobfuse2 2.5.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.5.0) (upstream).

### Bug Fixes ###

- [#767](https://github.com/Seagate/cloudfuse/pull/767) Improve size tracker concurrency and accuracy

## **2.0.1** ##

October 23rd 2025
This version is based on [blobfuse2 2.5.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.5.0) (upstream).

### Features ###

- [#681](https://github.com/Seagate/cloudfuse/pull/681) Bump blobfuse version to 2.5.0

### Bug Fixes ###

- [#691](https://github.com/Seagate/cloudfuse/pull/691) Add libfuse as explicit dependency
- [#729](https://github.com/Seagate/cloudfuse/pull/729) Fix size_tracker bug

## **2.0.0** ##

September 4th 2025
This version is based on [blobfuse2 2.4.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.4.2) (upstream).

### Highlights

- New terminal-based configuration UI (TUI): `cloudfuse config`
- New `update` command so cloudfuse can update itself
- New `gather-logs` command to easily collect all cloudfuse logs and generate a zip file
- Support for FUSE3 on Linux bringing faster directory listing and other performance improvements
- Faster file cache performance on Linux
- APT and RPM repositories

### Breaking changes

- GUI removed from this repo
  The GUI is no longer bundled. Install it separately: <https://github.com/Seagate/cloudfuse-gui>
  Use the new TUI with `cloudfuse config` for in-terminal setup.

- CLI cleanup and removals
  Some v1 command-line options were removed. All removed options can still be configured in the `config.yaml`.
  Refer to the docs for the current CLI options.

- Passphrase handling simplified
  Base64 encoding of the passphrase is no longer required. Existing base64-encoded values continue to work.

### Features ###

- [#481](https://github.com/Seagate/cloudfuse/pull/481) Add update command
- [#599](https://github.com/Seagate/cloudfuse/pull/599) Add log collector
- [#641](https://github.com/Seagate/cloudfuse/pull/641) Add TUI interface
- [#657](https://github.com/Seagate/cloudfuse/pull/657) Improve file_cache performance on linux
- [#631](https://github.com/Seagate/cloudfuse/pull/631) Bump blobfuse version to 2.4.2
- [#500](https://github.com/Seagate/cloudfuse/pull/500) Add builds for fuse3
- [#591](https://github.com/Seagate/cloudfuse/pull/591) Add rpm and apt repository
- [#499](https://github.com/Seagate/cloudfuse/pull/499) Remove GUI
- [#624](https://github.com/Seagate/cloudfuse/pull/624) Remove V1 Command Line Options
- [#650](https://github.com/Seagate/cloudfuse/pull/650) Remove requirement for base64 encoded passphrase
- [#633](https://github.com/Seagate/cloudfuse/pull/633) Add sync command for size_tracker

### Bug Fixes ###

- [#661](https://github.com/Seagate/cloudfuse/pull/661) Don't mount if enable remount is set and mount failed
- [#642](https://github.com/Seagate/cloudfuse/pull/642) Fix issue with block cache caching files in directories

## **1.12.2** ##

August 22nd 2025
This version is based on [blobfuse2 2.4.1](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.4.1) (upstream).

### Features ###

- [#527](https://github.com/Seagate/cloudfuse/pull/527) Bump blobfuse version to 2.4.1

### Bug Fixes ###

- [#642](https://github.com/Seagate/cloudfuse/pull/642) Fix issue with files in directory not being cached when using block_cache
- [#648](https://github.com/Seagate/cloudfuse/pull/648) Stop linking against development dependencies of fuse libraries

## **1.12.1** ##

July 18th 2025
This version is based on [blobfuse2 2.4.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.4.0) (upstream).

### Bug Fixes ###

- [#601](https://github.com/Seagate/cloudfuse/pull/601) Allow use of persistent cache across reboots
- [#598](https://github.com/Seagate/cloudfuse/pull/598) If no bucket name specified, default to first accessible bucket
- [#583](https://github.com/Seagate/cloudfuse/pull/583) Move to latest version of WinFSP

## **1.12.0** ##

June 6th 2025
This version is based on [blobfuse2 2.4.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.4.0) (upstream).

### Bug Fixes ###

- [#539](https://github.com/Seagate/cloudfuse/pull/539) Fix issues with block cache and s3
- [#566](https://github.com/Seagate/cloudfuse/pull/566) Fix race conditions with block cache on Windows
- [#562](https://github.com/Seagate/cloudfuse/pull/562) A default file cache path is now set if not in config file

### Features ###

- [#565](https://github.com/Seagate/cloudfuse/pull/565) Data cached in the file cache is now persisted on remounts

## **1.11.2** ##

May 15th 2025
This version is based on [blobfuse2 2.4.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.4.0) (upstream).

### Bug Fixes ###

- [#553](https://github.com/Seagate/cloudfuse/pull/553) Fix prompt to install WinFSP when already installed
- [#542](https://github.com/Seagate/cloudfuse/pull/542) Improve rename file and prevent data loss in rare cases

### Features ###

- [#554](https://github.com/Seagate/cloudfuse/pull/554) Checksum on release is now signed

## **1.11.1** ##

April 29th 2025
This version is based on [blobfuse2 2.4.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.4.0) (upstream).

### Bug Fixes ###

- [#532](https://github.com/Seagate/cloudfuse/pull/532) Fix releases for RHEL that did not correctly link to glibc

## **1.11.0** ##

April 25th 2025
This version is based on [blobfuse2 2.4.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.4.0) (upstream).

### Features ###

- [#494](https://github.com/Seagate/cloudfuse/pull/494) Adds initial support for block cache on Windows
- [#507](https://github.com/Seagate/cloudfuse/pull/507) Change default display capacity and other defaults to more
sensible values
- [#478](https://github.com/Seagate/cloudfuse/pull/478) Improve checksum support when using s3storage

### Bug Fixes ###

- [#520](https://github.com/Seagate/cloudfuse/pull/520) Unmount on Windows is no longer case sensitive
- [#513](https://github.com/Seagate/cloudfuse/pull/513) Change default log level of GUI

## **1.10.0** ##

March 28th 2025
This version is based on [blobfuse2 2.3.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.2) (upstream).

### Features ###

- [#490](https://github.com/Seagate/cloudfuse/pull/490) Adds new mount flags 'enable-remount-system' and 'enable-remount-user' to enable remount on system startup or user login
along with corresponding unmount flags 'disable-remount-system' and 'disable-remount-user'

## **1.9.3** ##

March 25th 2025
This version is based on [blobfuse2 2.3.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.2) (upstream).

### Bug Fixes ###

- [#484](https://github.com/Seagate/cloudfuse/pull/484) Use registry for a more reliable remount on startup

## **1.9.2** ##

March 12th 2025
This version is based on [blobfuse2 2.3.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.2) (upstream).

### Bug Fixes ###

- [#475](https://github.com/Seagate/cloudfuse/pull/475) Fix bug where cloudfuse mounts with secure config files on Windows without foreground did not start correctly

## **1.9.1** ##

March 6th 2025
This version is based on [blobfuse2 2.3.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.2) (upstream).

### Bug Fixes ###

- [#471](https://github.com/Seagate/cloudfuse/pull/471) Fix bug where passphrase for secure encryption was not properly decoded as base64

## **1.9.0** ##

March 4th 2025
This version is based on [blobfuse2 2.3.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.2) (upstream).

### Changes ###

- [#469](https://github.com/Seagate/cloudfuse/pull/469) Add enable-remount and disable-remount flags to CLI for Windows to better enable customizability on which mounts should remount on restart
- [#467](https://github.com/Seagate/cloudfuse/pull/467) Fixed bug with creation of windows startup utility

## **1.8.2** ##

March 3rd 2025
This version is based on [blobfuse2 2.3.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.2) (upstream).

### Changes ###

- [#445](https://github.com/Seagate/cloudfuse/pull/462) Support custom SDDL strings on Windows to customize mount permissions
- [#464](https://github.com/Seagate/cloudfuse/pull/464) No_gui installer now is able to restart mounts on restart if the user installs cloudfuse as a service
- [#445](https://github.com/Seagate/cloudfuse/pull/450) GUI is able to mount as a drive letter on Windows

## **1.8.1** ##

February 20th 2025
This version is based on [blobfuse2 2.3.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.2) (upstream).

### Changes ###

- [#286](https://github.com/Seagate/cloudfuse/pull/286) Prevent heap inspection of stored secrets
- [#445](https://github.com/Seagate/cloudfuse/pull/445) Cleanup and remove unused debug info in GUI
- [#338](https://github.com/Seagate/cloudfuse/pull/338) The service command now works on Linux to install cloudfuse as a service

### Bug Fixes ###

- [#444](https://github.com/Seagate/cloudfuse/pull/444) Fix issue when renaming directories
- [#455](https://github.com/Seagate/cloudfuse/pull/455) Network share on Windows now correctly uses the hostname

## **1.8.0** ##

February 4th 2025
This version is based on [blobfuse2 2.3.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.2) (upstream).

### Changes ###

- [#423](https://github.com/Seagate/cloudfuse/pull/423) Add a new component size_track which tracks the total number of bytes in the cloud
- [#338](https://github.com/Seagate/cloudfuse/pull/338) The service command now works on Linux to install cloudfuse as a service

## **1.7.4** ##

January 15th 2025
This version is based on [blobfuse2 2.3.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.2) (upstream).

### Bug Fixes ###

- [#374](https://github.com/Seagate/cloudfuse/pull/374) Fix issue with items remaining in cache and remove race conditions

## **1.7.3** ##

January 13th 2025
This version is based on [blobfuse2 2.3.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.2) (upstream).

### Bug Fixes ###

- [#404](https://github.com/Seagate/cloudfuse/pull/404) Fix error with open file handles when renaming files
- [#397](https://github.com/Seagate/cloudfuse/pull/370) Add man pages and follow best practices for RHEL and Debian builds

## **1.7.2** ##

December 11th 2024
This version is based on [blobfuse2 2.3.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.2) (upstream).

### Bug Fixes ###

- [#379](https://github.com/Seagate/cloudfuse/pull/379) Fix race conditions in file cache
- [#370](https://github.com/Seagate/cloudfuse/pull/370) Fix writing file with append flag

## **1.7.1** ##

November 8th 2024
This version is based on [blobfuse2 2.3.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.3.2) (upstream).

### Bug Fixes ###

- [#364](https://github.com/Seagate/cloudfuse/pull/364) Fix daemon issue on builds with golang 1.23.2

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
