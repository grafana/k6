#!/bin/sh -e
sha=cb4a6c8074671c00df8cbc17a620c0f9462b312a # this is just the commit it was last tested with
mkdir -p ./TestTC39/test262
cd ./TestTC39/test262
git init
git remote add origin https://github.com/tc39/test262.git
git fetch origin --depth=1 "${sha}"
git reset --hard "${sha}"
cd -
