module go.k6.io/k6/examples/grpc_server

go 1.25.0

replace go.k6.io/k6 => ../../

require (
	go.k6.io/k6/v2 v2.0.0-rc1
	google.golang.org/grpc v1.83.2
)

require (
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
