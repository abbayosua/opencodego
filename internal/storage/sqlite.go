package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(1)

	store := &SQLiteStore{db: db}

	// Also set foreign keys pragma explicitly
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return store, nil
}

func NewInMemoryStore() (*SQLiteStore, error) {
	return NewSQLiteStore(":memory:")
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS session (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL DEFAULT '',
		directory TEXT NOT NULL DEFAULT '',
		title TEXT NOT NULL DEFAULT '',
		agent TEXT NOT NULL DEFAULT '',
		model TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		summary TEXT NOT NULL DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_session_project ON session(project_id);
	CREATE INDEX IF NOT EXISTS idx_session_updated ON session(updated_at);

	CREATE TABLE IF NOT EXISTS message (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL REFERENCES session(id) ON DELETE CASCADE,
		role TEXT NOT NULL,
		content TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_message_session ON message(session_id);

	CREATE TABLE IF NOT EXISTS message_part (
		id TEXT PRIMARY KEY,
		message_id TEXT NOT NULL REFERENCES message(id) ON DELETE CASCADE,
		session_id TEXT NOT NULL,
		type TEXT NOT NULL,
		content TEXT NOT NULL DEFAULT '',
		metadata TEXT NOT NULL DEFAULT '{}',
		created_at INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_part_message ON message_part(message_id);
	CREATE INDEX IF NOT EXISTS idx_part_session ON message_part(session_id);

	CREATE TABLE IF NOT EXISTS bot_token (
		id TEXT PRIMARY KEY,
		token TEXT NOT NULL,
		label TEXT NOT NULL DEFAULT '',
		owner_chat_id INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		last_used_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS telegram_session (
		chat_id INTEGER PRIMARY KEY,
		session_id TEXT NOT NULL,
		history TEXT NOT NULL DEFAULT '[]',
		model TEXT NOT NULL DEFAULT 'big-pickle',
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);
	`

	_, err := s.db.Exec(schema)
	return err
}

func now() int64 {
	return time.Now().UnixMilli()
}

func (s *SQLiteStore) CreateSession(ctx context.Context, input CreateSessionInput) (*Session, error) {
	now := now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO session (id, project_id, directory, title, agent, model, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		input.ID, input.ProjectID, input.Directory, input.Title, input.Agent, input.Model, now, now)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return s.GetSession(ctx, input.ID)
}

func (s *SQLiteStore) GetSession(ctx context.Context, id string) (*Session, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, directory, title, agent, model, created_at, updated_at, summary
		 FROM session WHERE id = ?`, id)

	sess := &Session{}
	var createdAt, updatedAt int64
	err := row.Scan(&sess.ID, &sess.ProjectID, &sess.Directory, &sess.Title,
		&sess.Agent, &sess.Model, &createdAt, &updatedAt, &sess.Summary)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	sess.CreatedAt = time.UnixMilli(createdAt)
	sess.UpdatedAt = time.UnixMilli(updatedAt)
	return sess, nil
}

func (s *SQLiteStore) ListSessions(ctx context.Context, filter ListSessionsFilter) ([]*Session, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	query := `SELECT id, project_id, directory, title, agent, model, created_at, updated_at, summary
			   FROM session`

	var args []any
	if filter.ProjectID != "" {
		query += " WHERE project_id = ?"
		args = append(args, filter.ProjectID)
	}

	query += " ORDER BY updated_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, filter.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		sess := &Session{}
		var createdAt, updatedAt int64
		if err := rows.Scan(&sess.ID, &sess.ProjectID, &sess.Directory, &sess.Title,
			&sess.Agent, &sess.Model, &createdAt, &updatedAt, &sess.Summary); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sess.CreatedAt = time.UnixMilli(createdAt)
		sess.UpdatedAt = time.UnixMilli(updatedAt)
		sessions = append(sessions, sess)
	}

	return sessions, nil
}

func (s *SQLiteStore) UpdateSession(ctx context.Context, session *Session) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE session SET project_id=?, directory=?, title=?, agent=?, model=?, updated_at=?, summary=?
		 WHERE id=?`,
		session.ProjectID, session.Directory, session.Title, session.Agent,
		session.Model, now(), session.Summary, session.ID)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DeleteSession(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM session WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *SQLiteStore) CreateMessage(ctx context.Context, input CreateMessageInput) (*Message, error) {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO message (id, session_id, role, content, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		input.ID, input.SessionID, input.Role, input.Content, now())
	if err != nil {
		return nil, fmt.Errorf("create message: %w", err)
	}
	return s.GetMessage(ctx, input.ID)
}

func (s *SQLiteStore) GetMessage(ctx context.Context, id string) (*Message, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, session_id, role, content, created_at FROM message WHERE id=?`, id)

	msg := &Message{}
	var createdAt int64
	err := row.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}
	msg.CreatedAt = time.UnixMilli(createdAt)
	return msg, nil
}

func (s *SQLiteStore) ListMessages(ctx context.Context, filter ListMessagesFilter) ([]*Message, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, role, content, created_at FROM message
		 WHERE session_id=? ORDER BY created_at ASC LIMIT ? OFFSET ?`,
		filter.SessionID, limit, filter.Offset)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		msg := &Message{}
		var createdAt int64
		if err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &createdAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msg.CreatedAt = time.UnixMilli(createdAt)
		messages = append(messages, msg)
	}

	return messages, nil
}

func (s *SQLiteStore) DeleteMessage(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM message WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete message: %w", err)
	}
	return nil
}

func (s *SQLiteStore) CreatePart(ctx context.Context, input CreatePartInput) (*Part, error) {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO message_part (id, message_id, session_id, type, content, metadata, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		input.ID, input.MessageID, input.SessionID, input.Type, input.Content, input.Metadata, now())
	if err != nil {
		return nil, fmt.Errorf("create part: %w", err)
	}
	return s.GetPart(ctx, input.ID)
}

func (s *SQLiteStore) GetPart(ctx context.Context, id string) (*Part, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, message_id, session_id, type, content, metadata, created_at
		 FROM message_part WHERE id=?`, id)

	part := &Part{}
	var createdAt int64
	err := row.Scan(&part.ID, &part.MessageID, &part.SessionID, &part.Type,
		&part.Content, &part.Metadata, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get part: %w", err)
	}
	part.CreatedAt = time.UnixMilli(createdAt)
	return part, nil
}

