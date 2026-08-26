# Spec: Upgrade franz-go Dependency

## Requirement
Update `twmb/franz-go` from v1.21.3 to v1.21.5 in `go.mod`.

## Acceptance Criteria
- [ ] go.mod shows `github.com/twmb/franz-go v1.21.5`
- [ ] `go mod tidy` completes without error
- [ ] Integration tests pass (make integration)
