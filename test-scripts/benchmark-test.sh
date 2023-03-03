#!/bin/bash

#let script exit if an unsed variable is used
set -o nounset

# Load variables
source ./helper/var.env

# Mount the directory
source ./helper/mount.sh
exit_code=$?
if [ $exit_code -ne 0 ]; then
    echo "command failed with exit code ${exit_code}"
    echo "Stopping script"
    exit $exit_code
fi

# Run e2e tests
echo "-------------------------------------------------------------------"
echo "Starting Benchmark Test"

# run test
go test -timeout=120m -p 1 -v ../test/benchmark_test/benchmark_test.go -args -mnt-path=$MOUNT_DIR

cat lyvecloudfuse-logs.txt

# Cleanup test
source ./helper/cleanup.sh
