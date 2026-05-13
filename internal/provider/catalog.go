package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type CatalogEntry struct {
	ID           string             `json:"id"`
	Env          string             `json:"env,omitempty"`
	NPM          string             `json:"npm,omitempty"`
	API          string             `json:"api,omitempty"`
	Name         string             `json:"name,omitempty"`
	Doc          string             `json:"doc,omitempty"`
	Models       map[string]ModelEntry `json:"models"`
}

type ModelEntry struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	Family         string           `json:"family,omitempty"`
	ToolCall       bool             `json:"tool_call"`
	StructuredOutput bool           `json:"structured_output,omitempty"`
	Reasoning      bool             `json:"reasoning,omitempty"`
	Limit          *ModelLimit      `json:"limit,omitempty"`
	Cost           *ModelCost       `json:"cost,omitempty"`
}

type ModelLimit struct {
	Context int `json:"context"`
	Output  int `json:"output"`
}

type ModelCost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read,omitempty"`
	CacheWrite float64 `json:"cache_write,omitempty"`
}

type Catalog struct {
	mu          sync.RWMutex
	providers   map[string]CatalogEntry
	cachedAt    time.Time
	cacheTTL    time.Duration
	sourceURL   string
}

func NewCatalog() *Catalog {
	return &Catalog{
		providers: make(map[string]CatalogEntry),
		cacheTTL:  5 * time.Minute,
		sourceURL: "https://models.dev/api.json",
	}
}

func (c *Catalog) SetSourceURL(url string) {
	c.sourceURL = url
}

func (c *Catalog) LoadRaw(providers map[string]CatalogEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.providers = providers
	c.cachedAt = time.Now()
}

func (c *Catalog) Refresh() error {
	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Get(c.sourceURL)
	if err != nil {
		return fmt.Errorf("fetch catalog: %w", err)
	}
	defer resp.Body.Close()

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return fmt.Errorf("decode catalog: %w", err)
	}

	providers := make(map[string]CatalogEntry)
	for id, data := range raw {
		var entry CatalogEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}
		entry.ID = id
		providers[id] = entry
	}

	c.mu.Lock()
	c.providers = providers
	c.cachedAt = time.Now()
	c.mu.Unlock()

	return nil
}

func (c *Catalog) EnsureFresh() error {
	c.mu.RLock()
	age := time.Since(c.cachedAt)
	c.mu.RUnlock()

	if age > c.cacheTTL || age < 0 {
		return c.Refresh()
	}
	return nil
}

func (c *Catalog) ListProviders() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ids := make([]string, 0, len(c.providers))
	for id := range c.providers {
		ids = append(ids, id)
	}
	return ids
}

func (c *Catalog) GetProvider(id string) (CatalogEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.providers[id]
	return e, ok
}

func (c *Catalog) ListModels(providerID string) []ModelEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	prov, ok := c.providers[providerID]
	if !ok {
		return nil
	}

	models := make([]ModelEntry, 0, len(prov.Models))
	for _, m := range prov.Models {
		models = append(models, m)
	}
	return models
}

func (c *Catalog) ListFreeModels(providerID string) []ModelEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	prov, ok := c.providers[providerID]
	if !ok {
		return nil
	}

	var free []ModelEntry
	for _, m := range prov.Models {
		if m.Cost != nil && m.Cost.Input == 0 {
			free = append(free, m)
		}
	}
	return free
}

func (c *Catalog) GetModel(providerID, modelID string) (ModelEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	prov, ok := c.providers[providerID]
	if !ok {
		return ModelEntry{}, false
	}
	m, ok := prov.Models[modelID]
	return m, ok
}

func (c *Catalog) FindModel(modelID string) (string, ModelEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for pid, prov := range c.providers {
		if m, ok := prov.Models[modelID]; ok {
			return pid, m, true
		}
	}
	return "", ModelEntry{}, false
}

func (c *Catalog) BestFreeModel() (string, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	priority := []string{"big-pickle", "gpt-5-nano", "kimi-k2.5-free"}

	for _, modelID := range priority {
		for pid, prov := range c.providers {
			if m, ok := prov.Models[modelID]; ok {
				if m.Cost != nil && m.Cost.Input == 0 {
					return pid, modelID
				}
			}
		}
	}

	// Fallback: first free model from any provider
	for pid, prov := range c.providers {
		for mid, m := range prov.Models {
			if m.Cost != nil && m.Cost.Input == 0 {
				return pid, mid
			}
		}
	}

	return "", ""
}

func (c *Catalog) ProviderForModel(modelID string) string {
	pid, _, _ := c.FindModel(modelID)
	return pid
}

func (c *Catalog) ModelSupportsTools(modelID string) bool {
	_, m, ok := c.FindModel(modelID)
	return ok && m.ToolCall
}

func (c *Catalog) ModelContextLimit(modelID string) int {
	_, m, ok := c.FindModel(modelID)
	if ok && m.Limit != nil {
		return m.Limit.Context
	}
	return 0
}

func (c *Catalog) IsModelFree(modelID string) bool {
	_, m, ok := c.FindModel(modelID)
	return ok && m.Cost != nil && m.Cost.Input == 0
}

func (c *Catalog) String() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return fmt.Sprintf("Catalog(%d providers, cached %s ago)", len(c.providers), time.Since(c.cachedAt).Round(time.Second))
}
