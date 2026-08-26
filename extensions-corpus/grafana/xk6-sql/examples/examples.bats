#!/usr/bin/env bats

setup() {
  cd "$BATS_TEST_DIRNAME"
  BASEDIR="$(git rev-parse --show-toplevel)"
  EXE="$BASEDIR/k6"

  if [ ! -x "$EXE" ]; then
    echo "    - building k6" >&3
    cd "$BASEDIR"
    xk6 build --with github.com/grafana/xk6-sql=. --with github.com/grafana/xk6-sql-driver-ramsql
    cd "$BATS_TEST_DIRNAME"
  fi
}

@test 'example.js' {
  run $EXE run example.js
  [ $status -eq 0 ]
  echo "$output" | grep -q 'msg="Pan, Peter"'
}
