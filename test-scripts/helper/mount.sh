#!/bin/bash

# Let script exit if an unused variable is used
set -o nounset

# Load variables
source ./helper/var.env

# Cleanup
if ! source ./helper/cleanup.sh; then
    ret=$?
    echo "command failed with exit code $ret"
    echo "Stopping script"
    exit $ret
fi

# Mount step
echo "Mounting into mount directory"
"$WORK_DIR"/cloudfuse mount "$MOUNT_DIR" --config-file="$WORK_DIR"/config.yaml

sleep 5s

# Check for mount
echo "Checking for mount"
sudo ps -aux | grep cloudfuse
