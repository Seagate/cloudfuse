#!/bin/sh
set -e
rm -rf manpages
mkdir manpages
go run . man --output-location="manpages"
find manpages -name "*.1" -exec gzip -9n {} \;
find manpages -name "*.1" -exec touch -t 197001010000 {} \;
