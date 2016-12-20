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

package v2

import (
	"github.com/loadimpact/k6/lib"
	"gopkg.in/guregu/null.v3"
)

type Status struct {
	Running null.Bool `json:"running"`
	VUs     null.Int  `json:"vus"`
	VUsMax  null.Int  `json:"vus-max"`

	// Readonly.
	Tainted bool `json:"tainted"`
}

func NewStatus(engine *lib.Engine) Status {
	return Status{
		Running: null.BoolFrom(engine.Status.Running.Bool),
		VUs:     null.IntFrom(engine.Status.VUs.Int64),
		VUsMax:  null.IntFrom(engine.Status.VUsMax.Int64),
		Tainted: engine.Status.Tainted.Bool,
	}
}

func (s Status) GetName() string {
	return "status"
}

func (s Status) GetID() string {
	return "default"
}

func (s Status) SetID(id string) error {
	return nil
}
