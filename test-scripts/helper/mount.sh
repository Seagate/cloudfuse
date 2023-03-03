#!/bin/bash

#let script exit if an unsed variable is used
set -o nounset

# Load variables
source ./helper/var.env

# Cleanup
source ./helper/cleanup.sh
exit_code=$?
if [ $exit_code -ne 0 ]; then
    echo "command failed with exit code ${exit_code}"
    echo "Stopping script"
    exit $exit_code
fi

# Mount step
echo "Mounting into mount directory"
$WORK_DIR/lyvecloudfuse mount $MOUNT_DIR --config-file=$WORK_DIR/config.yaml

sleep 5s

# Check for mount
echo "Checking for mount"
sudo ps -aux | grep lyvecloudfuse

# Delete the files in mount directory for test
echo "Deleting files from mount directory with container mounted"
#rm -rf $MOUNT_DIR/*

#df | grep lyvecloudfuse
#exit $?
