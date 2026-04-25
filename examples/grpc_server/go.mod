module go.k6.io/k6/examples/grpc_server

go 1.25.0

replace go.k6.io/k6 => ../../

require (
	go.k6.io/k6 v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.80.0
)

require (
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260209200024-4cfbd4190f57 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
