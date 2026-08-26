# Design: Upgrade franz-go to v1.21.5

## Goals
- Update `go.mod` to use `twmb/franz-go v1.21.5` (latest v1.x patch)
- Verify all integration tests pass
- Maintain 100% API compatibility

## Implementation
1. Update dependency in `go.mod`
2. Run `go mod tidy` to resolve any transitive deps
3. Run `make integration` to verify Kafka broker tests pass

## No Changes To
- Package exports or APIs (franz-go v1.21.5 is fully compatible)
- Test surface or behavior
