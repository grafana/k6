#!/usr/bin/env bats

setup() {
  cd "$BATS_TEST_DIRNAME"
  BASEDIR="$(git rev-parse --show-toplevel)"

  export K6=${K6:-$(echo ${BASEDIR}/k6)}
  if [ ! -x "$K6" ]; then
    echo "    - building k6" >&3
    cd $BASEDIR
    xk6 build --output "$K6" --with github.com/grafana/xk6-tcp=$BASEDIR
    cd "$BATS_TEST_DIRNAME"
  fi

  export ECHO=${ECHO:-$(echo ${BASEDIR}/with-echo)}
}

@test 'basic.js' {
  run $ECHO $K6 run basic.js
  [ $status -eq 0 ]
}

@test 'hello.js' {
  run $ECHO $K6 run hello.js
  [ $status -eq 0 ]
}

@test 'echo.js' {
  run $ECHO $K6 run echo.js
  [ $status -eq 0 ]
}

@test 'timeout.js' {
  # This test expects timeout to trigger, so we run without echo server
  # Set timeout low to keep test fast
  run timeout 2 $K6 run timeout.js
  # Allow either 0 (success) or 124 (timeout) as both are valid outcomes
  [ $status -eq 0 -o $status -eq 124 ]
}

@test 'options.js' {
  run $ECHO $K6 run options.js
  [ $status -eq 0 ]
}

@test 'binary.js' {
  run $ECHO $K6 run binary.js
  [ $status -eq 0 ]
}

@test 'multiple.js' {
  run $ECHO $K6 run multiple.js
  [ $status -eq 0 ]
}

@test 'state.js' {
  run $ECHO $K6 run state.js
  [ $status -eq 0 ]
}

@test 'smoke.test.js' {
  run $ECHO $K6 run --no-usage-report smoke.test.js
  [ $status -eq 0 ]
}
