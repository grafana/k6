#!/bin/sh

npm i --force .
modules="$(cat shim.js | grep ^require  | sed "s|require('\./modules/||g"  | sed "s/');//g" | tr "\n" ",")"
npm run grunt "build:${modules}" "--library=off" "--path=custom" "uglify"
