package js

import (
	"context"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/eventloop"
	"go.k6.io/k6/lib"
)

type moduleVUImpl struct {
	ctx       context.Context
	initEnv   *common.InitEnvironment
	state     *lib.State
	runtime   *goja.Runtime
	eventLoop *eventloop.EventLoop
}

func (m *moduleVUImpl) Context() context.Context {
	return m.ctx
}

func (m *moduleVUImpl) InitEnv() *common.InitEnvironment {
	return m.initEnv
}

func (m *moduleVUImpl) State() *lib.State {
	return m.state
}

func (m *moduleVUImpl) Runtime() *goja.Runtime {
	return m.runtime
}

func (m *moduleVUImpl) RegisterCallback() func(func() error) {
	return m.eventLoop.RegisterCallback()
}
