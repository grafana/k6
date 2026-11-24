#!/bin/sh

# Usage:
#  go test -v | ./extract_passed_tests.sh
#
# In order to count how many test files are passing:
#  go test -v | ./extract_passed_tests.sh | wc -l
#
# Compare passed test files before & after certain change:
#  go test -v | ./extract_passed_tests.sh > new.txt
#  git checkout <REF> && go test -v | ./extract_passed_tests.sh > old.txt
#  diff -u old.txt new.txt

sed -En 's/^.*PASS: TestTC39\/tc39\/(test\/.*.js).*$/\1/p' | sort -u
