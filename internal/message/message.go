package message

import "encoding/json"

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

type ContentType string

const (
	ContentText      ContentType = "text"
	ContentToolUse   ContentType = "tool_use"
	ContentToolResult ContentType = "tool_result"
	ContentReasoning ContentType = "reasoning"
)

type Content struct {
	Type ContentType `json:"type"`
	Text string      `json:"text,omitempty"`

	ToolUseID   string `json:"tool_use_id,omitempty"`
	ToolName    string `json:"tool_name,omitempty"`
	ToolInput   any    `json:"tool_input,omitempty"`
	ToolOutput  string `json:"tool_output,omitempty"`
	IsError     bool   `json:"is_error,omitempty"`

	Reasoning string `json:"reasoning,omitempty"`
}

type Message struct {
	Role    Role     `json:"role"`
	Content []Content `json:"content"`
}

func NewTextMessage(role Role, text string) Message {
	return Message{
		Role: role,
		Content: []Content{
			{Type: ContentText, Text: text},
		},
	}
}

func NewToolResultMessage(toolUseID, toolName string, output any, isError bool) Message {
	var outStr string
	switch v := output.(type) {
	case string:
		outStr = v
	default:
		b, _ := json.Marshal(v)
		outStr = string(b)
	}

	return Message{
		Role: RoleTool,
		Content: []Content{
			{
				Type:      ContentToolResult,
				ToolUseID: toolUseID,
				ToolName:  toolName,
				ToolOutput: outStr,
				IsError:   isError,
			},
		},
	}
}

type StreamEvent struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Input   any    `json:"input,omitempty"`
	Output  string `json:"output,omitempty"`
	IsError bool   `json:"is_error,omitempty"`
}
