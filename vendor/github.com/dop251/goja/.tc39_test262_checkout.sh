#!/bin/sh
sha=ddfe24afe3043388827aa220ef623b8540958bbd # this is just the commit it was last tested with
mkdir -p testdata/test262
cd testdata/test262
git init
git remote add origin https://github.com/tc39/test262.git
git fetch origin --depth=1 "${sha}"
git reset --hard "${sha}"
cd -
