package js

import (
	"context"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// This whole file is about tests around https://github.com/grafana/k6/issues/2674

func TestOpenPathResolution(t *testing.T) {
	t.Parallel()
	t.Run("simple", func(t *testing.T) {
		t.Parallel()
		fs := afero.NewMemMapFs()
		require.NoError(t, afero.WriteFile(fs, "/path/to/data.txt", []byte("data file"), 0o644))
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
		fs := afero.NewMemMapFs()
		require.NoError(t, afero.WriteFile(fs, "/path/to/data.txt", []byte("data file"), 0o644))
		require.NoError(t, afero.WriteFile(fs, "/path/another/script/script.js", []byte(`
            module.exports = open("../../to/data.txt");
        `), 0o644))
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
		fs := afero.NewMemMapFs()
		require.NoError(t, afero.WriteFile(fs, "/path/to/data.txt", []byte("data file"), 0o644))
		require.NoError(t, afero.WriteFile(fs, "/path/another/script/script.js", []byte(`
        module.exports = () =>  open("./../data.txt"); // Here the path is relative to this module but to the one calling
        `), 0o644))
		require.NoError(t, afero.WriteFile(fs, "/path/to/script/script.js", []byte(`
        module.exports = require("./../../another/script/script.js")();
        `), 0o644))
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
		fs := afero.NewMemMapFs()
		require.NoError(t, afero.WriteFile(fs, "/path/to/data.js", []byte("module.exports='export content'"), 0o644))
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
		fs := afero.NewMemMapFs()
		require.NoError(t, afero.WriteFile(fs, "/path/to/data.js", []byte("module.exports='export content'"), 0o644))
		require.NoError(t, afero.WriteFile(fs, "/path/another/script/script.js", []byte(`
            module.exports = require("../../to/data.js");
        `), 0o644))
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
		fs := afero.NewMemMapFs()
		require.NoError(t, afero.WriteFile(fs, "/path/to/data.js", []byte("module.exports='export content'"), 0o644))
		require.NoError(t, afero.WriteFile(fs, "/path/another/script/script.js", []byte(`
        module.exports = () =>  require("./../data.js"); // Here the path is relative to this module but to the one calling
        `), 0o644))
		require.NoError(t, afero.WriteFile(fs, "/path/to/script/script.js", []byte(`
        module.exports = require("./../../another/script/script.js")();
        `), 0o644))
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
