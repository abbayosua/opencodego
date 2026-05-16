package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/opencode-go/opencode/internal/bus"
	"github.com/opencode-go/opencode/internal/llm"
	"github.com/opencode-go/opencode/internal/message"
	"github.com/opencode-go/opencode/internal/session"
	"github.com/opencode-go/opencode/internal/storage"
)

func (b *Bot) cmdStart(ctx context.Context, chatID int64) {
	msg := `🤖 *OpenCode-Go Telegram Bot*

Siap membantu! Kirim prompt untuk memulai.

*Commands:*
/help - Bantuan
/model <name> - Ganti model
/new - Reset percakapan

*Contoh:*
- buat index.html
- cari fungsi main
- list semua file
`
	b.sendMarkdown(ctx, chatID, msg)
}

func (b *Bot) cmdHelp(ctx context.Context, chatID int64) {
	msg := fmt.Sprintf(`📖 *Bantuan*

*Commands:*
/start - Mulai
/help - Bantuan ini
/model <name> - Ganti model (default: big-pickle)
/new - Reset history percakapan

*Info:*
Model: %s
`, b.model)
	b.sendMarkdown(ctx, chatID, msg)
}

func (b *Bot) cmdNew(ctx context.Context, chatID int64) {
	b.mu.Lock()
	delete(b.sessions, chatID)
	b.mu.Unlock()
	b.store.DeleteTelegramSession(ctx, chatID)
	b.sendText(ctx, chatID, "Percakapan di-reset. History sebelumnya dihapus.")
}

func (b *Bot) cmdModel(ctx context.Context, chatID int64, modelName string) {
	if modelName == "" {
		b.sendText(ctx, chatID, "Gunakan: /model <nama>")
		return
	}
	b.mu.Lock()
	b.model = modelName
	b.mu.Unlock()
	b.sendText(ctx, chatID, fmt.Sprintf("Model diubah ke: %s", modelName))
}

func (b *Bot) handlePrompt(ctx context.Context, chatID int64, prompt string) {
	cs := b.getOrCreateSession(ctx, chatID)

	client := llm.NewClient(b.apiURL)
	client.SetAPIKey(b.apiKey)

	system := "You are a helpful AI assistant with access to tools."
	eventBus := bus.New()

	store, err := storage.NewInMemoryStore()
	if err != nil {
		b.sendText(ctx, chatID, fmt.Sprintf("Storage error: %v", err))
		return
	}
	defer store.Close()

	// Subscribe progress events for real-time updates
	lastSent := time.Now()
	sendProgress := func(msg string) {
		if time.Since(lastSent) < 300*time.Millisecond {
			return
		}
		lastSent = time.Now()
		b.sendText(ctx, chatID, msg)
	}

	subIDs := []int{
		eventBus.Subscribe(bus.TypeLLMStarted, func(e bus.Event) {
			sendProgress("🧠 Thinking...")
		}),
		eventBus.Subscribe(bus.TypeToolCalled, func(e bus.Event) {
			tc := e.(bus.ToolEvent)
			msg := fmt.Sprintf("🔧 %s %s", tc.ToolName, truncateStr(tc.Input, 60))
			sendProgress(msg)
		}),
		eventBus.Subscribe(bus.TypeToolCompleted, func(e bus.Event) {
			tc := e.(bus.ToolEvent)
			sendProgress(fmt.Sprintf("✅ %s (%dms)", tc.ToolName, tc.DurationMs))
		}),
		eventBus.Subscribe(bus.TypeToolFailed, func(e bus.Event) {
			tc := e.(bus.ToolEvent)
			sendProgress(fmt.Sprintf("❌ %s gagal: %s", tc.ToolName, truncateStr(tc.Error, 80)))
		}),
	}
	defer func() {
		for _, id := range subIDs {
			eventBus.UnsubscribeAll(id)
		}
	}()

	proc := session.NewProcessor(b.reg, client, store, eventBus, cs.Model, system)
	proc.EnableSubAgents()

	start := time.Now()
	result, err := proc.Run(ctx, prompt, cs.SessionID, fmt.Sprintf("tg_%d", chatID), cs.History)
	elapsed := time.Since(start)

	if err != nil {
		b.sendText(ctx, chatID, fmt.Sprintf("Error: %v", err))
		return
	}

	if result != nil && len(result.Messages) > 0 {
		b.mu.Lock()
		cs.History = result.Messages
		if len(cs.History) > 100 {
			cs.History = cs.History[len(cs.History)-100:]
		}
		cs.Model = b.model
		b.mu.Unlock()
		b.saveSession(ctx, chatID, cs)
	}

	if result == nil || result.FinalText == "" {
		b.sendText(ctx, chatID, "Selesai (tidak ada output)")
		return
	}

	full := fmt.Sprintf("%s\n\n%s | %d tool calls",
		result.FinalText,
		formatDuration(elapsed),
		result.ToolCalls)

	const maxLen = 4000
	if len(full) <= maxLen {
		b.sendText(ctx, chatID, full)
		return
	}

	parts := splitMessage(full, maxLen)
	for i, part := range parts {
		header := fmt.Sprintf("Hasil (%d/%d)\n", i+1, len(parts))
		b.sendText(ctx, chatID, header+part)
	}
}

func (b *Bot) getOrCreateSession(ctx context.Context, chatID int64) *ChatSession {
	b.mu.Lock()
	defer b.mu.Unlock()

	if s, ok := b.sessions[chatID]; ok {
		return s
	}

	dbSession, err := b.store.GetTelegramSession(ctx, chatID)
	if err == nil && dbSession != nil {
		var history []message.Message
		if err := json.Unmarshal([]byte(dbSession.History), &history); err == nil {
			s := &ChatSession{
				ChatID:    chatID,
				SessionID: dbSession.SessionID,
				History:   history,
				Model:     dbSession.Model,
			}
			b.sessions[chatID] = s
			return s
		}
	}

	s := &ChatSession{
		ChatID:    chatID,
		SessionID: fmt.Sprintf("tg_%d_%d", chatID, time.Now().UnixNano()),
		History:   []message.Message{},
		Model:     b.model,
	}
	b.sessions[chatID] = s
	b.store.SaveTelegramSession(ctx, storage.CreateTelegramSessionInput{
		ChatID: chatID, SessionID: s.SessionID, Model: b.model,
	})
	return s
}

func (b *Bot) saveSession(ctx context.Context, chatID int64, cs *ChatSession) {
	historyJSON, err := json.Marshal(cs.History)
	if err != nil {
		return
	}
	_ = ctx
	b.store.UpdateTelegramSession(context.Background(), chatID, string(historyJSON), cs.Model)
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm %.0fs", int(d.Minutes()), d.Seconds()-float64(int(d.Minutes()))*60)
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func splitMessage(text string, maxLen int) []string {
	runes := []rune(text)
	var parts []string
	for i := 0; i < len(runes); i += maxLen {
		end := i + maxLen
		if end > len(runes) {
			end = len(runes)
		}
		parts = append(parts, string(runes[i:end]))
	}
	return parts
}
