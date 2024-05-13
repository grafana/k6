module github.com/liuxd6825/k6server/examples/grpc_server

go 1.22

toolchain go1.22.2

replace go.k6.io/k6 => ../../

require (
	go.k6.io/k6 v0.50.0
	google.golang.org/grpc v1.63.2
)

require (
	golang.org/x/net v0.24.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240401170217-c3f982113cda // indirect
	google.golang.org/protobuf v1.33.0 // indirect
)
