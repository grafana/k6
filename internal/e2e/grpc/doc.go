// Package grpce2e contains end-to-end tests that exercise k6's gRPC support by
// running a real gRPC server and the compiled k6 binary as separate OS
// processes. Because the server and client do not share a process, the client
// cannot rely on the server's process-global protobuf registrations: it must
// load the proto definitions at runtime itself (from a .proto file or via
// server reflection). See https://github.com/grafana/k6/issues/3552.
//
// The tests live behind the "grpc_e2e" build tag because they build the k6
// binary and a helper server binary, which is too slow for the default unit
// test run. Run them with:
//
//	go test -tags grpc_e2e ./internal/e2e/grpc/...
package grpce2e
