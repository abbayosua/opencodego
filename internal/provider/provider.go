package provider

import (
	"context"
	"encoding/json"
	"time"

	"github.com/opencode-go/opencode/internal/message"
)

type ProviderType string

const (
	TypeOpenAI    ProviderType = "openai"
	TypeAnthropic ProviderType = "anthropic"
	TypeCustom    ProviderType = "custom"
	TypeOpenCode  ProviderType = "opencode"
)

type ModelInfo struct {
	ID           string   `yaml:"id" json:"id"`
	ProviderID   string   `yaml:"provider_id" json:"provider_id"`
	Name         string   `yaml:"name" json:"name"`
	BaseURL      string   `yaml:"base_url" json:"base_url"`
	Capabilities []string `yaml:"capabilities" json:"capabilities"`
}

type ProviderInfo struct {
	ID           string      `yaml:"id" json:"id"`
	Type         ProviderType `yaml:"type" json:"type"`
	Name         string      `yaml:"name" json:"name"`
	DefaultModel string      `yaml:"default_model" json:"default_model"`
	Models       []ModelInfo `yaml:"models" json:"models"`
}

type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type Request struct {
	Model     string
	Messages  []message.Message
	Tools     []ToolDef
	System    string
	MaxTokens int
	Stream    bool
}

type ToolCall struct {
	ID     string
	Name   string
	Input  json.RawMessage
}

type Response struct {
	Content   string
	ToolCalls []ToolCall
	Usage     Usage
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type StreamEventType string

const (
	StreamEventText      StreamEventType = "text"
	StreamEventToolCall  StreamEventType = "tool_call"
	StreamEventToolResult StreamEventType = "tool_result"
	StreamEventError     StreamEventType = "error"
	StreamEventDone      StreamEventType = "done"
)

type StreamEvent struct {
	Type     StreamEventType
	Text     string
	ToolCall *ToolCall
	Error    string
	Usage    *Usage
}

type Provider interface {
	Info() ProviderInfo
	Complete(ctx context.Context, req Request) (*Response, error)
	Stream(ctx context.Context, req Request) (<-chan StreamEvent, error)
}

type Config struct {
	APIKey      string        `yaml:"api_key"`
	BaseURL     string        `yaml:"base_url"`
	DefaultModel string       `yaml:"default_model"`
	Models      []string      `yaml:"models"`
	Timeout     time.Duration `yaml:"timeout"`
	Type        string        `yaml:"type"`
}
