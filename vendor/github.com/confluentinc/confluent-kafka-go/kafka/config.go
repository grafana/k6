package kafka

/**
 * Copyright 2016 Confluent Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import (
	"fmt"
	"reflect"
	"strings"
	"unsafe"
)

/*
#include <stdlib.h>
#include <librdkafka/rdkafka.h>
*/
import "C"

// ConfigValue supports the following types:
//  bool, int, string, any type with the standard String() interface
type ConfigValue interface{}

// ConfigMap is a map contaning standard librdkafka configuration properties as documented in:
// https://github.com/edenhill/librdkafka/tree/master/CONFIGURATION.md
//
// The special property "default.topic.config" (optional) is a ConfigMap containing default topic
// configuration properties.
type ConfigMap map[string]ConfigValue

// SetKey sets configuration property key to value.
// For user convenience a key prefixed with {topic}. will be
// set on the "default.topic.config" sub-map.
func (m ConfigMap) SetKey(key string, value ConfigValue) error {
	if strings.HasPrefix(key, "{topic}.") {
		_, found := m["default.topic.config"]
		if !found {
			m["default.topic.config"] = ConfigMap{}
		}
		m["default.topic.config"].(ConfigMap)[strings.TrimPrefix(key, "{topic}.")] = value
	} else {
		m[key] = value
	}

	return nil
}

// Set implements flag.Set (command line argument parser) as a convenience
// for `-X key=value` config.
func (m ConfigMap) Set(kv string) error {
	i := strings.Index(kv, "=")
	if i == -1 {
		return Error{ErrInvalidArg, "Expected key=value"}
	}

	k := kv[:i]
	v := kv[i+1:]

	return m.SetKey(k, v)
}

func value2string(v ConfigValue) (ret string, errstr string) {

	switch x := v.(type) {
	case bool:
		if x {
			ret = "true"
		} else {
			ret = "false"
		}
	case int:
		ret = fmt.Sprintf("%d", x)
	case string:
		ret = x
	case fmt.Stringer:
		ret = x.String()
	default:
		return "", fmt.Sprintf("Invalid value type %T", v)
	}

	return ret, ""
}

// rdkAnyconf abstracts rd_kafka_conf_t and rd_kafka_topic_conf_t
// into a common interface.
type rdkAnyconf interface {
	set(cKey *C.char, cVal *C.char, cErrstr *C.char, errstrSize int) C.rd_kafka_conf_res_t
}

func anyconfSet(anyconf rdkAnyconf, key string, value string) (err error) {
	cKey := C.CString(key)
	cVal := C.CString(value)
	cErrstr := (*C.char)(C.malloc(C.size_t(128)))
	defer C.free(unsafe.Pointer(cErrstr))

	if anyconf.set(cKey, cVal, cErrstr, 128) != C.RD_KAFKA_CONF_OK {
		C.free(unsafe.Pointer(cKey))
		C.free(unsafe.Pointer(cVal))
		return newErrorFromCString(C.RD_KAFKA_RESP_ERR__INVALID_ARG, cErrstr)
	}

	return nil
}

// we need these typedefs to workaround a crash in golint
// when parsing the set() methods below
type rdkConf C.rd_kafka_conf_t
type rdkTopicConf C.rd_kafka_topic_conf_t

func (cConf *rdkConf) set(cKey *C.char, cVal *C.char, cErrstr *C.char, errstrSize int) C.rd_kafka_conf_res_t {
	return C.rd_kafka_conf_set((*C.rd_kafka_conf_t)(cConf), cKey, cVal, cErrstr, C.size_t(errstrSize))
}

func (ctopicConf *rdkTopicConf) set(cKey *C.char, cVal *C.char, cErrstr *C.char, errstrSize int) C.rd_kafka_conf_res_t {
	return C.rd_kafka_topic_conf_set((*C.rd_kafka_topic_conf_t)(ctopicConf), cKey, cVal, cErrstr, C.size_t(errstrSize))
}

func configConvertAnyconf(m ConfigMap, anyconf rdkAnyconf) (err error) {

	for k, v := range m {
		switch v.(type) {
		case ConfigMap:
			/* Special sub-ConfigMap, only used for default.topic.config */

			if k != "default.topic.config" {
				return Error{ErrInvalidArg, fmt.Sprintf("Invalid type for key %s", k)}
			}

			var cTopicConf = C.rd_kafka_topic_conf_new()

			err = configConvertAnyconf(v.(ConfigMap),
				(*rdkTopicConf)(cTopicConf))
			if err != nil {
				C.rd_kafka_topic_conf_destroy(cTopicConf)
				return err
			}

			C.rd_kafka_conf_set_default_topic_conf(
				(*C.rd_kafka_conf_t)(anyconf.(*rdkConf)),
				(*C.rd_kafka_topic_conf_t)((*rdkTopicConf)(cTopicConf)))

		default:
			val, errstr := value2string(v)
			if errstr != "" {
				return Error{ErrInvalidArg, fmt.Sprintf("%s for key %s (expected string,bool,int,ConfigMap)", errstr, k)}
			}

			err = anyconfSet(anyconf, k, val)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// convert ConfigMap to C rd_kafka_conf_t *
func (m ConfigMap) convert() (cConf *C.rd_kafka_conf_t, err error) {
	cConf = C.rd_kafka_conf_new()

	err = configConvertAnyconf(m, (*rdkConf)(cConf))
	if err != nil {
		C.rd_kafka_conf_destroy(cConf)
		return nil, err
	}
	return cConf, nil
}

// get finds key in the configmap and returns its value.
// If the key is not found defval is returned.
// If the key is found but the type is mismatched an error is returned.
func (m ConfigMap) get(key string, defval ConfigValue) (ConfigValue, error) {
	if strings.HasPrefix(key, "{topic}.") {
		defconfCv, found := m["default.topic.config"]
		if !found {
			return defval, nil
		}
		return defconfCv.(ConfigMap).get(strings.TrimPrefix(key, "{topic}."), defval)
	}

	v, ok := m[key]
	if !ok {
		return defval, nil
	}

	if defval != nil && reflect.TypeOf(defval) != reflect.TypeOf(v) {
		return nil, Error{ErrInvalidArg, fmt.Sprintf("%s expects type %T, not %T", key, defval, v)}
	}

	return v, nil
}

// extract performs a get() and if found deletes the key.
func (m ConfigMap) extract(key string, defval ConfigValue) (ConfigValue, error) {

	v, err := m.get(key, defval)
	if err != nil {
		return nil, err
	}

	delete(m, key)

	return v, nil
}

// Get finds the given key in the ConfigMap and returns its value.
// If the key is not found `defval` is returned.
// If the key is found but the type does not match that of `defval` (unless nil)
// an ErrInvalidArg error is returned.
func (m ConfigMap) Get(key string, defval ConfigValue) (ConfigValue, error) {
	return m.get(key, defval)
}
