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

type OpenAIProvider struct {
	info    ProviderInfo
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewOpenAIProvider(apiKey, baseURL string, models []string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	defaultModel := "gpt-4o"
	if len(models) > 0 {
		defaultModel = models[0]
	}

	modelList := models
	if len(modelList) == 0 {
		modelList = []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo"}
	}

	var modelInfos []ModelInfo
	for _, m := range modelList {
		modelInfos = append(modelInfos, ModelInfo{
			ID:         m,
			ProviderID: "openai",
			Name:       m,
			BaseURL:    baseURL,
			Capabilities: []string{"tools", "streaming", "vision"},
		})
	}

	return &OpenAIProvider{
		info: ProviderInfo{
			ID:           "openai",
			Type:         TypeOpenAI,
			Name:         "OpenAI",
			DefaultModel: defaultModel,
			Models:       modelInfos,
		},
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 5 * time.Minute},
	}
}

func (p *OpenAIProvider) Info() ProviderInfo { return p.info }

func (p *OpenAIProvider) Complete(ctx context.Context, req Request) (*Response, error) {
	body := p.buildBody(req)
	bodyJSON, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

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
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID   string `json:"id"`
					Type string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	response := &Response{}
	if len(result.Choices) > 0 {
		msg := result.Choices[0].Message
		response.Content = msg.Content
		for _, tc := range msg.ToolCalls {
			response.ToolCalls = append(response.ToolCalls, ToolCall{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: json.RawMessage(tc.Function.Arguments),
			})
		}
	}
	if result.Usage != nil {
		response.Usage = Usage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		}
	}

	return response, nil
}

func (p *OpenAIProvider) Stream(ctx context.Context, req Request) (<-chan StreamEvent, error) {
	events := make(chan StreamEvent)

	body := p.buildBody(req)
	body["stream"] = true
	bodyJSON, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	go func() {
		defer resp.Body.Close()
		defer close(events)

		scanner := bufio.NewScanner(resp.Body)
		var pendingToolCalls []struct {
			id   string
			name string
			args string
		}

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				events <- StreamEvent{Type: StreamEventDone}
				return
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content   string `json:"content"`
						ToolCalls []struct {
							ID   string `json:"id"`
							Type string `json:"type"`
							Function struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							} `json:"function"`
						} `json:"tool_calls"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if len(chunk.Choices) == 0 {
				continue
			}

			delta := chunk.Choices[0].Delta

			if delta.Content != "" {
				events <- StreamEvent{Type: StreamEventText, Text: delta.Content}
			}

			for _, tc := range delta.ToolCalls {
				if tc.Function.Name != "" {
					pendingToolCalls = append(pendingToolCalls, struct {
						id   string
						name string
						args string
					}{tc.ID, tc.Function.Name, tc.Function.Arguments})
				} else if len(pendingToolCalls) > 0 {
					last := &pendingToolCalls[len(pendingToolCalls)-1]
					last.args += tc.Function.Arguments
				}
			}

			if chunk.Choices[0].FinishReason != nil {
				for _, tc := range pendingToolCalls {
					events <- StreamEvent{
						Type: StreamEventToolCall,
						ToolCall: &ToolCall{
							ID:    tc.id,
							Name:  tc.name,
							Input: json.RawMessage(tc.args),
						},
					}
				}
				events <- StreamEvent{Type: StreamEventDone}
				return
			}
		}

		if err := scanner.Err(); err != nil {
			events <- StreamEvent{Type: StreamEventError, Error: err.Error()}
		}
	}()

	return events, nil
}

func (p *OpenAIProvider) buildBody(req Request) map[string]any {
	body := map[string]any{
		"model":    req.Model,
		"messages": p.buildMessages(req),
	}

	if len(req.Tools) > 0 {
		var tools []map[string]any
		for _, t := range req.Tools {
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  t.Parameters,
				},
			})
		}
		body["tools"] = tools
	}

	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}

	return body
}

func (p *OpenAIProvider) buildMessages(req Request) []map[string]any {
	var msgs []map[string]any

	if req.System != "" {
		msgs = append(msgs, map[string]any{
			"role": "system",
			"content": []map[string]any{
				{"type": "text", "text": req.System},
			},
		})
	}

	for _, m := range req.Messages {
		switch m.Role {
		case message.RoleUser:
			var content []map[string]any
			for _, c := range m.Content {
				if c.Text != "" {
					content = append(content, map[string]any{"type": "text", "text": c.Text})
				}
			}
			msgs = append(msgs, map[string]any{"role": "user", "content": content})

		case message.RoleAssistant:
			msg := map[string]any{"role": "assistant"}
			var toolCalls []map[string]any
			var textParts []map[string]any

			for _, c := range m.Content {
				if c.Type == message.ContentToolUse {
					args, _ := json.Marshal(c.ToolInput)
					toolCalls = append(toolCalls, map[string]any{
						"id":   c.ToolUseID,
						"type": "function",
						"function": map[string]any{
							"name":      c.ToolName,
							"arguments": string(args),
						},
					})
				} else if c.Text != "" {
					textParts = append(textParts, map[string]any{"type": "text", "text": c.Text})
				}
			}

			if len(textParts) > 0 {
				msg["content"] = textParts
			}
			if len(toolCalls) > 0 {
				msg["tool_calls"] = toolCalls
			}
			msgs = append(msgs, msg)

		case message.RoleTool:
			for _, c := range m.Content {
				msgs = append(msgs, map[string]any{
					"role":       "tool",
					"tool_call_id": c.ToolUseID,
					"content":    c.ToolOutput,
				})
			}
		}
	}

	return msgs
}
