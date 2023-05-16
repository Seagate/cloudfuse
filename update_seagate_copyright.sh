#!/bin/bash

# URL of blobfuse2 repo
blobfuse2_repo="https://github.com/Azure/azure-storage-fuse.git"
tags="tags/blobfuse2-2.0.3"
year="2023"

# Create temporary directory to clone into
tmp_dir=$(mktemp -d)

# Clone repo to temp location
git clone "$blobfuse2_repo" "$tmp_dir" || { exit 1; }

cd "$tmp_dir"
git checkout "$tags" || { exit 1; }
cd -

# Get list of diff files
diff_files=$(diff -x .git -qr . "$tmp_dir" | grep differ | awk '{print $2}')

# Get list of file that are new and do not exist in original repo
new_files=$(diff -x .git -qr . "$tmp_dir" | grep "Only in ." | awk -F': ' '{print $2}')

# Get a list of all diff files and new files
all_file="$diff_files"

# New files only have the file name, so we need to find the full path
for file in $new_files
do
    found_file=$(find . -name "$file")

    if [ -n "$found_file" ]
    then
        all_file+="$found_file"$'\n'
    fi
done

for file in $all_file
do
    # Check if the file exists in our repo
    if [ -f "$file" ]
    then
        # If this file has a copyright statement
        if grep -q "Copyright © .* Microsoft Corporation" "$file"
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
                    sed -i "${line_number}i \ \ \ Copyright © $year Seagate Technology LLC and/or its Affiliates" "$file"
                fi
            fi
        fi
    fi
done

rm -rf "$tmp_dir"
