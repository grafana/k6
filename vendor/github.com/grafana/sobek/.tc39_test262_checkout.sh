#!/bin/sh -e
# this is just the commit it was last tested with
sha=3af36bec45bd4f72d4b57366653578e1e4dafef7

mkdir -p testdata/test262
cd testdata/test262
git init
git remote add origin https://github.com/tc39/test262.git
git fetch origin --depth=1 "${sha}"
git reset --hard "${sha}"
cd -
