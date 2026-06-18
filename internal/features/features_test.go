package features

import (
	"reflect"
	"testing"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testFlags struct {
	Alpha bool `lifecycle:"experimental" help:"a"`
	Beta  bool `lifecycle:"experimental" help:"b"`
	Gamma bool `lifecycle:"ga" help:"g"`
	Delta bool `lifecycle:"deprecated" help:"d"`
}

const aliasEnv = "K6_TEST_ALIAS"

func TestFeatureDefinitionNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		typ  reflect.Type
		want string
	}{
		{
			name: "camel case",
			typ: reflect.TypeOf(struct {
				MultiWordFeature bool `lifecycle:"experimental" help:"x"`
			}{}),
			want: "multi-word-feature",
		},
		{
			name: "name override",
			typ: reflect.TypeOf(struct {
				HTTPCache bool `lifecycle:"experimental" help:"x" name:"http-cache"`
			}{}),
			want: "http-cache",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			defs, err := parseDefinitions(tt.typ)
			require.NoError(t, err)

			require.Len(t, defs.definitions, 1)
			assert.Equal(t, tt.want, defs.definitions[0].flag.Name)
		})
	}
}

func TestFeatureDefinitionsRejectInvalidStructs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		typ  reflect.Type
	}{
		{
			name: "non struct",
			typ:  reflect.TypeFor[int](),
		},
		{
			name: "non bool field",
			typ: reflect.TypeOf(struct {
				Foo string `lifecycle:"experimental" help:"x"`
			}{}),
		},
		{
			name: "missing lifecycle",
			typ: reflect.TypeOf(struct {
				Foo bool `help:"x"`
			}{}),
		},
		{
			name: "invalid lifecycle",
			typ: reflect.TypeOf(struct {
				Foo bool `lifecycle:"beta" help:"x"`
			}{}),
		},
		{
			name: "missing help",
			typ: reflect.TypeOf(struct {
				Foo bool `lifecycle:"experimental"`
			}{}),
		},
		{
			name: "invalid name",
			typ: reflect.TypeOf(struct {
				Foo bool `lifecycle:"experimental" help:"x" name:"Bad-Name"`
			}{}),
		},
		{
			name: "duplicate name",
			typ: reflect.TypeOf(struct {
				Foo bool `lifecycle:"experimental" help:"x" name:"dup"`
				Bar bool `lifecycle:"experimental" help:"y" name:"dup"`
			}{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseDefinitions(tt.typ)

			assert.Error(t, err)
		})
	}
}

func TestFeatureDefinitions(t *testing.T) {
	t.Parallel()

	flags, err := allOf(reflect.TypeFor[testFlags]())
	require.NoError(t, err)

	assert.Equal(t, []Flag{
		{Name: "alpha", Lifecycle: Experimental, Description: "a"},
		{Name: "beta", Lifecycle: Experimental, Description: "b"},
		{Name: "delta", Lifecycle: Deprecated, Description: "d"},
		{Name: "gamma", Lifecycle: GA, Description: "g"},
	}, flags)
}

func TestResolveFeatures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cli        Source
		env        Source
		json       Source
		wantActive []string
		wantFlags  testFlags
	}{
		{
			name:       "cli wins over env and json",
			cli:        supplied("alpha"),
			env:        supplied("beta"),
			json:       supplied("delta"),
			wantActive: []string{"alpha"},
			wantFlags:  testFlags{Alpha: true, Gamma: true},
		},
		{
			name:       "env wins over json and splits values",
			env:        supplied(" alpha, beta,, "),
			json:       supplied("delta"),
			wantActive: []string{"alpha", "beta"},
			wantFlags:  testFlags{Alpha: true, Beta: true, Gamma: true},
		},
		{
			name:       "empty supplied cli clears lower surfaces",
			cli:        Source{Supplied: true},
			env:        supplied("alpha"),
			json:       supplied("beta"),
			wantActive: []string{},
			wantFlags:  testFlags{Gamma: true},
		},
		{
			name:       "no surface activates only ga fields",
			wantActive: []string{},
			wantFlags:  testFlags{Gamma: true},
		},
		{
			name:       "unknown names stay inactive",
			cli:        supplied("not-a-flag"),
			wantActive: []string{},
			wantFlags:  testFlags{Gamma: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			flags, active := resolveTestFlags(t, tt.cli, tt.env, tt.json)

			assert.Equal(t, tt.wantActive, active)
			assert.Equal(t, tt.wantFlags, *flags)
		})
	}
}

