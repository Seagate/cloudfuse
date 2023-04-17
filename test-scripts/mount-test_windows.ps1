#! /usr/bin/pwsh

# Cleanup test
${mount_dir} = ""

# This is the directory where you cloned the repo
${work_dir} = ""

Write-Output "-------------------------------------------------------------------"
Write-Output "Starting Mount Test"

go test -timeout=120m -p 1 -v ..\test\mount_test\mount_test.go -args -working-dir="${work_dir}"  -mnt-path="${mount_dir}" -config-file="${work_dir}\config.yaml"
