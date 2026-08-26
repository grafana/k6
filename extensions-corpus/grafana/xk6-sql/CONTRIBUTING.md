# Contributing Guidelines

Thank you for your interest in contributing!

All types of contributions are encouraged and valued. Please make sure to read this document before making your contribution. It will make it a lot easier for us maintainers and smooth out the experience for all involved. The community looks forward to your contributions. 

> And if you like the project, but just don't have time to contribute, that's fine. There are other easy ways to support the project and show your appreciation, which we would also be very happy about:
> - Star the project
> - Tweet about it
> - Refer this project in your project's readme
> - Mention the project at local meetups and tell your friends/colleagues

## Code of Conduct

Before you begin, make sure to familiarize yourself with the [Code of Conduct](CODE_OF_CONDUCT.md). If you've previously contributed to other open source project, you may recognize it as the classic [Contributor Covenant](https://contributor-covenant.org/).

## Asking questions

Before you ask a question, it is best to search for existing [Issues](https://github.com/grafana/xk6-sql/issues) that might help you. It is also advisable to search the internet for answers first.

If you then still feel the need to ask a question and need clarification or if you want to chat with the team or the community, you can [join our community forums](https://community.grafana.com/c/grafana-k6).

## Reporting Bugs

A good bug report shouldn't leave others needing to chase you up for more information. Therefore, we ask you to investigate carefully, collect information and describe the issue in detail in your report.

We use [GitHub issues](https://github.com/grafana/xk6-sql/issues) to track bugs and errors. If you run into an issue with the project:

- To see if other users have experienced (and potentially already solved) the same issue you are having, check if there is not already a bug report existing for your bug or error.
- Also make sure to search the internet (including Stack Overflow) to see if users outside of the GitHub community have discussed the issue.
- Open an [Issue](https://github.com/grafana/xk6-sql/issues).
- Explain the behavior you would expect and the actual behavior.
- Please provide as much context as possible and describe the *reproduction steps* that someone else can follow to recreate the issue on their own.

## Suggesting Enhancements

Enhancement suggestions are tracked as [GitHub issues](https://github.com/grafana/xk6-sql/issues).

- Make sure that you are using the latest version.
- Read the documentation carefully and find out if the functionality is already covered, maybe by an individual configuration.
- Perform a search in [Issues](https://github.com/grafana/xk6-sql/issues) to see if the enhancement has already been suggested. If it has, add a comment to the existing issue instead of opening a new one.
- Find out whether your idea fits with the scope and aims of the project. It's up to you to make a strong case to convince the project's developers of the merits of this feature. Keep in mind that we want features that will be useful to the majority of our users and not just a small subset.
- Use a **clear and descriptive title** for the issue to identify the suggestion.
- Provide a **step-by-step description of the suggested enhancement** in as many details as possible.
- **Describe the current behavior** and **explain which behavior you expected to see instead** and why. At this point you can also tell which alternatives do not work for you.
- **Explain why this enhancement would be useful** to most of users. You may also want to point out the other projects that solved it better and which could serve as inspiration.

## Contributing code

If you'd like to contribute code, this is the basic procedure.

1. Find an [issue](https://github.com/grafana/xk6-sql/issues) you'd like to fix. If there is none already, or you'd like to add a feature, please open one, and we can talk about how to do it. Out of respect for your time, please start a discussion regarding any bigger contributions either in a [GitHub Issue](https://github.com/grafana/xk6-sql/issues), in the community forums **before** you get started on the implementation.
  
   Remember, there's more to software development than code; if it's not properly planned, stuff gets messy real fast.

2. Create a fork and open a feature branch - `feature/my-cool-feature` is the classic way to name these, but it really doesn't matter.

3. Create a pull request!

4. We will discuss implementation details until everyone is happy, then a maintainer will merge it.

## Development Environment

We use [Development Containers](https://containers.dev/) to provide a reproducible development environment. We recommend that you do the same. In this way, it is guaranteed that the appropriate version of the tools required for development will be available.

**Without installing software**

You can *contribute without installing any software* using [GitHub Codespaces](https://docs.github.com/en/codespaces). After forking the repository, [create a codespace for your repository](https://docs.github.com/en/codespaces/developing-in-a-codespace/creating-a-codespace-for-a-repository).

**Using an IDE**

You can contribute conveniently using [Visual Studio Code](https://code.visualstudio.com/) by installing the [Dev Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) extension. You *don't need to install any other software* to contribute. Clone your repository and open the folder with Visual Studio Code. It will automatically detect that the folder contains a Dev Container configuration and ask you whether to open the folder in a container. Choose **"Reopen in Container"**.

[JetBrains GoLand](https://www.jetbrains.com/help/go/connect-to-devcontainer.html) also has DevContainers support.

It is worth mentioning [DevPod](https://devpod.sh/docs/) in addition to the above.

**The hard way**

All the tools used for development are free and open-source, so you can install them without using *Development Containers*. The `.devcontainer/devcontainer.json` file contains a list of the tools to be installed and their version numbers.

### Updating tool versions

The version numbers of tools used in GitHub workflows are defined as [repository variables](https://github.com/grafana/xk6-sql/settings/variables/actions). The version numbers of tools used in the *Development Containers* are only defined in the `.devcontainer/devcontainer.json` file. The version numbers should be updated carefully to be consistent.

## Tasks

The usual contributor tasks can be performed using GNU make. The `Makefile` defines a target for each task. To execute a task, the name of the task must be specified as an argument to the make command.

```bash
make taskname
```

Help on the available targets and their descriptions can be obtained by issuing the `make` command without any arguments.

```bash
make
```

More detailed help can be obtained for individual tasks using the [cdo](https://github.com/szkiba/cdo) command:

```bash
cdo taskname --help
```

**Authoring the Makefile**

The `Makefile` is generated from the task list defined in the `CONTRIBUTING.md` file using the [cdo](https://github.com/szkiba/cdo) tool. If a contribution has been made to the task list, the `Makefile` must be regenerated using the [makefile] target.

```bash
make makefile
```

### security - Run security and vulnerability checks

The [gosec] tool is used for security checks. The [govulncheck] tool is used to check the vulnerability of dependencies.

```bash
gosec -quiet ./...
govulncheck ./...
```

[gosec]: https://github.com/securego/gosec
[govulncheck]: https://github.com/golang/vuln
[security]: <#security---run-security-and-vulnerability-checks>

### lint - Run the linter

The [golangci-lint] tool is used for static analysis of the source code. It is advisable to run it before committing the changes.

```bash
golangci-lint run
```

[lint]: <#lint---run-the-linter>
[golangci-lint]: https://github.com/golangci/golangci-lint

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

### build - Build custom k6 with extension

The [xk6] tool is used to build the k6. In addition to the xk6-sql extension, the database driver extension you want to use should also be specified.

```bash
xk6 build --with github.com/grafana/xk6-sql=. --with github.com/grafana/xk6-sql-driver-ramsql
```

[xk6]: https://github.com/grafana/xk6
[build]: <#build---build-custom-k6-with-extension>

### example - Run the examples

Run the examples embedded in `README.md`.

```bash
bats ./examples
```

[example]: #example---run-the-examples

### readme - Update README.md

Update the example code and its output in `README.md` using [mdcode] tool.

```bash
mdcode update
```

[mdcode]: <https://github.com/szkiba/mdcode>
[readme]: #readme---update-readmemd

### clean - Clean the working directory

Delete the work files created in the work directory (also included in .gitignore).

```bash
rm -rf ./k6 ./coverage.txt ./build ./node_modules ./bun.lockb
```

[clean]: #clean---clean-the-working-directory

### doc - Generate API documentation

Generate API documentation using `typedoc`. The generated documentation will be placed in the `build/docs` folder.

```bash
bun x typedoc --out build/docs
```

[doc]: #doc---generate-api-documentation

### all - Run all

Performs the most important tasks. It can be used to check whether the CI workflow will run successfully.

Requires
: [clean], [lint], [security], [test], [build], [doc], [example], [readme], [makefile]

### format - Format the go source codes

```bash
go fmt ./...
```

[format]: #format---format-the-go-source-codes

### makefile - Generate the Makefile

```bash
cdo --makefile Makefile
```
[makefile]: <#makefile---generate-the-makefile>
