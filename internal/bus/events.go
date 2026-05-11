package bus

type Event interface {
	Type() string
}

type BaseEvent struct {
	EventType string `json:"type"`
}

func (e BaseEvent) Type() string { return e.EventType }

// Predefined event types
const (
	TypeSessionCreated = "session.created"
	TypeSessionUpdated = "session.updated"
	TypeSessionDeleted = "session.deleted"
	TypeMessageSent    = "message.sent"
	TypeToolCalled     = "tool.called"
	TypeToolCompleted  = "tool.completed"
	TypeToolFailed     = "tool.failed"
	TypeLLMStarted     = "llm.started"
	TypeLLMStream      = "llm.stream"
	TypeLLMCompleted   = "llm.completed"
	TypeLLMError       = "llm.error"
	TypeAgentStarted   = "agent.started"
	TypeAgentCompleted = "agent.completed"
)

type SessionEvent struct {
	BaseEvent
	SessionID string `json:"session_id"`
	Title     string `json:"title,omitempty"`
	Model     string `json:"model,omitempty"`
	Agent     string `json:"agent,omitempty"`
}

func NewSessionCreated(id, title, model, agent string) SessionEvent {
	return SessionEvent{
		BaseEvent: BaseEvent{EventType: TypeSessionCreated},
		SessionID: id, Title: title, Model: model, Agent: agent,
	}
}

func NewSessionUpdated(id, title string) SessionEvent {
	return SessionEvent{
		BaseEvent: BaseEvent{EventType: TypeSessionUpdated},
		SessionID: id, Title: title,
	}
}

type MessageEvent struct {
	BaseEvent
	SessionID string `json:"session_id"`
	Role      string `json:"role"`
	Content   string `json:"content,omitempty"`
}

func NewMessageSent(sessionID, role, content string) MessageEvent {
	return MessageEvent{
		BaseEvent: BaseEvent{EventType: TypeMessageSent},
		SessionID: sessionID, Role: role, Content: content,
	}
}

type ToolEvent struct {
	BaseEvent
	SessionID string `json:"session_id"`
	ToolName  string `json:"tool_name"`
	Input     string `json:"input,omitempty"`
	Output    string `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
	DurationMs int64 `json:"duration_ms,omitempty"`
}

func NewToolCalled(sessionID, toolName, input string) ToolEvent {
	return ToolEvent{
		BaseEvent: BaseEvent{EventType: TypeToolCalled},
		SessionID: sessionID, ToolName: toolName, Input: input,
	}
}

func NewToolCompleted(sessionID, toolName, output string, durationMs int64) ToolEvent {
	return ToolEvent{
		BaseEvent:  BaseEvent{EventType: TypeToolCompleted},
		SessionID:  sessionID, ToolName: toolName,
		Output:     output, DurationMs: durationMs,
	}
}

func NewToolFailed(sessionID, toolName, err string, durationMs int64) ToolEvent {
	return ToolEvent{
		BaseEvent:  BaseEvent{EventType: TypeToolFailed},
		SessionID:  sessionID, ToolName: toolName,
		Error:      err, DurationMs: durationMs,
	}
}

type LLMEvent struct {
	BaseEvent
	Model      string `json:"model"`
	Prompt     string `json:"prompt,omitempty"`
	Response   string `json:"response,omitempty"`
	Error      string `json:"error,omitempty"`
	TokensIn   int    `json:"tokens_in,omitempty"`
	TokensOut  int    `json:"tokens_out,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
}

func NewLLMStarted(model, prompt string) LLMEvent {
	return LLMEvent{
		BaseEvent: BaseEvent{EventType: TypeLLMStarted},
		Model: model, Prompt: prompt,
	}
}

func NewLLMCompleted(model, response string, tokensIn, tokensOut int, durationMs int64) LLMEvent {
	return LLMEvent{
		BaseEvent:  BaseEvent{EventType: TypeLLMCompleted},
		Model: model, Response: response,
		TokensIn: tokensIn, TokensOut: tokensOut, DurationMs: durationMs,
	}
}

func NewLLMError(model, err string) LLMEvent {
	return LLMEvent{
		BaseEvent: BaseEvent{EventType: TypeLLMError},
		Model: model, Error: err,
	}
}

type AgentEvent struct {
	BaseEvent
	AgentName string `json:"agent_name"`
	SessionID string `json:"session_id"`
	Action    string `json:"action,omitempty"`
}

func NewAgentStarted(agentName, sessionID string) AgentEvent {
	return AgentEvent{
		BaseEvent: BaseEvent{EventType: TypeAgentStarted},
		AgentName: agentName, SessionID: sessionID,
	}
}

func NewAgentCompleted(agentName, sessionID string) AgentEvent {
	return AgentEvent{
		BaseEvent: BaseEvent{EventType: TypeAgentCompleted},
		AgentName: agentName, SessionID: sessionID,
	}
}
