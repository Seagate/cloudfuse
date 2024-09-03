#!/bin/bash
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    rm -rf cloudfuse
    if [ "$1" == "fuse2" ]
    then
        # Build cloudfuse with fuse2
        go build -tags fuse2 -o cloudfuse
    else
        # Build cloudfuse with fuse3
        go build -o cloudfuse
    fi

    # Build Health Monitor binary
    rm -rf cfusemon
    go build -o cfusemon ./tools/health-monitor/
else
    rm -rf cfusemon
    go build -o cfusemon.exe ./tools/health-monitor/

    # Build Windows Startup Tool
    rm -rf windows-startup
    go build -o windows-startup.exe ./tools/windows-startup/

    # Build cloudfuse
    rm -rf cloudfuse.exe
    go build -o cloudfuse.exe
fi
