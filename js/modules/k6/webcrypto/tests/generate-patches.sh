#!/bin/sh

cd ./wpt

# Generate patches for the WebCryptoAPI tests
git diff --name-only | while read file; do
    safe_name=$(echo "$file" | sed 's/\//__/g')
    git diff "$file" > "../wpt-patches/${safe_name}.patch"
done

# Return to the original directory
cd -