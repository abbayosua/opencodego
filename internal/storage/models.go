package storage

import "time"

type Session struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Directory string    `json:"directory,omitempty"`
	Title     string    `json:"title,omitempty"`
	Agent     string    `json:"agent,omitempty"`
	Model     string    `json:"model,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Summary   string    `json:"summary,omitempty"`
}

type Message struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type Part struct {
	ID        string    `json:"id"`
	MessageID string    `json:"message_id"`
	SessionID string    `json:"session_id"`
	Type      string    `json:"type"`
	Content   string    `json:"content,omitempty"`
	Metadata  string    `json:"metadata,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateSessionInput struct {
	ID        string
	ProjectID string
	Directory string
	Title     string
	Agent     string
	Model     string
}

type CreateMessageInput struct {
	ID        string
	SessionID string
	Role      string
	Content   string
}

type CreatePartInput struct {
	ID        string
	MessageID string
	SessionID string
	Type      string
	Content   string
	Metadata  string
}

type ListSessionsFilter struct {
	ProjectID string
	Limit     int
	Offset    int
}

type ListMessagesFilter struct {
	SessionID string
	Limit     int
	Offset    int
}
