package tool

import "sync"

type Registry struct {
	mu    sync.RWMutex
	tools map[string]*Info
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]*Info),
	}
}

func (r *Registry) Register(info *Info) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[info.ID] = info
}

func (r *Registry) Get(id string) (*Def, error) {
	r.mu.RLock()
	info, ok := r.tools[id]
	r.mu.RUnlock()
	if !ok {
		return nil, nil
	}
	return info.Init()
}

func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.tools))
	for id := range r.tools {
		ids = append(ids, id)
	}
	return ids
}

func (r *Registry) All() ([]*Def, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var defs []*Def
	for _, info := range r.tools {
		def, err := info.Init()
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}
	return defs, nil
}
