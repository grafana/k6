## Why

`twmb/franz-go` v1.21.3 is pinned in `go.mod`. Renovate has offered v1.21.5 (latest v1 patch).
Upgrading patches closes known issues and aligns with latest v1 maintenance. No breaking changes
in v1.21.5; purely a maintenance update.

## What Changes

- `go.mod`: `github.com/twmb/franz-go v1.21.3` → v1.21.5
- No breaking API changes; all franz-go usage in Writer/Reader/Connection/Admin remains stable
- CI integration tests unchanged; Kafka broker compatibility stable

## Capabilities

### New Capabilities
<!-- None: this is a dependency maintenance update, not a new capability -->

### Modified Capabilities
<!-- None: v1.21.5 is fully API-compatible with v1.21.3 for all existing capabilities -->

## Impact

- **Code**: No changes to `pkg/kafka/` (Writer, Reader, Connection, Admin, etc.)
- **Tests**: Rerun `make integration` to verify all tests pass on v1.21.5
- **Dependencies**: Bumps `twmb/franz-go` to latest v1.x patch
- **Compatibility**: Maintains compatibility with k6 v2.0.0 and all existing scripts
