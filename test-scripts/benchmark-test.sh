#!/bin/bash

#let script exit if an unsed variable is used
set -o nounset

# Mount the directory
source ./helper/mount.sh
exit $?

# Run e2e tests
echo "-------------------------------------------------------------------"
echo "Starting Benchmark Test"

# run test
go test -timeout=120m -p 1 -v ../test/benchmark_test/benchmark_test.go -args -mnt-path=$mount_dir

cat lyvecloudfuse-logs.txt

# Cleanup test
source ./helper/cleanup.sh
