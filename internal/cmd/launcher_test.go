package cmd

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/grafana/k6provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/ext"

	"go.k6.io/k6/v2/internal/cmd/tests"
)

func TestCheckBuiltinDependencies(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		k6Version string
		deps      map[string]string
		exts      []*ext.Extension
		wantErr   bool
	}{
		{
			name:      "k6 satisfied",
			k6Version: "v1.0.0",
			deps:      map[string]string{"k6": "=v1.0.0"},

			wantErr: false,
		},
		{
			name:      "k6 not satisfied",
			k6Version: "v1.0.0",
			deps:      map[string]string{"k6": ">v1.0.0"},

			wantErr: true,
		},
		{
			name:      "extension not present",
			k6Version: "v1.0.0",
			deps:      map[string]string{"k6": "*", "k6/x/faker": "*"},

			wantErr: true,
		},
		{
			name:      "extension satisfied",
			k6Version: "v1.0.0",
			deps:      map[string]string{"k6/x/faker": "=v0.4.0"},
			exts: []*ext.Extension{
				{Name: "k6/x/faker", Module: "github.com/grafana/xk6-faker", Version: "v0.4.0"},
			},
			wantErr: false,
		},
		{
			name:      "extension not satisfied",
			k6Version: "v1.0.0",
			deps:      map[string]string{"k6/x/faker": ">v0.4.0"},
			exts: []*ext.Extension{
				{Name: "k6/x/faker", Module: "github.com/grafana/xk6-faker", Version: "v0.4.0"},
			},
			wantErr: true,
		},
		{
			name:      "k6 and extension satisfied",
			k6Version: "v1.0.0",
			deps:      map[string]string{"k6": "=v1.0.0", "k6/x/faker": "=v0.4.0"},
			exts: []*ext.Extension{
				{Name: "k6/x/faker", Module: "github.com/grafana/xk6-faker", Version: "v0.4.0"},
			},
			wantErr: false,
		},
		{
			name:      "k6 satisfied, extension not satisfied",
			k6Version: "v1.0.0",
			deps:      map[string]string{"k6": "=v1.0.0", "k6/x/faker": ">v0.4.0"},
			exts: []*ext.Extension{
				{Name: "k6/x/faker", Module: "github.com/grafana/xk6-faker", Version: "v0.4.0"},
			},
			wantErr: true,
		},
		{
			name:      "k6 not satisfied, extension satisfied",
			k6Version: "v1.0.0",
			deps:      map[string]string{"k6": ">v1.0.0", "k6/x/faker": "=v0.4.0"},
			exts: []*ext.Extension{
				{Name: "k6/x/faker", Module: "github.com/grafana/xk6-faker", Version: "v0.4.0"},
			},
			wantErr: true,
		},
		// Pseudo-version cases
		{
			name:      "k6 build-metadata pseudo-version matches exact constraint",
			k6Version: "v0.0.0+abc123456789",
			deps:      map[string]string{"k6": "=v0.0.0+abc123456789"},

			wantErr: false,
		},
		{
			name:      "k6 go pseudo-version matches exact constraint",
			k6Version: "v0.0.0-20241015123456-abc123456789",
			deps:      map[string]string{"k6": "=v0.0.0-20241015123456-abc123456789"},

			wantErr: false,
		},
		{
			name:      "k6 build-metadata pseudo-version matches constraint with same SHA",
			k6Version: "v0.0.0+abc123456789",
			deps:      map[string]string{"k6": "=v0.0.0-20241015123456-abc123456789"},

			wantErr: false,
		},
		{
			name:      "k6 go pseudo-version matches constraint with build-metadata SHA",
			k6Version: "v0.0.0-20241015123456-abc123456789",
			deps:      map[string]string{"k6": "=v0.0.0+abc123456789"},

			wantErr: false,
		},
		{
			name:      "k6 pseudo-version does not match different SHA",
			k6Version: "v0.0.0+abc123456789",
			deps:      map[string]string{"k6": "=v0.0.0+def456789012"},

			wantErr: true,
		},
		{
			name:      "k6 pseudo-version does not satisfy strict greater-than constraint with same SHA",
			k6Version: "v0.0.0+abc123456789",
			deps:      map[string]string{"k6": ">v0.0.0+abc123456789"},

			wantErr: true,
		},
		{
			name:      "k6 pseudo-version does not satisfy not-equal constraint with same SHA",
			k6Version: "v0.0.0+abc123456789",
			deps:      map[string]string{"k6": "!=v0.0.0+abc123456789"},

			wantErr: true,
		},
		{
			name:      "k6 pseudo-version does not satisfy range constraint",
			k6Version: "v0.0.0+abc123456789",
			deps:      map[string]string{"k6": ">=v1.0.0"},

			wantErr: true,
		},
		{
			name:      "extension build-metadata pseudo-version matches exact constraint",
			k6Version: "v1.0.0",
			deps:      map[string]string{"k6/x/sql": "=v0.0.0+abc123456789"},
			exts: []*ext.Extension{
				{Name: "k6/x/sql", Module: "github.com/grafana/xk6-sql", Version: "v0.0.0+abc123456789"},
			},
			wantErr: false,
		},
		{
			name:      "extension go pseudo-version matches exact constraint",
			k6Version: "v1.0.0",
			deps:      map[string]string{"k6/x/sql": "=v0.0.0-20241015123456-abc123456789"},
			exts: []*ext.Extension{
				{Name: "k6/x/sql", Module: "github.com/grafana/xk6-sql", Version: "v0.0.0-20241015123456-abc123456789"},
			},
			wantErr: false,
		},
		// Pseudo-versions with non-zero base version against a plain semver range.
		// go pseudo-versions (pre-release form) do not satisfy non-pre-release constraints
		// per Masterminds semver semantics, regardless of whether the base version is higher.
		// build-metadata pseudo-versions (+sha) behave like normal releases because metadata
		// is stripped before comparison.
		// Using >=vX.Y.Z-0 as the constraint lower bound opts pre-release versions back in.
		{
			name:      "k6 go pseudo-version with lower base version does not satisfy range constraint",
			k6Version: "v1.4.0-20241015123456-abc123456789",
			deps:      map[string]string{"k6": ">=v1.5.0"},

			wantErr: true,
		},
		{
			name:      "k6 go pseudo-version with higher base version does not satisfy non-pre-release range constraint",
			k6Version: "v1.6.0-20241015123456-abc123456789",
			deps:      map[string]string{"k6": ">=v1.5.0"},

			wantErr: true,
		},
		{
			name:      "k6 go pseudo-version with higher base version satisfies pre-release range constraint",
			k6Version: "v1.6.0-20241015123456-abc123456789",
			deps:      map[string]string{"k6": ">=v1.5.0-0"},

			wantErr: false,
		},
		{
			name:      "k6 go pseudo-version with lower base version does not satisfy pre-release range constraint",
			k6Version: "v1.4.0-20241015123456-abc123456789",
			deps:      map[string]string{"k6": ">=v1.5.0-0"},

			wantErr: true,
		},
		{
			name:      "k6 build-metadata pseudo-version with lower base version does not satisfy range constraint",
			k6Version: "v1.4.0+abc123456789",
			deps:      map[string]string{"k6": ">=v1.5.0"},

			wantErr: true,
		},
		{
			name:      "k6 build-metadata pseudo-version with higher base version satisfies range constraint",
			k6Version: "v1.6.0+abc123456789",
			deps:      map[string]string{"k6": ">=v1.5.0"},

			wantErr: false,
		},
		// Pseudo-version constraints (the constraint itself is a pseudo-version)
		{
			name:      "normal k6 version does not match pseudo-version exact constraint",
			k6Version: "v1.0.0",
			deps:      map[string]string{"k6": "=v0.0.0+abc123456789"},
			wantErr:   true,
		},
		{
			name:      "normal k6 version satisfies pseudo-version lower-bound range",
			k6Version: "v1.0.0",
			deps:      map[string]string{"k6": ">=v0.0.0-20241015123456-abc123456789"},
			wantErr:   false,
		},
		{
			name:      "k6 pseudo-version satisfies matching pseudo-version lower-bound range",
			k6Version: "v0.0.0+abc123456789",
			deps:      map[string]string{"k6": ">=v0.0.0-20241015123456-abc123456789"},
			wantErr:   false,
		},
		{
			name:      "extension go pseudo-version matches build-metadata pseudo-version exact constraint",
			k6Version: "v1.0.0",
			deps:      map[string]string{"k6/x/sql": "=v0.0.0+abc123456789"},
			exts: []*ext.Extension{
				{Name: "k6/x/sql", Module: "github.com/grafana/xk6-sql", Version: "v0.0.0-20241015123456-abc123456789"},
			},
			wantErr: false,
		},
		// Multiple constraints (compound range)
		{
			name:      "k6 satisfies compound range constraint",
			k6Version: "v1.0.0",
			deps:      map[string]string{"k6": ">=v0.9.0 <v2.0.0"},
			wantErr:   false,
		},
		{
			name:      "k6 below compound range constraint",
			k6Version: "v1.0.0",
			deps:      map[string]string{"k6": ">=v1.1.0 <v2.0.0"},
			wantErr:   true,
		},
		{
			name:      "k6 above compound range constraint",
			k6Version: "v1.0.0",
			deps:      map[string]string{"k6": ">=v0.9.0 <v1.0.0"},
			wantErr:   true,
		},
		{
			name:      "k6 build-metadata pseudo-version does not satisfy compound range constraint",
			k6Version: "v0.0.0+abc123456789",
			deps:      map[string]string{"k6": ">=v0.9.0 <v2.0.0"},
			wantErr:   true,
		},
		{
			name:      "k6 go pseudo-version does not satisfy compound range constraint",
			k6Version: "v0.0.0-20241015123456-abc123456789",
			deps:      map[string]string{"k6": ">=v0.9.0 <v2.0.0"},
			wantErr:   true,
		},
		// Compound constraint: exact pseudo-version SHA plus an upper limit.
		// Note: compound constraints (containing spaces) bypass SHA comparison and
		// rely entirely on semver, so cross-format SHA matching does not apply.
		{
			name:      "k6 go pseudo-version does not satisfy exact-sha-plus-upper-limit compound constraint",
			k6Version: "v1.6.0-20241015123456-abc123456789",
			deps:      map[string]string{"k6": "=v1.6.0-20241015123456-abc123456789 <v2.0.0"},

			wantErr: true,
		},
		{
			name:      "k6 go pseudo-version with different SHA does not satisfy exact-sha-plus-upper-limit compound constraint",
			k6Version: "v1.7.0-20241015123456-def456789012",
			deps:      map[string]string{"k6": "=v1.6.0-20241015123456-abc123456789 <v2.0.0"},

			wantErr: true,
		},
		{
			name:      "k6 build-metadata pseudo-version does not match go pseudo-version in compound constraint",
			k6Version: "v1.6.0+abc123456789",
			deps:      map[string]string{"k6": "=v1.6.0-20241015123456-abc123456789 <v2.0.0"},

			wantErr: true,
		},
		{
			name:      "extension satisfies compound range constraint",
			k6Version: "v1.0.0",
			deps:      map[string]string{"k6/x/faker": ">=v0.3.0 <v1.0.0"},
			exts: []*ext.Extension{
				{Name: "k6/x/faker", Module: "github.com/grafana/xk6-faker", Version: "v0.4.0"},
			},
			wantErr: false,
		},
		{
			name:      "extension does not satisfy compound range constraint",
			k6Version: "v1.0.0",
			deps:      map[string]string{"k6/x/faker": ">=v0.5.0 <v1.0.0"},
			exts: []*ext.Extension{
				{Name: "k6/x/faker", Module: "github.com/grafana/xk6-faker", Version: "v0.4.0"},
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := make(map[string]*semver.Constraints)
			for name, constraintStr := range tc.deps {
				constraint, err := semver.NewConstraint(constraintStr)
				if err != nil {
					t.Fatalf("parsing %q constraint: %v", constraintStr, err)
				}
				deps[name] = constraint
			}

			err := checkBuiltinDependencies(deps, tc.k6Version, tc.exts)
			assert.Equal(t, tc.wantErr, err != nil)
		})
	}
}

