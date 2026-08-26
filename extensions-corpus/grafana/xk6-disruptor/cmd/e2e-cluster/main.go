// Package main implements the main function for the e2e environment setup tool
package main

import (
	"fmt"
	"os"

	"github.com/grafana/xk6-disruptor/cmd/e2e-cluster/commands"
)

func main() {
	rootCmd := commands.BuildRootCmd()
	rootCmd.AddCommand(commands.BuildSetupCmd())
	rootCmd.AddCommand(commands.BuildCleanupCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
