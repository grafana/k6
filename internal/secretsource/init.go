// Package secretsource registers all the internal secret sources when imported
package secretsource

import (
	_ "go.k6.io/k6/internal/secretsource/file" // import them for init
	_ "go.k6.io/k6/internal/secretsource/mock" // import them for init
)
