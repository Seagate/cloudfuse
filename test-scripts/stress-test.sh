#!/bin/bash

# Let script exit if an unsed variable is used
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
echo "Starting Stress Test"

# run test
go test -timeout=120m -v ../test/stress_test/stress_test.go -args -mnt-path=$MOUNT_DIR -quick=true

# Cleanup test
source ./helper/cleanup.sh
