package common

import (
	"context"

	"github.com/dop251/goja"
	"github.com/grafana/xk6-browser/k6ext"
)

// Credentials holds HTTP authentication credentials.
type Credentials struct {
	Username string `js:"username"`
	Password string `js:"password"`
}

// NewCredentials return a new Credentials.
func NewCredentials() *Credentials {
	return &Credentials{}
}

// Parse credentials details from a given goja credentials value.
func (c *Credentials) Parse(ctx context.Context, credentials goja.Value) error {
	rt := k6ext.Runtime(ctx)
	if credentials != nil && !goja.IsUndefined(credentials) && !goja.IsNull(credentials) {
		credentials := credentials.ToObject(rt)
		for _, k := range credentials.Keys() {
			switch k {
			case "username":
				c.Username = credentials.Get(k).String()
			case "password":
				c.Password = credentials.Get(k).String()
			}
		}
	}
	return nil
}

// HTTPHeader is a single HTTP header.
type HTTPHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HTTPMessageSize are the sizes in bytes of the HTTP message header and body.
type HTTPMessageSize struct {
	Headers int64 `json:"headers"`
	Body    int64 `json:"body"`
}

// Total returns the total size in bytes of the HTTP message.
func (s HTTPMessageSize) Total() int64 {
	return s.Headers + s.Body
}
