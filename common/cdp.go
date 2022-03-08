package common

import (
	"context"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/dop251/goja"
	"github.com/mailru/easyjson"
)

type executorEmitter interface {
	cdp.Executor
	EventEmitter
}

type connection interface {
	executorEmitter
	Close(...goja.Value)
	getSession(target.SessionID) *Session
}

type session interface {
	cdp.Executor
	executorEmitter
	ExecuteWithoutExpectationOnReply(context.Context, string, easyjson.Marshaler, easyjson.Unmarshaler) error
	SessionID() target.SessionID
	TargetID() target.ID
	Done() <-chan struct{}
}
