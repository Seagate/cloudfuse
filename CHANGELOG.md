# Cloudfuse Changelog #

## 1.1.2 ##

This version is based on [blobfuse2 2.2.1](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.2.1) (upstream).
**Changes**
-- [#122](https://github.com/Seagate/cloudfuse/pull/122) Log file timestamps now include milliseconds
-- cleanup CLI documentation
-- update to latest go dependencies
**Bug Fixes**
-- [#125](https://github.com/Seagate/cloudfuse/pull/125) Fix scrolling of text window on GUI when new info appears

## 1.1.1 ##

This version is based on [blobfuse2 2.2.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.2.0) (upstream).
**Bug Fixes**
-- fix version output

## 1.1.0 ##

This version is based on [blobfuse2 2.2.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.2.0) (upstream).
**Changes**
-- improved performance of directory listing
-- merged upstream version 2.2.0


## 1.0.1 ##

This version is based on [blobfuse2 2.1.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.1.2) (upstream).
**Bug Fixes**
-- [#102](https://github.com/Seagate/cloudfuse/pull/102) Fix S3 connection error caused by GUI defaulting profile to 'default'
-- [#103](https://github.com/Seagate/cloudfuse/pull/103) Improve --dry-run to detect config errors on Windows properly

## 1.0.0 ##

This version is based on [blobfuse2 2.1.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.1.2) (upstream).
**Changes**
-- service mount & unmount commands removed (just use mount & unmount)
-- mount now runs as a service by default (foreground flag is respected) on Windows
-- `mount list` and `unmount all` added to Windows CLI
-- GUI now restores most recent mount directory on launch
-- sample config files now install to %APPDATA%\Cloudfuse\ on Windows or /usr/share/doc/examples/ on Linux
-- config defaults in GUI and samples set for persistent file_cache and improved performance
**Bug Fixes**
-- [#93](https://github.com/Seagate/cloudfuse/pull/93) Respect no-symlinks Flag
-- [#97](https://github.com/Seagate/cloudfuse/pull/97) Validate config YAML to prevent GUI issues

## 0.3.0 ##

This version is based on [blobfuse2 2.1.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.1.2) (upstream).
**Changes**
-- Windows mount no longer requires admin rights
-- Replaced service dedicated to restart mounts on bootup, with a new Windows startup tool that restarts mounts on login.
-- Persistent mounts are now stored in AppData on Windows rather than the registry.
-- Add --dry-run option
-- Bump golang.org/x/crypto from 0.15.0 to 0.17.0
**Bug Fixes**
-- [#58](https://github.com/Seagate/cloudfuse/pull/58) Fix Windows permissions
-- [#61](https://github.com/Seagate/cloudfuse/pull/61) Keep window open on failed config write
-- [#62](https://github.com/Seagate/cloudfuse/pull/62) Don't delete file cache on unmount when allow-non-empty-temp is set
-- [#70](https://github.com/Seagate/cloudfuse/pull/70) Fix window position issue

## 0.2.1 ##

This version is based on [blobfuse2 2.1.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.1.2) (upstream).
**Changes**
-- Changed sync-to-flush to true by default.
**Bug Fixes**
-- [#48](https://github.com/Seagate/cloudfuse/pull/48) Prevent "Access Denied" when running as a Windows service

## 0.2.0 ##

**Features**
-- Include an installer on Windows
-- Linux installers now include the GUI and add it to the user applications.
-- Unmount now accepts --lazy as an argument to unmount in background
**Bug Fixes**
-- [#29](https://github.com/Seagate/cloudfuse/pull/29) Listing directories on S3 now correctly lists directories up until the maximum configured
-- [#28](https://github.com/Seagate/cloudfuse/pull/28)  User-Agent header is now sent on S3 requests

## 0.1.0 ##

This release includes all features planned for the 1.0.0 release of cloudfuse, which will be released after additional bug fixes.
This version is based on [blobfuse2 2.1.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.1.0) (upstream).
**Features**
-- Runs on Windows using WinFSP, and runs as a Windows service to provide a persistent mount
-- S3 cloud storage is now a supported cloud backend
-- GUI for easy configuration on Linux and Windows
