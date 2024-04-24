[![Go Reference](https://pkg.go.dev/badge/github.com/grafana/k6pack.svg)](https://pkg.go.dev/github.com/grafana/k6pack)
[![GitHub Release](https://img.shields.io/github/v/release/grafana/k6pack)](https://github.com/grafana/k6pack/releases/)
[![Go Report Card](https://goreportcard.com/badge/github.com/grafana/k6pack)](https://goreportcard.com/report/github.com/grafana/k6pack)
[![GitHub Actions](https://github.com/grafana/k6pack/actions/workflows/test.yml/badge.svg)](https://github.com/grafana/k6pack/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/grafana/k6pack/graph/badge.svg?token=krmjUlDGM5)](https://codecov.io/gh/grafana/k6pack)
![GitHub Downloads](https://img.shields.io/github/downloads/grafana/k6pack/total)

# k6pack

**TypeScript transpiler and module bundler for k6**

The main goal of **k6pack** is to make TypeScript and modern JavaScript features available in [k6](https://k6.io/) tests.

**Features**

- Supports TypeScript (.ts) and JavaScript (.js) input
- Supports remote (https) modules
- Single executable file, no external dependencies
- No Node.js installation required

## Install

Precompiled binaries can be downloaded and installed from the [Releases](https://github.com/grafana/k6pack/releases) page.

If you have a go development environment, the installation can also be done with the following command:

```
go install github.com/grafana/k6pack/cmd/k6pack@latest
```

## Usage

The name of the entry point must be specified as a parameter. The k6 compatible script is sent to the standard output, so it can be executed directly with k6.

```sh
k6pack script.ts | k6 run -
```
<details>
<summary>script.ts</summary>

```ts file=examples/script.ts
import { User, newUser } from "./user";

export default () => {
  const user: User = newUser("John");
  console.log(user);
};
```

</details>

<details>
<summary>user.ts</summary>


```ts file=examples/user.ts
interface User {
  name: string;
  id: number;
}

class UserAccount implements User {
  name: string;
  id: number;

  constructor(name: string) {
    this.name = name;
    this.id = Math.floor(Math.random() * Number.MAX_SAFE_INTEGER);
  }
}

function newUser(name: string): User {
  return new UserAccount(name);
}

export { User, newUser };
```

</details>

### Flags

```
Usage:
  k6pack [flags] filename

Flags:
      --external stringArray   exclude module(s) from the bundle
  -h, --help                   help for k6pack
      --minify                 minify the output
  -o, --output string          write output to file (default stdout)
      --source-root string     sets the sourceRoot field in generated source maps
      --sourcemap              emit the source map with an inline data URL
      --timeout duration       HTTP timeout for remote modules (default 30s)
      --typescript             force TypeScript loader
  -v, --version                version for k6pack
```

**sourcemap**

If sourcemap is enabled, by default, the current directory will be set in the sourcemap as the source root directory. This can be changed by using the `--source-root` flag. You can even disable the source root setting by specifying the empty string.

## How It Works

Under the hood, k6pack uses the [esbuild](https://github.com/evanw/esbuild) library. A special esbuild plugin contains k6 specific configuration and another esbuild plugin implements loading from http/https URL.

k6pack can also be used as a [go library](https://pkg.go.dev/github.com/grafana/k6pack).

## Development

### Tasks

This section contains a description of the tasks performed during development. If you have the [xc (Markdown defined task runner)](https://github.com/joerdav/xc) command-line tool, individual tasks can be executed simply by using the `xc task-name` command.

<details><summary>Click to expand</summary>

#### lint

Run the static analyzer.

```
golangci-lint run
```

#### test

Run the tests.

```
go test -count 1 -race -coverprofile=build/coverage.txt ./...
```

#### coverage

View the test coverage report.

```
go tool cover -html=build/coverage.txt
```

#### build

Build the executable binary.

This is the easiest way to create an executable binary (although the release process uses the goreleaser tool to create release versions).

```
go build -ldflags="-w -s" -o build/k6pack .
```

#### snapshot

Creating an executable binary with a snapshot version.

The goreleaser command-line tool is used during the release process. During development, it is advisable to create binaries with the same tool from time to time.

```
goreleaser build --snapshot --clean --single-target -o build/k6pack
```

#### examples

Run scripts from examples directory.

```
go run ./cmd/k6pack examples/script.ts | go run go.k6.io/k6@latest run -
go run ./cmd/k6pack examples/simple.ts | go run go.k6.io/k6@latest run -
```

#### clean

Delete the build directory.

```
rm -rf build
```

#### all

Run all tasks.

Requires: lint,test,build,snapshot,examples

</details>
