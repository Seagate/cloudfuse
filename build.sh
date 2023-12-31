#!/bin/bash
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # Build Health Monitor binary
    rm -rf cfusemon
    go build -o cfusemon ./tools/health-monitor/

    # Build cloudfuse
    rm -rf cloudfuse
    go build -o cloudfuse
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
