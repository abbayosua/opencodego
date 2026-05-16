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

	// Bot Token operations
	SaveBotToken(ctx context.Context, input CreateBotTokenInput) (*BotToken, error)
	ListBotTokens(ctx context.Context) ([]*BotToken, error)
	GetBotToken(ctx context.Context, id string) (*BotToken, error)
	DeleteBotToken(ctx context.Context, id string) error
	UpdateBotTokenLastUsed(ctx context.Context, id string) error

	// Telegram Session operations
	SaveTelegramSession(ctx context.Context, input CreateTelegramSessionInput) (*TelegramSession, error)
	GetTelegramSession(ctx context.Context, chatID int64) (*TelegramSession, error)
	UpdateTelegramSession(ctx context.Context, chatID int64, history string, model string) error
	DeleteTelegramSession(ctx context.Context, chatID int64) error

	// Close
	Close() error
}
