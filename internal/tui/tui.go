package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-go/opencode/internal/bus"
	"github.com/opencode-go/opencode/internal/llm"
	"github.com/opencode-go/opencode/internal/log"
	"github.com/opencode-go/opencode/internal/message"
	"github.com/opencode-go/opencode/internal/session"
	"github.com/opencode-go/opencode/internal/storage"
	"github.com/opencode-go/opencode/internal/tool"
)

type chatMessage struct {
	Role    string
	Content string
}

type progressMsg struct {
	role    string
	content string
	id      string
}

type runResultMsg struct {
	text      string
	err       string
	toolCalls int
}

func defaultModelName() string {
	m := os.Getenv("OPENCODE_MODEL")
	if m != "" {
		return m
	}
	return "big-pickle"
}

func defaultAPIURL() string {
	u := os.Getenv("OPENCODE_API_URL")
	if u != "" {
		return u
	}
	return "https://opencode.ai/zen/v1"
}

func defaultAPIKey() string {
	k := os.Getenv("OPENCODE_API_KEY")
	if k != "" {
		return k
	}
	return "public"
}

type tuiModel struct {
	proc        *session.Processor
	store       storage.Store
	eventBus    *bus.Bus
	program     *tea.Program

	ready        bool
	width        int
	height       int
	messages     []chatMessage
	input        string
	isLoading    bool
	prompt       string
	status       string
	lastError    string
	currentModel string
	apiURL       string
	hasAPIKey    bool

	progressList []string
	subIDs       []int
	turnCount    int
	chatHistory  []message.Message
	sessionID    string
}

func initialModel() tuiModel {
	return tuiModel{
		messages: []chatMessage{
			{Role: "system", Content: "Welcome to OpenCode-Go! Type a prompt and press Enter to start."},
		},
		status: "Ready",
	}
}

func Run(reg *tool.Registry, modelName, apiURL, apiKey string) error {
	store, err := storage.NewSQLiteStore(fmt.Sprintf("%s%copencode-tui.db", os.TempDir(), os.PathSeparator))
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer store.Close()

	eventBus := bus.New()

	// Subscribe bus events to global logger
	if log.Default.Enabled(log.LevelDebug) {
		eventBus.SubscribeAll(func(e bus.Event) {
			log.Debug("bus."+e.Type(), "type", e.Type())
		})
	}

	client := llm.NewClient(apiURL)
	if apiKey != "" {
		client.SetAPIKey(apiKey)
	}

	system := "You are a helpful AI assistant with access to tools."
	proc := session.NewProcessor(reg, client, store, eventBus, modelName, system)
	proc.EnableSubAgents()

	initModel := initialModel()
	initModel.proc = proc
	initModel.store = store
	initModel.eventBus = eventBus
	initModel.currentModel = modelName
	initModel.apiURL = apiURL
	initModel.hasAPIKey = apiKey != ""
	initModel.status = fmt.Sprintf("Model: %s | %s", modelName, apiURL)

	p := tea.NewProgram(
		&initModel,
		tea.WithAltScreen(),
	)
	initModel.program = p

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui error: %w", err)
	}

	return nil
}

func (m *tuiModel) Init() tea.Cmd {
	m.ready = true

	welcomeMsg := "Welcome to OpenCode-Go! Type a prompt and press Enter."
	if !m.hasAPIKey {
		welcomeMsg += "\n\nFree model (big-pickle) via Zen API. "
		welcomeMsg += "Set OPENCODE_API_KEY to connect other providers. "
		welcomeMsg += "Type /help for commands."
	}

	m.messages = []chatMessage{
		{Role: "system", Content: welcomeMsg},
	}

	// Subscribe to event bus for live progress
	m.subIDs = append(m.subIDs, m.eventBus.Subscribe(bus.TypeToolCalled, func(e bus.Event) {
		tc := e.(bus.ToolEvent)
		content := fmt.Sprintf("🔧 Step %d: %s %s", m.turnCount, tc.ToolName, truncateStr(tc.Input, 60))
		m.program.Send(progressMsg{role: "tool", content: content, id: tc.ToolName + "_" + tc.SessionID})
	}))

	m.subIDs = append(m.subIDs, m.eventBus.Subscribe(bus.TypeToolCompleted, func(e bus.Event) {
		tc := e.(bus.ToolEvent)
		content := fmt.Sprintf("✅ Step %d: %s completed (%dms)", m.turnCount, tc.ToolName, tc.DurationMs)
		m.program.Send(progressMsg{role: "tool", content: content, id: tc.SessionID + "_done"})
	}))

	m.subIDs = append(m.subIDs, m.eventBus.Subscribe(bus.TypeToolFailed, func(e bus.Event) {
		tc := e.(bus.ToolEvent)
		content := fmt.Sprintf("❌ Step %d: %s failed: %s (%dms)", m.turnCount, tc.ToolName, truncateStr(tc.Error, 80), tc.DurationMs)
		m.program.Send(progressMsg{role: "error", content: content, id: tc.SessionID + "_fail"})
	}))

	m.subIDs = append(m.subIDs, m.eventBus.Subscribe(bus.TypeLLMStarted, func(e bus.Event) {
		m.turnCount++
		content := fmt.Sprintf("🧠 Step %d: Thinking...", m.turnCount)
		m.program.Send(progressMsg{role: "system", content: content, id: "llm_start"})
	}))

	m.subIDs = append(m.subIDs, m.eventBus.Subscribe(bus.TypeLLMCompleted, func(e bus.Event) {
		le := e.(bus.LLMEvent)
		content := fmt.Sprintf("📝 Step %d: Response received (%dms)", m.turnCount, le.DurationMs)
		m.program.Send(progressMsg{role: "system", content: content, id: "llm_done"})
	}))

	return nil
}

