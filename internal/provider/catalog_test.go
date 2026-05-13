package provider_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/opencode-go/opencode/internal/provider"
)

func TestNewCatalog(t *testing.T) {
	c := provider.NewCatalog()
	if c == nil {
		t.Fatal("expected non-nil catalog")
	}
}

func TestCatalogRefreshFromServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"opencode": {
				"id": "opencode",
				"name": "OpenCode Zen",
				"api": "https://opencode.ai/zen/v1",
				"models": {
					"big-pickle": {
						"id": "big-pickle",
						"name": "Big Pickle",
						"tool_call": true,
						"cost": { "input": 0, "output": 0 }
					},
					"gpt-5-nano": {
						"id": "gpt-5-nano",
						"name": "GPT-5 Nano",
						"tool_call": true,
						"cost": { "input": 0, "output": 0 }
					},
					"gpt-4o": {
						"id": "gpt-4o",
						"name": "GPT-4o",
						"tool_call": true,
						"cost": { "input": 2.5, "output": 10 }
					}
				}
			},
			"openai": {
				"id": "openai",
				"name": "OpenAI",
				"api": "https://api.openai.com/v1",
				"models": {
					"gpt-4o": {
						"id": "gpt-4o",
						"name": "GPT-4o",
						"tool_call": true,
						"limit": { "context": 128000 },
						"cost": { "input": 2.5, "output": 10 }
					}
				}
			}
		}`))
	}))
	defer srv.Close()

	c := provider.NewCatalog()
	// Override the source URL for testing
	c.SetSourceURL(srv.URL)
	if err := c.Refresh(); err != nil {
		t.Fatal(err)
	}

	providers := c.ListProviders()
	if len(providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(providers))
	}

	// Check opencode provider
	oc, ok := c.GetProvider("opencode")
	if !ok {
		t.Fatal("expected opencode provider")
	}
	if oc.Name != "OpenCode Zen" {
		t.Errorf("expected 'OpenCode Zen', got %q", oc.Name)
	}
	if oc.API != "https://opencode.ai/zen/v1" {
		t.Errorf("expected Zen API URL, got %q", oc.API)
	}
}

func TestListFreeModels(t *testing.T) {
	c := provider.NewCatalog()
	c.LoadRaw(map[string]provider.CatalogEntry{
		"opencode": {
			ID: "opencode", Name: "OpenCode Zen",
			API: "https://opencode.ai/zen/v1",
			Models: map[string]provider.ModelEntry{
				"big-pickle":  {ID: "big-pickle", Name: "Big Pickle", ToolCall: true, Cost: &provider.ModelCost{Input: 0, Output: 0}},
				"gpt-5-nano":  {ID: "gpt-5-nano", Name: "GPT-5 Nano", ToolCall: true, Cost: &provider.ModelCost{Input: 0, Output: 0}},
				"gpt-4o":      {ID: "gpt-4o", Name: "GPT-4o", ToolCall: true, Cost: &provider.ModelCost{Input: 2.5, Output: 10}},
			},
		},
	})

	free := c.ListFreeModels("opencode")
	if len(free) != 2 {
		t.Errorf("expected 2 free models, got %d", len(free))
	}
}

func TestBestFreeModel(t *testing.T) {
	c := provider.NewCatalog()
	c.LoadRaw(map[string]provider.CatalogEntry{
		"opencode": {
			ID: "opencode", Name: "OpenCode Zen",
			API: "https://opencode.ai/zen/v1",
			Models: map[string]provider.ModelEntry{
				"gpt-4o":      {ID: "gpt-4o", Cost: &provider.ModelCost{Input: 2.5}},
				"gpt-5-nano":  {ID: "gpt-5-nano", Cost: &provider.ModelCost{Input: 0}},
				"big-pickle":  {ID: "big-pickle", Cost: &provider.ModelCost{Input: 0}},
			},
		},
	})

	pid, mid := c.BestFreeModel()
	if mid != "big-pickle" {
		t.Errorf("expected 'big-pickle', got %q (provider: %s)", mid, pid)
	}
}

func TestFindModel(t *testing.T) {
	c := provider.NewCatalog()
	c.LoadRaw(map[string]provider.CatalogEntry{
		"opencode": {
			ID: "opencode", Models: map[string]provider.ModelEntry{
				"big-pickle": {ID: "big-pickle", ToolCall: true, Limit: &provider.ModelLimit{Context: 200000}},
			},
		},
	})

	pid, m, ok := c.FindModel("big-pickle")
	if !ok {
		t.Fatal("expected to find big-pickle")
	}
	if pid != "opencode" {
		t.Errorf("expected 'opencode', got %q", pid)
	}
	if !m.ToolCall {
		t.Error("expected tool_call=true")
	}
	if m.Limit.Context != 200000 {
		t.Errorf("expected 200000 context, got %d", m.Limit.Context)
	}
}

func TestModelSupportTools(t *testing.T) {
	c := provider.NewCatalog()
	c.LoadRaw(map[string]provider.CatalogEntry{
		"test": {Models: map[string]provider.ModelEntry{
			"with-tools":  {ID: "with-tools", ToolCall: true},
			"no-tools":    {ID: "no-tools", ToolCall: false},
		}},
	})

	if !c.ModelSupportsTools("with-tools") {
		t.Error("expected with-tools to support tools")
	}
	if c.ModelSupportsTools("no-tools") {
		t.Error("expected no-tools to NOT support tools")
	}
}

func TestModelContextLimit(t *testing.T) {
	c := provider.NewCatalog()
	c.LoadRaw(map[string]provider.CatalogEntry{
		"test": {Models: map[string]provider.ModelEntry{
			"big-context": {ID: "big-context", Limit: &provider.ModelLimit{Context: 200000}},
			"no-limit":    {ID: "no-limit"},
		}},
	})

	if c.ModelContextLimit("big-context") != 200000 {
		t.Errorf("expected 200000")
	}
	if c.ModelContextLimit("no-limit") != 0 {
		t.Errorf("expected 0")
	}
	if c.ModelContextLimit("nonexistent") != 0 {
		t.Errorf("expected 0 for nonexistent")
	}
}

func TestIsModelFree(t *testing.T) {
	c := provider.NewCatalog()
	c.LoadRaw(map[string]provider.CatalogEntry{
		"test": {Models: map[string]provider.ModelEntry{
			"free-model":  {ID: "free-model", Cost: &provider.ModelCost{Input: 0}},
			"paid-model":  {ID: "paid-model", Cost: &provider.ModelCost{Input: 2.5}},
			"no-cost":     {ID: "no-cost"},
		}},
	})

	if !c.IsModelFree("free-model") {
		t.Error("expected free-model to be free")
	}
	if c.IsModelFree("paid-model") {
		t.Error("expected paid-model to NOT be free")
	}
	if c.IsModelFree("no-cost") {
		t.Error("expected no-cost to NOT be free (nil cost)")
	}
}

func TestModelNotFound(t *testing.T) {
	c := provider.NewCatalog()

	_, _, ok := c.FindModel("nonexistent")
	if ok {
		t.Error("expected not found")
	}

	if c.ModelSupportsTools("nonexistent") {
		t.Error("expected false for nonexistent")
	}
}

func TestListModels(t *testing.T) {
	c := provider.NewCatalog()
	c.LoadRaw(map[string]provider.CatalogEntry{
		"test": {Models: map[string]provider.ModelEntry{
			"a": {ID: "a"},
			"b": {ID: "b"},
		}},
	})

	models := c.ListModels("test")
	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}

	models = c.ListModels("nonexistent")
	if models != nil {
		t.Errorf("expected nil for nonexistent provider")
	}
}
