package kubernetes

import (
	"errors"
	"os"
	"path/filepath"
)

// getConfigPath Copied from ahmetb/kubectx source code:
// https://github.com/ahmetb/kubectx/blob/29850e1a75cb5cad8d93f74a4114311eb9feba9f/internal/kubeconfig/kubeconfigloader.go#L59
func getConfigPath() (string, error) {
	// KUBECONFIG env var
	if v := os.Getenv("KUBECONFIG"); v != "" {
		list := filepath.SplitList(v)
		if len(list) > 1 {
			// TODO KUBECONFIG=file1:file2 currently not supported
			return "", errors.New("multiple files in KUBECONFIG are currently not supported")
		}
		return v, nil
	}

	// default path
	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE") // windows
	}
	if home == "" {
		return "", errors.New("HOME or USERPROFILE environment variable not set")
	}
	return filepath.Join(home, ".kube", "config"), nil
}