func TestExtractVersionSHA(t *testing.T) {
	t.Parallel()

	cases := []struct {
		version string
		sha     string
		ok      bool
	}{
		{"v0.0.0-20241015123456-abc123456789", "abc123456789", true},
		{"v1.2.3-0.20241015123456-abc123456789", "abc123456789", true},
		{"v0.0.0+abc123456789", "abc123456789", true},
		{"v0.0.0+abc123", "abc123", true},
		{"v1.0.0", "", false},
		{"v0.0.0-alpha", "", false},
		{"1.0.0", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.version, func(t *testing.T) {
			t.Parallel()
			sha, ok := extractVersionSHA(tc.version)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.sha, sha)
		})
	}
}

func TestPseudoVersionSatisfies(t *testing.T) {
	t.Parallel()

	cases := []struct {
		version    string
		constraint string
		satisfies  bool
	}{
		{"v0.0.0+abc123456789", "=v0.0.0+abc123456789", true},
		{"v0.0.0-20241015123456-abc123456789", "=v0.0.0-20241015123456-abc123456789", true},
		// cross-format: same SHA, different pseudo-version style
		{"v0.0.0+abc123456789", "=v0.0.0-20241015123456-abc123456789", true},
		{"v0.0.0-20241015123456-abc123456789", "=v0.0.0+abc123456789", true},
		// short SHA prefix match
		{"v0.0.0+abc123456789", "=v0.0.0+abc123", true},
		// different SHA
		{"v0.0.0+abc123456789", "=v0.0.0+def456789012", false},
		// non-exact operators with matching SHA — SHA comparison should not apply
		{"v0.0.0+abc123456789", ">v0.0.0+abc123456789", false},
		{"v0.0.0+abc123456789", "!=v0.0.0+abc123456789", false},
		// range constraint — no SHA comparison
		{"v0.0.0+abc123456789", ">=v1.0.0", false},
		// compound constraint — no SHA comparison
		{"v0.0.0+abc123456789", ">=v0.0.0 <v1.0.0", false},
		// version has no SHA
		{"v1.0.0", "=v0.0.0+abc123456789", false},
	}

	for _, tc := range cases {
		t.Run(tc.version+"/"+tc.constraint, func(t *testing.T) {
			t.Parallel()
			c, err := semver.NewConstraint(tc.constraint)
			require.NoError(t, err)
			assert.Equal(t, tc.satisfies, pseudoVersionSatisfies(tc.version, c))
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
				K6ModPath:        "go.k6.io/k6/v2",
			},
		},
		{
			name:  "K6_CLOUD_TOKEN set",
			token: "K6CLOUDTOKEN",
			expectConfig: k6provider.Config{
				BuildServiceURL:  "https://ingest.k6.io/builder/api/v1",
				BinaryCacheDir:   filepath.Join(".cache", "k6", "builds"),
				BuildServiceAuth: "K6CLOUDTOKEN",
				K6ModPath:        "go.k6.io/k6/v2",
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
			expectedError: `error while parsing use directives constraint "+1.4.0" for "k6/x/A" in "name.js": improper constraint: "+1.4.0"`,
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

func TestBuildSubprocessEnv(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"HOME":                        "/home/user",
		"K6_NO_USAGE_REPORT":          "true",
		state.AutoExtensionResolution: "true",
	}

	result := buildSubprocessEnv(env)

	assert.Contains(t, result, state.AutoExtensionResolution+"=false")
	assert.Contains(t, result, state.ProvisionHostVersion+"="+runtimeK6Version())
	assert.Contains(t, result, "HOME=/home/user")
	assert.Contains(t, result, "K6_NO_USAGE_REPORT=true")

	// AutoExtensionResolution from the original env should not be forwarded.
	assert.NotContains(t, result, state.AutoExtensionResolution+"=true")
}

type fakeCommandExecutor struct {
	runCalled bool
	runErr    error
}

func (f *fakeCommandExecutor) run(_ context.Context, _ *state.GlobalState) error {
	f.runCalled = true
	return f.runErr
}

type fakeProvisioner struct {
	gotDeps map[string]string
	bin     commandExecutor
	err     error
}

func (f *fakeProvisioner) provision(_ context.Context, deps map[string]string) (commandExecutor, error) {
	f.gotDeps = deps
	return f.bin, f.err
}

func Test_completeExtension(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name          string
		manifest      string
		provisionErr  error
		runErr        error
		wantRunCalled bool
		wantDepsKey   string
	}{
		{
			name:          "cache hit runs the binary",
			wantRunCalled: true,
			wantDepsKey:   "subcommand:docs",
		},
		{
			name:         "cache miss returns nil without running",
			provisionErr: fs.ErrNotExist,
		},
		{
			name:         "provisioner error is swallowed",
			provisionErr: errors.New("provider boom"),
		},
		{
			name:     "deps build error is swallowed",
			manifest: `{not valid json`,
		},
		{
			name:          "binary run error propagates",
			runErr:        errors.New("subprocess failed"),
			wantRunCalled: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := tests.NewGlobalTestState(t)
			if tt.manifest != "" {
				ts.Flags.DependenciesManifest = tt.manifest
			}
			exec := &fakeCommandExecutor{runErr: tt.runErr}
			prov := &fakeProvisioner{bin: exec, err: tt.provisionErr}

			err := completeExtension(ts.GlobalState, "docs", prov)
			if tt.runErr != nil {
				require.ErrorIs(t, err, tt.runErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantRunCalled, exec.runCalled)
			if tt.wantDepsKey != "" {
				require.Contains(t, prov.gotDeps, tt.wantDepsKey)
			}
		})
	}
}
