#! /usr/bin/pwsh

# Load the variables
. ".\helper\var.ps1"

Write-Output "-------------------------------------------------------------------"
Write-Output "Starting e2e Tests"

go test -v -timeout=2h ../test/e2e_tests/data_validation_test.go ../test/e2e_tests/dir_test.go ../test/e2e_tests/file_test.go -args -mnt-path="${mount_dir}" -adls=false -clone=false -tmp-path="${mount_tmp}" -quick-test=true -stream-direct-test=true -distro-name="windows"

#go test -v -timeout=2h ../test/e2e_tests/file_test.go -args -mnt-path="${mount_dir}" -adls=false -clone=false -stream-direct-test=false -distro-name="windows"

#go test -v -timeout=2h ../test/e2e_tests/dir_test.go -args -mnt-path="${mount_dir}" -adls=false -clone=false -stream-direct-test=true

#go test -v -timeout=2h ../test/e2e_tests/data_validation_test.go -args -mnt-path="${mount_dir}" -adls=false -tmp-path="${mount_tmp}" -quick-test=true -stream-direct-test=false -distro-name="windows"