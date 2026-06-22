package ageval

// toolRegistry maps tool names to their executors and preserves registration
// order so tool schemas are sent to the model deterministically (stable order
// keeps prompt caching effective and evals reproducible).
type toolRegistry struct {
	order []string
	tools map[string]*evalTool
}

func newToolRegistry() *toolRegistry {
	return &toolRegistry{tools: make(map[string]*evalTool)}
}

// add registers a tool, replacing any existing tool with the same name (later
// registrations win, which lets a skill override a base tool).
func (r *toolRegistry) add(t *evalTool) {
	if _, exists := r.tools[t.name]; !exists {
		r.order = append(r.order, t.name)
	}
	r.tools[t.name] = t
}

func (r *toolRegistry) get(name string) (*evalTool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// schemas returns the tool schemas in registration order for the provider.
func (r *toolRegistry) schemas() []toolSchema {
	out := make([]toolSchema, 0, len(r.order))
	for _, name := range r.order {
		t := r.tools[name]
		out = append(out, toolSchema{name: t.name, description: t.description, inputSchema: t.inputSchema})
	}
	return out
}
