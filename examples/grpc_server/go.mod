module go.k6.io/k6/examples/grpc_server

go 1.25.0

replace go.k6.io/k6 => ../../

require (
	go.k6.io/k6/v2 v2.0.0-rc1
	google.golang.org/grpc v1.80.0
)

require (
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260401024825-9d38bb4040a9 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
