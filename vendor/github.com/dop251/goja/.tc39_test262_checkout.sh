#!/bin/sh
sha=26f1f4567ee7e33163d961c867d689173cbb9065 # this is just the commit it was last tested with
mkdir -p testdata/test262
cd testdata/test262
git init
git remote add origin https://github.com/tc39/test262.git
git fetch origin --depth=1 "${sha}"
git reset --hard "${sha}"
cd -
