#!/bin/bash

set -eux

xk6 build --with github.com/phymbert/xk6-sse=.
for script in examples/*.js
do
  if [ $script == "examples/llm.js" ]; then
      # Disable as it requires a running local inference server
      continue
  fi
  echo "Running script $script"
   ./k6 run --vus 5 --duration 10s "${script}"
done