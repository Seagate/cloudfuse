#!/bin/bash
echo "Cloudfuse Regenerating component imports..."

# Regenrate the loadcomponent.go to include all components present in folder
loader_file="./cmd/imports.go"

echo "package cmd" > $loader_file
echo "" >> $loader_file
echo "import (" >> $loader_file

for i in $(find . -type d | grep "component/" | cut -c 3- | sort -u); do # Not recommended, will break on whitespace
    echo "    _ \"cloudfuse/$i\"" >> $loader_file
done
echo ")"  >> $loader_file
