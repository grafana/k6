// k6-extension-api builds a local k6 binary with extensions that use the new
// standalone extension API.
package main

import (
	_ "github.com/grafana/xk6-ssh"
	_ "github.com/tango-tango/xk6-msgpack"
	"go.k6.io/k6/v2/cmd"
)

func main() {
	cmd.Execute()
}
