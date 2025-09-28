package grpccompress

import (
	"fmt"
	"strings"
	"sync"

	"google.golang.org/grpc"
)

type Spec struct {
	Name    string
	Options map[string]any
}

type Plugin interface {
	Name() string
	EnsureRegistered() error
	Configure(options map[string]any) error
	CallOption() grpc.CallOption
}

var (
	regMu    sync.RWMutex
	registry = map[string]Plugin{}
)

func Register(p Plugin) {
	regMu.Lock()
	defer regMu.Unlock()
	registry[strings.ToLower(p.Name())] = p
}

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
