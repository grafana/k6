/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
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

// Package lib is a kitchen sink of... basically anything that doesn't belong in
// a specific part of the codebase, ranging from utility functions to universal
// types to core interfaces.
//
// Some of the things in lib do not actually belong in lib, and need to be moved
// into either submodules, or separate modules like core. Other things are in
// files that are far too long, or that do not actually make sense.
//
// Feel free to move these things.
package lib
