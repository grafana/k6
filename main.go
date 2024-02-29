// Package main is the entry point for the k6 CLI application. It assembles all the crucial components for the running.
package main

import (
	"plugin"

	"go.k6.io/k6/cmd"
)

func main() {
	_, err := plugin.Open("plugin_name.so") // only init is important
	if err != nil {
		panic(err)
	}
	cmd.Execute()
}
