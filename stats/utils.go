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

package stats

import (
	"strings"
)

func ParseThresholdName(name string) (string, string, map[string]string){
	parts := strings.SplitN(strings.TrimSuffix(name, "}"), "{", 2)
	if len(parts) == 1 {
		return parts[0], "", make(map[string]string, 0)
	}

	kvs := strings.Split(parts[1], ",")
	tags := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		if kv == "" {
			continue
		}
		parts := strings.SplitN(kv, ":", 2)

		key := strings.TrimSpace(strings.Trim(parts[0], `"'`))
		if len(parts) != 2 {
			tags[key] = ""
			continue
		}

		value := strings.TrimSpace(strings.Trim(parts[1], `"'`))
		tags[key] = value
	}
	return parts[0], parts[1], tags
}
