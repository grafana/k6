package insights

import (
	"go.k6.io/k6/cloudapi"
)

// Enabled returns true if the k6 x Tempo feature is enabled.
func Enabled(config cloudapi.Config) bool {
	// TODO(lukasz): Check if k6 x Tempo is enabled
	//
	// We want to check whether a given organization is
	// eligible for k6 x Tempo feature. If it isn't, we may
	// consider to skip the traces output.
	//
	// We currently don't have a backend API to check this
	// information.
	return config.TracesEnabled.Bool
}
