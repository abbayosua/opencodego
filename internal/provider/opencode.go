package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/opencode-go/opencode/internal/message"
)

type OpenCodeProvider struct {
	info       ProviderInfo
	listenAddr string
	upstream   Provider
	srv        *http.Server
}

func NewOpenCodeProvider(listenAddr string, upstream Provider) *OpenCodeProvider {
	models := []ModelInfo{
		{
			ID:           "opencode-default",
			ProviderID:   "opencode",
			Name:         "OpenCode-Go Default",
			BaseURL:      fmt.Sprintf("http://%s/v1", listenAddr),
			Capabilities: []string{"tools", "streaming"},
		},
	}

	return &OpenCodeProvider{
		info: ProviderInfo{
			ID:           "opencode",
			Type:         TypeOpenCode,
			Name:         "OpenCode-Go",
			DefaultModel: "opencode-default",
			Models:       models,
		},
		listenAddr: listenAddr,
		upstream:   upstream,
	}
}

func (p *OpenCodeProvider) Info() ProviderInfo { return p.info }

func (p *OpenCodeProvider) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", p.handleChat)
	mux.HandleFunc("/v1/models", p.handleModels)

	p.srv = &http.Server{
		Addr:    p.listenAddr,
		Handler: mux,
	}

	go p.srv.ListenAndServe()
	return nil
}

func (p *OpenCodeProvider) Stop() error {
	if p.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return p.srv.Shutdown(ctx)
	}
	return nil
}

func (p *OpenCodeProvider) Complete(ctx context.Context, req Request) (*Response, error) {
	if p.upstream != nil {
		return p.upstream.Complete(ctx, req)
	}
	return p.localComplete(req)
}

func (p *OpenCodeProvider) Stream(ctx context.Context, req Request) (<-chan StreamEvent, error) {
	if p.upstream != nil {
		return p.upstream.Stream(ctx, req)
	}
	return p.localStream(req)
}

func (p *OpenCodeProvider) localComplete(req Request) (*Response, error) {
	response := &Response{}
	for _, m := range req.Messages {
		for _, c := range m.Content {
			if c.Type == message.ContentToolUse {
				response.Content = "Please configure an upstream LLM provider for full functionality."
				return response, nil
			}
		}
	}
	response.Content = "Hello from OpenCode-Go local mode! Configure OPENCODE_API_KEY to use a real AI model."
	return response, nil
}

func (p *OpenCodeProvider) localStream(req Request) (<-chan StreamEvent, error) {
	events := make(chan StreamEvent, 10)
	go func() {
		defer close(events)
		events <- StreamEvent{
			Type: StreamEventText,
			Text: "OpenCode-Go local mode. Configure an upstream LLM provider.",
		}
		events <- StreamEvent{Type: StreamEventDone}
	}()
	return events, nil
}

func (p *OpenCodeProvider) handleChat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"choices": []map[string]any{
			{
				"message": map[string]any{
					"role":    "assistant",
					"content": "OpenCode-Go server mode. No upstream LLM configured.",
				},
			},
		},
	})
}

func (p *OpenCodeProvider) handleModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": []map[string]any{
			{"id": "opencode-default", "object": "model"},
		},
	})
}
