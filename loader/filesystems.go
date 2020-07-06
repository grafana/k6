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

package loader

import (
	"runtime"

	"github.com/spf13/afero"

	"github.com/loadimpact/k6/lib/fsext"
)

// CreateFilesystems creates the correct filesystem map for the current OS
func CreateFilesystems() map[string]afero.Fs {
	// We want to eliminate disk access at runtime, so we set up a memory mapped cache that's
	// written every time something is read from the real filesystem. This cache is then used for
	// successive spawns to read from (they have no access to the real disk).
	// Also initialize the same for `https` but the caching is handled manually in the loader package
	osfs := afero.NewOsFs()
	if runtime.GOOS == "windows" {
		// This is done so that we can continue to use paths with /|"\" through the code but also to
		// be easier to traverse the cachedFs later as it doesn't work very well if you have windows
		// volumes
		osfs = fsext.NewTrimFilePathSeparatorFs(osfs)
	}
	return map[string]afero.Fs{
		"file":  fsext.NewCacheOnReadFs(osfs, afero.NewMemMapFs(), 0),
		"https": afero.NewMemMapFs(),
	}
}
