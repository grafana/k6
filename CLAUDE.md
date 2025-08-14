# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

k6 is a modern load testing tool built in Go. It allows developers to write JavaScript test scripts to simulate user behavior and measure system performance. The tool features configurable load generation, multiple protocol support (HTTP, WebSockets, gRPC, Browser), and flexible metrics output.

## Common Development Commands

### Building and Testing
```bash
# Build the k6 binary
make build
# or
go build

# Run all tests with race detection  
make tests
# or
go test -race -timeout 210s ./...

# Run linting
make lint

# Run both linting and tests
make check

# Format code
make format
```

### Single Test Execution
```bash
# Run tests for a specific package
go test ./lib/executor/
go test ./internal/js/modules/k6/http/
go test -v -run TestSpecificFunction ./path/to/package/

# Run browser tests (requires special setup)
go test ./internal/js/modules/k6/browser/tests/
```

### Code Generation
```bash
# Install tools and regenerate generated code
make generate-tools-installs
make generate
```

### Working with Examples
```bash
# Run gRPC server example
make grpc-server-run

# Test scripts are in examples/ directory
./k6 run examples/http_get.js
./k6 run examples/browser/querying.js
```

## Architecture Overview

### Core Components

**Main Entry Point**: `main.go` → `cmd/execute.go` → `internal/cmd/root.go`
- The CLI is built with Cobra and organized into subcommands in `internal/cmd/`
- Key commands: `run`, `archive`, `cloud`, `inspect`, `new`, `pause`, `resume`, `scale`, `stats`, `status`, `version`

**JavaScript Runtime**: `internal/js/` and `js/`
- `internal/js/`: Core JavaScript execution engine using Sobek (Goja fork)
- `js/modules/`: Built-in k6 modules (http, browser, crypto, etc.)
- `js/common/`: Shared JavaScript runtime utilities

**Load Testing Core**: `lib/`
- `lib/executor/`: Different execution strategies (constant VUs, ramping, arrival rate, etc.)
- `lib/netext/`: Network extensions and HTTP transport
- `lib/types/`: Core type definitions
- `lib/options.go`: Test configuration options

**Outputs**: `internal/output/` and `output/`
- Multiple output formats: JSON, CSV, InfluxDB, Prometheus, Cloud, OpenTelemetry
- Each output has its own package with config and implementation

**Browser Testing**: `internal/js/modules/k6/browser/`
- CDP-based browser automation using chromium
- Page, frame, element handle abstractions
- Browser context and session management

### Key Patterns

**Module System**: k6 modules are Go packages that expose JavaScript APIs
- `internal/js/modules/k6/`: Built-in k6 modules  
- Modules follow a mapping pattern to bridge Go and JavaScript
- Example: `k6/http` module maps HTTP functionality

**Executor Pattern**: Different load testing strategies implemented as executors
- `constant-vus`: Fixed number of virtual users
- `ramping-vus`: Gradually increasing/decreasing VUs  
- `constant-arrival-rate`: Fixed rate of iterations
- `per-vu-iterations`: Fixed iterations per VU

**Extension System**: `ext/` package allows extending k6 with custom functionality
- Extensions can add new JavaScript modules
- Used by xk6 ecosystem for community extensions

## Testing Structure

**Unit Tests**: Co-located with source code (`*_test.go`)
**Integration Tests**: `internal/cmd/tests/` for CLI integration tests  
**Browser E2E Tests**: `internal/js/modules/k6/browser/tests/` with static HTML fixtures
**TC39/WPT Conformance**: `internal/js/tc39/` and modules with WPT test suites

## Important Configuration Files

- `.golangci.yml`: Linter configuration (version pinned at top)
- `Makefile`: Build targets and development commands
- `go.mod`: Go module dependencies (use Go 1.23+)
- `Dependencies.md`: Dependency update policy and guidelines
- `modtools_frozen.yml`: Dependencies that should not be auto-updated

## Development Guidelines

**Code Style**: 
- Use `gofmt -s` for formatting
- Follow golangci-lint rules (run `make lint`)
- Comments should wrap at 100 characters

**Dependencies**:
- Update direct dependencies only after releases
- Some dependencies are intentionally frozen (see Dependencies.md)
- Use `modtools check --direct-only` to check for updates

**Testing**:
- All new code should have tests
- Use `go test -race` for concurrency testing
- Browser tests require additional setup and fixtures

**Generated Code**: 
- Several files are auto-generated (look for `*_gen.go`)
- Run `make generate` after modifying source templates
- Protobuf definitions in `internal/cloudapi/insights/proto/`

## Git Workflow and Branching Strategy

**Branching Strategy**:
- Create a new branch for each feature or bugfix
- Branch names must be descriptive and reflect the problem being solved
- Examples: `fix-http-timeout-handling`, `add-websocket-compression`, `improve-browser-element-selection`
- Base new branches off the `master` branch

**Allowed Git Commands**:
```bash
# Create and switch to new branch
git checkout -b descriptive-branch-name
git switch -c descriptive-branch-name

# Regular commits (using user from git config)
git add .
git commit -m "descriptive commit message"

# Push branch to remote
git push origin branch-name
git push -u origin branch-name  # first push with upstream

# Check status and view changes
git status
git diff
git log
```

**Strictly Forbidden Git Operations**:
- **NO force pushes**: `git push --force`, `git push -f`
- **NO history modification**: `git reset --hard`, `git rebase`, `git rebase -i`
- **NO direct pushes to master**: `git push origin master`
- **NO commit author overrides**: Always use the default git config user

**Commit Guidelines**:
- All commits must be created on behalf of the user listed in Git config (default behavior)
- Write clear, descriptive commit messages that could be included in a changelog
- Close related issues with commit messages: `Closes #123`, `Fixes #456`
- Follow existing commit message style in the repository

**Pull Request Process**:
- Create pull requests from feature branches to `master`
- Ensure all tests pass before creating PR
- Follow the existing PR template and guidelines

## Common File Locations

**CLI Commands**: `internal/cmd/` (run.go, cloud.go, etc.)
**HTTP Module**: `js/modules/k6/http/` and `internal/js/modules/k6/http/`  
**Browser Module**: `internal/js/modules/k6/browser/`
**Metrics**: `metrics/` package for metric definitions
**Examples**: `examples/` directory with various test script examples
**Test Data**: `internal/cmd/testdata/` and module-specific test fixtures