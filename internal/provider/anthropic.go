package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/opencode-go/opencode/internal/message"
)

type AnthropicProvider struct {
	info    ProviderInfo
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewAnthropicProvider(apiKey, baseURL string, models []string) *AnthropicProvider {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	defaultModel := "claude-sonnet-4-20250514"
	if len(models) > 0 {
		defaultModel = models[0]
	}

	modelList := models
	if len(modelList) == 0 {
		modelList = []string{
			"claude-sonnet-4-20250514",
			"claude-3-5-sonnet-20241022",
			"claude-3-5-haiku-20241022",
			"claude-3-opus-20240229",
		}
	}

	var modelInfos []ModelInfo
	for _, m := range modelList {
		modelInfos = append(modelInfos, ModelInfo{
			ID:         m,
			ProviderID: "anthropic",
			Name:       m,
			BaseURL:    baseURL,
			Capabilities: []string{"tools", "streaming", "reasoning"},
		})
	}

	return &AnthropicProvider{
		info: ProviderInfo{
			ID:           "anthropic",
			Type:         TypeAnthropic,
			Name:         "Anthropic",
			DefaultModel: defaultModel,
			Models:       modelInfos,
		},
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 5 * time.Minute},
	}
}

func (p *AnthropicProvider) Info() ProviderInfo { return p.info }

func (p *AnthropicProvider) Complete(ctx context.Context, req Request) (*Response, error) {
	body := p.buildBody(req)
	body["stream"] = false
	bodyJSON, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	response := &Response{}
	for _, c := range result.Content {
		if c.Type == "text" {
			response.Content += c.Text
		}
	}

	response.Usage = Usage{
		PromptTokens:     result.Usage.InputTokens,
		CompletionTokens: result.Usage.OutputTokens,
		TotalTokens:      result.Usage.InputTokens + result.Usage.OutputTokens,
	}

	return response, nil
}

func (p *AnthropicProvider) Stream(ctx context.Context, req Request) (<-chan StreamEvent, error) {
	events := make(chan StreamEvent)

	body := p.buildBody(req)
	body["stream"] = true
	bodyJSON, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	go func() {
		defer resp.Body.Close()
		defer close(events)

		scanner := bufio.NewScanner(resp.Body)
		var currentText string

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				if strings.HasPrefix(line, "event: ") || line == "" {
					continue
				}
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			var event struct {
				Type string `json:"type"`
				Delta *struct {
					Text string `json:"text"`
				} `json:"delta"`
				ContentBlock *struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content_block"`
				StopReason *string `json:"stop_reason"`
				Usage      *struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}

			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			switch event.Type {
			case "content_block_delta":
				if event.Delta != nil && event.Delta.Text != "" {
					currentText += event.Delta.Text
					events <- StreamEvent{Type: StreamEventText, Text: event.Delta.Text}
				}

			case "message_stop":
				events <- StreamEvent{
					Type: StreamEventDone,
					Usage: &Usage{
						PromptTokens:     p.safeUsage(event.Usage).InputTokens,
						CompletionTokens: p.safeUsage(event.Usage).OutputTokens,
						TotalTokens:      p.safeUsage(event.Usage).InputTokens + p.safeUsage(event.Usage).OutputTokens,
					},
				}
				return
			}
		}

		if err := scanner.Err(); err != nil {
			events <- StreamEvent{Type: StreamEventError, Error: err.Error()}
		}

		events <- StreamEvent{Type: StreamEventDone}
	}()

	return events, nil
}

func (p *AnthropicProvider) safeUsage(u *struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}) struct {
	InputTokens  int
	OutputTokens int
} {
	if u == nil {
		return struct {
			InputTokens  int
			OutputTokens int
		}{0, 0}
	}
	return struct {
		InputTokens  int
		OutputTokens int
	}{u.InputTokens, u.OutputTokens}
}

func (p *AnthropicProvider) buildBody(req Request) map[string]any {
	body := map[string]any{
		"model":      req.Model,
		"max_tokens": 4096,
		"messages":   p.buildMessages(req),
	}

	if req.System != "" {
		body["system"] = req.System
	}

	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}

	return body
}

func (p *AnthropicProvider) buildMessages(req Request) []map[string]any {
	var msgs []map[string]any

	for _, m := range req.Messages {
		switch m.Role {
		case message.RoleUser:
			var content []map[string]any
			for _, c := range m.Content {
				if c.Text != "" {
					content = append(content, map[string]any{
						"type": "text",
						"text": c.Text,
					})
				}
			}
			msgs = append(msgs, map[string]any{"role": "user", "content": content})

		case message.RoleAssistant:
			var content []map[string]any
			for _, c := range m.Content {
				if c.Type == message.ContentToolUse {
					content = append(content, map[string]any{
						"type":  "tool_use",
						"id":    c.ToolUseID,
						"name":  c.ToolName,
						"input": c.ToolInput,
					})
				} else if c.Text != "" {
					content = append(content, map[string]any{
						"type": "text",
						"text": c.Text,
					})
				}
			}
			msgs = append(msgs, map[string]any{"role": "assistant", "content": content})

		case message.RoleTool:
			for _, c := range m.Content {
				msgs = append(msgs, map[string]any{
					"role":           "user",
					"content": []map[string]any{
						{
							"type":        "tool_result",
							"tool_use_id": c.ToolUseID,
							"content":     c.ToolOutput,
						},
					},
				})
			}
		}
	}

	return msgs
}
