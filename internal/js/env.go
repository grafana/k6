package js

import (
	"fmt"

	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/js/common"
)

// newFrozenEnvObject builds the global __ENV object exposed to user scripts.
// Each variable is defined as a non-writable, non-configurable data property
// and the object is frozen so assignments like __ENV.foo = "bar" are rejected.
func newFrozenEnvObject(rt *sobek.Runtime, env map[string]string) (*sobek.Object, error) {
	envObj := rt.NewObject()
	for k, v := range env {
		if err := envObj.DefineDataProperty(
			k, rt.ToValue(v), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE,
		); err != nil {
			return nil, fmt.Errorf("setting __ENV property %s: %w", k, err)
		}
	}
	if err := common.FreezeObject(rt, envObj); err != nil {
		return nil, fmt.Errorf("freezing __ENV: %w", err)
	}
	return envObj, nil
}
