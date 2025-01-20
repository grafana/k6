package js

import (
	"context"
	"net/url"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/lib/fsext"
)

// This whole file is about tests around https://github.com/grafana/k6/issues/2674

func TestPathResolution(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		fsMap         map[string]any
		expectedLogs  []string
		expectedError string
	}{
		"open simple": {
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
		"open intermediate": {
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
		"open complex": {
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
			expectedLogs: []string{
				`open() was used and is currently relative to 'file:///A/B/B/', but in the future it will be aligned with how`,
			},
		},
		"open space in path": {
			fsMap: map[string]any{
				"/A/B D/data.txt": "data file",
				"/A/C D/B/script.js": `
					// Here the path is relative to this module but to the one calling
					module.exports = () =>  open("./../data.txt");
				`,
				"/A/B D/B/script.js": `
					module.exports = require("./../../C D/B/script.js")();
				`,
				"/A/A/A/A/script.js": `
					let data = require("./../../../B D/B/script.js");
					if (data != "data file") {
						throw new Error("wrong content " + data);
					}
					export default function() {}
				`,
			},
			expectedLogs: []string{
				`open() was used and is currently relative to 'file:///A/B%20D/B/', but in the future it will be aligned with how `,
			},
		},
		"require simple": {
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
		"require intermediate": {
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
		"require complex": {
			fsMap: map[string]any{
				"/A/B/data.js": "module.exports='export content'",
				"/A/C/B/script.js": `
					module.exports = () =>  require("./../../B/data.js");
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
		"require complex wrong": {
			fsMap: map[string]any{
				"/A/B/data.js": "module.exports='export content'",
				"/A/C/B/script.js": `
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
			expectedError: `The moduleSpecifier "./../data.js" couldn't be found on local disk.`,
		},
		"ESM and require": {
			fsMap: map[string]any{
				"/A/B/data.js": "module.exports='export content'",
				"/A/C/B/script.js": `
					export default function () {
						// Here the path is relative to this module not the calling one
						return require("./../../B/data.js");
					}
				`,
				"/A/B/B/script.js": `
					import s from "./../../C/B/script.js"
					export default require("./../../C/B/script.js").default();
				`,
				"/A/A/A/A/script.js": `
					import data from "./../../../B/B/script.js"
					if (data != "export content") {
						throw new Error("wrong content " + data);
					}
					export default function() {}
				`,
			},
		},
		"ESM and require wrong": {
			fsMap: map[string]any{
				"/A/B/data.js": "module.exports='export content'",
				"/A/C/B/script.js": `
					export default function () {
						return require("./../data.js");
					}
				`,
				"/A/B/B/script.js": `
					import s from "./../../C/B/script.js"
					export default require("./../../C/B/script.js").default();
				`,
				"/A/A/A/A/script.js": `
					import data from "./../../../B/B/script.js"
					if (data != "export content") {
						throw new Error("wrong content " + data);
					}
					export default function() {}
				`,
			},
			expectedError: `The moduleSpecifier "./../data.js" couldn't be found on local disk.`,
		},
		"full ESM": {
			fsMap: map[string]any{
				"/A/B/data.js": "export default 'export content'",
				"/A/C/B/script.js": `
					export default function () {
						// Here the path is relative to this module not the calling one
						return require("./../../B/data.js").default;
					}
				`,
				"/A/B/B/script.js": `
					import s from "./../../C/B/script.js"
					let l = s();
					export default l;
				`,
				"/A/A/A/A/script.js": `
					import data from "./../../../B/B/script.js"
					if (data != "export content") {
						throw new Error("wrong content " + data);
					}
					export default function() {}
				`,
			},
		},
		"full ESM wrong": {
			fsMap: map[string]any{
				"/A/B/data.js": "export default 'export content'",
				"/A/C/B/script.js": `
					export default function () {
						return require("./../data.js").default;
					}
				`,
				"/A/B/B/script.js": `
					import s from "./../../C/B/script.js"
					let l = s();
					export default l;
				`,
				"/A/A/A/A/script.js": `
					import data from "./../../../B/B/script.js"
					if (data != "export content") {
						throw new Error("wrong content " + data);
					}
					export default function() {}
				`,
			},
			expectedError: `The moduleSpecifier "./../data.js" couldn't be found on local disk.`,
		},
	}
	for name, testCase := range testCases {
		name, testCase := name, testCase

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fs := fsext.NewMemMapFs()
			err := writeToFs(fs, testCase.fsMap)
			fs = fsext.NewCacheOnReadFs(fs, fsext.NewMemMapFs(), 0)
			require.NoError(t, err)
			logger, hook := testutils.NewLoggerWithHook(t, logrus.WarnLevel)
			b, err := getSimpleBundle(t, "/main.js", `export { default } from "/A/A/A/A/script.js"`, fs, logger)

			if testCase.expectedError != "" {
				require.ErrorContains(t, err, testCase.expectedError)
				return
			}
			require.NoError(t, err)

			_, err = b.Instantiate(context.Background(), 0)
			require.NoError(t, err)
			logs := hook.Drain()

			if len(testCase.expectedLogs) == 0 {
				require.Empty(t, logs)
				return
			}
			require.Equal(t, len(logs), len(testCase.expectedLogs))

			for i, log := range logs {
				require.Contains(t, log.Message, testCase.expectedLogs[i], "log line %d", i)
			}
		})
	}

	pwd, err := url.Parse("file:///A/A/A/A/")
	require.NoError(t, err)

	for name, testCase := range testCases {
		name, testCase := name, testCase

		t.Run("STDIN-"+name, func(t *testing.T) {
			t.Parallel()
			fs := fsext.NewMemMapFs()
			err := writeToFs(fs, testCase.fsMap)
			fs = fsext.NewCacheOnReadFs(fs, fsext.NewMemMapFs(), 0)
			require.NoError(t, err)
			logger, hook := testutils.NewLoggerWithHook(t, logrus.WarnLevel)

			b, err := getSimpleBundleStdin(t, pwd, testCase.fsMap["/A/A/A/A/script.js"].(string), fs, logger)
			if testCase.expectedError != "" {
				require.ErrorContains(t, err, testCase.expectedError)
				return
			}
			require.NoError(t, err)

			_, err = b.Instantiate(context.Background(), 0)
			require.NoError(t, err)
			logs := hook.Drain()

			if len(testCase.expectedLogs) == 0 {
				require.Empty(t, logs)
				return
			}
			require.Equal(t, len(logs), len(testCase.expectedLogs))

			for i, log := range logs {
				require.Contains(t, log.Message, testCase.expectedLogs[i], "log line %d", i)
			}
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

func TestImportMetaResolve(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		fsMap         map[string]any
		expectedLogs  []string
		expectedError string
	}{
		"simple": {
			fsMap: map[string]any{
				"/A/B/data.js": "module.exports='export content'",
				"/A/A/A/A/script.js": `
					let data = require(import.meta.resolve("../../../B/data.js"));
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
					module.exports = require(import.meta.resolve("../../B/data.js"));
				`,
				"/A/A/A/A/script.js": `
					let data = require(import.meta.resolve("./../../../C/B/script.js"))
					if (data != "export content") {
						throw new Error("wrong content " + data);
					}
					export default function() {}
				`,
			},
			expectedError: `import not supported in script`,
		},
		"intermediate fixed": {
			fsMap: map[string]any{
				"/A/B/data.js": "export default 'export content'",
				"/A/C/B/script.js": `
					export default require("../../B/data.js").default;
				`,
				"/A/A/A/A/script.js": `
					let data = require("./../../../C/B/script.js").default;
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
					export default () =>  require(import.meta.resolve("./../../B/data.js"));
				`,
				"/A/B/B/script.js": `
					export default require(import.meta.resolve("./../../C/B/script.js")).default();
				`,
				"/A/A/A/A/script.js": `
					let data = require(import.meta.resolve("./../../../B/B/script.js")).default;
					if (data != "export content") {
						throw new Error("wrong content " + data);
					}
					export default function() {}
				`,
			},
		},
		"complex wrong": {
			fsMap: map[string]any{
				"/A/B/data.js": "module.exports='export content'",
				"/A/C/B/script.js": `
					export default () =>  require(import.meta.resolve("./../data.js"));
				`,
				"/A/B/B/script.js": `
					export default require(import.meta.resolve("./../../C/B/script.js")).default();
				`,
				"/A/A/A/A/script.js": `
					let data = require(import.meta.resolve("./../../../B/B/script.js")).default;
					if (data != "export content") {
						throw new Error("wrong content " + data);
					}
					export default function() {}
				`,
			},
			expectedError: `A/C/data.js" couldn't be found`,
		},
		"complex wrong fixed": {
			fsMap: map[string]any{
				"/A/B/data.js": "module.exports='export content'",
				"/A/C/B/script.js": `
					export default (specifier) =>  require(specifier);
				`,
				"/A/B/B/script.js": `
					export default require(import.meta.resolve("./../../C/B/script.js")).default(
						import.meta.resolve("./../data.js")
					);
				`,
				"/A/A/A/A/script.js": `
					let data = require(import.meta.resolve("./../../../B/B/script.js")).default;
					if (data != "export content") {
						throw new Error("wrong content " + data);
					}
					export default function() {}
				`,
			},
		},
		"ESM and require": {
			fsMap: map[string]any{
				"/A/B/data.js": "module.exports='export content'",
				"/A/C/B/script.js": `
					export default function () {
						// Here the path is relative to this module not the calling one
						return require(import.meta.resolve("./../../B/data.js"));
					}
				`,
				"/A/B/B/script.js": `
					import s from "./../../C/B/script.js"
					export default require(import.meta.resolve("./../../C/B/script.js")).default();
				`,
				"/A/A/A/A/script.js": `
					import data from "./../../../B/B/script.js"
					if (data != "export content") {
						throw new Error("wrong content " + data);
					}
					export default function() {}
				`,
			},
		},
		"ESM and require wrong": {
			fsMap: map[string]any{
				"/A/B/data.js": "module.exports='export content'",
				"/A/C/B/script.js": `
					export default function () {
						return require(import.meta.resolve("./../data.js"));
					}
				`,
				"/A/B/B/script.js": `
					import s from "./../../C/B/script.js"
					export default require(import.meta.resolve("./../../C/B/script.js")).default();
				`,
				"/A/A/A/A/script.js": `
					import data from "./../../../B/B/script.js"
					if (data != "export content") {
						throw new Error("wrong content " + data);
					}
					export default function() {}
				`,
			},
			expectedError: `The moduleSpecifier "file:///A/C/data.js" couldn't be found on local disk.`,
		},
		"full ESM": {
			fsMap: map[string]any{
				"/A/B/data.js": "export default 'export content'",
				"/A/C/B/script.js": `
					export default function (specifier) {
						return require(specifier).default;
					}
				`,
				"/A/B/B/script.js": `
					import s from "./../../C/B/script.js"
					let l = s(import.meta.resolve("../data.js")); // this will resolve with this module as root
					export default l;
				`,
				"/A/A/A/A/script.js": `
					import data from "./../../../B/B/script.js"
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
			fs = fsext.NewCacheOnReadFs(fs, fsext.NewMemMapFs(), 0)
			require.NoError(t, err)
			logger, hook := testutils.NewLoggerWithHook(t, logrus.WarnLevel)
			b, err := getSimpleBundle(t, "/main.js", `export { default } from "/A/A/A/A/script.js"`, fs, logger)

			if testCase.expectedError != "" {
				require.ErrorContains(t, err, testCase.expectedError)
				return
			}
			require.NoError(t, err)

			_, err = b.Instantiate(context.Background(), 0)
			require.NoError(t, err)
			logs := hook.Drain()

			if len(testCase.expectedLogs) == 0 {
				require.Empty(t, logs)
				return
			}
			require.Equal(t, len(logs), len(testCase.expectedLogs))

			for i, log := range logs {
				require.Contains(t, log.Message, testCase.expectedLogs[i], "log line %d", i)
			}
		})
	}
}
