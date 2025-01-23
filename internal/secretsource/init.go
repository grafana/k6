package secretsource

// TODO(@mstoykov): do we want this? or do we want to have a function like createOutputs?
import (
	_ "go.k6.io/k6/internal/secretsource/file" //nolint:revive
	_ "go.k6.io/k6/internal/secretsource/mock" //nolint:revive
)
