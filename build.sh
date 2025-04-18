#!/bin/bash

LDFLAGS="-s -w"


if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    ARCH=$(uname -m)
    if [[ "$ARCH" == "x86_64" ]]; then
        ZIG_TARGET="x86_64-linux-gnu"
        LIBDIR="/usr/lib/x86_64-linux-gnu"
    elif [[ "$ARCH" == "aarch64" ]] || [[ "$ARCH" == "arm64" ]]; then
        ZIG_TARGET="aarch64-linux-gnu"
        LIBDIR="/usr/lib/aarch64-linux-gnu"
    else
        echo "Unsupported architecture: $ARCH"
        exit 1
    fi

    export CGO_ENABLED=1
    export CGO_LDFLAGS="-L$LIBDIR"
    export CC="zig cc -target $ZIG_TARGET -isystem $LIBDIR -iwithsysroot /usr/include"
    export CXX="zig c++ -target $ZIG_TARGET -isystem $LIBDIR -iwithsysroot /usr/include"
    LDFLAGS="$LDFLAGS -L $LIBDIR"

    if [[ "$1" == "fuse2" ]]; then
        rm -rf cloudfuse
        # Build cloudfuse with fuse2
        go build -o cloudfuse -ldflags="$LDFLAGS" -buildmode=pie -trimpath
    else
        rm -rf cloudfuse
        # Build cloudfuse with fuse3
        go build -o cloudfuse -tags fuse3 -ldflags="$LDFLAGS" -buildmode=pie -trimpath
    fi

    # Build Health Monitor binary
    rm -rf cfusemon
    CGO_ENABLED=0 go build -o cfusemon -ldflags="$LDFLAGS" -buildmode=pie -trimpath ./tools/health-monitor/
else
    rm -rf cfusemon
    go build -o cfusemon.exe -ldflags="$LDFLAGS" -buildmode=pie -trimpath ./tools/health-monitor/

    # Build Windows Startup Tool
    rm -rf windows-startup
    go build -o windows-startup.exe -ldflags="$LDFLAGS" -buildmode=pie -trimpath ./tools/windows-startup/

    # Build Windows Service Tool
    rm -rf windows-service
    go build -o windows-service.exe -ldflags="$LDFLAGS" -buildmode=pie -trimpath ./tools/windows-service/

    # Build cloudfuse
    rm -rf cloudfuse.exe
    go build -o cloudfuse.exe -ldflags="$LDFLAGS" -buildmode=pie -trimpath
fi
