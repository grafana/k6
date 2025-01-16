// Package webcrypto exports the webcrypto API.
package webcrypto

import (
	"github.com/grafana/xk6-webcrypto/webcrypto"
	"go.k6.io/k6/js/modules"
)

// Register the extension on module initialization, available to
// import from JS as "k6/x/webcrypto".
func init() {
	modules.Register("k6/x/webcrypto", new(webcrypto.RootModule))
}
