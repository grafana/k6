module go.k6.io/k6/examples/grpc_server

go 1.19

replace go.k6.io/k6 => ../../

require (
	go.k6.io/k6 v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.64.1
)

require (
	golang.org/x/net v0.26.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240515191416-fc5f0ca64291 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
)
