#!/bin/bash

# Let script exit if an unsed variable is used
set -o nounset

# Load variables
source ./helper/var.env

# Cleanup test
source ./helper/cleanup.sh
if [ $? -ne 0 ]; then
    echo "command failed with exit code $?"
    echo "Stopping script"
    exit $?
fi

echo "-------------------------------------------------------------------"
echo "Starting Mount Test"

go test -timeout=120m -p 1 -v ../test/mount_test/mount_test.go -args -working-dir=$WORK_DIR -mnt-path=$MOUNT_DIR -config-file=$WORK_DIR/config.yaml

# Cleanup test
source ./helper/cleanup.sh
