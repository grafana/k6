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

package plugin

// A Plugin ...
type Plugin interface {
	// Returns a user-friendly name for the plugin.
	Name() string

	// Gets the plugin in a state where it can be utilized in a load balancing test.
	Setup() error

	// Cleans up any state / files that need cleaning right before k6 shuts down.
	Teardown() error
}

type JavaScriptPlugin interface {
	Plugin

	// Returns a list of modules that this plugin exposes.
	GetModules() map[string]interface{}
}
