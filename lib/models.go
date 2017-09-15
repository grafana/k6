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
	"crypto/md5"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/guregu/null.v3"
)

const groupSeparator = "::"

var ErrNameContainsGroupSeparator = errors.Errorf("group and check names may not contain '%s'", groupSeparator)

type SourceData struct {
	Data     []byte
	Filename string
}

type Stage struct {
	Duration NullDuration `json:"duration"`
	Target   null.Int     `json:"target"`
}

func (s *Stage) UnmarshalText(b []byte) error {
	var stage Stage
	parts := strings.SplitN(string(b), ":", 2)
	if len(parts) > 0 && parts[0] != "" {
		d, err := time.ParseDuration(parts[0])
		if err != nil {
			return err
		}
		stage.Duration = NullDurationFrom(d)
	}
	if len(parts) > 1 && parts[1] != "" {
		t, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return err
		}
		stage.Target = null.IntFrom(t)
	}
	*s = stage
	return nil
}

type Group struct {
	ID     string            `json:"id"`
	Path   string            `json:"path"`
	Name   string            `json:"name"`
	Parent *Group            `json:"parent"`
	Groups map[string]*Group `json:"groups"`
	Checks map[string]*Check `json:"checks"`

	groupMutex sync.Mutex
	checkMutex sync.Mutex
}

func NewGroup(name string, parent *Group) (*Group, error) {
	if strings.Contains(name, groupSeparator) {
		return nil, ErrNameContainsGroupSeparator
	}

	path := name
	if parent != nil {
		path = parent.Path + groupSeparator + path
	}

	hash := md5.Sum([]byte(path))
	id := hex.EncodeToString(hash[:])

	return &Group{
		ID:     id,
		Path:   path,
		Name:   name,
		Parent: parent,
		Groups: make(map[string]*Group),
		Checks: make(map[string]*Check),
	}, nil
}

func (g *Group) Group(name string) (*Group, error) {
	snapshot := g.Groups
	group, ok := snapshot[name]
	if !ok {
		g.groupMutex.Lock()
		defer g.groupMutex.Unlock()

		group, ok := g.Groups[name]
		if !ok {
			group, err := NewGroup(name, g)
			if err != nil {
				return nil, err
			}
			g.Groups[name] = group
			return group, nil
		}
		return group, nil
	}
	return group, nil
}

func (g *Group) Check(name string) (*Check, error) {
	snapshot := g.Checks
	check, ok := snapshot[name]
	if !ok {
		g.checkMutex.Lock()
		defer g.checkMutex.Unlock()
		check, ok := g.Checks[name]
		if !ok {
			check, err := NewCheck(name, g)
			if err != nil {
				return nil, err
			}
			g.Checks[name] = check
			return check, nil
		}
		return check, nil
	}
	return check, nil
}

type Check struct {
	ID    string `json:"id"`
	Path  string `json:"path"`
	Group *Group `json:"group"`
	Name  string `json:"name"`

	Passes int64 `json:"passes"`
	Fails  int64 `json:"fails"`
}

func NewCheck(name string, group *Group) (*Check, error) {
	if strings.Contains(name, groupSeparator) {
		return nil, ErrNameContainsGroupSeparator
	}

	path := group.Path + groupSeparator + name
	hash := md5.Sum([]byte(path))
	id := hex.EncodeToString(hash[:])

	return &Check{
		ID:    id,
		Path:  path,
		Group: group,
		Name:  name,
	}, nil
}