func TestResolveLogs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		feature   string
		wantLevel logrus.Level
		lifecycle string
	}{
		{name: "experimental", feature: "alpha", wantLevel: logrus.InfoLevel, lifecycle: "experimental"},
		{name: "ga", feature: "gamma", wantLevel: logrus.InfoLevel, lifecycle: "ga"},
		{name: "deprecated", feature: "delta", wantLevel: logrus.WarnLevel, lifecycle: "deprecated"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hook := resolveTestLogs(t, supplied(tt.feature), Source{}, Source{})

			entry := assertSingleLog(t, hook, tt.wantLevel)
			assert.Equal(t, tt.feature, entry.Data["feature"])
			assert.Equal(t, tt.lifecycle, entry.Data["lifecycle"])
		})
	}
}

func TestResolveLogsUnknownNames(t *testing.T) {
	t.Parallel()

	hook := resolveTestLogs(t, supplied("not-a-flag"), Source{}, Source{})

	entry := assertSingleLog(t, hook, logrus.ErrorLevel)
	assert.Equal(t, "not-a-flag", entry.Data["feature"])
	assert.Equal(t, "unknown", entry.Data["outcome"])
	assert.Equal(t, "cli", entry.Data["source"])
}

func TestInitIntoResolvesAliases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cli        Source
		env        map[string]string
		aliases    []alias
		wantActive []string
		wantFlags  testFlags
	}{
		{
			name:       "honored alias activates when env wins",
			env:        map[string]string{aliasEnv: "true"},
			aliases:    []alias{{EnvVar: aliasEnv, Canonical: "alpha", Phase: honored}},
			wantActive: []string{"alpha"},
			wantFlags:  testFlags{Alpha: true, Gamma: true},
		},
		{
			name:       "cli wins over alias env",
			cli:        supplied("beta"),
			env:        map[string]string{aliasEnv: "true"},
			aliases:    []alias{{EnvVar: aliasEnv, Canonical: "alpha", Phase: honored}},
			wantActive: []string{"beta"},
			wantFlags:  testFlags{Beta: true, Gamma: true},
		},
		{
			name:       "dangling alias canonical stays inactive",
			env:        map[string]string{aliasEnv: "true"},
			aliases:    []alias{{EnvVar: aliasEnv, Canonical: "gone-feature", Phase: honored}},
			wantActive: []string{},
			wantFlags:  testFlags{Gamma: true},
		},
		{
			name:       "tombstoned alias does not activate",
			env:        map[string]string{aliasEnv: "true"},
			aliases:    []alias{{EnvVar: aliasEnv, Canonical: "alpha", Phase: tombstoned}},
			wantActive: []string{},
			wantFlags:  testFlags{Gamma: true},
		},
		{
			name:       "removed alias is ignored",
			env:        map[string]string{aliasEnv: "true"},
			aliases:    []alias{{EnvVar: aliasEnv, Canonical: "alpha", Phase: removed}},
			wantActive: []string{},
			wantFlags:  testFlags{Gamma: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			flags, active := runTestInit(t, tt.cli, Source{}, tt.env, tt.aliases)

			assert.Equal(t, tt.wantActive, active)
			assert.Equal(t, tt.wantFlags, *flags)
		})
	}
}

func TestInitIntoLogsAliases(t *testing.T) {
	t.Parallel()

	hook := runTestInitForLogs(t, Source{}, map[string]string{aliasEnv: "true"},
		[]alias{{EnvVar: aliasEnv, Canonical: "alpha", Phase: honored}})

	warn := entryByLevel(t, hook, logrus.WarnLevel)
	assert.Equal(t, "alpha", warn.Data["feature"])
	assert.Equal(t, "env_legacy_alias", warn.Data["source"])
	assert.Equal(t, "deprecated_alias", warn.Data["lifecycle"])
	assert.Equal(t, aliasEnv, warn.Data["env"])

	info := entryByLevel(t, hook, logrus.InfoLevel)
	assert.Equal(t, "alpha", info.Data["feature"])
	assert.Equal(t, "experimental", info.Data["lifecycle"])
}

