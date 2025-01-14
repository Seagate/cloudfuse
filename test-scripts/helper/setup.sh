#!/bin/bash

# The following creates a file called var.env and then adds the lines
# up until the EOF to the file. This is based on the following
# stack overflow post
# https://stackoverflow.com/questions/4879025/creating-files-with-some-content-with-shell-script

cat <<EOF >var.env
# This is the mount directory that the container will mount in
export MOUNT_DIR=~/e2e-test

# This is a temporary folder used to write some files during the test
export MOUNT_TMP=~/e2e-temp

# This is the directory where you cloned the repo
export WORK_DIR=~/cloudfuse
EOF
