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
	"bytes"
	"encoding/json"
	"time"
)

type Duration time.Duration

func (d Duration) String() string {
	return time.Duration(d).String()
}

func (d *Duration) UnmarshalText(data []byte) error {
	v, err := time.ParseDuration(string(data))
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}

		v, err := time.ParseDuration(str)
		if err != nil {
			return err
		}

		*d = Duration(v)
	} else {
		var v time.Duration
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		*d = Duration(v)
	}

	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

type NullDuration struct {
	Duration
	Valid bool
}

func NullDurationFrom(d time.Duration) NullDuration {
	return NullDuration{Duration(d), true}
}

func (d *NullDuration) UnmarshalText(data []byte) error {
	if len(data) == 0 {
		*d = NullDuration{}
		return nil
	}
	if err := d.Duration.UnmarshalText(data); err != nil {
		return err
	}
	d.Valid = true
	return nil
}

func (d *NullDuration) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte(`null`)) {
		d.Valid = false
		return nil
	}
	if err := json.Unmarshal(data, &d.Duration); err != nil {
		return err
	}
	d.Valid = true
	return nil
}

func (d NullDuration) MarshalJSON() ([]byte, error) {
	if !d.Valid {
		return []byte(`null`), nil
	}
	return d.Duration.MarshalJSON()
}
