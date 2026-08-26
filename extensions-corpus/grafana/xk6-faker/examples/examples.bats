#!/usr/bin/env bats

setup() {
  cd "$BATS_TEST_DIRNAME"
  BASEDIR="$(git rev-parse --show-toplevel)"
  EXE="$BASEDIR/k6"

  if [ ! -x "$EXE" ]; then
    echo "    - building k6" >&3
    cd "$BASEDIR"
    xk6 build --with github.com/grafana/xk6-faker=.
    cd "$BATS_TEST_DIRNAME"
  fi
}

@test 'default-faker.js' {
  run $EXE run default-faker.js
  [ $status -eq 0 ]
}

@test 'custom-faker.js' {
  run $EXE run custom-faker.js
  [ $status -eq 0 ]
  echo "$output" | grep -q "msg=Josiah"
}

@test 'default-faker-env.js' {
  export XK6_FAKER_SEED=11
  run $EXE run default-faker-env.js
  [ $status -eq 0 ]
  echo "$output" | grep -q "msg=Josiah"
}
