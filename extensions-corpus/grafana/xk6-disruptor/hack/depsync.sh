#!/usr/bin/env bash

set -eo pipefail

go install github.com/grafana/go-depsync@latest

tmpdir=$(mktemp -d)



gogetcmd=$(go-depsync --parent go.k6.io/k6  2>"$tmpdir/deps.log")

if [[ -z $gogetcmd ]]; then
	echo "Nothing to do."
	exit 0
fi

echo "Running $gogetcmd"
$gogetcmd 2>&1 | tee -a "$tmpdir/go-get.log"

go mod tidy

cat <<EOF >depsync-pr-body.txt
This automated PR aligns the following dependency mismatches with k6 core:
\`\`\`
$(cat "$tmpdir/deps.log")
\`\`\`

The following command was run to align the dependencies:
\`\`\`shell
$gogetcmd
\`\`\`

And produced the following output:
\`\`\`
$(cat "$tmpdir/go-get.log")
\`\`\`

Due to a limitation of GitHub Actions, to run CI checks for this PR, close it and reopen it again.
EOF
