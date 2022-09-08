package cmd

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/errext/exitcodes"
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

			testScript, err := ioutil.ReadFile(testCase.testFilename)
			require.NoError(t, err)

			testState := newGlobalTestState(t)
			require.NoError(t, afero.WriteFile(testState.fs, filepath.Join(testState.cwd, testCase.testFilename), testScript, 0o644))
			testState.args = []string{"k6", "archive", testCase.testFilename}
			if testCase.noThresholds {
				testState.args = append(testState.args, "--no-thresholds")
			}

			if testCase.wantErr {
				testState.expectedExitCode = int(exitcodes.InvalidConfig)
			}
			newRootCommand(testState.globalState).execute()
		})
	}
}

func TestArchiveContainsEnv(t *testing.T) {
	t.Parallel()

	// given some script that will be archived
	fileName := "script.js"
	testScript := []byte(`export default function () {}`)
	testState := newGlobalTestState(t)
	require.NoError(t, afero.WriteFile(testState.fs, filepath.Join(testState.cwd, fileName), testScript, 0o644))

	// when we do archiving and passing the `--env` flags
	testState.args = []string{"k6", "--env", "ENV1=lorem", "--env", "ENV2=ipsum", "archive", fileName}

	newRootCommand(testState.globalState).execute()
	require.NoError(t, untar(t, testState.fs, "archive.tar", "tmp/"))

	data, err := afero.ReadFile(testState.fs, "tmp/metadata.json")
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
	testState := newGlobalTestState(t)
	require.NoError(t, afero.WriteFile(testState.fs, filepath.Join(testState.cwd, fileName), testScript, 0o644))

	// when we do archiving and passing the `--env` flags altogether with `--exclude-env-vars` flag
	testState.args = []string{"k6", "--env", "ENV1=lorem", "--env", "ENV2=ipsum", "archive", "--exclude-env-vars", fileName}

	newRootCommand(testState.globalState).execute()
	require.NoError(t, untar(t, testState.fs, "archive.tar", "tmp/"))

	data, err := afero.ReadFile(testState.fs, "tmp/metadata.json")
	require.NoError(t, err)

	metadata := struct {
		Env map[string]string
	}{}

	// then unpacked metadata should not contain any environment variables passed at the moment of archive creation
	require.NoError(t, json.Unmarshal(data, &metadata))
	require.Len(t, metadata.Env, 0)
}

// untar untars a `fileName` file to a `destination` path
func untar(t *testing.T, fs afero.Fs, fileName string, destination string) error {
	t.Helper()

	archiveFile, err := afero.ReadFile(fs, fileName)
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
			if _, err := fs.Stat(target); err != nil && !os.IsNotExist(err) {
				return err
			}

			if err := fs.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			f, err := fs.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
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
