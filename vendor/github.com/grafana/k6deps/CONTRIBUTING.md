# Contributing Guidelines

Thank you for your interest in contributing to **k6deps**!

Before you begin, make sure to familiarize yourself with the [Code of Conduct](CODE_OF_CONDUCT.md). If you've previously contributed to other open source project, you may recognize it as the classic [Contributor Covenant](https://contributor-covenant.org/).

If you want to chat with the team or the community, you can [join our community forums](https://community.grafana.com/c/grafana-k6).

### Filing issues

Don't be afraid to file issues! Nobody can fix a bug we don't know exists, or add a feature we didn't think of.

The worst that can happen is that someone closes it and points you in the right direction.

That said, "how do I..."-type questions are often more suited for [community forums](https://community.grafana.com/c/grafana-k6).

### Contributing code

If you'd like to contribute code, this is the basic procedure.

1. Find an issue you'd like to fix. If there is none already, or you'd like to add a feature, please open one, and we can talk about how to do it. Out of respect for your time, please start a discussion regarding any bigger contributions either in a GitHub Issue, in the community forums **before** you get started on the implementation.
  
   Remember, there's more to software development than code; if it's not properly planned, stuff gets messy real fast.

2. Create a fork and open a feature branch - `feature/my-cool-feature` is the classic way to name these, but it really doesn't matter.

3. Create a pull request!

4. We will discuss implementation details until everyone is happy, then a maintainer will merge it.

## Prerequisites

Prerequisites are listed in the [tools] section in addition to the [go toolchain](https://go101.org/article/go-toolchain.html) and [git](https://git-scm.com/) CLI.

The `Makefile` is generated from the task list defined in the `CONTRIBUTING.md` file using the [cdo] tool. If the contribution is made to the task list, the `Makefile` must be regenerated, which is why the [cdo] tool is needed. The [cdo] tool can most conveniently be installed using the [eget] tool.

```bash
eget szkiba/cdo
```

[cdo]: https://github.com/szkiba/cdo
[eget]: https://github.com/zyedidia/eget

## Tasks

The tasks defined here can be executed manually or conveniently using the make or [cdo] tool.

**Help about tasks**

The command below lists the possible tasks.

using make:

```bash
make
```

using [cdo]:

```bash
cdo
```

**Execute task**

Tasks are executed by passing the name of the task as a parameter.

using make:

```bash
make taskname
```

using [cdo]:

```bash
cdo taskname
```

### tools - Install the required tools

Contributing will require the use of some tools, which can be installed most easily with a well-configured [eget] tool.

```bash
eget szkiba/mdcode
eget golangci/golangci-lint
eget goreleaser/goreleaser
```

[tools]: #tools---install-the-required-tools
[mdcode]: https://github.com/szkiba/mdcode
[golangci-lint]: https://github.com/golangci/golangci-lint
[goreleaser]: https://github.com/goreleaser/goreleaser

### lint - Run the linter

The [golangci-lint] tool is used for static analysis of the source code. It is advisable to run it before committing the changes.

```bash
golangci-lint run ./...
```

### test - Run the tests

The `go test` command is used to run the tests and generate the coverage report.

```bash
go test -count 1 -race -coverprofile=coverage.txt -timeout 60s ./...
```

[test]: <#test---run-the-tests>

### coverage - View the test coverage report

The go `cover` tool should be used to display the coverage report in the browser.

Requires
: [test]

```bash
go tool cover -html=coverage.txt
```

### build - Build the executable binary

This is the easiest way to create an executable binary (although the release process uses the [goreleaser] tool to create release versions).

```bash
go build -ldflags="-w -s" -o build/k6deps ./cmd/k6deps
```

[build]: <#build---build-the-executable-binary>

#### snapshot - Create the executable binary with a snapshot version

The [goreleaser] command-line tool is used during the release process. During development, it is advisable to create binaries with the same tool from time to time.

```bash
goreleaser build --snapshot --clean --single-target -o build/k6deps
```

[snapshot]: <#snapshot---create-the-executable-binary-with-a-snapshot-version>

### readme - Update README.md

Update the CLI documentation and the example code in `README.md`.

```bash
go run ./tools/gendoc README.md
mdcode update
```

[readme]: <#readme---update-readmemd>

### clean - Clean the working directory

Delete the work files created in the work directory (also included in .gitignore).

```bash
rm -rf ./coverage.txt ./build
```

[clean]: <#clean---clean-the-working-directory>

### all - Clean build

Performs the most important tasks. It can be used to check whether the CI workflow will run successfully.

Requires
: [clean], [makefile], [format], [test], [build], [snapshot], [readme]

### format - Format the go source codes

```bash
go fmt ./...
```

[format]: <#format---format-the-go-source-codes>

### makefile - Generate the Makefile

```bash
cdo --makefile Makefile
```

[makefile]: <#makefile---generate-the-makefile>
