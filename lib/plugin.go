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
	"fmt"
	goplugin "plugin"

	"github.com/loadimpact/k6/plugin"
)

// LoadJavaScriptPlugin tries to load a dynamic library that should conform to
// the `plug.JavaScriptPlugin` interface.
func LoadJavaScriptPlugin(path string) (plugin.JavaScriptPlugin, error) {
	p, err := loadPlugin(path)
	if err != nil {
		return nil, err
	}

	jsSym, err := p.Lookup("JavaScriptPlugin")
	if err != nil {
		return nil, err
	}

	var jsPlugin plugin.JavaScriptPlugin
	jsPlugin, ok := jsSym.(plugin.JavaScriptPlugin)
	if !ok {
		return nil, fmt.Errorf("could not cast plugin to the right type")
	}

	return jsPlugin, nil
}

func loadPlugin(path string) (*goplugin.Plugin, error) {
	p, err := goplugin.Open(path)
	if err != nil {
		err = fmt.Errorf("error while loading plugin: %w", err)
	}
	return p, err
}
