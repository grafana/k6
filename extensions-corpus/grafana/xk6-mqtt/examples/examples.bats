#!/usr/bin/env bats

setup() {
  cd "$BATS_TEST_DIRNAME"
  BASEDIR="$(git rev-parse --show-toplevel)"

  export K6=${K6:-$(echo ${BASEDIR}/k6)}
  if [ ! -x "$K6" ]; then
    echo "    - building k6" >&3
    cd $BASEDIR
    xk6 build --output "$K6" --with github.com/grafana/xk6-mqtt=$BASEDIR
    cd "$BATS_TEST_DIRNAME"
  fi
}

@test 'smoke.test.js' {
  run $K6 run smoke.test.js
  [ $status -eq 0 ]
}

@test 'hello.js' {
  run $K6 run hello.js
  [ $status -eq 0 ]
}

@test 'ping.js' {
  run $K6 run ping.js
  [ $status -eq 0 ]
}
