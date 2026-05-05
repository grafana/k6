package cmd

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/lib/fsext"
)

const (
	fakerJs = `
import { Faker } from "k6/x/faker";

const faker = new Faker(11);

export default function () {
  console.log(faker.person.firstName());
}
`

	scriptJS = `
"use k6 with k6/x/faker > 0.4.0";
import faker from "./faker.js";

export default () => {
  faker();
};
`
)

func TestCollectTestDependenciesPreManifestSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("pre-manifest deps reflect script constraints before override", func(t *testing.T) {
		t.Parallel()

		// manifest overrides the default "*" k6 constraint
		manifest := `{"k6": ">=1.0.0"}`
		deps, preDeps, err := collectTestDependencies(nil, nil, nil, manifest)
		require.NoError(t, err)

		// post-manifest: k6 constraint is overridden to ">=1.0.0"
		require.Equal(t, ">=1.0.0", deps["k6"].String())
		// pre-manifest: k6 constraint is still "*" (nil)
		require.Nil(t, preDeps["k6"])
	})

	t.Run("pre-manifest deps are independent from post-manifest deps", func(t *testing.T) {
		t.Parallel()

		// manifest overrides k6's nil constraint to ">=0.9.0"
		manifest := `{"k6": ">=0.9.0"}`
		deps, preDeps, err := collectTestDependencies(nil, nil, nil, manifest)
		require.NoError(t, err)

		// post-manifest has the constraint; mutating it must not affect the pre-manifest snapshot
		require.Equal(t, ">=0.9.0", deps["k6"].String())
		require.Nil(t, preDeps["k6"], "pre-manifest snapshot should still be nil/unconstrained")

		// Mutate the post-manifest map and verify the snapshot is unaffected.
		delete(deps, "k6")
		require.Nil(t, preDeps["k6"], "deleting from deps must not affect preDeps")
		require.Contains(t, preDeps, "k6", "preDeps should still contain k6 after mutation of deps")
	})

	t.Run("explicit use directive constraint is preserved in pre-manifest deps", func(t *testing.T) {
		t.Parallel()

		fs := testutils.MakeMemMapFs(t, map[string][]byte{
			"/script.js": []byte(`"use k6 >= 0.9.0";
export default function () {}
`),
		})

		manifest := `{"k6": ">=1.0.0"}`
		deps, preDeps, err := collectTestDependencies(nil, []string{"file:///script.js"}, map[string]fsext.Fs{"file": fs}, manifest)
		require.NoError(t, err)

		// manifest does not override an explicit constraint — both pre and post should match
		require.Equal(t, ">=0.9.0", deps["k6"].String())
		require.Equal(t, ">=0.9.0", preDeps["k6"].String())
	})

	t.Run("pre-manifest deps include use-directive extension constraints", func(t *testing.T) {
		t.Parallel()

		fs := testutils.MakeMemMapFs(t, map[string][]byte{
			"/script.js": []byte(`"use k6 with k6/x/sql >= 0.4.0";
export default function () {}
`),
		})

		manifest := `{"k6/x/sql": ">=1.0.0"}`
		deps, preDeps, err := collectTestDependencies(nil, []string{"file:///script.js"}, map[string]fsext.Fs{"file": fs}, manifest)
		require.NoError(t, err)

		// The explicit ">=0.4.0" constraint from use directive is not overridden by manifest
		require.Equal(t, ">=0.4.0", deps["k6/x/sql"].String())
		require.Equal(t, ">=0.4.0", preDeps["k6/x/sql"].String())
	})

	t.Run("pre-manifest snapshot has nil constraint for unconstrained star dep", func(t *testing.T) {
		t.Parallel()

		// no manifest, no use directives — k6 gets the default nil/"*" constraint
		deps, preDeps, err := collectTestDependencies(nil, nil, nil, "")
		require.NoError(t, err)

		require.Nil(t, deps["k6"])
		require.Nil(t, preDeps["k6"])
	})
}

