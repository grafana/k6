package loader

import (
	"runtime"

	"go.k6.io/k6/lib/fsext"
)

// CreateFilesystems creates the correct filesystem map for the current OS
func CreateFilesystems(osfs fsext.Fs) map[string]fsext.Fs {
	// We want to eliminate disk access at runtime, so we set up a memory mapped cache that's
	// written every time something is read from the real filesystem. This cache is then used for
	// successive spawns to read from (they have no access to the real disk).
	// Also initialize the same for `https` but the caching is handled manually in the loader package
	if runtime.GOOS == "windows" {
		// This is done so that we can continue to use paths with /|"\" through the code but also to
		// be easier to traverse the cachedFs later as it doesn't work very well if you have windows
		// volumes
		osfs = fsext.NewTrimFilePathSeparatorFs(osfs)
	}
	return map[string]fsext.Fs{
		"file":  fsext.NewCacheOnReadFs(osfs, fsext.NewMemMapFs(), 0),
		"https": fsext.NewMemMapFs(),
	}
}
