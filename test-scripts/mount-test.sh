#!/bin/bash

#let script exit if an unsed variable is used
set -o nounset

# Cleanup test
source ./helper/cleanup.sh
if $?; then
    echo $?
    echo "Stopping script"
    exit $?
fi

echo "-------------------------------------------------------------------"
echo "Starting Mount Test"

go test -timeout=120m -p 1 -v ../test/mount_test/mount_test.go -args -working-dir=$work_dir -mnt-path=$mount_dir -config-file=$work_dir/config.yaml

# Cleanup test
source ./helper/cleanup.sh
