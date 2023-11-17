# Cloudfuse Changelog #

## 0.2.1 (WIP) ##

This version is based on [blobfuse2 2.1.2](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.1.2) (upstream).
**Changes**
Changed sync-to-flush to true by default.

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
