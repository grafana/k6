// Package trace lets JS modules (built-in or extensions) declare themselves
// as valid targets of the --tracing/K6_TRACING module list, and lets
// internal/cmd resolve and validate that list into an immutable Set.
package trace

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	// ErrUnknownModule indicates a --tracing entry that no module registered.
	ErrUnknownModule = errors.New("unknown tracing module")
	// ErrInvalidTracing indicates a malformed --tracing value, such as an
	// empty token or "all"/"none" combined with other module names.
	ErrInvalidTracing = errors.New("invalid tracing configuration")
)

//nolint:gochecknoglobals
var (
	mu         sync.Mutex
	registered = map[string]struct{}{}
)

// Register declares name as a valid --tracing/K6_TRACING target. Modules
// should call this from their own package init(). It panics if name is
// already registered, since that indicates a programming error rather than
// something to swallow silently.
func Register(name string) {
	mu.Lock()
	defer mu.Unlock()

	if _, ok := registered[name]; ok {
		panic(fmt.Sprintf("tracing module already registered: %s", name))
	}
	registered[name] = struct{}{}
}

// RegisteredNames returns the sorted list of all names registered so far.
func RegisteredNames() []string {
	return sortedKeys(registeredSnapshot())
}

func registeredSnapshot() map[string]struct{} {
	mu.Lock()
	defer mu.Unlock()

	out := make(map[string]struct{}, len(registered))
	for n := range registered {
		out[n] = struct{}{}
	}
	return out
}

// Set is the resolved, immutable list of modules whose tracing defaults to
// enabled for a run. The zero value Set is valid and equivalent to "none".
type Set struct {
	names map[string]struct{}
}

// Enabled reports whether name's tracing is enabled by default in this run.
func (s Set) Enabled(name string) bool {
	_, ok := s.names[name]
	return ok
}

// Any reports whether at least one module has tracing enabled. This is the
// signal used to decide whether a real (non-noop) TracerProvider is needed.
func (s Set) Any() bool {
	return len(s.names) > 0
}

// ParseSet validates raw (the --tracing/K6_TRACING value) against the names
// registered via Register, and returns the resolved Set.
//
// raw may be:
//   - "" or "none": nothing enabled (the zero Set).
//   - "all": every name registered via Register at call time.
//   - a comma-separated list of registered names, e.g. "http,grpc".
//
// Mixing "all"/"none" with other names, unknown module names, and empty
// tokens (e.g. a trailing comma) are all reported as errors.
func ParseSet(raw string) (Set, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "none" {
		return Set{}, nil
	}

	known := registeredSnapshot()

	if raw == "all" {
		return Set{names: known}, nil
	}

	names := make(map[string]struct{})
	for tok := range strings.SplitSeq(raw, ",") {
		tok = strings.TrimSpace(tok)
		switch tok {
		case "":
			return Set{}, fmt.Errorf("%w: empty module name in %q", ErrInvalidTracing, raw)
		case "all", "none":
			return Set{}, fmt.Errorf(
				"%w: %q cannot be combined with other module names in %q", ErrInvalidTracing, tok, raw,
			)
		}
		if _, ok := known[tok]; !ok {
			return Set{}, fmt.Errorf(
				"%w %q, valid values are: %s", ErrUnknownModule, tok, strings.Join(sortedKeys(known), ", "),
			)
		}
		names[tok] = struct{}{}
	}
	return Set{names: names}, nil
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
