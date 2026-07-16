package js

import (
	"fmt"
	"maps"

	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/internal/features"
	"go.k6.io/k6/v2/js/common"
)

// setupEnvObject configures the global __ENV object on the given runtime.
// When freeze is true, each property is defined as non-writable, non-configurable
// and the object is frozen so assignments like __ENV.foo = "bar" are rejected.
// When freeze is false, the old mutable map behavior is used (default).
func setupEnvObject(rt *sobek.Runtime, env map[string]string, flags *features.Flags) error {
	if flags != nil && flags.FreezeEnv {
		envObj := rt.NewObject()
		for k, v := range env {
			if err := envObj.DefineDataProperty(
				k, rt.ToValue(v), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE,
			); err != nil {
				return fmt.Errorf("setting __ENV property %s: %w", k, err)
			}
		}
		if err := common.FreezeObject(rt, envObj); err != nil {
			return fmt.Errorf("freezing __ENV: %w", err)
		}
		return rt.Set("__ENV", envObj)
	}

	// Old behavior: mutable map
	envMap := make(map[string]string, len(env))
	maps.Copy(envMap, env)
	return rt.Set("__ENV", envMap)
}