func (s *SQLiteStore) ListParts(ctx context.Context, messageID string) ([]*Part, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, message_id, session_id, type, content, metadata, created_at
		 FROM message_part WHERE message_id=? ORDER BY created_at ASC`, messageID)
	if err != nil {
		return nil, fmt.Errorf("list parts: %w", err)
	}
	defer rows.Close()

	var parts []*Part
	for rows.Next() {
		part := &Part{}
		var createdAt int64
		if err := rows.Scan(&part.ID, &part.MessageID, &part.SessionID, &part.Type,
			&part.Content, &part.Metadata, &createdAt); err != nil {
			return nil, fmt.Errorf("scan part: %w", err)
		}
		part.CreatedAt = time.UnixMilli(createdAt)
		parts = append(parts, part)
	}

	return parts, nil
}

func (s *SQLiteStore) DeletePart(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM message_part WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete part: %w", err)
	}
	return nil
}

func (s *SQLiteStore) SaveBotToken(ctx context.Context, input CreateBotTokenInput) (*BotToken, error) {
	now := now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO bot_token (id, token, label, owner_chat_id, created_at, last_used_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET token=excluded.token, label=excluded.label, owner_chat_id=excluded.owner_chat_id, last_used_at=excluded.last_used_at`,
		input.ID, input.Token, input.Label, input.OwnerChatID, now, now)
	if err != nil {
		return nil, fmt.Errorf("save bot token: %w", err)
	}
	return s.GetBotToken(ctx, input.ID)
}

func (s *SQLiteStore) GetBotToken(ctx context.Context, id string) (*BotToken, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, token, label, owner_chat_id, created_at, last_used_at FROM bot_token WHERE id=?`, id)
	bt := &BotToken{}
	var createdAt, lastUsedAt int64
	err := row.Scan(&bt.ID, &bt.Token, &bt.Label, &bt.OwnerChatID, &createdAt, &lastUsedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get bot token: %w", err)
	}
	bt.CreatedAt = time.UnixMilli(createdAt)
	bt.LastUsedAt = time.UnixMilli(lastUsedAt)
	return bt, nil
}

func (s *SQLiteStore) ListBotTokens(ctx context.Context) ([]*BotToken, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, token, label, owner_chat_id, created_at, last_used_at FROM bot_token ORDER BY last_used_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list bot tokens: %w", err)
	}
	defer rows.Close()
	var tokens []*BotToken
	for rows.Next() {
		bt := &BotToken{}
		var createdAt, lastUsedAt int64
		if err := rows.Scan(&bt.ID, &bt.Token, &bt.Label, &bt.OwnerChatID, &createdAt, &lastUsedAt); err != nil {
			return nil, fmt.Errorf("scan bot token: %w", err)
		}
		bt.CreatedAt = time.UnixMilli(createdAt)
		bt.LastUsedAt = time.UnixMilli(lastUsedAt)
		tokens = append(tokens, bt)
	}
	return tokens, nil
}

func (s *SQLiteStore) DeleteBotToken(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM bot_token WHERE id=?`, id)
	return err
}

func (s *SQLiteStore) UpdateBotTokenLastUsed(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE bot_token SET last_used_at=? WHERE id=?`, now(), id)
	return err
}

func (s *SQLiteStore) SaveTelegramSession(ctx context.Context, input CreateTelegramSessionInput) (*TelegramSession, error) {
	now := now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO telegram_session (chat_id, session_id, history, model, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(chat_id) DO UPDATE SET session_id=excluded.session_id, updated_at=excluded.updated_at`,
		input.ChatID, input.SessionID, "[]", input.Model, now, now)
	if err != nil {
		return nil, fmt.Errorf("save telegram session: %w", err)
	}
	return s.GetTelegramSession(ctx, input.ChatID)
}

func (s *SQLiteStore) GetTelegramSession(ctx context.Context, chatID int64) (*TelegramSession, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT chat_id, session_id, history, model, created_at, updated_at FROM telegram_session WHERE chat_id=?`, chatID)
	ts := &TelegramSession{}
	var createdAt, updatedAt int64
	err := row.Scan(&ts.ChatID, &ts.SessionID, &ts.History, &ts.Model, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get telegram session: %w", err)
	}
	ts.CreatedAt = time.UnixMilli(createdAt)
	ts.UpdatedAt = time.UnixMilli(updatedAt)
	return ts, nil
}

func (s *SQLiteStore) UpdateTelegramSession(ctx context.Context, chatID int64, history string, model string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE telegram_session SET history=?, model=?, updated_at=? WHERE chat_id=?`,
		history, model, now(), chatID)
	return err
}

func (s *SQLiteStore) DeleteTelegramSession(ctx context.Context, chatID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM telegram_session WHERE chat_id=?`, chatID)
	return err
}

func (s *SQLiteStore) GetSessionWithMessages(ctx context.Context, sessionID string) (*Session, []*Message, error) {
	sess, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	if sess == nil {
		return nil, nil, nil
	}

	msgs, err := s.ListMessages(ctx, ListMessagesFilter{SessionID: sessionID, Limit: 1000})
	if err != nil {
		return nil, nil, err
	}

	return sess, msgs, nil
}
