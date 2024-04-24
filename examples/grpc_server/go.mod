module go.k6.io/k6/examples/grpc_server

go 1.19

replace go.k6.io/k6 => ../../

require (
	go.k6.io/k6 v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.63.0
)

require (
	golang.org/x/net v0.23.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240227224415-6ceb2ff114de // indirect
	google.golang.org/protobuf v1.33.0 // indirect
)
