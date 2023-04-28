#!/bin/bash
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # Build Health Monitor binary
    rm -rf bfusemon
    go build -o bfusemon ./tools/health-monitor/

    # Build lyvecloudfuse
    rm -rf lyvecloudfuse
    go build -o lyvecloudfuse
else
    rm -rf bfusemon
    go build -o bfusemon.exe ./tools/health-monitor/

    # Build lyvecloudfuse
    rm -rf lyvecloudfuse.exe
    go build -o lyvecloudfuse.exe
fi
