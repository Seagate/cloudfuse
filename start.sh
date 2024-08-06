#!/bin/bash
./cloudfuse unmount all && ./build.sh && ./cloudfuse mount ~/mycontainer --config-file=./config.yaml