package testscript

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"go.k6.io/k6/v2/cmd"
)

//nolint:gochecknoglobals
var (
	subprocess = flag.Bool("subprocess", false, "Indicates if the integration test is running in a subprocess.")
	script     = flag.String("script", "", "Script file to run in integration test.")
)

// RunFileIntegration runs a single testscript file
// in a subprocess using k6's cmd.Execute().
func RunFileIntegration(t *testing.T, file string) {
	t.Helper()

	runIntegrationSubprocess(t)

	t.Attr("script", file)

	exe, err := os.Executable() //nolint:forbidigo // test harness needs the test binary path
	if err != nil {
		t.Fatalf("failed to get current executable: %v", err)
	}

	cmd := exec.CommandContext( //#nosec:G204
		t.Context(),
		exe,
		"-test.run="+t.Name(),
		"-subprocess",
		"-script="+file,
	)

	cmd.Stdout = t.Output()
	cmd.Stderr = t.Output()

	err = cmd.Run()
	if err != nil {
		t.Fatalf("Subprocess exited with error: %v", err)
	}
}

// RunFilesIntegration runs all provided testscript files
// in subprocesses using k6's cmd.Execute().
func RunFilesIntegration(t *testing.T, files ...string) {
	t.Helper()

	runIntegrationSubprocess(t)

	if len(files) == 0 {
		t.Fatal("no test files provided")

		return
	}

	for _, file := range files {
		t.Run(filepath.ToSlash(file), func(t *testing.T) {
			RunFileIntegration(t, file)
		})
	}
}

// RunGlobIntegration runs all testscript files matching the provided glob pattern
// in subprocesses using k6's cmd.Execute().
func RunGlobIntegration(t *testing.T, glob string) {
	t.Helper()

	runIntegrationSubprocess(t)

	files, err := filepath.Glob(glob)
	if err != nil {
		t.Fatalf("glob %q failed: %v", glob, err)

		return
	}

	RunFilesIntegration(t, files...)
}

func runIntegrationSubprocess(t *testing.T) {
	t.Helper()

	if !*subprocess || len(*script) == 0 {
		return
	}

	os.Args = []string{ //nolint:forbidigo // test harness sets argv to invoke the embedded k6
		"k6",
		"--quiet",
		"--log-format=raw",
		"--summary-mode=disabled",
		"--no-usage-report",
		"--address=127.0.0.1:0",
		"--vus=1",
		"--iterations=1",
		"--no-thresholds",
		"--no-setup",
		"--no-teardown",
		"--no-color",
		"run",
		*script,
	}

	cmd.Execute()
}
