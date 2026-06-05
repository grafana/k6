// Package features implements a feature flag system for k6.
package features

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unicode"
)

// Lifecycle represents the maturity stage of a feature flag.
type Lifecycle int

const (
	// Experimental flags are opt-in and may change without notice.
	Experimental Lifecycle = iota
	// GA flags are generally available and enabled by default.
	GA
	// Deprecated flags are scheduled for removal.
	Deprecated
)

// String returns the title-case lifecycle name.
func (l Lifecycle) String() string {
	switch l {
	case Experimental:
		return "Experimental"
	case GA:
		return "GA"
	case Deprecated:
		return "Deprecated"
	default:
		return "Unknown"
	}
}

// Flags declares every feature flag as a tagged bool field.
type Flags struct {
	_         noCopy
	activated []string
}

// Activated returns a copy of the sorted canonical names of active features.
func (f *Flags) Activated() []string {
	out := make([]string, len(f.activated))
	copy(out, f.activated)
	return out
}

// Tags returns per-flag metric tags (k6_feature_<snake>="true") for active features.
func (f *Flags) Tags() map[string]string {
	tags := make(map[string]string, len(f.activated))
	for _, name := range f.activated {
		snake := strings.ReplaceAll(name, "-", "_")
		tags["k6_feature_"+snake] = "true"
	}
	return tags
}

// Flag is a single feature flag's metadata.
type Flag struct {
	Name        string
	Lifecycle   Lifecycle
	Description string
}

// definition holds the parsed metadata and field index for one flag field.
type definition struct {
	flag  Flag
	index []int
}

// definitions holds parsed flag metadata for a specific struct type.
type definitions struct {
	typ         reflect.Type
	definitions []definition
	byName      map[string]int
}

var kebabRe = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

func deriveKebabName(fieldName string) string {
	var b strings.Builder
	for i, r := range fieldName {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteByte('-')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func parseLifecycle(s string) (Lifecycle, bool) {
	switch s {
	case "experimental":
		return Experimental, true
	case "ga":
		return GA, true
	case "deprecated":
		return Deprecated, true
	default:
		return 0, false
	}
}

// parseDefinitions reflects over a struct type to build a definitions.
func parseDefinitions(t reflect.Type) (*definitions, error) {
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("features: parseDefinitions requires a struct type, got %s", t.Kind())
	}

	ds := &definitions{
		typ:    t,
		byName: make(map[string]int),
	}

	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		if f.Type.Kind() != reflect.Bool {
			return nil, fmt.Errorf("field %s: feature flags must be bool, got %s", f.Name, f.Type.Kind())
		}

		lcTag, ok := f.Tag.Lookup("lifecycle")
		if !ok {
			return nil, fmt.Errorf("field %s: missing lifecycle tag", f.Name)
		}
		lc, valid := parseLifecycle(lcTag)
		if !valid {
			return nil, fmt.Errorf("field %s: invalid lifecycle %q", f.Name, lcTag)
		}

		help := f.Tag.Get("help")
		if help == "" {
			return nil, fmt.Errorf("field %s: missing or empty help tag", f.Name)
		}

		name := f.Tag.Get("name")
		if name == "" {
			name = deriveKebabName(f.Name)
		}
		if !kebabRe.MatchString(name) {
			return nil, fmt.Errorf("field %s: name %q is not valid kebab-case", f.Name, name)
		}

		if _, exists := ds.byName[name]; exists {
			return nil, fmt.Errorf("field %s: duplicate canonical name %q", f.Name, name)
		}

		idx := len(ds.definitions)
		ds.definitions = append(ds.definitions, definition{
			flag: Flag{
				Name:        name,
				Lifecycle:   lc,
				Description: help,
			},
			index: f.Index,
		})
		ds.byName[name] = idx
	}

	return ds, nil
}

// noCopy prevents Flags from being copied by value.
type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}
