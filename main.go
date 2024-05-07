// Package main is the entry point for the k6 CLI application. It assembles all the crucial components for the running.
package main

import (
	"github.com/liuxd6825/k6server/cmd"
)

func main() {
	cmd.Execute()
}
