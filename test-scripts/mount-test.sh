#!/bin/bash

#let script exit if an unsed variable is used
set -o nounset

# Cleanup test
source ./helper/cleanup.sh
if [ $exit_code -ne 0 ]; then
    echo "command failed with exit code ${exit_code}"
    echo "Stopping script"
    exit $exit_code
fi

echo "-------------------------------------------------------------------"
echo "Starting Mount Test"

go test -timeout=120m -p 1 -v ../test/mount_test/mount_test.go -args -working-dir=$work_dir -mnt-path=$mount_dir -config-file=$work_dir/config.yaml

# Cleanup test
source ./helper/cleanup.sh
