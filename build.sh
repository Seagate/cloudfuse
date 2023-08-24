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

    # Build cloudfuse
    rm -rf cloudfuse.exe
    go build -o cloudfuse.exe
fi
