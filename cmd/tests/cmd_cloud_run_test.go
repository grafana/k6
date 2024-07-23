package tests

import "testing"

func TestK6CloudRun(t *testing.T) {
	t.Parallel()
	runCloudTests(t, setupK6CloudRunCmd)
}

func setupK6CloudRunCmd(cliFlags []string) []string {
	return append([]string{"k6", "cloud", "run"}, append(cliFlags, "test.js")...)
}
