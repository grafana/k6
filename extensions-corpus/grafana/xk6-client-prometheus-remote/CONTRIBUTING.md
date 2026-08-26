# Contributing Guidelines

Thank you for your interest in contributing!

Contributions are welcome and appreciated. Please review this document to ensure a positive experience and efficient project maintenance. Your involvement is valued.

## Code of Conduct

Please review the [Code of Conduct](CODE_OF_CONDUCT.md) before contributing. This document is based on the [Contributor Covenant](https://contributor-covenant.org/), a standard in open source projects, so you may already be familiar with it.

## Asking Questions

Before asking a question, please check existing [issues](https://github.com/grafana/xk6-client-prometheus-remote/issues) and search online for solutions. If you still need help, clarification, or want to connect with the team and community, join our [community forums](https://community.grafana.com/c/grafana-k6).

## Reporting Bugs

Report bugs and errors using GitHub [issues](https://github.com/grafana/xk6-client-prometheus-remote/issues).

Before opening a new issue:

* Check existing bug reports to see if your issue has already been reported and potentially solved.  
* Search the internet (including Stack Overflow) for discussions outside the GitHub community.

To open a new issue:

* Explain the expected and actual behavior.  
* Provide as much context as possible.  
* Describe the exact steps to reproduce the issue.

## Suggesting Enhancements

Feature requests are managed as GitHub [issues](https://github.com/grafana/xk6-client-prometheus-remote/issues).

Before submitting a new suggestion:

* Ensure you are using the latest version.  
* Carefully review the documentation to see if the functionality already exists.  
* Search existing [issues](https://github.com/grafana/xk6-client-prometheus-remote/issues) to avoid duplicates and add a comment if your idea is already present.  
* Consider if your suggestion aligns with the project's scope and goals.

When creating a new issue, please follow these guidelines:

* Use a clear and descriptive title for your suggestion.  
* Provide a detailed, step-by-step description of the proposed enhancement.  
* Describe the current behavior and explain the desired behavior and the reasoning behind it.  
* Mention any alternative solutions you've considered and why they are unsuitable.  
* Explain the broader usefulness of this enhancement to the majority of users.

## Contributing Code

To contribute code:

1. Check existing [issues](https://github.com/grafana/xk6-client-prometheus-remote/issues) or create a new one to discuss your intended fix or feature.  
2. For significant contributions, please start a discussion *before* coding.  
3. Create a fork of the repository and open a feature branch.  
4. Submit your changes as a pull request.  
5. Implementation details will be discussed until consensus is reached.

## Development Environment

Use [Development Containers](https://containers.dev) for a consistent development environment. This ensures that you will have the correct tool versions available for development.

### Without installing software

Contribute without installing software by using [GitHub Codespaces](https://docs.github.com/en/codespaces). Fork the repository, then [create a codespace for your repository](https://docs.github.com/en/codespaces/developing-in-a-codespace/creating-a-codespace-for-a-repository).

GitHub will initiate the creation of a virtual development environment for the repository. The initial codespace creation may take time, but subsequent startups are faster. Once the codespace is ready, it will open in your browser as a VS Code-like environment, allowing you to begin working on your project with the repository code already checked out.

### Using an IDE

While using GitHub Codespace in the browser is a simple starting point, it is worth setting up a local development environment for a better developer experience.

Fork the repository, clone it, and then open the folder in [VS Code](https://code.visualstudio.com/). If prompted, install the [Dev Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) extension. VS Code will detect the [development container](https://containers.dev/) configuration and show a popup to open the project in a dev container. After you accept it, it opens the project in the dev container and rebuilds the container image if necessary.

[JetBrains GoLand](https://www.jetbrains.com/go/) also offers [Dev Containers support](https://www.jetbrains.com/help/go/connect-to-devcontainer.html). In addition to the tools mentioned, [DevPod](https://devpod.sh/docs/) is another noteworthy option.

## Tasks

Use [GNU make](https://www.gnu.org/software/make/) to execute common contributor tasks. The `Makefile` outlines these tasks as distinct targets. To run a specific task, provide its name as an argument when executing the `make` command:

```shell
make taskname
```

To see a list of available targets and their descriptions, run the \`make\` command without any arguments:

```shell
make
```

The `Makefile` is automatically created from the task list in `CONTRIBUTING.md` using the [cdo](https://github.com/szkiba/cdo) tool. After any modification to the task list, the `Makefile` needs to be updated by running the `makefile` target.

```shell
cdo makefile
```

For more in-depth information on specific tasks, utilize the [cdo](https://github.com/szkiba/cdo) command:

```shell
cdo taskname --help
```

This development container automatically installs all necessary tools, including [GNU make](https://www.gnu.org/software/make/) and [cdo](https://github.com/szkiba/cdo), so no manual installation is required.

### security - Run security checks

Use [gosec] for security checks and [govulncheck] for dependency vulnerability checks.

```bash
gosec -quiet ./...
govulncheck ./...
```

[security]: #security---run-security-checks
[gosec]: https://github.com/securego/gosec
[govulncheck]: https://github.com/golang/vuln

### lint - Run the linter

Use the [golangci-lint] tool for static code analysis. It is recommended to run this tool before committing changes. Use the [xk6] `lint` subcommand for k6 extension specific analysis.

```bash
golangci-lint run ./...
xk6 lint
```

[lint]: #lint---run-the-linter
[xk6]: https://github.com/grafana/xk6
[golangci-lint]: https://github.com/golangci/golangci-lint

### test - Run the tests

Use the `go test` command to execute tests and produce a coverage report.

```bash
go test -count 1 -race -coverprofile=coverage.txt -timeout 60s ./...
```

[test]: #test---run-the-tests

### build - Build custom k6 with extension

Use the `xk6 build` command to build custom k6 with extension.

```bash
xk6 build --with github.com/grafana/xk6-client-prometheus-remote=.
```

[build]: #build---build-custom-k6-with-extension

### doc - Generate API documentation

Use [TypeDoc] to generate API documentation and place the output in the `build/docs` folder.

```bash
bun x typedoc --out build/docs
```

[doc]: #doc---generate-api-documentation
[TypeDoc]: https://typedoc.org/

### format - Format the go source codes

Use the `go fmt` command to format Go source code before committing.

```bash
go fmt ./...
```

[format]: #format---format-the-go-source-codes

### makefile - Generate the Makefile

Use the [cdo](https://github.com/szkiba/cdo) tool to regenerate the `Makefile` after modifying `CONTRIBUTING.md`

```bash
cdo --makefile Makefile
```

[makefile]: #makefile---generate-the-makefile

### clean - Clean the working directory

Delete the work files created in the work directory (also included in `.gitignore`).

```bash
rm -rf coverage.txt ./k6 ./k6.exe ./build ./node_modules ./bun.lockb
```

[clean]: #clean---clean-the-working-directory

### all - Run relevant tasks

Run all tasks relevant for the CI workflow. Use this command to ensure the CI workflow will run successfully.

Requires
: [clean], [format], [lint], [security], [test], [build], [makefile]
