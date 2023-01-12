#!/bin/bash

if [ "$1" == "fuse2" ]
then
    # Build lyvecloudfuse with fuse2
    rm -rf lyvecloudfuse
    rm -rf azure-storage-fuse
    go build -tags fuse2 -o lyvecloudfuse
elif [ "$1" == "health" ]
then
    # Build Health Monitor binary
    rm -rf bfusemon
    go build -o bfusemon ./tools/health-monitor/
else
    # Build lyvecloudfuse with fuse3
    rm -rf lyvecloudfuse
    rm -rf azure-storage-fuse
    go build -o lyvecloudfuse
fi