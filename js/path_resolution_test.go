package js

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/fsext"
)

// This whole file is about tests around https://github.com/grafana/k6/issues/2674

func TestOpenPathResolution(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		fsMap map[string]any
	}{
		"simple": {
			fsMap: map[string]any{
				"/A/B/data.txt": "data file",
				"/A/A/A/A/script.js": `
					export let data = open("../../../B/data.txt");
					if (data != "data file") {
						throw new Error("wrong content " + data);
					}
					export default function() {}
				`,
			},
		},
		"intermediate": {
			fsMap: map[string]any{
				"/A/B/data.txt": "data file",
				"/A/C/B/script.js": `
					module.exports = open("../../B/data.txt");
				`,
				"/A/A/A/A/script.js": `
					let data = require("./../../../C/B/script.js")
					if (data != "data file") {
						throw new Error("wrong content " + data);
					}
					export default function() {}
				`,
			},
		},
		"complex": {
			fsMap: map[string]any{
				"/A/B/data.txt": "data file",
				"/A/C/B/script.js": `
					// Here the path is relative to this module but to the one calling
					module.exports = () =>  open("./../data.txt");
				`,
				"/A/B/B/script.js": `
					module.exports = require("./../../C/B/script.js")();
				`,
				"/A/A/A/A/script.js": `
					let data = require("./../../../B/B/script.js");
					if (data != "data file") {
						throw new Error("wrong content " + data);
					}
					export default function() {}
				`,
			},
		},
	}

	for name, testCase := range testCases {
		name, testCase := name, testCase

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fs := fsext.NewMemMapFs()
			err := writeToFs(fs, testCase.fsMap)
			require.NoError(t, err)
			b, err := getSimpleBundle(t, "/main.js", `export { default } from "/A/A/A/A/script.js"`, fs)
			require.NoError(t, err)

			_, err = b.Instantiate(context.Background(), 0)
			require.NoError(t, err)
		})
	}
}

func TestRequirePathResolution(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		fsMap map[string]any
	}{
		"simple": {
			fsMap: map[string]any{
				"/A/B/data.js": "module.exports='export content'",
				"/A/A/A/A/script.js": `
					let data = require("../../../B/data.js");
					if (data != "export content") {
						throw new Error("wrong content " + data);
					}
					export default function() {}
				`,
			},
		},
		"intermediate": {
			fsMap: map[string]any{
				"/A/B/data.js": "module.exports='export content'",
				"/A/C/B/script.js": `
					module.exports = require("../../B/data.js");
				`,
				"/A/A/A/A/script.js": `
					let data = require("./../../../C/B/script.js")
					if (data != "export content") {
						throw new Error("wrong content " + data);
					}
					export default function() {}
				`,
			},
		},
		"complex": {
			fsMap: map[string]any{
				"/A/B/data.js": "module.exports='export content'",
				"/A/C/B/script.js": `
					// Here the path is relative to this module but to the one calling
					module.exports = () =>  require("./../data.js");
				`,
				"/A/B/B/script.js": `
					module.exports = require("./../../C/B/script.js")();
				`,
				"/A/A/A/A/script.js": `
					let data = require("./../../../B/B/script.js");
					if (data != "export content") {
						throw new Error("wrong content " + data);
					}
					export default function() {}
				`,
			},
		},
	}
	for name, testCase := range testCases {
		name, testCase := name, testCase

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fs := fsext.NewMemMapFs()
			err := writeToFs(fs, testCase.fsMap)
			require.NoError(t, err)
			b, err := getSimpleBundle(t, "/main.js", `export { default } from "/A/A/A/A/script.js"`, fs)
			require.NoError(t, err)

			_, err = b.Instantiate(context.Background(), 0)
			require.NoError(t, err)
		})
	}
}

// writeToFs is a small helper to write a map of paths to contents to the filesystem provided.
// the content can be either string or []byte anything else panics
func writeToFs(fs fsext.Fs, in map[string]any) error {
	for path, contentAny := range in {
		var content []byte
		switch contentAny := contentAny.(type) {
		case []byte:
			content = contentAny
		case string:
			content = []byte(contentAny)
		default:
			panic("content for " + path + " wasn't []byte or string")
		}
		if err := fsext.WriteFile(fs, path, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}
