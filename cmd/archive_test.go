package cmd

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/cmd/tests"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/lib/fsext"
)

func TestArchiveThresholds(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		noThresholds bool
		testFilename string

		wantErr bool
	}{
		{
			name:         "archive should fail with exit status 104 on a malformed threshold expression",
			noThresholds: false,
			testFilename: "testdata/thresholds/malformed_expression.js",
			wantErr:      true,
		},
		{
			name:         "archive should on a malformed threshold expression but --no-thresholds flag set",
			noThresholds: true,
			testFilename: "testdata/thresholds/malformed_expression.js",
			wantErr:      false,
		},
		{
			name:         "run should fail with exit status 104 on a threshold applied to a non existing metric",
			noThresholds: false,
			testFilename: "testdata/thresholds/non_existing_metric.js",
			wantErr:      true,
		},
		{
			name:         "run should succeed on a threshold applied to a non existing metric with the --no-thresholds flag set",
			noThresholds: true,
			testFilename: "testdata/thresholds/non_existing_metric.js",
			wantErr:      false,
		},
		{
			name:         "run should succeed on a threshold applied to a non existing submetric with the --no-thresholds flag set",
			noThresholds: true,
			testFilename: "testdata/thresholds/non_existing_metric.js",
			wantErr:      false,
		},
		{
			name:         "run should fail with exit status 104 on a threshold applying an unsupported aggregation method to a metric",
			noThresholds: false,
			testFilename: "testdata/thresholds/unsupported_aggregation_method.js",
			wantErr:      true,
		},
		{
			name:         "run should succeed on a threshold applying an unsupported aggregation method to a metric with the --no-thresholds flag set",
			noThresholds: true,
			testFilename: "testdata/thresholds/unsupported_aggregation_method.js",
			wantErr:      false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			testScript, err := os.ReadFile(testCase.testFilename) //nolint:forbidigo
			require.NoError(t, err)

			ts := tests.NewGlobalTestState(t)
			require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, testCase.testFilename), testScript, 0o644))
			ts.CmdArgs = []string{"k6", "archive", testCase.testFilename}
			if testCase.noThresholds {
				ts.CmdArgs = append(ts.CmdArgs, "--no-thresholds")
			}

			if testCase.wantErr {
				ts.ExpectedExitCode = int(exitcodes.InvalidConfig)
			}
			newRootCommand(ts.GlobalState).execute()
		})
	}
}

func TestArchiveContainsEnv(t *testing.T) {
	t.Parallel()

	// given some script that will be archived
	fileName := "script.js"
	testScript := []byte(`export default function () {}`)
	ts := tests.NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, fileName), testScript, 0o644))

	// when we do archiving and passing the `--env` flags
	ts.CmdArgs = []string{"k6", "--env", "ENV1=lorem", "--env", "ENV2=ipsum", "archive", fileName}

	newRootCommand(ts.GlobalState).execute()
	require.NoError(t, untar(t, ts.FS, "archive.tar", "tmp/"))

	data, err := fsext.ReadFile(ts.FS, "tmp/metadata.json")
	require.NoError(t, err)

	metadata := struct {
		Env map[string]string
	}{}

	// then unpacked metadata should contain the environment variables with the proper values
	require.NoError(t, json.Unmarshal(data, &metadata))
	require.Len(t, metadata.Env, 2)

	require.Contains(t, metadata.Env, "ENV1")
	require.Contains(t, metadata.Env, "ENV2")

	require.Equal(t, "lorem", metadata.Env["ENV1"])
	require.Equal(t, "ipsum", metadata.Env["ENV2"])
}

func TestArchiveNotContainsEnv(t *testing.T) {
	t.Parallel()

	// given some script that will be archived
	fileName := "script.js"
	testScript := []byte(`export default function () {}`)
	ts := tests.NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, fileName), testScript, 0o644))

	// when we do archiving and passing the `--env` flags altogether with `--exclude-env-vars` flag
	ts.CmdArgs = []string{"k6", "--env", "ENV1=lorem", "--env", "ENV2=ipsum", "archive", "--exclude-env-vars", fileName}

	newRootCommand(ts.GlobalState).execute()
	require.NoError(t, untar(t, ts.FS, "archive.tar", "tmp/"))

	data, err := fsext.ReadFile(ts.FS, "tmp/metadata.json")
	require.NoError(t, err)

	metadata := struct {
		Env map[string]string
	}{}

	// then unpacked metadata should not contain any environment variables passed at the moment of archive creation
	require.NoError(t, json.Unmarshal(data, &metadata))
	require.Len(t, metadata.Env, 0)
}

// untar untars a `fileName` file to a `destination` path
func untar(t *testing.T, fileSystem fsext.Fs, fileName string, destination string) error {
	t.Helper()

	archiveFile, err := fsext.ReadFile(fileSystem, fileName)
	if err != nil {
		return err
	}

	reader := bytes.NewBuffer(archiveFile)

	tr := tar.NewReader(reader)

	for {
		header, err := tr.Next()
		switch {
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return err
		case header == nil:
			continue
		}

		// as long as this code in a test helper, we can safely
		// omit G305: File traversal when extracting zip/tar archive
		target := filepath.Join(destination, header.Name) //nolint:gosec

		switch header.Typeflag {
		case tar.TypeDir:
			if _, err := fileSystem.Stat(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}

			if err := fileSystem.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			f, err := fileSystem.OpenFile(target, syscall.O_CREAT|syscall.O_RDWR, fs.FileMode(header.Mode))
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()

			// as long as this code in a test helper, we can safely
			// omit G110: Potential DoS vulnerability via decompression bomb
			if _, err := io.Copy(f, tr); err != nil { //nolint:gosec
				return err
			}
		}
	}
}
