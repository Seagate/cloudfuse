#!/bin/bash

#let script exit if an unsed variable is used
set -o nounset

source ./helper/env_var.sh
if $?; then
    echo $?
    echo "Stopping script"
    exit $?
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
