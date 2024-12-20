#!/bin/bash

# Let script exit if an unused variable is used
set -o nounset

# Load variables
source ./helper/var.env

# Mount the directory
if ! source ./helper/mount.sh; then
    ret=$?
    echo "command failed with exit code $ret"
    echo "Stopping script"
    exit $ret
fi

# Run e2e tests
echo "-------------------------------------------------------------------"
echo "Starting Stress Test"

# run test
go test -timeout=120m -v ../test/stress_test/stress_test.go -args -mnt-path="$MOUNT_DIR" -quick=true

# Cleanup test
source ./helper/cleanup.sh
