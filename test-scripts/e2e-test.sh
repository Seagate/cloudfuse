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
echo "Starting End-to-End Test"

# run test
go test -v -timeout=2h ../test/e2e_tests/data_validation_test.go ../test/e2e_tests/dir_test.go ../test/e2e_tests/file_test.go -args -mnt-path="$MOUNT_DIR" -adls=false -clone=false -tmp-path="$MOUNT_TMP" -quick-test=true -stream-direct-test=false -distro-name="Ubuntu"

# Cleanup test
source ./helper/cleanup.sh
