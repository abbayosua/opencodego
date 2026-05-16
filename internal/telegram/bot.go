package telegram

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/opencode-go/opencode/internal/llm"
	"github.com/opencode-go/opencode/internal/message"
	"github.com/opencode-go/opencode/internal/session"
	"github.com/opencode-go/opencode/internal/storage"
	"github.com/opencode-go/opencode/internal/tool"
)

type Bot struct {
	token    string
	apiURL   string
	apiKey   string
	model    string
	reg      *tool.Registry
	client   *llm.Client
	bot      *bot.Bot

	mu       sync.Mutex
	ownerID  int64
	sessions map[int64]*ChatSession
	store    storage.Store
	running  bool
	cancel   context.CancelFunc
}

type ChatSession struct {
	ChatID    int64
	SessionID string
	History   []message.Message
	Model     string
}

func New(token string, reg *tool.Registry, apiURL, apiKey, model string, store storage.Store) *Bot {
	b := &Bot{
		token:    token,
		apiURL:   apiURL,
		apiKey:   apiKey,
		model:    model,
		reg:      reg,
		store:    store,
		sessions: make(map[int64]*ChatSession),
	}
	if apiKey == "" {
		b.apiKey = "public"
	}
	b.client = llm.NewClient(apiURL)
	b.client.SetAPIKey(b.apiKey)
	return b
}

func (b *Bot) Run(ctx context.Context, ownerID int64) error {
	b.mu.Lock()
	b.ownerID = ownerID
	b.mu.Unlock()

	var err error
	b.bot, err = bot.New(b.token, bot.WithDefaultHandler(func(ctx context.Context, bt *bot.Bot, u *models.Update) {
		b.handleUpdate(ctx, u)
	}))
	if err != nil {
		return fmt.Errorf("create bot: %w", err)
	}

	me, err := b.bot.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}
	log.Printf("Telegram bot @%s started (owner chat ID: %d)", me.Username, ownerID)

	b.loadSessions(ctx)

	ctx, b.cancel = context.WithCancel(ctx)
	b.running = true

	b.bot.Start(ctx)
	return nil
}

func (b *Bot) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cancel != nil {
		b.cancel()
	}
	b.running = false
}

func (b *Bot) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

func (b *Bot) OwnerID() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ownerID
}

func (b *Bot) loadSessions(ctx context.Context) {
	tokens, err := b.store.ListBotTokens(ctx)
	if err != nil {
		return
	}
	for _, t := range tokens {
		if t.OwnerChatID > 0 {
			b.mu.Lock()
			b.ownerID = t.OwnerChatID
			b.mu.Unlock()
			break
		}
	}
}

func (b *Bot) handleUpdate(ctx context.Context, u *models.Update) {
	if u.Message == nil || u.Message.Text == "" {
		return
	}
	chatID := u.Message.Chat.ID

	b.mu.Lock()
	ownerID := b.ownerID
	b.mu.Unlock()

	if ownerID > 0 && chatID != ownerID {
		b.sendText(ctx, chatID, "Bot ini private.")
		return
	}

	if ownerID == 0 {
		b.mu.Lock()
		b.ownerID = chatID
		b.mu.Unlock()
		b.saveTokenOwner(ctx, chatID)
	}

	text := u.Message.Text
	switch {
	case text == "/start":
		b.cmdStart(ctx, chatID)
	case text == "/help":
		b.cmdHelp(ctx, chatID)
	case text == "/new":
		b.cmdNew(ctx, chatID)
	case len(text) > 7 && text[:7] == "/model ":
		b.cmdModel(ctx, chatID, text[7:])
	default:
		b.handlePrompt(ctx, chatID, text)
	}
}

func (b *Bot) saveTokenOwner(ctx context.Context, chatID int64) {
	tokens, err := b.store.ListBotTokens(ctx)
	if err != nil {
		return
	}
	for _, t := range tokens {
		if t.OwnerChatID == 0 {
			b.store.SaveBotToken(ctx, storage.CreateBotTokenInput{
				ID: t.ID, Token: t.Token,
				Label: t.Label, OwnerChatID: chatID,
			})
			break
		}
	}
}

func (b *Bot) sendText(ctx context.Context, chatID int64, text string) {
	b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
}

func (b *Bot) sendMarkdown(ctx context.Context, chatID int64, text string) {
	b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
}

// Keep unused imports for handler.go
var (
	_ = log.Println
	_ = session.NewProcessor
	_ = time.Second
	_ = message.RoleUser
)
