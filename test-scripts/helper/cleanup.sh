#!/bin/bash

#let script exit if an unsed variable is used
set -o nounset

source ./helper/env_var.sh
exit_code=$?
if [ $exit_code -ne 0 ]; then
    echo "command failed with exit code ${exit_code}"
    echo "Stopping script"
    exit $exit_code
fi

# cleanup step
echo "Ensuring no container mounted in mount directory"
sudo fusermount -u $mount_dir
sudo fusermount3 -u $mount_dir

echo "Stopping previous run of lyvecloudfuse"
sudo kill -9 `pidof lyvecloudfuse` || true

echo "Deleting files in mount and temp directories"
rm -rf $mount_dir/*
rm -rf $mount_tmp/*
