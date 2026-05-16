package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/opencode-go/opencode/internal/message"
)

type EventType string

const (
	EventTextStart      EventType = "text-start"
	EventTextDelta      EventType = "text-delta"
	EventTextEnd        EventType = "text-end"
	EventReasoningStart EventType = "reasoning-start"
	EventReasoningDelta EventType = "reasoning-delta"
	EventReasoningEnd   EventType = "reasoning-end"
	EventToolCall       EventType = "tool-call"
	EventToolResult     EventType = "tool-result"
	EventToolError      EventType = "tool-error"
	EventStart          EventType = "start"
	EventFinish         EventType = "finish"
	EventError          EventType = "error"
	EventStartStep      EventType = "start-step"
	EventFinishStep     EventType = "finish-step"
	EventToolInputStart EventType = "tool-input-start"
)

type Event struct {
	Type             EventType       `json:"type"`
	Text             string          `json:"text,omitempty"`
	ID               string          `json:"id,omitempty"`
	Name             string          `json:"name,omitempty"`
	Input            json.RawMessage `json:"input,omitempty"`
	ToolCallID       string          `json:"toolCallId,omitempty"`
	Output           json.RawMessage `json:"output,omitempty"`
	Error            string          `json:"error,omitempty"`
	FinishReason     string          `json:"finishReason,omitempty"`
	ProviderMetadata map[string]any  `json:"providerMetadata,omitempty"`
	TokensIn         int             `json:"tokensIn,omitempty"`
	TokensOut        int             `json:"tokensOut,omitempty"`
}

type ToolDef struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type StreamRequest struct {
	Model    string           `json:"model"`
	System   string           `json:"system,omitempty"`
	Messages []message.Message `json:"messages"`
	Tools    []ToolDef        `json:"tools,omitempty"`
	Stream   bool             `json:"stream"`
	MaxTokens int             `json:"max_tokens,omitempty"`
}

type openAIChatMessage struct {
	Role             string             `json:"role"`
	Content          []openAIChatPart   `json:"content,omitempty"`
	ToolCalls        []openAIToolCall   `json:"tool_calls,omitempty"`
	ToolCallID       string             `json:"tool_call_id,omitempty"`
	ReasoningContent *string            `json:"reasoning_content,omitempty"`
}

type openAIChatPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type openAIToolCall struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Function openAIFunction    `json:"function"`
}

type openAIFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Client struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func (c *Client) setRequestHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	// Zen API requires these headers
	if strings.Contains(c.baseURL, "opencode.ai") {
		req.Header.Set("HTTP-Referer", "https://opencode.ai/")
		req.Header.Set("X-Title", "opencode")
	}
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{},
	}
}

func (c *Client) SetAPIKey(key string) {
	c.apiKey = key
}

