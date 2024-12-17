#!/bin/bash

# Let script exit if an unused variable is used
set -o nounset

# Load variables
source ./helper/var.env

# Cleanup test
if ! source ./helper/cleanup.sh; then
    ret=$?
    echo "command failed with exit code $ret"
    echo "Stopping script"
    exit $ret
fi

echo "-------------------------------------------------------------------"
echo "Starting Mount Test"

go test -timeout=120m -p 1 -v ../test/mount_test/mount_test.go -args -working-dir="$WORK_DIR" -mnt-path="$MOUNT_DIR" -config-file="$WORK_DIR"/config.yaml

# Cleanup test
source ./helper/cleanup.sh
