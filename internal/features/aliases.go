package features

import (
	"fmt"
	"strconv"

	"github.com/sirupsen/logrus"
)

type aliasPhase int

const (
	honored aliasPhase = iota
	tombstoned
	removed
)

type alias struct {
	EnvVar    string
	Canonical string
	Phase     aliasPhase
}

func defaultAliases() []alias {
	return nil
}

func registerAlias(defs *definitions, a alias) error {
	if !kebabRe.MatchString(a.Canonical) {
		return fmt.Errorf("alias %s: canonical name %q is not valid kebab-case", a.EnvVar, a.Canonical)
	}
	for _, existing := range defs.aliases {
		if existing.EnvVar == a.EnvVar {
			return fmt.Errorf("alias %s: already registered", a.EnvVar)
		}
	}
	defs.aliases = append(defs.aliases, a)
	return nil
}

func resolveEnvAliases(defs *definitions,
	env map[string]string, logger logrus.FieldLogger,
) (names []string, supplied bool, aliasOf map[string]string) {
	aliasOf = make(map[string]string)
	for _, a := range defs.aliases {
		val, set := env[a.EnvVar]
		if !set {
			continue
		}

		switch a.Phase {
		case honored:
			if b, err := strconv.ParseBool(val); err != nil || !b {
				continue
			}
			names = append(names, a.Canonical)
			aliasOf[a.Canonical] = a.EnvVar
			supplied = true

		case tombstoned:
			logger.WithFields(logrus.Fields{
				"env":     a.EnvVar,
				"source":  "env_tombstoned_alias",
				"outcome": "unknown",
			}).Error("Legacy env var is no longer supported, use --features or K6_FEATURES instead")

		case removed:
		}
	}

	return names, supplied, aliasOf
}

func resolveEnvSurface(defs *definitions, osEnv map[string]string, logger logrus.FieldLogger) Source {
	names, aliasSupplied, aliasOf := resolveEnvAliases(defs, osEnv, logger)

	k6Features, k6FeaturesSet := osEnv["K6_FEATURES"]
	if k6FeaturesSet {
		names = append(names, k6Features)
	}

	return Source{
		Values:   names,
		Supplied: k6FeaturesSet || aliasSupplied,
		aliasOf:  aliasOf,
	}
}