func TestCollectTestDependenciesRegisteredExtensions(t *testing.T) {
	t.Parallel()

	t.Run("k6/x/ import with no error is still captured as dep", func(t *testing.T) {
		t.Parallel()
		// Simulate the second run of auto-extension-resolution: k6/x/foo is now registered
		// so LoadMainModule succeeds (originalError = nil), but k6/x/foo still appears in
		// the imports list returned by moduleResolver.Imported().
		deps, preDeps, err := collectTestDependencies(nil, []string{"k6/x/foo", "k6/http"}, nil, "")
		require.NoError(t, err)

		require.Nil(t, deps["k6/x/foo"], "registered extension should be in deps with nil constraint")
		require.Nil(t, preDeps["k6/x/foo"], "registered extension should be in pre-manifest deps")
		require.NotContains(t, deps, "k6/http", "built-in k6 modules should not be added as deps")
	})

	t.Run("multiple k6/x/ imports are all captured", func(t *testing.T) {
		t.Parallel()
		// All k6/x/ imports from Imported() should appear in deps,
		// even when originalError is nil (registered extensions).
		deps, preDeps, err := collectTestDependencies(nil, []string{"k6/x/foo", "k6/x/bar"}, nil, "")
		require.NoError(t, err)

		require.Nil(t, deps["k6/x/foo"])
		require.Nil(t, deps["k6/x/bar"])
		require.Nil(t, preDeps["k6/x/foo"])
		require.Nil(t, preDeps["k6/x/bar"])
		require.Len(t, deps, 3, "should have k6, k6/x/foo, k6/x/bar")
	})

	t.Run("k6/x/ import with use directive constraint is preserved", func(t *testing.T) {
		t.Parallel()
		// The new loop adds k6/x/foo with nil, then analyseUseContraints overrides it
		// with the explicit constraint from the use directive.
		fs := testutils.MakeMemMapFs(t, map[string][]byte{
			"/script.js": []byte(`"use k6 with k6/x/foo >= 1.0.0";
export default function () {}
`),
		})
		deps, preDeps, err := collectTestDependencies(
			nil,
			[]string{"file:///script.js", "k6/x/foo"},
			map[string]fsext.Fs{"file": fs},
			"",
		)
		require.NoError(t, err)

		require.Equal(t, ">=1.0.0", deps["k6/x/foo"].String(), "use directive should set the constraint")
		require.Equal(t, ">=1.0.0", preDeps["k6/x/foo"].String())
	})
}

func TestDependenciesToStringMap(t *testing.T) {
	t.Parallel()

	t.Run("nil constraint becomes star", func(t *testing.T) {
		t.Parallel()
		d := dependencies{"k6": nil}
		m := d.toStringMap()
		require.Equal(t, map[string]string{"k6": "*"}, m)
	})

	t.Run("constraint string is preserved", func(t *testing.T) {
		t.Parallel()
		c, err := semver.NewConstraint(">=0.4.0")
		require.NoError(t, err)
		d := dependencies{"k6/x/sql": c}
		m := d.toStringMap()
		require.Equal(t, map[string]string{"k6/x/sql": ">=0.4.0"}, m)
	})

	t.Run("empty dependencies produces empty map", func(t *testing.T) {
		t.Parallel()
		d := dependencies{}
		m := d.toStringMap()
		require.Empty(t, m)
	})

	t.Run("mixed nil and constrained", func(t *testing.T) {
		t.Parallel()
		c, err := semver.NewConstraint(">=1.2.3")
		require.NoError(t, err)
		d := dependencies{"k6": nil, "k6/x/foo": c}
		m := d.toStringMap()
		require.Equal(t, map[string]string{"k6": "*", "k6/x/foo": ">=1.2.3"}, m)
	})
}

func TestAnalyseUseConstraints(t *testing.T) {
	t.Parallel()

	fs := testutils.MakeMemMapFs(t, map[string][]byte{
		"/script.js": []byte(scriptJS),
		"/faker.js":  []byte(fakerJs),
	})
	deps := make(map[string]*semver.Constraints)

	err := analyseUseContraints([]string{"file:///script.js", "file:///faker.js"}, map[string]fsext.Fs{"file": fs}, deps)

	require.NoError(t, err)
	require.Len(t, deps, 1)
	require.Equal(t, deps["k6/x/faker"].String(), ">0.4.0")
}
