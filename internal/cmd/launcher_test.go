package cmd

import (
	"path/filepath"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/grafana/k6provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/ext"

	"go.k6.io/k6/internal/cmd/tests"
)

func TestIsCustomBuildRequired(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title  string
		deps   map[string]string
		exts   []*ext.Extension
		expect bool
	}{
		{
			title:  "k6 satisfied",
			deps:   map[string]string{"k6": "=v1.0.0"},
			exts:   []*ext.Extension{},
			expect: false,
		},
		{
			title:  "k6 not satisfied",
			deps:   map[string]string{"k6": ">v1.0.0"},
			exts:   []*ext.Extension{},
			expect: true,
		},
		{
			title:  "extension not present",
			deps:   map[string]string{"k6": "*", "k6/x/faker": "*"},
			exts:   []*ext.Extension{},
			expect: true,
		},
		{
			title: "extension satisfied",
			deps:  map[string]string{"k6/x/faker": "=v0.4.0"},
			exts: []*ext.Extension{
				{Name: "k6/x/faker", Module: "github.com/grafana/xk6-faker", Version: "v0.4.0"},
			},
			expect: false,
		},
		{
			title: "extension not satisfied",
			deps:  map[string]string{"k6/x/faker": ">v0.4.0"},
			exts: []*ext.Extension{
				{Name: "k6/x/faker", Module: "github.com/grafana/xk6-faker", Version: "v0.4.0"},
			},
			expect: true,
		},
		{
			title: "k6 and extension satisfied",
			deps:  map[string]string{"k6": "=v1.0.0", "k6/x/faker": "=v0.4.0"},
			exts: []*ext.Extension{
				{Name: "k6/x/faker", Module: "github.com/grafana/xk6-faker", Version: "v0.4.0"},
			},
			expect: false,
		},
		{
			title: "k6 satisfied, extension not satisfied",
			deps:  map[string]string{"k6": "=v1.0.0", "k6/x/faker": ">v0.4.0"},
			exts: []*ext.Extension{
				{Name: "k6/x/faker", Module: "github.com/grafana/xk6-faker", Version: "v0.4.0"},
			},
			expect: true,
		},
		{
			title: "k6 not satisfied, extension satisfied",
			deps:  map[string]string{"k6": ">v1.0.0", "k6/x/faker": "=v0.4.0"},
			exts: []*ext.Extension{
				{Name: "k6/x/faker", Module: "github.com/grafana/xk6-faker", Version: "v0.4.0"},
			},
			expect: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			deps := make(map[string]*semver.Constraints)
			for name, constraintStr := range tc.deps {
				constraint, err := semver.NewConstraint(constraintStr)
				if err != nil {
					t.Fatalf("parsing %q constraint: %v", constraintStr, err)
				}
				deps[name] = constraint
			}

			k6Version := "v1.0.0"
			required := isCustomBuildRequired(deps, k6Version, tc.exts)
			assert.Equal(t, tc.expect, required)
		})
	}
}

func TestGetProviderConfig(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		token        string
		expectConfig k6provider.Config
	}{
		{
			name:  "no token",
			token: "",
			expectConfig: k6provider.Config{
				BuildServiceURL:  "https://ingest.k6.io/builder/api/v1",
				BinaryCacheDir:   filepath.Join(".cache", "k6", "builds"),
				BuildServiceAuth: "",
			},
		},
		{
			name:  "K6_CLOUD_TOKEN set",
			token: "K6CLOUDTOKEN",
			expectConfig: k6provider.Config{
				BuildServiceURL:  "https://ingest.k6.io/builder/api/v1",
				BinaryCacheDir:   filepath.Join(".cache", "k6", "builds"),
				BuildServiceAuth: "K6CLOUDTOKEN",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := tests.NewGlobalTestState(t)
			if tc.token != "" {
				ts.Env["K6_CLOUD_TOKEN"] = tc.token
			}

			config := getProviderConfig(ts.GlobalState)

			assert.Equal(t, tc.expectConfig, config)
		})
	}
}

