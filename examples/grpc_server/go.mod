module go.k6.io/k6/examples/grpc_server

go 1.25.0

replace go.k6.io/k6 => ../../

require (
	go.k6.io/k6/v2 v2.0.0-rc1
	google.golang.org/grpc v1.82.1
)

require (
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.39.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
