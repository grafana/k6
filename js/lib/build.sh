#!/bin/sh

sha="a3616d74c0b9b9409d580337e04c32da6e3877e4" # v2.5.1
mkdir -p ./corejs-repo
cd ./corejs-repo
git init
git remote add origin https://github.com/zloirock/core-js.git
git fetch origin --depth=1 "${sha}"
git reset --hard "${sha}"
cp ../k6-shim.js shim.js
cp ../corejs-build.sh corejs-build.sh
docker run --user "$(id -u):$(id -g)" --rm -ti  -v "$(pwd):/opt/g" --entrypoint /opt/g/corejs-build.sh --workdir /opt/g node
cp custom.min.js ../shim.min.js
cd -
rm -rf corejs-repo
mkdir core-js
mv shim.min.js core-js/shim.min.js
rice embed-go
rm -rf core-js
