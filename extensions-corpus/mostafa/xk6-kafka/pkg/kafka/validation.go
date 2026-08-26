package kafka

import (
	"encoding/json"
	"fmt"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
)

func newInvalidConfigError(component string, originalErr error) *Xk6KafkaError {
	return NewXk6KafkaError(invalidConfiguration, "Invalid "+component, originalErr)
}

func newMissingConfigError(component string) *Xk6KafkaError {
	return NewXk6KafkaError(invalidConfiguration, component+" is required", nil)
}

func throwConfigError(runtime *sobek.Runtime, err *Xk6KafkaError) {
	logger.WithField("error", err).Error(err)
	common.Throw(runtime, err)
}

func exportArgumentMap(runtime *sobek.Runtime, value sobek.Value, component string) map[string]any {
	exported := value.Export()
	if exported == nil {
		throwConfigError(runtime, newMissingConfigError(component))
		return nil
	}

	params, ok := exported.(map[string]any)
	if !ok {
		throwConfigError(runtime, newInvalidConfigError(
			component,
			fmt.Errorf("%w, got %T", errExpectedObject, exported),
		))
		return nil
	}

	return params
}

func decodeArgument(runtime *sobek.Runtime, value sobek.Value, target any, component string) {
	decodeArgumentMap(runtime, exportArgumentMap(runtime, value, component), target, component)
}

func decodeArgumentMap(runtime *sobek.Runtime, params map[string]any, target any, component string) {
	if params == nil {
		throwConfigError(runtime, newMissingConfigError(component))
		return
	}

	b, err := json.Marshal(params)
	if err != nil {
		throwConfigError(runtime, newInvalidConfigError(component, err))
		return
	}

	if err := json.Unmarshal(b, target); err != nil {
		throwConfigError(runtime, newInvalidConfigError(component, err))
	}
}
