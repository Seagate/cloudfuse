# Cloudfuse Changelog #

## 0.1.0 ##

This release includes all features planned for the 1.0.0 release of cloudfuse, which will be released after additional bug fixes.
This version is based on [blobfuse2 2.1.0](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.1.0) (upstream).
**Features**
Added support for running on Windows, S3 cloud storage, and adds a GUI for easy configuration.
On Windows, cloudfuse uses WinFSP. With WinFSP installed, users can use the cloudfuse service to add persistent cloud storage mounts using mount directories or drive letters.
