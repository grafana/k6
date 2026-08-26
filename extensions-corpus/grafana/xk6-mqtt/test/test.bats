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

  WITH_BROKER=${WITH_BROKER:-$(echo ${BASEDIR}/with-broker)}
  if [ ! -x "$WITH_BROKER" ]; then
    echo "    - building with-broker" >&3
    cd $BASEDIR
    go build -o "$WITH_BROKER" $BASEDIR/tools/with-broker
    cd "$BATS_TEST_DIRNAME"
  fi
}

@test 'environment.test.js' {
  run "$WITH_BROKER" "$K6" run environment.test.js
  [ $status -eq 0 ]
}

@test 'connect.test.js' {
  run "$WITH_BROKER" "$K6" run connect.test.js
  [ $status -eq 0 ]
}

@test 'end_async.test.js' {
  run "$WITH_BROKER" "$K6" run end_async.test.js
  [ $status -eq 0 ]
}

@test 'subscribe.test.js' {
  run "$WITH_BROKER" "$K6" run subscribe.test.js
  [ $status -eq 0 ]
}

@test 'subscribe_async.test.js' {
  run "$WITH_BROKER" "$K6" run subscribe_async.test.js
  [ $status -eq 0 ]
}

@test 'publish_async.test.js' {
  run "$WITH_BROKER" "$K6" run publish_async.test.js
  [ $status -eq 0 ]
}
