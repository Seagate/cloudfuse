#!/bin/bash

#let script exit if an unsed variable is used
set -o nounset

# Cleanup
source ./helper/cleanup.sh
if $?; then
    echo $?
    echo "Stopping script"
    exit $?
fi

source ./helper/env_variatbles.sh
if $?; then
    echo $?
    echo "Stopping script"
    exit $?
fi

# Mount step
echo "Mounting into mount directory"
$work_dir/lyvecloudfuse mount $mount_dir --config-file=$work_dir/config.yaml

sleep 5s

# Check for mount
echo "Checking for mount"
sudo ps -aux | grep lyvecloudfuse

# Delete the files in mount directory for test
echo "Deleting files from mount directory with container mounted"
#rm -rf $mount_dir/*

#df | grep lyvecloudfuse
#exit $?
