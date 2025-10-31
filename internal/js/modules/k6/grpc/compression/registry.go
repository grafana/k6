// Package grpccompress provides a minimal plugin registry for gRPC message compression.
// It allows registering compression plugins and constructing writers at runtime.
package grpccompress

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"google.golang.org/grpc"
)

var (
	errUnsupportedCompression = errors.New("unsupported compression")
	errDuplicateRegistration  = errors.New("duplicate compressor registration")
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

// Registry is a threadsafe plugin registry.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin // key: lowercased plugin name
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{plugins: make(map[string]Plugin)}
}

// Register adds a compression plugin to the registry.
// It lowercases the name and rejects duplicates.
func (r *Registry) Register(p Plugin) error {
	if p == nil {
		return fmt.Errorf("nil plugin")
	}
	key := strings.ToLower(strings.TrimSpace(p.Name()))
	if key == "" {
		return fmt.Errorf("plugin name is empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[key]; exists {
		return fmt.Errorf("%w: %q", errDuplicateRegistration, key)
	}
	r.plugins[key] = p
	return nil
}

// Configure resolves a plugin by name and applies the compression options.
// Returns the configured Plugin so callers can obtain CallOption().
func (r *Registry) Configure(spec Spec) (Plugin, error) {
	name := strings.ToLower(strings.TrimSpace(spec.Name))
	r.mu.RLock()
	p, ok := r.plugins[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", errUnsupportedCompression, spec.Name)
	}
	if err := p.EnsureRegistered(); err != nil {
		return nil, err
	}
	if err := p.Configure(spec.Options); err != nil {
		return nil, err
	}
	return p, nil
}
