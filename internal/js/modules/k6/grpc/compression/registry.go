// Package grpccompress provides a minimal plugin registry for gRPC message compression.
// It allows registering compression plugins and constructing writers at runtime.
package grpccompress

import (
	"fmt"
	"strings"
	"sync"

	"google.golang.org/grpc"
)

// Spec describes the compression plugin to use and its configuration.
type Spec struct {
	Name    string
	Options map[string]any
}

// Plugin represents a compression implementation that can produce writers
// and be configured with a map-based configuration.
type Plugin interface {
	Name() string
	EnsureRegistered() error
	Configure(options map[string]any) error
	CallOption() grpc.CallOption
}

var (
	regMu    sync.RWMutex          //nolint:gochecknoglobals
	registry = map[string]Plugin{} //nolint:gochecknoglobals
)

// Register adds a compression plugin to the global registry.
func Register(p Plugin) {
	regMu.Lock()
	defer regMu.Unlock()
	registry[strings.ToLower(p.Name())] = p
}

// Configure resolves a plugin by name and applies the compression options.
func Configure(spec Spec) (Plugin, error) {
	name := strings.ToLower(spec.Name)
	regMu.RLock()
	p, ok := registry[name]
	regMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unsupported compression: %q", spec.Name)
	}
	if err := p.EnsureRegistered(); err != nil {
		return nil, err
	}
	if err := p.Configure(spec.Options); err != nil {
		return nil, err
	}
	return p, nil
}
