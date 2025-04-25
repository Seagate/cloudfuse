#!/bin/bash
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    if [[ "$1" == "fuse2" ]]; then
        rm -rf cloudfuse
        # Build cloudfuse with fuse2
        go build -o cloudfuse
    else
        rm -rf cloudfuse
        # Build cloudfuse with fuse3
        go build -o cloudfuse -tags fuse3
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

    # Build Windows Service Tool
    rm -rf windows-service
    go build -o windows-service.exe ./tools/windows-service/

    # Build cloudfuse
    rm -rf cloudfuse.exe
    go build -o cloudfuse.exe
fi
