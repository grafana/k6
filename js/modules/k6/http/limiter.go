/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
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

package http

type SlotLimiter struct {
	ch chan struct{}
}

func NewSlotLimiter(slots int) SlotLimiter {
	if slots <= 0 {
		return SlotLimiter{nil}
	}

	ch := make(chan struct{}, slots)
	for i := 0; i < slots; i++ {
		ch <- struct{}{}
	}
	return SlotLimiter{ch}
}

func (l *SlotLimiter) Begin() {
	if l.ch != nil {
		<-l.ch
	}
}

func (l *SlotLimiter) End() {
	if l.ch != nil {
		l.ch <- struct{}{}
	}
}

type MultiSlotLimiter struct {
	m     map[string]*SlotLimiter
	slots int
}

func NewMultiSlotLimiter(slots int) MultiSlotLimiter {
	return MultiSlotLimiter{make(map[string]*SlotLimiter), slots}
}

func (l *MultiSlotLimiter) Slot(s string) *SlotLimiter {
	if l.slots == 0 {
		return nil
	}
	ll, ok := l.m[s]
	if !ok {
		tmp := NewSlotLimiter(l.slots)
		ll = &tmp
		l.m[s] = ll
	}
	return ll
}
