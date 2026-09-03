package tools

import "fmt"

type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}
	if _, exists := r.tools[tool.Name()]; exists {
		return fmt.Errorf("tool %q already registered", tool.Name())
	}
	r.tools[tool.Name()] = tool
	return nil
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

func (r *Registry) Definitions(names []string) ([]Definition, error) {
	definitions := make([]Definition, 0, len(names))
	for _, name := range names {
		tool, ok := r.Get(name)
		if !ok {
			return nil, fmt.Errorf("unknown tool %q", name)
		}

		schema := EmptyObjectSchema()
		if provider, ok := tool.(SchemaProvider); ok {
			schema = provider.ParameterSchema()
		}

		definitions = append(definitions, Definition{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  schema.Map(),
		})
	}

	return definitions, nil
}
