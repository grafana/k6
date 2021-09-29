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
	"time"
)

type HookID int

const (
	HookApplySlowMo HookID = iota
)

type Hook func(context.Context)

type Hooks struct {
	hooks map[HookID]Hook
}

func applySlowMo(ctx context.Context) {
	hooks := GetHooks(ctx)
	if hooks != nil {
		hook := hooks.GetHook(HookApplySlowMo)
		if hook != nil {
			hook(ctx)
		}
	}
}

func defaultSlowMo(ctx context.Context) {
	l := GetLaunchOptions(ctx)
	if l.SlowMo > 0 {
		select {
		case <-ctx.Done():
		case <-time.After(l.SlowMo):
		}
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
	h.hooks[HookApplySlowMo] = defaultSlowMo
}

func (h *Hooks) GetHook(id HookID) Hook {
	return h.hooks[id]
}

func (h *Hooks) RegisterHook(id HookID, hook Hook) {
	h.hooks[id] = hook
}
