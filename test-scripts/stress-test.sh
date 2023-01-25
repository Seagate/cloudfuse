#!/bin/bash

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
echo "Starting Stress Test"

# run test
go test -timeout=120m -v ../test/stress_test/stress_test.go -args -mnt-path=$mount_dir -quick=false

# Cleanup test
source ./helper/cleanup.sh
