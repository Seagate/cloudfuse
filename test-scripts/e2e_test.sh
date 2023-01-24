#!/bin/bash

#let script exit if an unsed variable is used
set -o nounset

# Mount the directory
source ./helper/mount.sh
exit $?

# Run e2e tests
echo "-------------------------------------------------------------------"
echo "Starting End-to-End Test"

# run test
go test -v -timeout=2h ../test/e2e_tests/data_validation_test.go ../test/e2e_tests/dir_test.go ../test/e2e_tests/file_test.go -args -mnt-path=$mount_dir -adls=false -clone=true -tmp-path=$mount_tmp -quick-test=false -stream-direct-test=false -distro-name="Ubuntu"

# Cleanup test
source ./helper/cleanup.sh
