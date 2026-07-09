package adapters

import (
	"fmt"
	"sort"

	"github.com/pax-beehive/memory-adaptor/internal/adapters/local"
	"github.com/pax-beehive/memory-adaptor/internal/config"
	"github.com/pax-beehive/memory-adaptor/internal/memory"
)

type Factory func(name string, cfg config.ProviderConfig) (memory.Provider, error)

type Registry struct {
	factories map[string]Factory
}

func DefaultRegistry() Registry {
	registry := Registry{factories: make(map[string]Factory)}
	registry.Register("local", func(name string, cfg config.ProviderConfig) (memory.Provider, error) {
		return local.New(name, cfg.Path)
	})
	return registry
}

func (r Registry) Register(providerType string, factory Factory) {
	r.factories[providerType] = factory
}

func (r Registry) BuildRouter(cfg config.Config) (*memory.Router, error) {
	var names []string
	for name := range cfg.Providers {
		names = append(names, name)
	}
	sort.Strings(names)

	var bindings []memory.ProviderBinding
	for _, name := range names {
		providerCfg := cfg.Providers[name]
		if !providerCfg.Enabled {
			continue
		}
		factory, ok := r.factories[providerCfg.Type]
		if !ok {
			return nil, fmt.Errorf("provider %q uses unsupported type %q", name, providerCfg.Type)
		}
		provider, err := factory(name, providerCfg)
		if err != nil {
			return nil, fmt.Errorf("provider %q: %w", name, err)
		}
		bindings = append(bindings, memory.ProviderBinding{
			Provider: provider,
			Read:     providerCfg.Read,
			Write:    providerCfg.Write,
			Required: providerCfg.Required,
			Weight:   providerCfg.Weight,
		})
	}
	return memory.NewRouter(bindings)
}
