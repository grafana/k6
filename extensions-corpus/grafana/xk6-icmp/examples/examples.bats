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

@test 'basic.js' {
  run $K6 run basic.js
  [ $status -eq 0 ]
}


@test 'blacklist.js' {
  run $K6 run blacklist.js
  [ $status -eq 0 ]
}

@test 'ip6.js' {
  run $K6 run ip6.js
  [ $status -eq 0 ]
}

@test 'callback.js' {
  run $K6 run callback.js
  [ $status -eq 0 ]
}

@test 'options.js' {
  run $K6 run options.js
  [ $status -eq 0 ]
}
