package cmd

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
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

// defaultCatalogURL returns the canonical k6 extension registry catalog URL
// for the running k6 binary's major version.
func defaultCatalogURL() string {
	return fmt.Sprintf("https://registry.k6.io/%s/catalog.json", catalogMajorVersion())
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

// catalogURL returns the extension registry catalog URL, honouring the
// K6_PROVISION_CATALOG_URL override.
func catalogURL(gs *state.GlobalState) string {
	return cmp.Or(gs.Env[state.ProvisionCatalogURL], defaultCatalogURL())
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
	raw, err := readCachedCatalogBytes(gs, cachePath)
	if raw == nil || err != nil {
		return nil, err
	}
	var subs registrySubcommands
	if err := json.Unmarshal(raw, &subs); err != nil {
		return nil, fmt.Errorf("parsing extensions cache: %w", err)
	}
	return subs, nil
}

// fetchCatalog downloads and parses the k6 extension registry catalog from
// url. Returns the parsed subcommands plus the raw bytes so the caller can
// persist them to the cache without re-marshaling.
func fetchCatalog(ctx context.Context, url string) (registrySubcommands, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("requesting extensions catalog: %w", err)
	}
	// The URL can be overridden via env for tests and development; that is the
	// intended behaviour of the catalog override, not an SSRF vector.
	resp, err := http.DefaultClient.Do(req) //nolint:gosec
	if err != nil {
		return nil, nil, fmt.Errorf("fetching extensions catalog: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("fetching extensions catalog: status %s", resp.Status)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("reading extensions catalog response: %w", err)
	}
	var subs registrySubcommands
	if err := json.Unmarshal(raw, &subs); err != nil {
		return nil, nil, fmt.Errorf("parsing extensions catalog: %w", err)
	}
	return subs, raw, nil
}

// catalogModulePaths returns the set of Go module paths advertised in the k6
// extension registry catalog. It shares the same cache file, TTL, and
// K6_PROVISION_CATALOG_URL override as `k6 x`: it reads the on-disk cache and,
// on a miss, fetches the catalog and writes the fresh bytes back. The caller
// bounds ctx so the fetch stays non-fatal and off the run's hot path; any
// failure yields an empty set.
func catalogModulePaths(ctx context.Context, gs *state.GlobalState) map[string]struct{} {
	cachePath := catalogCachePath(gs)
	raw, err := readCachedCatalogBytes(gs, cachePath)
	if err != nil {
		gs.Logger.WithError(err).Debug("k6 extension catalog")
	}
	if raw == nil {
		raw, err = fetchCatalogBytes(ctx, gs, cachePath)
		if err != nil {
			gs.Logger.WithError(err).Debug("k6 extension catalog")
			return nil
		}
	}
	return parseCatalogModulePaths(raw)
}

// readCachedCatalogBytes returns the raw catalog cache bytes when the cache
// exists and is fresh, honouring the K6_PROVISION_CATALOG_TTL override. A miss
// (absent or stale cache) returns (nil, nil); real I/O failures return an
// error so the caller can surface them at debug.
func readCachedCatalogBytes(gs *state.GlobalState, cachePath string) ([]byte, error) {
	maxAge := 24 * time.Hour
	if d, err := time.ParseDuration(gs.Env[state.ProvisionCatalogTTL]); err == nil {
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
	return raw, nil
}

// fetchCatalogBytes downloads the catalog exactly as `k6 x` does, honouring the
// K6_PROVISION_CATALOG_URL override, and writes the fresh bytes to the cache.
func fetchCatalogBytes(ctx context.Context, gs *state.GlobalState, cachePath string) ([]byte, error) {
	_, raw, err := fetchCatalog(ctx, catalogURL(gs))
	if err != nil {
		return nil, err
	}
	if err := writeCachedCatalog(gs, cachePath, raw); err != nil {
		gs.Logger.WithError(err).Debug("k6 extension catalog")
	}
	return raw, nil
}

// parseCatalogModulePaths reads the per-entry "module" field the
// registrySubcommands decoder drops, collecting each catalog entry's Go module
// path into a set. Entries without a module path are skipped.
func parseCatalogModulePaths(raw []byte) map[string]struct{} {
	var catalog map[string]struct {
		Module string `json:"module"`
	}
	if err := json.Unmarshal(raw, &catalog); err != nil {
		return nil
	}
	paths := make(map[string]struct{}, len(catalog))
	for _, e := range catalog {
		if e.Module != "" {
			paths[e.Module] = struct{}{}
		}
	}
	return paths
}

// writeCachedCatalog persists raw catalog bytes under cachePath, creating any
// missing parent directories.
func writeCachedCatalog(gs *state.GlobalState, cachePath string, raw []byte) error {
	if err := gs.FS.MkdirAll(filepath.Dir(cachePath), 0o750); err != nil {
		return fmt.Errorf("creating extensions cache dir: %w", err)
	}
	if err := fsext.WriteFile(gs.FS, cachePath, raw, 0o600); err != nil {
		return fmt.Errorf("writing extensions cache: %w", err)
	}
	return nil
}
