#!/bin/bash
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    rm -rf cloudfuse
    # Build cloudfuse with fuse3
    go build -o cloudfuse

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
