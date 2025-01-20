#!/bin/sh

# Last commit hash it was tested with
sha=75db68b05454e7f3a81e063373defd094c176a16

# Checkout concrete files from the web-platform-tests repository
mkdir -p ./wpt
cd ./wpt
git init
git remote add origin https://github.com/web-platform-tests/wpt
git sparse-checkout init --cone
git sparse-checkout set resources WebCryptoAPI
git fetch origin --depth=1 "${sha}"
git checkout ${sha}

# Apply custom patches needed to run the tests in k6/Sobek
for patch in ../wpt-patches/*.patch
do
    git apply "$patch"
    if [ $? -ne 0 ]; then
        exit $?
    fi
done

# Return to the original directory
cd -
