#!/bin/bash

# Let script exit if an unsed variable is used
set -o nounset

# Load variables
source ./helper/var.env

# cleanup step
echo "Ensuring no container mounted in mount directory"
sudo fusermount -u $MOUNT_DIR
sudo fusermount3 -u $MOUNT_DIR

echo "Stopping previous run of lyvecloudfuse"
sudo kill -9 `pidof lyvecloudfuse` || true

echo "Deleting files in mount and temp directories"
rm -rf $MOUNT_DIR/*
rm -rf $MOUNT_TMP/*
