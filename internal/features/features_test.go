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
