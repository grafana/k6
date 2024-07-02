#!/bin/sh
# this is just the commit it was last tested with
sha=634933a489f1bb8cf074a2a9b8616ade5f2f5cac

mkdir -p testdata/test262
cd testdata/test262
git init
git remote add origin https://github.com/tc39/test262.git
git fetch origin --depth=1 "${sha}"
git reset --hard "${sha}"
cd -
