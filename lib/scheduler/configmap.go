/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package scheduler

import (
	"encoding/json"
	"fmt"
	"sync"
)

// ConfigMap can contain mixed scheduler config types
type ConfigMap map[string]Config

// ConfigConstructor is a simple function that returns a concrete Config instance
// with the specified name and all default values correctly initialized
type ConfigConstructor func(name string, rawJSON []byte) (Config, error)

//nolint:gochecknoglobals
var (
	configTypesMutex   sync.RWMutex
	configConstructors = make(map[string]ConfigConstructor)
)

// RegisterConfigType adds the supplied ConfigConstructor as the constructor for its
// type in the configConstructors map, in a thread-safe manner
func RegisterConfigType(configType string, constructor ConfigConstructor) {
	configTypesMutex.Lock()
	defer configTypesMutex.Unlock()

	if constructor == nil {
		panic("scheduler configs: constructor is nil")
	}
	if _, configTypeExists := configConstructors[configType]; configTypeExists {
		panic("scheduler configs: RegisterConfigType called twice for  " + configType)
	}

	configConstructors[configType] = constructor
}

// GetParsedConfig returns a struct instance corresponding to the supplied
// config type. It will be fully initialized - with both the default values of
// the type, as well as with whatever the user had specified in the JSON
func GetParsedConfig(name, configType string, rawJSON []byte) (result Config, err error) {
	configTypesMutex.Lock()
	defer configTypesMutex.Unlock()

	constructor, exists := configConstructors[configType]
	if !exists {
		return nil, fmt.Errorf("unknown execution scheduler type '%s'", configType)
	}
	return constructor(name, rawJSON)
}

// UnmarshalJSON implements the json.Unmarshaler interface in a two-step manner,
// creating the correct type of configs based on the `type` property.
func (scs *ConfigMap) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	if len(data) == 4 && string(data) == "null" {
		return nil
	}

	//TODO: use a more sophisticated combination of dec.Token() and dec.More(),
	// which would allow us to support both arrays and maps for this config?
	var protoConfigs map[string]protoConfig
	if err := strictJSONUnmarshal(data, &protoConfigs); err != nil {
		return err
	}

	result := make(ConfigMap, len(protoConfigs))
	for k, v := range protoConfigs {
		if v.Type == "" {
			return fmt.Errorf("execution config '%s' doesn't have a type value", k)
		}
		config, err := GetParsedConfig(k, v.Type, v.rawJSON)
		if err != nil {
			return err
		}
		result[k] = config
	}

	*scs = result

	return nil
}

// Validate checks if all of the specified scheduler options make sense
func (scs ConfigMap) Validate() (errors []error) {
	for name, scheduler := range scs {
		if schedErr := scheduler.Validate(); len(schedErr) != 0 {
			errors = append(errors,
				fmt.Errorf("scheduler %s has errors: %s", name, concatErrors(schedErr, ", ")))
		}
	}
	return errors
}

type protoConfig struct {
	BaseConfig
	rawJSON json.RawMessage
}

// UnmarshalJSON just reads unmarshals the base config (to get the type), but it also
// stores the unprocessed JSON so we can parse the full config in the next step
func (pc *protoConfig) UnmarshalJSON(b []byte) error {
	*pc = protoConfig{BaseConfig{}, b}
	return json.Unmarshal(b, &pc.BaseConfig)
}
