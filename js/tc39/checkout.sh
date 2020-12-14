#!/bin/sh
sha=72154b17fc99a26e79b2586960f059360d4ce43d # this is just the commit it was last tested with
mkdir -p ./TestTC39/test262
cd ./TestTC39/test262
git init
git remote add origin https://github.com/tc39/test262.git
git fetch origin --depth=1 "${sha}"
git reset --hard "${sha}"
cd -