func (c *Client) Stream(ctx context.Context, req StreamRequest) (<-chan Event, <-chan error) {
	events := make(chan Event)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)

		body := buildOpenAIBody(req)
		bodyJSON, err := json.Marshal(body)
		if err != nil {
			errs <- fmt.Errorf("marshal request: %w", err)
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(bodyJSON))
		if err != nil {
			errs <- fmt.Errorf("create request: %w", err)
			return
		}
		c.setRequestHeaders(httpReq)

		resp, err := c.client.Do(httpReq)
		if err != nil {
			errs <- fmt.Errorf("http request: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			respBody, _ := io.ReadAll(resp.Body)
			errs <- fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
			return
		}

		events <- Event{Type: EventStart}
		events <- Event{Type: EventStartStep}

		scanner := bufio.NewScanner(resp.Body)
		var pendingToolCalls []openAIToolCall
		textContent := ""

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Role             string           `json:"role"`
						Content          string           `json:"content"`
						ReasoningContent string           `json:"reasoning_content"`
						ToolCalls        []openAIToolCall `json:"tool_calls"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				} `json:"choices"`
				Usage *struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
					TotalTokens      int `json:"total_tokens"`
				} `json:"usage"`
			}

			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			delta := chunk.Choices[0].Delta

			if delta.ReasoningContent != "" {
				events <- Event{
					Type: EventReasoningStart,
					ID:   "reasoning_1",
					Text: delta.ReasoningContent,
				}
				events <- Event{
					Type: EventReasoningDelta,
					ID:   "reasoning_1",
					Text: delta.ReasoningContent,
				}
				events <- Event{
					Type: EventReasoningEnd,
					ID:   "reasoning_1",
				}
			}

			if delta.Content != "" {
				if textContent == "" {
					events <- Event{Type: EventTextStart}
				}
				textContent += delta.Content
				events <- Event{
					Type: EventTextDelta,
					Text: delta.Content,
				}
			}

			for _, tc := range delta.ToolCalls {
				if tc.Function.Name != "" {
					pendingToolCalls = append(pendingToolCalls, tc)
				} else if len(pendingToolCalls) > 0 {
					last := &pendingToolCalls[len(pendingToolCalls)-1]
					last.Function.Arguments += tc.Function.Arguments
				}
			}

			if chunk.Choices[0].FinishReason != nil {
				reason := *chunk.Choices[0].FinishReason

				if textContent != "" {
					events <- Event{Type: EventTextEnd}
				}

				for _, tc := range pendingToolCalls {
					events <- Event{
						Type: EventToolInputStart,
						ID:   tc.ID,
						Name: tc.Function.Name,
					}
					events <- Event{
						Type:       EventToolCall,
						ID:         tc.ID,
						ToolCallID: tc.ID,
						Name:       tc.Function.Name,
						Input:      json.RawMessage(tc.Function.Arguments),
					}
				}

			tokensIn := 0
			tokensOut := 0
			if chunk.Usage != nil {
				tokensIn = chunk.Usage.PromptTokens
				tokensOut = chunk.Usage.CompletionTokens
			}

			events <- Event{
				Type:         EventFinishStep,
				FinishReason: reason,
				TokensIn:     tokensIn,
				TokensOut:    tokensOut,
			}
			events <- Event{
				Type:         EventFinish,
				FinishReason: reason,
				TokensIn:     tokensIn,
				TokensOut:    tokensOut,
			}
			}
		}

		if err := scanner.Err(); err != nil {
			errs <- fmt.Errorf("read stream: %w", err)
		}
	}()

	return events, errs
}

func buildOpenAIBody(req StreamRequest) map[string]any {
	var msgs []openAIChatMessage

	if req.System != "" {
		msgs = append(msgs, openAIChatMessage{
			Role: "system",
			Content: []openAIChatPart{
				{Type: "text", Text: req.System},
			},
		})
	}

	for _, m := range req.Messages {
		switch m.Role {
		case message.RoleUser:
			content := []openAIChatPart{}
			for _, c := range m.Content {
				if c.Text != "" {
					content = append(content, openAIChatPart{Type: "text", Text: c.Text})
				}
			}
			msgs = append(msgs, openAIChatMessage{Role: "user", Content: content})

		case message.RoleAssistant:
			var toolCalls []openAIToolCall
			var reasoningParts []string
			for _, c := range m.Content {
				if c.Type == message.ContentToolUse {
					args, _ := json.Marshal(c.ToolInput)
					toolCalls = append(toolCalls, openAIToolCall{
						ID:   c.ToolUseID,
						Type: "function",
						Function: openAIFunction{
							Name:      c.ToolName,
							Arguments: string(args),
						},
					})
				}
				if c.Reasoning != "" {
					reasoningParts = append(reasoningParts, c.Reasoning)
				}
			}
			msg := openAIChatMessage{Role: "assistant"}
			if len(toolCalls) > 0 {
				msg.ToolCalls = toolCalls
			}
			textContent := []openAIChatPart{}
			for _, c := range m.Content {
				if c.Type == message.ContentText && c.Text != "" {
					textContent = append(textContent, openAIChatPart{Type: "text", Text: c.Text})
				}
			}
			if len(textContent) > 0 {
				msg.Content = textContent
			}
			if len(reasoningParts) > 0 {
				reasoningText := strings.Join(reasoningParts, "")
				msg.ReasoningContent = &reasoningText
			}
			msgs = append(msgs, msg)

		case message.RoleTool:
			for _, c := range m.Content {
				msgs = append(msgs, openAIChatMessage{
					Role:       "tool",
					ToolCallID: c.ToolUseID,
					Content:    []openAIChatPart{{Type: "text", Text: c.ToolOutput}},
				})
			}
		}
	}

	// DeepSeek requires reasoning_content on ALL assistant messages, even if empty
	modelLower := strings.ToLower(req.Model)
	if strings.Contains(modelLower, "deepseek") {
		for i := range msgs {
			if msgs[i].Role == "assistant" && msgs[i].ReasoningContent == nil {
				empty := ""
				msgs[i].ReasoningContent = &empty
			}
		}
	}

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

	body := map[string]any{
		"model":      req.Model,
		"stream":     true,
		"messages":   msgs,
	}

	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}

	if len(tools) > 0 {
		body["tools"] = tools
	}

	return body
}
