@CLAUDE.md

## Cursor Cloud specific instructions

### Overview
k6 is a single Go binary CLI tool (no external services required). All development commands are in `CLAUDE.md` and the `Makefile`.

### PATH setup
`golangci-lint` is installed to `$HOME/go/bin`. Ensure `export PATH="$HOME/go/bin:$PATH"` is in your shell environment before running `make lint`. This is already configured in `~/.bashrc`.

### Running commands
- **Build:** `make build` (or `go build`)
- **Lint:** `make lint` (requires `golangci-lint` v2.10.1 on PATH)
- **Test:** `make tests` (runs `go test -race -timeout 210s ./...`)
- **Run:** `./k6 run <script.js>` — example scripts are in `examples/`

### Browser tests
Browser tests under `internal/js/modules/k6/browser/tests/` require Chromium and may be flaky when run as part of the full test suite due to resource contention. They typically pass when run in isolation. These are not blocking for non-browser changes.

### Dependencies
Dependencies are vendored in `vendor/`. No `go mod download` is needed. The Go toolchain (1.24+) auto-downloads itself via the `toolchain` directive in `go.mod`.