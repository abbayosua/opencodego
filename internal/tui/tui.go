package tui

import (
	"context"
	"fmt"
	"os"
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

func initialModel() tuiModel {
	return tuiModel{
		messages: []chatMessage{
			{Role: "system", Content: "Welcome to OpenCode-Go! Type a prompt and press Enter."},
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
	client := llm.NewClient(apiURL)
	client.SetAPIKey(apiKey)

	system := "You are a helpful AI assistant with access to tools."
	proc := session.NewProcessor(reg, client, store, eventBus, modelName, system)

	p := tea.NewProgram(
		&tuiModel{proc: proc, store: store},
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

	ready     bool
	width     int
	height    int
	messages  []chatMessage
	input     string
	isLoading bool
	prompt    string
	status    string
	lastError string
	sessionID string
}

type runResultMsg struct {
	text      string
	err       string
	toolCalls int
}

func (m *tuiModel) Init() tea.Cmd {
	m.ready = true
	m.messages = []chatMessage{
		{Role: "system", Content: "Welcome to OpenCode-Go! Type a prompt and press Enter."},
	}
	m.status = "Ready"
	m.sessionID = fmt.Sprintf("tui_%d", time.Now().UnixNano())
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
		case "ctrl+c", "ctrl+d", "q":
			return m, tea.Quit

		case "enter":
			if m.isLoading {
				break
			}
			input := trim(m.prompt)
			if input == "" {
				input = trim(m.input)
				m.input = ""
			}
			if input == "" {
				break
			}
			m.isLoading = true
			m.messages = append(m.messages, chatMessage{Role: "user", Content: input})
			m.status = "Running..."
			m.prompt = ""
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
			m.status = "Error"
			m.lastError = msg.err
		}
		if msg.text != "" {
			m.messages = append(m.messages, chatMessage{Role: "assistant", Content: msg.text})
			m.status = fmt.Sprintf("Done | %d tool calls", msg.toolCalls)
		}
	}

	return m, nil
}

func (m *tuiModel) View() string {
	return render(m)
}

func (m *tuiModel) runSession(prompt string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		result, err := m.proc.Run(ctx, prompt, m.sessionID, "tui")
		if err != nil {
			return runResultMsg{err: err.Error()}
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
