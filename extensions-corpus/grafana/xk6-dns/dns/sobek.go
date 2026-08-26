package dns

import (
	"fmt"

	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/js/modules"
)

// ensureVUContext ensures that the VU context is available.
//
// This function will return an InitContextError if the vu runtime is in the init context.
func ensureVUContext(vu modules.VU, resourceName string) error {
	if vu.State() == nil {
		return common.NewInitContextError(fmt.Sprintf("using %s in the init context is not supported", resourceName))
	}
	return nil
}
