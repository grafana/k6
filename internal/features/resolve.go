package features

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"
)

// Source is one configuration surface (CLI, env, or JSON).
type Source struct {
	Values   []string
	Supplied bool
}

func parseInput(raw string) []string {
	parts := strings.Split(raw, ",")
	for i, s := range parts {
		parts[i] = strings.TrimSpace(s)
	}
	return slices.DeleteFunc(parts, func(s string) bool { return s == "" })
}

// resolveInto activates flags from the winning surface (CLI > env > JSON) onto target.
// target must be an addressable reflect.Value of defs.typ. Returns sorted activated names.
func resolveInto(defs *definitions,
	target reflect.Value,
	cli, env, json Source,
	logger logrus.FieldLogger,
) ([]string, error) {
	if target.Type() != defs.typ {
		return nil, fmt.Errorf("features: resolveInto target type %s does not match definition type %s",
			target.Type(), defs.typ)
	}

	var winner Source

	switch {
	case cli.Supplied:
		winner = cli
	case env.Supplied:
		winner = env
	case json.Supplied:
		winner = json
	default:
		return []string{}, nil
	}

	activated := make(map[string]struct{})

	seen := make(map[string]struct{})
	for _, name := range parseValues(winner.Values) {
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}

		if _, known := defs.byName[name]; !known {
			continue
		}

		activated[name] = struct{}{}
	}

	setFields(defs, target, activated)

	return sortedKeys(activated), nil
}

func parseValues(vals []string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		out = append(out, parseInput(v)...)
	}
	return out
}

func setFields(defs *definitions, target reflect.Value, names map[string]struct{}) {
	for name := range names {
		target.FieldByIndex(defs.definitions[defs.byName[name]].index).SetBool(true)
	}
}

func sortedKeys(m map[string]struct{}) []string {
	// keep a non-nil empty set so the activation set stays a stable []string
	if len(m) == 0 {
		return []string{}
	}
	return slices.Sorted(maps.Keys(m))
}
