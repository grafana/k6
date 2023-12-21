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
	t.Run("simple", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()
		err := writeToFs(fs, map[string]any{
			"/path/to/data.txt": "data file",
		})
		require.NoError(t, err)
		data := `
		export let data = open("../to/data.txt");
		if (data != "data file") {
			throw new Error("wrong content " + data);
		}
		export default function() {}
	`
		b, err := getSimpleBundle(t, "/path/scripts/script.js", data, fs)
		require.NoError(t, err)

		_, err = b.Instantiate(context.Background(), 0)
		require.NoError(t, err)
	})

	t.Run("intermediate", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()
		err := writeToFs(fs, map[string]any{
			"/path/to/data.txt": "data file",
			"/path/another/script/script.js": `
            module.exports = open("../../to/data.txt");
        `,
		})
		require.NoError(t, err)
		data := `
        let data = require("./../../../another/script/script.js")
		if (data != "data file") {
			throw new Error("wrong content " + data);
		}
		export default function() {}
	`
		b, err := getSimpleBundle(t, "/path/totally/different/directory/script.js", data, fs)
		require.NoError(t, err)

		_, err = b.Instantiate(context.Background(), 0)
		require.NoError(t, err)
	})

	t.Run("complex", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()
		err := writeToFs(fs, map[string]any{
			"/path/to/data.txt": "data file",
			"/path/another/script/script.js": `
        module.exports = () =>  open("./../data.txt"); // Here the path is relative to this module but to the one calling
        `,
			"/path/to/script/script.js": `
        module.exports = require("./../../another/script/script.js")();
        `,
		})
		require.NoError(t, err)
		data := `
        let data = require("./../../../to/script/script.js");
		if (data != "data file") {
			throw new Error("wrong content " + data);
		}
		export default function() {}
	`
		b, err := getSimpleBundle(t, "/path/totally/different/directory/script.js", data, fs)
		require.NoError(t, err)

		_, err = b.Instantiate(context.Background(), 0)
		require.NoError(t, err)
	})
}

func TestRequirePathResolution(t *testing.T) {
	t.Parallel()
	t.Run("simple", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()
		err := writeToFs(fs, map[string]any{
			"/path/to/data.js": "module.exports='export content'",
		})
		require.NoError(t, err)
		data := `
		let data = require("../to/data.js");
		if (data != "export content") {
			throw new Error("wrong content " + data);
		}
		export default function() {}
	`
		b, err := getSimpleBundle(t, "/path/scripts/script.js", data, fs)
		require.NoError(t, err)

		_, err = b.Instantiate(context.Background(), 0)
		require.NoError(t, err)
	})

	t.Run("intermediate", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()
		err := writeToFs(fs, map[string]any{
			"/path/to/data.js": "module.exports='export content'",
			"/path/another/script/script.js": `
            module.exports = require("../../to/data.js");
        `,
		})
		require.NoError(t, err)
		data := `
        let data = require("./../../../another/script/script.js")
		if (data != "export content") {
			throw new Error("wrong content " + data);
		}
		export default function() {}
	`
		b, err := getSimpleBundle(t, "/path/totally/different/directory/script.js", data, fs)
		require.NoError(t, err)

		_, err = b.Instantiate(context.Background(), 0)
		require.NoError(t, err)
	})

	t.Run("complex", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()
		err := writeToFs(fs, map[string]any{
			"/path/to/data.js": "module.exports='export content'",
			"/path/another/script/script.js": `
        module.exports = () =>  require("./../data.js"); // Here the path is relative to this module but to the one calling
        `,
			"/path/to/script/script.js": `
        module.exports = require("./../../another/script/script.js")();
        `,
		})
		require.NoError(t, err)
		data := `
        let data = require("./../../../to/script/script.js");
		if (data != "export content") {
			throw new Error("wrong content " + data);
		}
		export default function() {}
	`
		b, err := getSimpleBundle(t, "/path/totally/different/directory/script.js", data, fs)
		require.NoError(t, err)

		_, err = b.Instantiate(context.Background(), 0)
		require.NoError(t, err)
	})
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
