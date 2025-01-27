// Package cmd is here to provide a way for xk6 to build a binary with added extensions
package cmd

import "go.k6.io/k6/internal/cmd"

// Execute exectues the k6 command
// It only is exported here for backwards compatibility and the ability to use xk6 to build extended k6
func Execute() {
	cmd.Execute()
}
