package grpc

import (
	"context"
	"net"
)

// ContextDialer defines a function that returns a connection for a context
type ContextDialer func(context.Context, string) (net.Conn, error)
