# Ping Service

This directory has an implementation of a grpc test service.

The service implements the following function:

```
func Ping(request *PingRequest) (*PingResponse, error)
```

The `PingRequest` structure is defined as follows:

```
type PingRequest struct {
        Error   int32  
	Message string
        Headers map[string]string
}
```

If the `Error` in the request is `OK(0)` the function returns a `PingResponse` with the same `Message` than the request.
Otherwise, the function generates and error with the status defined in `Error` and the message defined in `Message`.
The server will return the `Headers` defined in the request as part of the response's header metadata.

## Build

If you modify the [protobuf definition](./ping.proto) you must re-generate the grpc code.

Install the required toolchain:

```
go install google.golang.org/protobuf/cmd/protoc-gen-go
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc
```

From the root of this project, execute the following command

```
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative pkg/testutils/grpc/ping/ping.proto 
```