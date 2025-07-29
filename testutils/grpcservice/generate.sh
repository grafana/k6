#!/bin/bash

# Exit on error
set -e

# Generate gRPC code
protoc --go_out=. \
       --go_opt=paths=source_relative \
       --go-grpc_out=. \
       --go-grpc_opt=paths=source_relative \
       route_guide.proto 