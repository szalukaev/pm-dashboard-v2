package datasource

import "fmt"

type Registry struct {
	factories map[string]func(SourceConfig) DataSource
}

func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]func(SourceConfig) DataSource)}
}

func (r *Registry) Register(name string, factory func(SourceConfig) DataSource) {
	r.factories[name] = factory
}

func (r *Registry) Create(name string, cfg SourceConfig) (DataSource, error) {
	factory, ok := r.factories[name]
	if !ok {
		return nil, fmt.Errorf("unknown data source: %s", name)
	}
	return factory(cfg), nil
}

func (r *Registry) List() []string {
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}
