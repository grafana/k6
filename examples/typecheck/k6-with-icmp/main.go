// Command k6-with-icmp builds k6 with the typed xk6-icmp example embedded.
package main

import (
	"go.k6.io/k6/v2/cmd"

	_ "go.k6.io/k6/examples/typecheck/xk6-icmp"
)

func main() {
	cmd.Execute()
}
