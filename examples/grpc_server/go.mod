module go.k6.io/k6/examples/grpc_server

go 1.23.0

toolchain go1.23.7

replace go.k6.io/k6 => ../../

require (
	go.k6.io/k6 v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.75.0
)

require (
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250728155136-f173205681a0 // indirect
	google.golang.org/protobuf v1.36.8 // indirect
)
