#!/bin/sh
mvpkg --help &>/dev/null || (echo "you need mvpkg run 'go install github.com/vikstrous/mvpkg@latest'" && exit 1)
mkdir -p internal

list=`comm -3 <(go list ./... | grep -v '^go.k6.io/k6/internal' | sort | uniq) <( cat publicly_used_imports.txt )`

for  i in $list; do
	if [[ $i == "go.k6.io/k6" ]] then
		continue
	fi
	i=${i##go.k6.io/k6/}

	mvpkg $i internal/$i
	find $i -maxdepth 1 -type f | xargs -I '{}' -n 1 git mv '{}' internal/$i
done

git mv cmd/testdata                                         internal/cmd/testdata
git mv cmd/tests/testdata                                   internal/cmd/tests/testdata
git mv js/modules/k6/browser/tests/static                   internal/js/modules/k6/browser/tests/static
git mv js/modules/k6/webcrypto/tests                        internal/js/modules/k6/webcrypto/tests
git mv js/modules/k6/experimental/streams/tests             internal/js/modules/k6/experimental/streams/tests
git mv js/modules/k6/experimental/websockets/autobahn_tests internal/js/modules/k6/experimental/websockets/autobahn_tests
git mv lib/testutils/httpmultibin/grpc_protoset_testing     internal/lib/testutils/httpmultibin/grpc_protoset_testing
git mv output/cloud/expv2/integration/testdata              internal/output/cloud/expv2/integration/testdata

git apply internalize.patch

# clean empty folders that are left over after all the moving
while find -type d -empty | grep -v .git | xargs rm -r &>/dev/null
do
	true
done

git add internal
git add cmd
git commit -a -n -m "chore: Move not publicly used APIs in internal package"
