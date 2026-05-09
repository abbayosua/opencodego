package provider_test

import (
	"context"
	"testing"
	"time"

	"github.com/opencode-go/opencode/internal/llmtest"
	"github.com/opencode-go/opencode/internal/message"
	"github.com/opencode-go/opencode/internal/provider"
)

func TestOpenAIProviderInfo(t *testing.T) {
	p := provider.NewOpenAIProvider("test-key", "", nil)

	info := p.Info()
	if info.ID != "openai" {
		t.Errorf("expected ID 'openai', got %q", info.ID)
	}
	if info.DefaultModel == "" {
		t.Error("expected non-empty DefaultModel")
	}
	if len(info.Models) == 0 {
		t.Error("expected at least one model")
	}
}

func TestOpenAIProviderComplete(t *testing.T) {
	srv := llmtest.NewForTest(t)
	srv.Text("Hello from test!")

	p := provider.NewOpenAIProvider("test-key", srv.URL(), []string{"test-model"})

	resp, err := p.Complete(context.Background(), provider.Request{
		Model: "test-model",
		Messages: []message.Message{
			message.NewTextMessage(message.RoleUser, "say hello"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content == "" {
		t.Error("expected non-empty response")
	}
}

func TestOpenAIProviderStream(t *testing.T) {
	srv := llmtest.NewForTest(t)
	srv.Text("Hello streaming!")

	p := provider.NewOpenAIProvider("test-key", srv.URL(), []string{"test-model"})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, err := p.Stream(ctx, provider.Request{
		Model: "test-model",
		Messages: []message.Message{
			message.NewTextMessage(message.RoleUser, "stream test"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var texts []string
	for evt := range events {
		if evt.Type == provider.StreamEventText {
			texts = append(texts, evt.Text)
		}
	}

	if len(texts) == 0 {
		t.Error("expected at least one text event")
	}
}

func TestAnthropicProviderInfo(t *testing.T) {
	p := provider.NewAnthropicProvider("test-key", "", nil)

	info := p.Info()
	if info.ID != "anthropic" {
		t.Errorf("expected ID 'anthropic', got %q", info.ID)
	}
}

func TestRegistry(t *testing.T) {
	reg := provider.NewRegistry()

	p1 := provider.NewOpenAIProvider("key1", "", nil)
	p2 := provider.NewAnthropicProvider("key2", "", nil)

	reg.Register(p1)
	reg.Register(p2)

	infoList := reg.List()
	if len(infoList) != 2 {
		t.Errorf("expected 2 providers, got %d", len(infoList))
	}

	got, err := reg.Get("openai")
	if err != nil {
		t.Fatal(err)
	}
	if got.Info().ID != "openai" {
		t.Errorf("expected 'openai', got %q", got.Info().ID)
	}
}

func TestCustomOpenAIProvider(t *testing.T) {
	srv := llmtest.NewForTest(t)
	srv.Text("Custom provider says hi!")

	p, err := provider.NewCustomProvider("my-custom", "My Inference", provider.Config{
		APIKey:      "test-key",
		BaseURL:     srv.URL(),
		DefaultModel: "my-model",
		Models:      []string{"my-model"},
		Type:        "openai-compatible",
	})
	if err != nil {
		t.Fatal(err)
	}

	info := p.Info()
	if info.ID != "my-custom" {
		t.Errorf("expected 'my-custom', got %q", info.ID)
	}

	resp, err := p.Complete(context.Background(), provider.Request{
		Model: "my-model",
		Messages: []message.Message{
			message.NewTextMessage(message.RoleUser, "test"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content == "" {
		t.Error("expected non-empty response")
	}
}

func TestCustomAnthropicProvider(t *testing.T) {
	p, err := provider.NewCustomProvider("my-anthropic", "My Anthropic", provider.Config{
		APIKey:      "test-key",
		BaseURL:     "https://api.anthropic.com/v1",
		DefaultModel: "claude-sonnet-4-20250514",
		Models:       []string{"claude-sonnet-4-20250514"},
		Type:         "anthropic-compatible",
	})
	if err != nil {
		t.Fatal(err)
	}

	info := p.Info()
	if info.Type != provider.TypeCustom {
		t.Errorf("expected TypeCustom, got %v", info.Type)
	}

	// Just test info, actual API call would need real key
	if info.DefaultModel != "claude-sonnet-4-20250514" {
		t.Errorf("expected claude-sonnet-4-20250514, got %s", info.DefaultModel)
	}
}

func TestOpenCodeProvider(t *testing.T) {
	upstream := provider.NewOpenAIProvider("test-key", "", nil)
	p := provider.NewOpenCodeProvider("127.0.0.1:0", upstream)

	info := p.Info()
	if info.ID != "opencode" {
		t.Errorf("expected 'opencode', got %q", info.ID)
	}

	// Test local mode (no upstream)
	pLocal := provider.NewOpenCodeProvider("127.0.0.1:0", nil)
	resp, err := pLocal.Complete(context.Background(), provider.Request{
		Model: "opencode-default",
		Messages: []message.Message{
			message.NewTextMessage(message.RoleUser, "hi"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content == "" {
		t.Error("expected non-empty response")
	}
}

func TestProviderMessageConversion(t *testing.T) {
	t.Run("user message", func(t *testing.T) {
		p := provider.NewOpenAIProvider("key", "http://localhost:9999", nil)
		req := provider.Request{
			Model: "test",
			Messages: []message.Message{
				message.NewTextMessage(message.RoleUser, "hello"),
			},
		}
		_, err := p.Complete(context.Background(), req)
		if err == nil {
			t.Log("expected connection error, not message format error")
		}
	})

	t.Run("tool result message", func(t *testing.T) {
		msg := message.NewToolResultMessage("call_1", "bash", "output ok", false)
		if msg.Role != message.RoleTool {
			t.Errorf("expected RoleTool, got %v", msg.Role)
		}
		if len(msg.Content) != 1 {
			t.Errorf("expected 1 content, got %d", len(msg.Content))
		}
	})
}

func TestProviderCapabilities(t *testing.T) {
	openAI := provider.NewOpenAIProvider("key", "", nil)
	anthro := provider.NewAnthropicProvider("key", "", nil)

	checkCaps := func(p provider.Provider, expectTools, expectStream bool) {
		for _, m := range p.Info().Models {
			hasTools := false
			hasStream := false
			for _, cap := range m.Capabilities {
				if cap == "tools" {
					hasTools = true
				}
				if cap == "streaming" {
					hasStream = true
				}
			}
			if hasTools != expectTools {
				t.Errorf("%s model %s: expected tools=%v, got %v", p.Info().ID, m.ID, expectTools, hasTools)
			}
			if hasStream != expectStream {
				t.Errorf("%s model %s: expected stream=%v, got %v", p.Info().ID, m.ID, expectStream, hasStream)
			}
		}
	}

	checkCaps(openAI, true, true)
	checkCaps(anthro, true, true)
}
