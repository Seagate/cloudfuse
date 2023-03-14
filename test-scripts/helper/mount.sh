#!/bin/bash

# Let script exit if an unused variable is used
set -o nounset

# Load variables
source ./helper/var.env

# Cleanup
source ./helper/cleanup.sh
if [ $? -ne 0 ]; then
    echo "command failed with exit code $?"
    echo "Stopping script"
    exit $?
fi

# Mount step
echo "Mounting into mount directory"
$WORK_DIR/lyvecloudfuse mount $MOUNT_DIR --config-file=$WORK_DIR/config.yaml

sleep 5s

# Check for mount
echo "Checking for mount"
sudo ps -aux | grep lyvecloudfuse