func (m *tuiModel) cleanupSubs() {
	for _, id := range m.subIDs {
		m.eventBus.UnsubscribeAll(id)
	}
	m.subIDs = nil
}

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.ready = true
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+d":
			m.cleanupSubs()
			return m, tea.Quit

		case "enter":
			if m.isLoading {
				break
			}
			input := strings.TrimSpace(m.prompt)
			if input == "" {
				input = strings.TrimSpace(m.input)
				m.input = ""
			}
			if input == "" {
				break
			}

			if strings.HasPrefix(input, "/") {
				return m, m.handleCommand(input)
			}

			m.isLoading = true
			m.messages = append(m.messages, chatMessage{Role: "user", Content: input})
			m.status = fmt.Sprintf("Running on %s...", m.currentModel)
			m.prompt = ""
			m.lastError = ""
			m.progressList = nil
			m.turnCount = 0
			return m, m.runSession(input)

		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}

		default:
			if !m.isLoading && len(msg.Runes) > 0 {
				m.input += string(msg.Runes)
			}
		}

	case progressMsg:
		if strings.HasPrefix(msg.role, "tool") || msg.role == "error" || msg.role == "system" {
			m.progressList = append(m.progressList, msg.content)
		}

	case runResultMsg:
		m.isLoading = false
		m.progressList = nil
		m.messages = CleanProgressMessages(m.messages)

		if msg.err != "" {
			m.messages = append(m.messages, chatMessage{Role: "error", Content: msg.err})
			m.status = fmt.Sprintf("Error | Model: %s", m.currentModel)
			m.lastError = msg.err
		}
		if msg.text != "" {
			m.messages = append(m.messages, chatMessage{Role: "assistant", Content: msg.text})
			m.status = fmt.Sprintf("Done | Model: %s | %d tool calls", m.currentModel, msg.toolCalls)
		}
	}

	return m, nil
}

func (m *tuiModel) View() string {
	return render(m)
}

func (m *tuiModel) handleCommand(input string) tea.Cmd {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	switch parts[0] {
	case "/model":
		if len(parts) < 2 {
			m.messages = append(m.messages, chatMessage{
				Role: "system", Content: fmt.Sprintf("Current model: %s\nUsage: /model <name>", m.currentModel),
			})
			return nil
		}
		m.currentModel = parts[1]
		m.status = fmt.Sprintf("Model: %s", m.currentModel)
		m.messages = append(m.messages, chatMessage{
			Role: "system", Content: fmt.Sprintf("Model changed to %s", m.currentModel),
		})

	case "/apiurl":
		if len(parts) < 2 {
			m.messages = append(m.messages, chatMessage{
				Role: "system", Content: fmt.Sprintf("Current API URL: %s\nUsage: /apiurl <url>", m.apiURL),
			})
			return nil
		}
		m.apiURL = parts[1]
		m.status = fmt.Sprintf("API: %s", m.apiURL)
		m.messages = append(m.messages, chatMessage{
			Role: "system", Content: fmt.Sprintf("API URL changed to %s", m.apiURL),
		})

	case "/help":
		help := "Commands:\n  /model <name>  - Change model\n  /apiurl <url> - Change API URL\n  /help        - Show this\n  Ctrl+C       - Quit"
		m.messages = append(m.messages, chatMessage{Role: "system", Content: help})

	default:
		m.messages = append(m.messages, chatMessage{
			Role: "system", Content: fmt.Sprintf("Unknown: %s\nType /help", parts[0]),
		})
	}
	return nil
}

func (m *tuiModel) runSession(prompt string) tea.Cmd {
	sessionID := m.sessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("tui_%d", time.Now().UnixNano())
		m.sessionID = sessionID
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		client := llm.NewClient(m.apiURL)
		apiKey := os.Getenv("OPENCODE_API_KEY")
		if apiKey == "" {
			apiKey = "public"
		}
		client.SetAPIKey(apiKey)

		system := "You are a helpful AI assistant with access to tools."

		store, err := storage.NewSQLiteStore(fmt.Sprintf("%s%copencode-tui.db", os.TempDir(), os.PathSeparator))
		if err != nil {
			return runResultMsg{err: fmt.Sprintf("Storage error: %v", err)}
		}

		proc := session.NewProcessor(m.proc.ExportToolRegistry(), client, store, m.eventBus, m.currentModel, system)
		proc.EnableSubAgents()

		var history []message.Message
		if len(m.chatHistory) > 0 {
			history = m.chatHistory
		}

		result, procErr := proc.Run(ctx, prompt, sessionID, "tui", history)
		if procErr != nil {
			return runResultMsg{err: procErr.Error()}
		}

		if len(result.Messages) > 0 {
			m.chatHistory = result.Messages
		}

		return runResultMsg{
			text:      result.FinalText,
			toolCalls: result.ToolCalls,
		}
	}
}

func CleanProgressMessages(msgs []chatMessage) []chatMessage {
	var cleaned []chatMessage
	hasResult := false
	for _, m := range msgs {
		if m.Role == "system" && (strings.HasPrefix(m.Content, "🧠") || strings.HasPrefix(m.Content, "📝")) {
			continue
		}
		if m.Role == "assistant" && !hasResult {
			hasResult = true
			cleaned = append(cleaned, chatMessage{Role: "system", Content: "───── ⏎ Result ─────"})
		}
		cleaned = append(cleaned, m)
	}
	return cleaned
}
