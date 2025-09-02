#!/bin/bash
set -euo pipefail

protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    route_guide.proto
