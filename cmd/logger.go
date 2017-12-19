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

package cmd

import (
	"encoding/json"
	"time"

	log "github.com/sirupsen/logrus"
)

// LogstashJSONFormatter defines a logstash json formatter
type LogstashJSONFormatter struct{}

func setupLogstash() {
	log.SetFormatter(&LogstashJSONFormatter{})
}

// Format returns a formatted logstash message
func (f *LogstashJSONFormatter) Format(entry *log.Entry) ([]byte, error) {
	e := make(map[string]interface{})
	for k, v := range entry.Data {
		if err, ok := v.(error); ok {
			// Store error string value instead of error.
			e[k] = err.Error()
		} else {
			e[k] = v
		}
	}

	e["@timestamp"] = entry.Time.Format(time.RFC3339)
	e["@version"] = "1"

	v, ok := entry.Data["message"]
	if ok {
		e["fields.message"] = v
	}
	e["message"] = entry.Message

	v, ok = entry.Data["level"]
	if ok {
		e["fields.level"] = v
	}
	e["level_name"] = entry.Level.String()

	serialised, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return append(serialised, '\n'), nil
}