func TestInitIntoLogsRetiredAliases(t *testing.T) {
	t.Parallel()

	t.Run("tombstoned", func(t *testing.T) {
		t.Parallel()

		hook := runTestInitForLogs(t, Source{}, map[string]string{aliasEnv: "true"},
			[]alias{{EnvVar: aliasEnv, Canonical: "alpha", Phase: tombstoned}})

		entry := assertSingleLog(t, hook, logrus.ErrorLevel)
		assert.Equal(t, aliasEnv, entry.Data["env"])
		assert.Equal(t, "env_tombstoned_alias", entry.Data["source"])
		assert.Equal(t, "unknown", entry.Data["outcome"])
	})

	t.Run("dangling", func(t *testing.T) {
		t.Parallel()

		hook := runTestInitForLogs(t, Source{}, map[string]string{aliasEnv: "true"},
			[]alias{{EnvVar: aliasEnv, Canonical: "gone-feature", Phase: honored}})

		warn := entryByLevel(t, hook, logrus.WarnLevel)
		assert.Equal(t, "gone-feature", warn.Data["feature"])
		assert.Equal(t, "env_legacy_alias", warn.Data["source"])
		assert.Equal(t, "deprecated_alias", warn.Data["lifecycle"])
		assert.Equal(t, aliasEnv, warn.Data["env"])

		errEntry := entryByLevel(t, hook, logrus.ErrorLevel)
		assert.Equal(t, "gone-feature", errEntry.Data["feature"])
		assert.Equal(t, "unknown", errEntry.Data["outcome"])
		assert.Equal(t, "env_legacy_alias", errEntry.Data["source"])
		assert.Equal(t, aliasEnv, errEntry.Data["env"])
	})
}

func supplied(v ...string) Source {
	return Source{Values: v, Supplied: true}
}

func resolveTestFlags(t *testing.T, cli, env, json Source) (*testFlags, []string) {
	t.Helper()

	defs, err := parseDefinitions(reflect.TypeFor[testFlags]())
	require.NoError(t, err)

	flags := &testFlags{}
	logger, _ := logtest.NewNullLogger()
	activated, err := resolveInto(defs, reflect.ValueOf(flags).Elem(), cli, env, json, logger)
	require.NoError(t, err)

	return flags, activated
}

func runTestInit(
	t *testing.T, cli, json Source, env map[string]string, aliases []alias,
) (*testFlags, []string) {
	t.Helper()

	flags := &testFlags{}
	logger, _ := logtest.NewNullLogger()
	active, err := initInto(reflect.ValueOf(flags).Elem(), logger, cli, json, env, aliases)
	require.NoError(t, err)

	return flags, active
}

func runTestInitForLogs(t *testing.T, cli Source, env map[string]string, aliases []alias) *logtest.Hook {
	t.Helper()

	flags := &testFlags{}
	logger, hook := logtest.NewNullLogger()
	_, err := initInto(reflect.ValueOf(flags).Elem(), logger, cli, Source{}, env, aliases)
	require.NoError(t, err)

	return hook
}

func resolveTestLogs(t *testing.T, cli, env, json Source) *logtest.Hook {
	t.Helper()

	defs, err := parseDefinitions(reflect.TypeFor[testFlags]())
	require.NoError(t, err)

	flags := &testFlags{}
	logger, hook := logtest.NewNullLogger()
	_, err = resolveInto(defs, reflect.ValueOf(flags).Elem(), cli, env, json, logger)
	require.NoError(t, err)

	return hook
}

func assertSingleLog(t *testing.T, hook *logtest.Hook, level logrus.Level) *logrus.Entry {
	t.Helper()

	entries := hook.AllEntries()
	require.Len(t, entries, 1)

	entry := entries[0]
	assert.Equal(t, level, entry.Level)

	return entry
}

func entryByLevel(t *testing.T, hook *logtest.Hook, level logrus.Level) *logrus.Entry {
	t.Helper()

	var found *logrus.Entry
	for _, e := range hook.AllEntries() {
		if e.Level == level {
			require.Nil(t, found, "more than one %s entry", level)
			found = e
		}
	}
	require.NotNil(t, found, "no %s entry found", level)
	return found
}
