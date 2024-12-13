package common

import (
	"context"
	"sync"
	"time"
)

type HookID int

const (
	HookApplySlowMo HookID = iota
)

type Hook func(context.Context)

type Hooks struct {
	mu    sync.RWMutex
	hooks map[HookID]Hook
}

func applySlowMo(ctx context.Context) {
	hooks := GetHooks(ctx)
	if hooks == nil {
		return
	}
	if hook := hooks.Get(HookApplySlowMo); hook != nil {
		hook(ctx)
	}
}

func defaultSlowMo(ctx context.Context) {
	sm := GetBrowserOptions(ctx).SlowMo
	if sm <= 0 {
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(sm):
	}
}

func NewHooks() *Hooks {
	h := Hooks{
		hooks: make(map[HookID]Hook),
	}
	h.registerDefaultHooks()
	return &h
}

func (h *Hooks) registerDefaultHooks() {
	h.Register(HookApplySlowMo, defaultSlowMo)
}

func (h *Hooks) Get(id HookID) Hook {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.hooks[id]
}

func (h *Hooks) Register(id HookID, hook Hook) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.hooks[id] = hook
}
