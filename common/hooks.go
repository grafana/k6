/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

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
	sm := GetLaunchOptions(ctx).SlowMo
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
