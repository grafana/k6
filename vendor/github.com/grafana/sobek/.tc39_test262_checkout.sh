#!/bin/sh
# this is just the commit it was last tested with
sha=926b0960d737b9f1dfd0ec0c1dfd95d836016d33

mkdir -p testdata/test262
cd testdata/test262
git init
git remote add origin https://github.com/tc39/test262.git
git fetch origin --depth=1 "${sha}"
git reset --hard "${sha}"
cd -
