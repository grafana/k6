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
func k6buildProvision(gs *state.GlobalState, deps k6deps.Dependencies) (commandExecutor, error) {
	opt := newOptions(gs)
	if opt.BuildServiceToken == "" {
		return nil, errors.New("k6 cloud token is required when Binary provisioning feature is enabled." +
			" Set K6_CLOUD_TOKEN environment variable or execute the k6 cloud login command")
	}

	config := k6provider.Config{
		BuildServiceURL:  opt.BuildServiceURL,
		BuildServiceAuth: opt.BuildServiceToken,
	}

	provider, err := k6provider.NewProvider(config)
	if err != nil {
		return nil, err
	}

	binary, err := provider.GetBinary(gs.Ctx, deps)
	if err != nil {
		return nil, err
	}

	gs.Logger.
		Info("A new k6 binary has been provisioned with version(s): ", formatDependencies(binary.Dependencies))

	return &customBinary{binary.Path}, nil
}

func formatDependencies(deps map[string]string) string {
	buffer := &bytes.Buffer{}
	for dep, version := range deps {
		buffer.WriteString(fmt.Sprintf("%s:%s ", dep, version))
	}
	return strings.Trim(buffer.String(), " ")
}
