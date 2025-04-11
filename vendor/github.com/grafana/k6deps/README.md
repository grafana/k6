[![Go Reference](https://pkg.go.dev/badge/github.com/grafana/k6deps.svg)](https://pkg.go.dev/github.com/grafana/k6deps)
[![GitHub Release](https://img.shields.io/github/v/release/grafana/k6deps)](https://github.com/grafana/k6deps/releases/)
[![Go Report Card](https://goreportcard.com/badge/github.com/grafana/k6deps)](https://goreportcard.com/report/github.com/grafana/k6deps)
[![GitHub Actions](https://github.com/grafana/k6deps/actions/workflows/test.yml/badge.svg)](https://github.com/grafana/k6deps/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/grafana/k6deps/graph/badge.svg?token=PCRNQE9LGQ)](https://codecov.io/gh/grafana/k6deps)
![GitHub Downloads](https://img.shields.io/github/downloads/grafana/k6deps/total)

<h1 name="title">k6deps</h1>

**Dependency analysis for k6 tests**

The goal of k6deps is to extract dependencies from k6 test scripts. For this purpose, k6deps analyzes the k6 test scripts and the modules imported from it in a recursive manner.

k6deps is primarily used as a [go library](https://pkg.go.dev/github.com/grafana/k6deps). In addition, it also contains a [command-line tool](#cli), which is suitable for listing the dependencies of k6 test scripts.

The command line tool can be integrated into other command line tools as a subcommand. For this purpose, the library also contains the functionality of the command line tool as a factrory function that returns [cobra.Command](https://pkg.go.dev/github.com/spf13/cobra#Command).

## Install

Precompiled binaries can be downloaded and installed from the [Releases](https://github.com/grafana/k6deps/releases) page.

If you have a go development environment, the installation can also be done with the following command:

```
go install github.com/grafana/k6deps/cmd/k6deps@latest
```

## Usage

Dependencies can come from three [sources](#sources): k6 test script, manifest file, `K6_DEPENDENCIES` environment variable. Instead of these three sources, a k6 archive can also be specified, which can contain all three sources (currently two actually, because the manifest file is not yet included in the k6 archive).

In the simplest use, we only extract the dependencies from the script source.

<details><summary><strong>with pragma</strong></summary>

```go file=analyze_example_with_pragma_test.go
package k6deps_test

import (
	"fmt"

	"github.com/grafana/k6deps"
)

const scriptWithPragma = `
"use k6 > 0.54";
"use k6 with k6/x/faker > 0.4.0";
"use k6 with k6/x/sql >= 1.0.1";

import { Faker } from "k6/x/faker";
import sql from "k6/x/sql";
import driver from "k6/x/sql/driver/ramsql";

export default function() {
}
`

func ExampleAnalyze_with_pragma() {
	deps, _ := k6deps.Analyze(&k6deps.Options{
		Script: k6deps.Source{
			Name:     "script.js",
			Contents: []byte(scriptWithPragma),
		},
		// disable automatic source detection
		Manifest: k6deps.Source{Ignore: true},
		Env:      k6deps.Source{Ignore: true},
	})

	fmt.Println(deps.String())

	out, _ := deps.MarshalJSON()
	fmt.Println(string(out))
	// Output:
	// k6>0.54;k6/x/faker>0.4.0;k6/x/sql>=1.0.1;k6/x/sql/driver/ramsql*
	// {"k6":">0.54","k6/x/faker":">0.4.0","k6/x/sql":">=1.0.1","k6/x/sql/driver/ramsql":"*"}
}
```

</details>

<details><summary><strong>without pragma</strong></summary>

```go file=analyze_example_without_pragma_test.go
package k6deps_test

import (
	"fmt"

	"github.com/grafana/k6deps"
)

const scriptWithoutPragma = `
import { Faker } from "k6/x/faker";
import sql from "k6/x/sql";
import driver from "k6/x/sql/driver/ramsql";

export default function() {
}
`

func ExampleAnalyze_without_pragma() {
	deps, _ := k6deps.Analyze(&k6deps.Options{
		Script: k6deps.Source{
			Name:     "script.js",
			Contents: []byte(scriptWithoutPragma),
		},
		// disable automatic source detection
		Manifest: k6deps.Source{Ignore: true},
		Env:      k6deps.Source{Ignore: true},
	})

	fmt.Println(deps.String())

	out, _ := deps.MarshalJSON()
	fmt.Println(string(out))
	// Output:
	// k6/x/faker*;k6/x/sql*;k6/x/sql/driver/ramsql*
	// {"k6/x/faker":"*","k6/x/sql":"*","k6/x/sql/driver/ramsql":"*"}
}
```

</details>

## CLI

<!-- #region cli -->
## k6deps

Extension dependency detection for k6.

### Synopsis

Analyze the k6 test script and extract the extensions that the script depends on.

### Sources

Dependencies can come from three sources: k6 test script, manifest file, `K6_DEPENDENCIES` environment variable. Instead of these three sources, a k6 archive can also be specified, which can contain all three sources (currently two actually, because the manifest file is not yet included in the k6 archive). An archive is a tar file, which can be created using the k6 archive command.

> *NOTE*: It is assumed that the script and all dependencies are in the archive. No external dependencies are analyzed.

The name of k6 test script or archive can be specified as the positional argument in the command invocation. Alternatively, the content can be provided in the stdin. If stdin is used, the input format ('js' for script of or 'tar' for archive) must be specified using the `--input` parameter.

Primarily, the k6 test script is the source of dependencies. The test script and the local and remote JavaScript modules it uses are recursively analyzed. The extensions used by the test script are collected. In addition to the require function and import expression, the `"use k6 ..."` directive can be used to specify additional extension dependencies. If necessary, the `"use k6 ..."` directive can also be used to specify version constraints.

    "use k6 > 0.54";
    "use k6 with k6/x/faker > 0.4.0";
    "use k6 with k6/x/sql >= 1.0.1";

    import { Faker } from "k6/x/faker";
    import sql from "k6/x/sql";
    import driver from "k6/x/sql/driver/ramsql";

Dependencies and version constraints can also be specified in the so-called manifest file. The default name of the manifest file is `package.json` and it is automatically searched from the directory containing the test script up to the root directory. The `dependencies` property of the manifest file contains the dependencies in JSON format.

    {"dependencies":{"k6":">0.54","k6/x/faker":">0.4.0","k6/x/sql":>=v1.0.1"}}

Dependencies and version constraints can also be specified in the `K6_DEPENDENCIES` environment variable. The value of the variable is a list of dependencies in a one-line text format.

    k6>0.54;k6/x/faker>0.4.0;k6/x/sql>=v1.0.1

### Format

By default, dependencies are written as a JSON object. The property name is the name of the dependency and the property value is the version constraints of the dependency.

    {"k6":">0.54","k6/x/faker":">0.4.0","k6/x/sql":">=1.0.1","k6/x/sql/driver/ramsql":"*"}

Additional output formats:

 * `text` - One line text format. A semicolon-separated sequence of the text format of each dependency. The first element of the series is `k6` (if there is one), the following elements follow each other in lexically increasing order based on the name.

        k6>0.54;k6/x/faker>0.4.0;k6/x/sql>=1.0.1;k6/x/sql/driver/ramsql*

 * `js` - A consecutive, one-line JavaScript string directives. The first element of the series is `k6` (if there is one), the following elements follow each other in lexically increasing order based on the name.

        "use k6>0.54";
        "use k6 with k6/x/faker>0.4.0";
        "use k6 with k6/x/sql>=v1.0.1";

### Output

By default, dependencies are written to standard output. By using the `-o/--output` flag, the dependencies can be written to a file.


```
k6deps [flags] [script-file]
```

### Flags

```
      --format json|text|js   output format, possible values: json,env,script (default json)
  -h, --help                  help for k6deps
      --ignore-manifest       disable package.json detection and processing
      --ignore-script         disable script processing
      --ingnore-env           ignore K6_DEPENDENCIES environment variable processing
  -i, --input string          input format ('js', 'ts' or 'tar' for archives)
      --manifest string       manifest file to analyze (default 'package.json' nearest to script-file)
  -o, --output string         write output to file (default stdout)
```

<!-- #endregion cli -->

## Contribute

If you want to contribute or help with the development of **k6pack**, start by 
reading [CONTRIBUTING.md](CONTRIBUTING.md).
