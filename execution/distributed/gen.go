// Package distributed implements the execution.Controller interface for
// distributed (multi-instance) k6 execution.
package distributed

//go:generate protoc --go-grpc_opt=paths=source_relative --go_opt=paths=source_relative --go_out=./ --go-grpc_out=./ ./distributed.proto
