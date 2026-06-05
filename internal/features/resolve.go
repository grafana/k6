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
	var source string

	switch {
	case cli.Supplied:
		winner, source = cli, "cli"
	case env.Supplied:
		winner, source = env, "env"
	case json.Supplied:
		winner, source = json, "json"
	default:
		forceGA(defs, target)
		return []string{}, nil
	}

	activated := make(map[string]struct{})

	seen := make(map[string]struct{})
	for _, name := range parseValues(winner.Values) {
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}

		idx, known := defs.byName[name]
		if !known {
			logger.WithFields(logrus.Fields{
				"feature": name,
				"outcome": "unknown",
				"source":  source,
			}).Error("Unknown feature flag")
			continue
		}

		activated[name] = struct{}{}

		logActivation(logger, name, defs.definitions[idx].flag.Lifecycle)
	}

	setFields(defs, target, activated)
	forceGA(defs, target)

	return sortedKeys(activated), nil
}

func parseValues(vals []string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		out = append(out, parseInput(v)...)
	}
	return out
}

func logActivation(logger logrus.FieldLogger, name string, lc Lifecycle) {
	switch lc {
	case Experimental:
		logger.WithFields(logrus.Fields{"feature": name, "lifecycle": "experimental"}).
			Info("Experimental feature enabled")
	case GA:
		logger.WithFields(logrus.Fields{"feature": name, "lifecycle": "ga"}).
			Info("Feature is now available by default, please remove this flag")
	case Deprecated:
		logger.WithFields(logrus.Fields{"feature": name, "lifecycle": "deprecated"}).
			Warn("Deprecated feature enabled")
	}
}

func setFields(defs *definitions, target reflect.Value, names map[string]struct{}) {
	for name := range names {
		target.FieldByIndex(defs.definitions[defs.byName[name]].index).SetBool(true)
	}
}

func forceGA(defs *definitions, target reflect.Value) {
	for _, def := range defs.definitions {
		if def.flag.Lifecycle == GA {
			target.FieldByIndex(def.index).SetBool(true)
		}
	}
}

func sortedKeys(m map[string]struct{}) []string {
	// keep a non-nil empty set so the activation set stays a stable []string
	if len(m) == 0 {
		return []string{}
	}
	return slices.Sorted(maps.Keys(m))
}
