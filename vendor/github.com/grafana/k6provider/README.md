# k6provider

A library for providing custom k6 binaries that satisfy a given set of dependencies.

Dependencies are specified using [k6deps.Dependencies](https://pkg.go.dev/github.com/grafana/k6deps#Dependencies).

The binary is obtained from a [k6build service](https://github.com/grafana/k6build).

See the the configuration options in the [package documentation](https://pkg.go.dev/github.com/grafana/k6provider).

## Example

This [example](examples/example.go) shows how to use k6provider to obtain a k6 binary with an specific version.

Requires the URL to the [k6build service](https://github.com/grafana/k6build) defined in the `K6_BUILD_SERVICE_URL` environment variable.

```golang
// Package main is an example of how to use k6provider
package main

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/grafana/k6deps"
	"github.com/grafana/k6provider"
)

func main() {
	// get a k6 provider configured with a build service defined in K6_BUILD_SERVICE_URL
	provider, err := k6provider.NewDefaultProvider()
	if err != nil {
		panic(err)
	}

	// create dependencies for k6 version v0.52.0
	deps := make(k6deps.Dependencies)
	err = deps.UnmarshalText([]byte("k6=v0.52.0"))
	if err != nil {
		panic(err)
	}

	// obtain binary from the build service
	k6binary, err := provider.GetBinary(context.TODO(), deps)
	if err != nil {
		panic(err)
	}

	// execute k6 binary and check version
	cmd := exec.Command(k6binary.Path, "version")
	out, err := cmd.Output()
	if err != nil {
		panic(err)
	}

	fmt.Print(string(out))
}
```

Output:

```
k6 v0.52.0 (go1.22.4, linux/amd64)
```