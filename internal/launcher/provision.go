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

// k6buildProvision returns the path to a k6 binary that satisfies the dependencies and the list of versions it provides
func k6buildProvision(gs *state.GlobalState, deps k6deps.Dependencies) (string, string, error) {
	opt := newOptions(gs)
	if opt.BuildServiceToken == "" {
		return "", "", errors.New("k6 cloud token is required when Binary provisioning feature is enabled." +
			" Set K6_CLOUD_TOKEN environment variable or execute the k6 cloud login command.")
	}

	config := k6provider.Config{
		BuildServiceURL:  opt.BuildServiceURL,
		BuildServiceAuth: opt.BuildServiceToken,
	}

	provider, err := k6provider.NewProvider(config)
	if err != nil {
		return "", "", err
	}

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
