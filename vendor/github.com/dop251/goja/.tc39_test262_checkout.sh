#!/bin/sh
# this is just the commit it was last tested with
sha=e87b0048c402479df1d9cb391fb86620cf3200fd

mkdir -p testdata/test262
cd testdata/test262
git init
git remote add origin https://github.com/tc39/test262.git
git fetch origin --depth=1 "${sha}"
git reset --hard "${sha}"
cd -
