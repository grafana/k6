package cmd

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/lib/fsext"
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
