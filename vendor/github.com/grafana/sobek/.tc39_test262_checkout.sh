#!/bin/sh -e
# this is just the commit it was last tested with
sha=cb4a6c8074671c00df8cbc17a620c0f9462b312a

mkdir -p testdata/test262
cd testdata/test262
git init
git remote add origin https://github.com/tc39/test262.git
git fetch origin --depth=1 "${sha}"
git reset --hard "${sha}"
cd -
