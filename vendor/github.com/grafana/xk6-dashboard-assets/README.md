# xk6-dashboard-assets

Embeddable static assets for [xk6-dashboard](https://github.com/grafana/xk6-dashboard).

This repository contains the web-based user interface and reporting components that are embedded into the **xk6-dashboard** extension.

## Overview

The **xk6-dashboard-assets** repository is a monorepo that packages frontend assets into Go-embeddable static files. These assets provide:

- **UI**: Real-time web interface for viewing test execution metrics
- **Report**: Static HTML report generation for test results
- **Configuration**: Default dashboard configuration
- **Shared Libraries**: Common models and view utilities

## Project Structure

```
xk6-dashboard-assets/
├── packages/
│   ├── config/      # Default configuration generator
│   ├── model/       # TypeScript data models
│   ├── view/        # Shared view utilities
│   ├── report/      # Static HTML report SPA
│   └── ui/          # Real-time dashboard SPA
├── assets.go        # Go package for embedded assets
└── assets_test.go   # Tests for embedded assets
```

## Packages

| Package | Description |
|---------|-------------|
| `@xk6-dashboard-assets/config` | Generates default configuration for the dashboard. |
| `@xk6-dashboard-assets/model` | TypeScript type definitions and data models for k6 metrics and events. |
| `@xk6-dashboard-assets/view` | Shared utilities for formatting, charting, and data visualization. |
| `@xk6-dashboard-assets/report` | Single-page application for generating static HTML reports from test results. Built with Preact. |
| `@xk6-dashboard-assets/ui` | Real-time dashboard web interface for monitoring test execution. Built with React. |

## Public API

This repository is a Go module that provides embedded static assets for xk6-dashboard. The assets are designed to be embedded into the main xk6-dashboard Go extension using Go's `embed` package.

The module exposes three public functions:

- **`func UI() fs.FS`** - Returns an embedded filesystem containing the live dashboard single-page application. This includes all HTML, JavaScript, CSS, and other assets needed for the interactive web interface.

- **`func Report() []byte`** - Returns the embedded HTML content for the static report single-page application. This is a self-contained HTML file that can be saved and viewed offline.

- **`func Config() json.RawMessage`** - Returns the embedded default configuration as raw JSON. This provides the default panel and visualization settings for the dashboard.

The embedded assets are located in:
- `packages/ui/dist/*` - Complete UI application bundle
- `packages/report/dist/index.html` - Standalone report HTML
- `packages/config/dist/config.json` - Default configuration

## Versioning

This project uses a unique versioning approach due to its dual nature as both a Node.js monorepo and a Go module:

### Frontend Packages

The packages in the monorepo (`@xk6-dashboard-assets/*`) are **never published to npm**. They exist solely for internal dependency management and building the embedded assets. Each `package.json` contains a placeholder version `1.0.0` that remains unchanged, as these packages are only used internally within the monorepo.

The built frontend assets (JavaScript bundles, HTML files, and configuration) are embedded directly into the Go module and exposed through the public API functions (`UI()`, `Report()`, `Config()`).

### Go Module Versioning

The repository is versioned as a standard Go module using **GitHub releases** with semantic versioning tags (e.g., `v0.1.0`, `v1.0.0`).

- Release notes are maintained in the [`releases/`](./releases/) directory
- Each release includes the compiled frontend assets embedded in the Go module
- Consumers import this module to access the embedded assets at a specific version

Example:
```go
import "github.com/grafana/xk6-dashboard-assets" // Uses the version specified in go.mod
```

## Development

### Prerequisites

- Node.js 24+ and Yarn
- Go 1.24+

### Installation

```bash
# Install dependencies
yarn install
```

### Building

```bash
# Build all packages
yarn build

# Build specific package
cd packages/ui && yarn build
```

### Development

```bash
# Run development mode (watch)
yarn dev

# Run specific package in dev mode
cd packages/ui && yarn dev
```

### Linting and Formatting

```bash
# Lint all packages
yarn lint

# Format all packages
yarn format
```

### Testing

```bash
# Run Go tests
yarn test
```

### Cleaning

```bash
# Clean build artifacts
yarn clean
```

## Embedding in Go

The built assets are embedded in Go code via the `assets.go` file:

```go
import "github.com/grafana/xk6-dashboard-assets"

// Get embedded filesystem for UI
ui := assets.UI()

// Get embedded report HTML
reportHTML := assets.Report()

// Get default configuration
config := assets.Config()
```

## Contributing

Contributions are welcome! Please ensure all tests pass and code is properly formatted before submitting pull requests.
