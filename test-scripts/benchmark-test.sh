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
echo "Starting Benchmark Test"

# run test
go test -timeout=120m -p 1 -v ../test/benchmark_test/benchmark_test.go -args -mnt-path="$MOUNT_DIR"

cat cloudfuse-logs.txt

# Cleanup test
source ./helper/cleanup.sh
