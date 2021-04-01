/*
 *
 * k6 - a next-generation load testing tool
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

package kafka

import (
	"fmt"
	"strconv"
	"strings"

	client "github.com/influxdata/influxdb1-client/v2"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
)

type extractTagsToValuesFunc func(map[string]string, map[string]interface{}) map[string]interface{}

// format returns a string array of metrics in influx line-protocol
func formatAsInfluxdbV1(
	logger logrus.FieldLogger, samples []stats.Sample, extractTagsToValues extractTagsToValuesFunc,
) ([]string, error) {
	var metrics []string
	type cacheItem struct {
		tags   map[string]string
		values map[string]interface{}
	}
	cache := map[*stats.SampleTags]cacheItem{}
	for _, sample := range samples {
		var tags map[string]string
		values := make(map[string]interface{})
		if cached, ok := cache[sample.Tags]; ok {
			tags = cached.tags
			for k, v := range cached.values {
				values[k] = v
			}
		} else {
			tags = sample.Tags.CloneTags()
			extractTagsToValues(tags, values)
			cache[sample.Tags] = cacheItem{tags, values}
		}
		values["value"] = sample.Value
		p, err := client.NewPoint(
			sample.Metric.Name,
			tags,
			values,
			sample.Time,
		)
		if err != nil {
			logger.WithError(err).Error("InfluxDB: Couldn't make point from sample!")
			return nil, err
		}
		metrics = append(metrics, p.String())
	}

	return metrics, nil
}

// FieldKind defines Enum for tag-to-field type conversion
type FieldKind int

const (
	// String field (default)
	String FieldKind = iota
	// Int field
	Int
	// Float field
	Float
	// Bool field
	Bool
)

func newExtractTagsFields(fieldKinds map[string]FieldKind) extractTagsToValuesFunc {
	return func(tags map[string]string, values map[string]interface{}) map[string]interface{} {
		for tag, kind := range fieldKinds {
			if val, ok := tags[tag]; ok {
				var v interface{}
				var err error

				switch kind {
				case String:
					v = val
				case Bool:
					v, err = strconv.ParseBool(val)
				case Float:
					v, err = strconv.ParseFloat(val, 64)
				case Int:
					v, err = strconv.ParseInt(val, 10, 64)
				}
				if err == nil {
					values[tag] = v
				} else {
					values[tag] = val
				}

				delete(tags, tag)
			}
		}
		return values
	}
}

// makeFieldKinds reads the Config and returns a lookup map of tag names to
// the field type their values should be converted to.
func makeInfluxdbFieldKinds(tagsAsFields []string) (map[string]FieldKind, error) {
	fieldKinds := make(map[string]FieldKind)
	for _, tag := range tagsAsFields {
		var fieldName, fieldType string
		s := strings.SplitN(tag, ":", 2)
		if len(s) == 1 {
			fieldName, fieldType = s[0], "string"
		} else {
			fieldName, fieldType = s[0], s[1]
		}

		err := checkDuplicatedTypeDefinitions(fieldKinds, fieldName)
		if err != nil {
			return nil, err
		}

		switch fieldType {
		case "string":
			fieldKinds[fieldName] = String
		case "bool":
			fieldKinds[fieldName] = Bool
		case "float":
			fieldKinds[fieldName] = Float
		case "int":
			fieldKinds[fieldName] = Int
		default:
			return nil, fmt.Errorf("An invalid type (%s) is specified for an InfluxDB field (%s).",
				fieldType, fieldName)
		}
	}

	return fieldKinds, nil
}

func checkDuplicatedTypeDefinitions(fieldKinds map[string]FieldKind, tag string) error {
	if _, found := fieldKinds[tag]; found {
		return fmt.Errorf("A tag name (%s) shows up more than once in InfluxDB field type configurations.", tag)
	}
	return nil
}

func (c influxdbConfig) Apply(cfg influxdbConfig) influxdbConfig {
	if len(cfg.TagsAsFields) > 0 {
		c.TagsAsFields = cfg.TagsAsFields
	}
	return c
}

// ParseMap parses a map[string]interface{} into a Config
func influxdbParseMap(m map[string]interface{}) (influxdbConfig, error) {
	c := influxdbConfig{}
	if v, ok := m["tagsAsFields"].(string); ok {
		m["tagsAsFields"] = []string{v}
	}
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: types.NullDecoder,
		Result:     &c,
	})
	if err != nil {
		return c, err
	}

	err = dec.Decode(m)
	return c, err
}

type influxdbConfig struct {
	TagsAsFields []string `json:"tagsAsFields,omitempty" envconfig:"K6_INFLUXDB_TAGS_AS_FIELDS"`
}

func newInfluxdbConfig() influxdbConfig {
	c := influxdbConfig{
		TagsAsFields: []string{"vu", "iter", "url"},
	}
	return c
}
