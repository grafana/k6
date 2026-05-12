package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/lib/fsext"
)

// registrySubcommand is the projection of one entry in the k6 extension
// registry catalog that `k6 x` advertises to users: a subcommand name and a
// one-line description.
type registrySubcommand struct {
	Name  string
	Short string
}

type registrySubcommands []registrySubcommand

type catalogEntry struct {
	Subcommands []string `json:"subcommands"`
	Description string   `json:"description"`
	Short       string   `json:"short"`
}

// UnmarshalJSON flattens the registry's name→entry map into one
// registrySubcommand per advertised subcommand, falling back to Description
// when the forward-compatible Short field is missing.
func (r *registrySubcommands) UnmarshalJSON(data []byte) error {
	var catalog map[string]catalogEntry
	if err := json.Unmarshal(data, &catalog); err != nil {
		return err
	}
	for _, e := range catalog {
		short := e.Short
		if short == "" {
			short = e.Description
		}
		for _, name := range e.Subcommands {
			*r = append(*r, registrySubcommand{Name: name, Short: short})
		}
	}
	return nil
}

// catalogMajorVersion returns the registry path segment (e.g. "v2") matching
// the running k6 binary, so the catalog endpoint and on-disk cache track the
// k6 vN release line without manual coordination.
func catalogMajorVersion() string {
	v := runtimeK6Version()
	if dot := strings.Index(v[1:], "."); dot >= 0 {
		return v[:dot+1]
	}
	return v
}

// catalogCachePath returns the on-disk location of the k6 extension registry
// catalog cache, alongside the build cache so both share the user cache dir.
func catalogCachePath(gs *state.GlobalState) string {
	return filepath.Join(filepath.Dir(gs.Flags.BinaryCache), catalogMajorVersion(), "catalog.json")
}

// readCachedCatalog returns subcommands parsed from the on-disk extensions
// cache. A missing or stale cache returns (nil, nil); real I/O or parse
// failures return an error so the caller can surface them at debug.
func readCachedCatalog(gs *state.GlobalState, cachePath string) (registrySubcommands, error) {
	maxAge := 24 * time.Hour
	if d, err := time.ParseDuration(gs.Env[state.ProvisionCatalogCacheAge]); err == nil {
		maxAge = d
	}
	info, err := gs.FS.Stat(cachePath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("checking extensions cache: %w", err)
	}
	if time.Since(info.ModTime()) > maxAge {
		return nil, nil
	}
	raw, err := fsext.ReadFile(gs.FS, cachePath)
	if err != nil {
		return nil, fmt.Errorf("reading extensions cache: %w", err)
	}
	var subs registrySubcommands
	if err := json.Unmarshal(raw, &subs); err != nil {
		return nil, fmt.Errorf("parsing extensions cache: %w", err)
	}
	return subs, nil
}
