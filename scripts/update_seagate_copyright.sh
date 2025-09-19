#!/bin/bash

# URL of blobfuse2 repo
blobfuse2_repo="https://github.com/Azure/azure-storage-fuse.git"
tags="tags/blobfuse2-2.5.0"
year="2025"

# Create temporary directory to clone into
tmp_dir=$(mktemp -d)

# Clone repo to temp location
git clone "$blobfuse2_repo" "$tmp_dir" || { exit 1; }

cd "$tmp_dir" || { exit 1; }
git checkout "$tags" || { exit 1; }
cd - || { exit 1; }

# Get list of files that we have modified
# This diff command prints output like as follows:
#   Files ./cmd/mount.go and /tmp/tmp.yyhbYNR6UN/cmd/mount.go differ
# Then get the lines that differ and get the path to the file that differs
diff_files=$(diff -x .git -qr . "$tmp_dir" | grep "differ" | awk '{print $2}')

# Get list of file that we added and do not exist in original repo
# This diff command prints output like as follows:
#   Only in ./cmd: mount_linux_test.go
#Then get the lines that are only new in our repo get the file name
new_files=$(diff -x .git -qr . "$tmp_dir" | grep "Only in \." | awk '{print $3 "/" $4}' | sed 's/://')

# Get a list of all diff files and new files
all_file="$diff_files \n $new_files"

for file in $all_file
do
    # Check if the file exists in our repo
    if [ -f "$file" ]
    then
        # Get the line number
        line_number=$(grep -n "Copyright © .* Microsoft Corporation" "$file" | cut -d : -f 1 | head -n 1)
        # Check for line number
        if [ -n "$line_number" ]
        then
            # Check if the our copyright notice already exists
            if ! grep -q "Copyright © .* Seagate Technology" "$file"
            then
                # Add the new copyright notice above the one from Microsoft
                sed -i "${line_number}i \ \ \ Copyright © 2023-$year Seagate Technology LLC and/or its Affiliates" "$file"
            fi
        fi
    fi
done

rm -rf "$tmp_dir"
