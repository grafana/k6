package ws

import (
	"context"

	"github.com/dop251/goja"
)

type WebSocket interface {
	Connect(ctx context.Context, url string, args ...goja.Value) (*WSHTTPResponse, error)
	On(event string, handler goja.Value)
	handleEvent(event string, args ...goja.Value)
	Send(message string,args ...string)
	Ping()
	SetTimeout(fn goja.Callable, timeoutMs int)
	SetInterval(fn goja.Callable, intervalMs int)
	Close(args ...goja.Value)
}
