package appliers

import (
	"context"
	"sync"

	"github.com/thesabbir/hellfire/pkg/uci"
)

// Applier is the interface for applying configuration changes
type Applier interface {
	Name() string
	Apply(ctx context.Context, config *uci.Config) error
	Validate(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// Registry manages registered appliers
type Registry struct {
	mu       sync.RWMutex
	appliers map[string]Applier
}

// NewRegistry creates a new applier registry
func NewRegistry() *Registry {
	return &Registry{
		appliers: make(map[string]Applier),
	}
}

// Register registers an applier
func (r *Registry) Register(applier Applier) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.appliers[applier.Name()] = applier
}

// Get retrieves an applier by name
func (r *Registry) Get(name string) (Applier, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	applier, ok := r.appliers[name]
	return applier, ok
}

// List returns all registered applier names
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.appliers))
	for name := range r.appliers {
		names = append(names, name)
	}
	return names
}

// DefaultRegistry creates a registry with all default appliers
func DefaultRegistry() *Registry {
	registry := NewRegistry()
	registry.Register(NewNetworkApplier())
	registry.Register(NewFirewallApplier())
	registry.Register(NewDHCPApplier())
	return registry
}
