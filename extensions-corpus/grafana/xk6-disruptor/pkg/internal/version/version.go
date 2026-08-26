// Package version provide information about the build version
package version

import (
	"runtime/debug"
)

const xk6DisruptorPath = "github.com/grafana/xk6-disruptor"

// DisruptorVersion returns the version of the currently executed disruptor
func DisruptorVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, d := range bi.Deps {
			if d.Path == xk6DisruptorPath {
				if d.Replace != nil {
					return d.Replace.Version
				}
				return d.Version
			}
		}
	}

	return ""
}

// AgentImage returns the name of the agent image that corresponds to
// this version of the extension.
func AgentImage() string {
	tag := "latest"

	// if a specific version of the disruptor was built, use it for agent's tag
	// (go test sets version to "")
	dv := DisruptorVersion()
	if dv != "" && dv != "(devel)" {
		tag = dv
	}

	return "ghcr.io/grafana/xk6-disruptor-agent:" + tag
}
