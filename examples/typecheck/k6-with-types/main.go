// Command k6-with-types builds k6 with the example typed extension embedded.
package main

import (
	"go.k6.io/k6/v2/cmd"

	_ "go.k6.io/k6/v2/examples/typecheck/xk6-types"
)

func main() {
	cmd.Execute()
}
