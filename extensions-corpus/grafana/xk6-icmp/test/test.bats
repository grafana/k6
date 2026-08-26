#!/usr/bin/env bats

setup() {
  cd "$BATS_TEST_DIRNAME"
  BASEDIR="${BASEDIR:-$(git rev-parse --show-toplevel)}"

  export K6=${K6:-$(echo ${BASEDIR}/k6)}
  if [ ! -x "$K6" ]; then
    echo "    - building k6" >&3
    cd $BASEDIR
    MODULE=$(go list -f '{{.Module.Path}}')
    xk6 build --output "$K6" --with $MODULE=$BASEDIR
    cd "$BATS_TEST_DIRNAME"
  fi
}

@test 'basic.test.js' {
  run "$K6" run basic.test.js
  [ $status -eq 0 ]
}

@test 'blacklist.test.js' {
  run "$K6" run blacklist.test.js
  [ $status -eq 0 ]
}

@test 'callback.test.js' {
  run "$K6" run callback.test.js
  [ $status -eq 0 ]
}

@test 'options.test.js' {
  run "$K6" run options.test.js
  [ $status -eq 0 ]
}
