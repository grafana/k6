package launcher

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/grafana/k6deps"
	"github.com/grafana/k6provider"

	"go.k6.io/k6/cmd/state"
)

// given a set of dependencies, returns the path to a k6 binary and the list of versions it provides
func k6buildProvision(gs *state.GlobalState, deps k6deps.Dependencies) (string, string, error) {
	opt := NewOptions(gs)
	if opt.BuildServiceToken == "" {
		return "", "", errors.New("Need a k6 cloud token for binary provisioning. " +
			"Setting K6_CLOUD_TOKEN environment variable or executing k6 cloud login is required.")
	}

	config := k6provider.Config{
		BuildServiceURL:  opt.BuildServiceURL,
		BuildServiceAuth: opt.BuildServiceToken,
	}

	provider, err := k6provider.NewProvider(config)
	if err != nil {
		return "", "", err
	}

	// TODO: we need a better handle of errors here
	// like (network, auth, etc) and give a better error message
	// to the user
	binary, err := provider.GetBinary(gs.Ctx, deps)
	if err != nil {
		return "", "", err
	}

	return binary.Path, formatDependencies(binary.Dependencies), nil
}

func formatDependencies(deps map[string]string) string {
	buffer := &bytes.Buffer{}
	for dep, version := range deps {
		buffer.WriteString(fmt.Sprintf("%s:%s ", dep, version))
	}
	return strings.Trim(buffer.String(), " ")
}
