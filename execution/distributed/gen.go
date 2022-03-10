package distributed

//nolint:lll
//go:generate protoc --go-grpc_opt=paths=source_relative --go_opt=paths=source_relative --go_out=./ --go-grpc_out=./ ./distributed.proto