func TestProcessUseDirectives(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input          string
		expectedOutput map[string]string
		expectedError  string
	}{
		"nothing": {
			input: "export default function() {}",
		},
		"nothing really": {
			input: `"use k6"`,
			expectedOutput: map[string]string{
				"k6": "",
			},
		},
		"k6 pinning": {
			input: `"use k6 > 1.4.0"`,
			expectedOutput: map[string]string{
				"k6": "> 1.4.0",
			},
		},
		"a extension": {
			input: `"use k6 with k6/x/sql"`,
			expectedOutput: map[string]string{
				"k6/x/sql": "",
			},
		},
		"an extension with constraint": {
			input: `"use k6 with k6/x/sql > 1.4.0"`,
			expectedOutput: map[string]string{
				"k6/x/sql": "> 1.4.0",
			},
		},
		"complex": {
			input: `
				// something here
				"use k6 with k6/x/A"
				function a (){
					"use k6 with k6/x/B"
					let s = JSON.stringify( "use k6 with k6/x/C")
					"use k6 with k6/x/D"

					return s
				}

				export const b = "use k6 with k6/x/E"
				"use k6 with k6/x/F"

				// Here for esbuild and k6 warnings
				a()
				export default function(){}
				`,
			expectedOutput: map[string]string{
				"k6/x/A": "",
			},
		},

		"repeat": {
			input: `
				"use k6 with k6/x/A"
				"use k6 with k6/x/A"
				`,
			expectedOutput: map[string]string{
				"k6/x/A": "",
			},
		},
		"repeat with constraint first": {
			input: `
				"use k6 with k6/x/A > 1.4.0"
				"use k6 with k6/x/A"
				`,
			expectedOutput: map[string]string{
				"k6/x/A": "> 1.4.0",
			},
		},
		"constraint difference": {
			input: `
				"use k6 > 1.4.0"
				"use k6 = 1.2.3"
				`,
			expectedError: `error while parsing use directives in "name.js": already have constraint for "k6", when parsing "=1.2.3"`,
		},
		"constraint difference for extensions": {
			input: `
				"use k6 with k6/x/A > 1.4.0"
				"use k6 with k6/x/A = 1.2.3"
				`,
			expectedError: `error while parsing use directives in "name.js": already have constraint for "k6/x/A", when parsing "=1.2.3"`,
		},
		"constraint bad format": {
			input: `
				"use k6 with k6/x/A +1.4.0"
				`,
			expectedError: `error while parsing use directives constraint "+1.4.0" for "k6/x/A" in "name.js": improper constraint: +1.4.0`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			deps, err := dependenciesFromMap(test.expectedOutput)
			require.NoError(t, err)
			expected := constraintsMapToProvisionDependency(deps)
			if len(test.expectedError) > 0 {
				expected = nil
			}

			m := make(dependencies)
			err = processUseDirectives("name.js", []byte(test.input), m)
			if len(test.expectedError) > 0 {
				require.ErrorContains(t, err, test.expectedError)
			} else {
				require.EqualValues(t, expected, constraintsMapToProvisionDependency(m))
			}
		})
	}
}

func TestFindDirectives(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input          string
		expectedOutput []string
	}{
		"nothing": {
			input:          "export default function() {}",
			expectedOutput: nil,
		},
		"nothing really": {
			input:          `"use k6"`,
			expectedOutput: []string{"use k6"},
		},
		"multiline": {
			input: `
			"use k6 with k6/x/sql"
			"something"
			`,
			expectedOutput: []string{"use k6 with k6/x/sql", "something"},
		},
		"multiline start at beginning": {
			input: `
"use k6 with k6/x/sql"
"something"
			`,
			expectedOutput: []string{"use k6 with k6/x/sql", "something"},
		},
		"multiline comments": {
			input: `#!/bin/sh
			// here comment "hello"
"use k6 with k6/x/sql";
			/*
			"something else here as well"
			*/
	;
"something";
const l = 5
"more"
			`,
			expectedOutput: []string{"use k6 with k6/x/sql", "something"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := findDirectives([]byte(test.input))
			assert.EqualValues(t, test.expectedOutput, m)
		})
	}
}

func TestDependenciesApplyManifest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		deps          map[string]string
		manifest      string
		expected      map[string]string
		expectedError string
	}{
		{
			name:     "default constraint with no manifest",
			deps:     map[string]string{"k6/x/dep": "*"},
			manifest: "",
			expected: map[string]string{"k6/x/dep": "*"},
		},
		{
			name:     "empty constraint with no manifest",
			deps:     map[string]string{"k6/x/dep": ""},
			manifest: "",
			expected: map[string]string{"k6/x/dep": ""},
		},
		{
			name:     "default constraint with empty manifest",
			deps:     map[string]string{"k6/x/dep": "*"},
			manifest: "{}",
			expected: map[string]string{"k6/x/dep": "*"},
		},
		{
			name:     "default constraint with manifest overrides",
			deps:     map[string]string{"k6/x/dep": "*"},
			manifest: `{"k6/x/dep": "=v0.0.0"}`,
			expected: map[string]string{"k6/x/dep": "=v0.0.0"},
		},
		{
			name:     "dependency with version constraint",
			deps:     map[string]string{"k6/x/dep": "=v0.0.1"},
			manifest: `{"k6/x/dep": "=v0.0.0"}`,
			expected: map[string]string{"k6/x/dep": "=v0.0.1"},
		},
		{
			name:     "manifest with different dependency",
			deps:     map[string]string{"k6/x/dep": "*"},
			manifest: `{"k6/x/another": "=v0.0.0"}`,
			expected: map[string]string{"k6/x/dep": "*"},
		},
		{
			name:     "no dependencies",
			deps:     map[string]string{},
			manifest: `{"k6/x/dep": "=v0.0.0"}`,
			expected: map[string]string{},
		},
		{
			name:          "malformed manifest",
			deps:          map[string]string{"k6/x/dep": "*"},
			manifest:      `{"k6/x/dep": }`,
			expectedError: "invalid dependencies manifest",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var err error
			deps := make(dependencies)
			for k, v := range test.deps {
				if v == "" {
					deps[k] = nil
					return
				}
				deps[k], _ = semver.NewConstraint(v)
				require.NoError(t, err)
			}
			depsMan, err := parseManifest(test.manifest)
			require.NoError(t, deps.applyManifest(depsMan))
			if len(test.expectedError) > 0 {
				require.ErrorContains(t, err, test.expectedError)
			} else {
				require.EqualValues(t, test.expected, (map[string]string)(constraintsMapToProvisionDependency(deps)))
			}
		})
	}
}
