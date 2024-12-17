#!/bin/bash

# Function to process each shell script file
process_file() {
    local file="$1"
    # Use 'tr' to remove carriage return characters in place
    tr -d '\r' < "$file" > temp_file && mv temp_file "$file"
}

# Export the function so it can be used by 'find'
export -f process_file

# Find all shell script files recursively and process them
find . -type f -name "*.sh" -exec bash -c 'process_file "$0"' {} \;
