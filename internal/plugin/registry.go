package plugin

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
	hooks   []Hook
}

func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
	}
}

func (r *Registry) Register(p Plugin, ctx *Context) error {
	name := p.Manifest().Name
	if name == "" {
		return fmt.Errorf("plugin has no name")
	}

	if err := p.Init(ctx); err != nil {
		return fmt.Errorf("init %s: %w", name, err)
	}

	r.mu.Lock()
	r.plugins[name] = p
	r.mu.Unlock()

	if h, ok := p.(WithHooks); ok {
		r.mu.Lock()
		r.hooks = append(r.hooks, h.Hooks()...)
		r.mu.Unlock()
	}

	return nil
}

func (r *Registry) Get(name string) Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.plugins[name]
}

func (r *Registry) List() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		list = append(list, p)
	}
	return list
}

func (r *Registry) RunHooks(point string, ctx *Context, data map[string]any) error {
	r.mu.RLock()
	hooks := make([]Hook, len(r.hooks))
	copy(hooks, r.hooks)
	r.mu.RUnlock()

	for _, h := range hooks {
		if h.Point == point {
			if err := h.Handler(ctx, data); err != nil {
				return fmt.Errorf("hook %s: %w", point, err)
			}
		}
	}
	return nil
}

func (r *Registry) ToolDefs() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var defs []map[string]any
	for _, p := range r.plugins {
		if t, ok := p.(WithTools); ok {
			defs = append(defs, t.ToolDefs()...)
		}
	}
	return defs
}

func ExecuteExternalPlugin(plugin Plugin, method string, params map[string]any) (*ExternalResponse, error) {
	ep, ok := plugin.(*externalPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin is not external")
	}

	req := ExternalRequest{
		ID:     method,
		Method: method,
		Params: params,
	}
	reqJSON, _ := json.Marshal(req)

	entryPath := ep.Manifest().Entrypoint

	cmd := exec.Command(entryPath)
	cmd.Stdin = strings.NewReader(string(reqJSON))

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("execute plugin: %w", err)
	}

	var resp ExternalResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return &ExternalResponse{
			ID:     method,
			Result: string(output),
		}, nil
	}

	return &resp, nil
}
