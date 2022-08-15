#!/bin/sh

sed -En 's/^.*FAIL: TestTC39\/tc39\/(test\/.*.js).*$/"\1": true,/p'
