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
	"github.com/opencode-go/opencode/internal/session"
	"github.com/opencode-go/opencode/internal/storage"
	"github.com/opencode-go/opencode/internal/tool"
)

type chatMessage struct {
	Role    string
	Content string
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

func Run(reg *tool.Registry, modelName, apiURL, apiKey string) error {
	store, err := storage.NewSQLiteStore(fmt.Sprintf("%s%copencode-tui.db", os.TempDir(), os.PathSeparator))
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer store.Close()

	eventBus := bus.New()
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
	initModel.currentModel = modelName
	initModel.apiURL = apiURL
	initModel.hasAPIKey = apiKey != ""
	initModel.status = fmt.Sprintf("Model: %s | %s", modelName, apiURL)

	p := tea.NewProgram(
		&initModel,
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui error: %w", err)
	}

	return nil
}

type tuiModel struct {
	proc    *session.Processor
	store   storage.Store

	ready        bool
	width        int
	height       int
	messages     []chatMessage
	input        string
	isLoading    bool
	prompt       string
	status       string
	lastError    string
	sessionID    string
	currentModel string
	apiURL       string
	hasAPIKey    bool
}

type runResultMsg struct {
	text      string
	err       string
	toolCalls int
}

func initialModel() tuiModel {
	return tuiModel{
		messages: []chatMessage{
			{Role: "system", Content: "Welcome to OpenCode-Go! Type a prompt and press Enter to start."},
		},
		status: "Ready",
	}
}

func (m *tuiModel) Init() tea.Cmd {
	m.ready = true
	m.sessionID = fmt.Sprintf("tui_%d", time.Now().UnixNano())

	welcomeMsg := "Welcome to OpenCode-Go! Type a prompt and press Enter."
	if !m.hasAPIKey {
		welcomeMsg += "\n\nNo API key configured. Set OPENCODE_API_KEY to use a real LLM. " +
			"Type /model <name> to change model or /apiurl <url> to change API endpoint."
	}

	m.messages = []chatMessage{
		{Role: "system", Content: welcomeMsg},
	}

	return nil
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
			return m, m.runSession(input)

		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}

		default:
			if !m.isLoading {
				m.input += msg.String()
			}
		}

	case runResultMsg:
		m.isLoading = false
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
		oldModel := m.currentModel
		m.currentModel = parts[1]
		m.status = fmt.Sprintf("Model: %s", m.currentModel)
		m.messages = append(m.messages, chatMessage{
			Role: "system", Content: fmt.Sprintf("Model changed from %s to %s", oldModel, m.currentModel),
		})
		return nil

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
		return nil

	case "/help":
		help := "Commands:\n  /model <name>  - Change model (e.g. /model gpt-4o)\n  /apiurl <url> - Change API URL\n  /help        - Show this help\n  Ctrl+C       - Quit"
		m.messages = append(m.messages, chatMessage{Role: "system", Content: help})
		return nil

	default:
		m.messages = append(m.messages, chatMessage{
			Role: "system", Content: fmt.Sprintf("Unknown command: %s\nType /help for available commands.", parts[0]),
		})
		return nil
	}
}

func (m *tuiModel) runSession(prompt string) tea.Cmd {
	return func() tea.Msg {
		sessionID := fmt.Sprintf("tui_%d_%d", time.Now().UnixNano(), time.Now().UnixMilli())
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		client := llm.NewClient(m.apiURL)
		apiKey := os.Getenv("OPENCODE_API_KEY")
		if apiKey == "" {
			apiKey = "public"
		}
		client.SetAPIKey(apiKey)

		system := "You are a helpful AI assistant with access to tools."
		eventBus := bus.New()
		store, err := storage.NewSQLiteStore(fmt.Sprintf("%s%copencode-tui.db", os.TempDir(), os.PathSeparator))
		if err != nil {
			return runResultMsg{err: fmt.Sprintf("Storage error: %v", err)}
		}
		defer store.Close()

		proc := session.NewProcessor(m.proc.ExportToolRegistry(), client, store, eventBus, m.currentModel, system)
		proc.EnableSubAgents()

		result, procErr := proc.Run(ctx, prompt, sessionID, "tui")
		if procErr != nil {
			return runResultMsg{err: procErr.Error()}
		}

		return runResultMsg{
			text:      result.FinalText,
			toolCalls: result.ToolCalls,
		}
	}
}

func trim(s string) string {
	if len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == ' ' {
		s = s[:len(s)-1]
	}
	return s
}
