package grpc

import "google.golang.org/protobuf/reflect/protoregistry"

// Types return the client's registry of descriptor types.
// For testing purposes only.
func (c *Client) Types() *protoregistry.Types {
	return c.types
}
