/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package lib

import (
	"gopkg.in/guregu/null.v3"
	"sync"
	"sync/atomic"
	"time"
)

type Stage struct {
	Duration time.Duration `json:"duration"`
	Target   null.Int      `json:"target"`
}

type Group struct {
	ID int64

	Name   string
	Parent *Group
	Groups map[string]*Group
	Checks map[string]*Check

	groupMutex sync.Mutex
	checkMutex sync.Mutex
}

func NewGroup(name string, parent *Group, idCounter *int64) *Group {
	var id int64
	if idCounter != nil {
		id = atomic.AddInt64(idCounter, 1)
	}

	return &Group{
		ID:     id,
		Name:   name,
		Parent: parent,
		Groups: make(map[string]*Group),
		Checks: make(map[string]*Check),
	}
}

func (g *Group) Group(name string, idCounter *int64) (*Group, bool) {
	snapshot := g.Groups
	group, ok := snapshot[name]
	if !ok {
		g.groupMutex.Lock()
		group, ok = g.Groups[name]
		if !ok {
			group = NewGroup(name, g, idCounter)
			g.Groups[name] = group
		}
		g.groupMutex.Unlock()
	}
	return group, ok
}

func (g *Group) Check(name string, idCounter *int64) (*Check, bool) {
	snapshot := g.Checks
	check, ok := snapshot[name]
	if !ok {
		g.checkMutex.Lock()
		check, ok = g.Checks[name]
		if !ok {
			check = NewCheck(name, g, idCounter)
			g.Checks[name] = check
		}
		g.checkMutex.Unlock()
	}
	return check, ok
}

type Check struct {
	ID int64

	Group *Group
	Name  string

	Passes int64
	Fails  int64
}

func NewCheck(name string, group *Group, idCounter *int64) *Check {
	var id int64
	if idCounter != nil {
		id = atomic.AddInt64(idCounter, 1)
	}
	return &Check{ID: id, Name: name, Group: group}
}
