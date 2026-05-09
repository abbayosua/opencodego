package storage

import "context"

type Store interface {
	// Session operations
	CreateSession(ctx context.Context, input CreateSessionInput) (*Session, error)
	GetSession(ctx context.Context, id string) (*Session, error)
	ListSessions(ctx context.Context, filter ListSessionsFilter) ([]*Session, error)
	UpdateSession(ctx context.Context, session *Session) error
	DeleteSession(ctx context.Context, id string) error

	// Message operations
	CreateMessage(ctx context.Context, input CreateMessageInput) (*Message, error)
	GetMessage(ctx context.Context, id string) (*Message, error)
	ListMessages(ctx context.Context, filter ListMessagesFilter) ([]*Message, error)
	DeleteMessage(ctx context.Context, id string) error

	// Part operations
	CreatePart(ctx context.Context, input CreatePartInput) (*Part, error)
	GetPart(ctx context.Context, id string) (*Part, error)
	ListParts(ctx context.Context, messageID string) ([]*Part, error)
	DeletePart(ctx context.Context, id string) error

	// Session + Messages (full tree)
	GetSessionWithMessages(ctx context.Context, sessionID string) (*Session, []*Message, error)

	// Close
	Close() error
}
