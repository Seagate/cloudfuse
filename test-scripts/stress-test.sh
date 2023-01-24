#!/bin/bash

# Mount the directory
source ./helper/mount.sh

# Run e2e tests
echo "-------------------------------------------------------------------"
echo "Starting Stress Test"

# run test
go test -timeout=120m -v ../test/stress_test/stress_test.go -args -mnt-path=$mount_dir -quick=false

# Cleanup test
source ./helper/cleanup.sh
