package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	appStyle    = lipgloss.NewStyle().Padding(0, 1)
	msgListStyle = lipgloss.NewStyle().Padding(0, 0, 1, 0)
	inputStyle  = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("62")).Padding(0, 1)
	statusStyle = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("240"))
	headerStyle = lipgloss.NewStyle().Padding(0, 1).Background(lipgloss.Color("62")).Foreground(lipgloss.Color("229")).Bold(true)

	userStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	assistantStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
	toolStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	systemStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	loadingStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
)

func render(m *tuiModel) string {
	if !m.ready {
		return "\n  Initializing..."
	}

	var b strings.Builder

	headerText := " OpenCode-Go "
	if m.currentModel != "" {
		headerText = fmt.Sprintf(" OpenCode-Go | Model: %s ", m.currentModel)
	}
	h := headerStyle.Width(m.width - 2).Render(headerText)
	b.WriteString(h)
	b.WriteString("\n")

	msgHeight := m.height - 6
	if msgHeight < 3 {
		msgHeight = 3
	}

	showMsgs := m.messages
	if len(showMsgs) > 0 {
		totalLines := 0
		start := len(showMsgs)
		for i := len(showMsgs) - 1; i >= 0; i-- {
			rendered := renderMessage(showMsgs[i], m.width)
			lines := strings.Count(rendered, "\n") + 1
			if totalLines+lines > msgHeight {
				break
			}
			totalLines += lines
			start = i
		}

		b.WriteString("\n")
		for i := start; i < len(showMsgs); i++ {
			b.WriteString(renderMessage(showMsgs[i], m.width))
			b.WriteString("\n")
		}
	}

	if m.isLoading {
		if len(m.progressMsgs) > 0 {
			b.WriteString("\n")
			for _, v := range m.progressMsgs {
				b.WriteString(toolStyle.Render("  " + v))
				b.WriteString("\n")
			}
		} else {
			b.WriteString(loadingStyle.Render("\n  🤔 Processing...\n"))
		}
	}

	fillHeight := m.height - 2
	lines := strings.Count(b.String(), "\n")
	for lines < fillHeight {
		b.WriteString("\n")
		lines++
	}

	inputText := m.input
	if m.isLoading {
		inputText = "Waiting for response..."
	} else if inputText == "" {
		inputText = "Type a prompt and press Enter (Ctrl+C to quit)"
	}
	inp := inputStyle.Width(m.width - 4).Render(inputText)
	b.WriteString(inp)
	b.WriteString("\n")

	stat := m.status
	if m.lastError != "" {
		stat = "Error: " + truncateStr(m.lastError, 40)
	}
	b.WriteString(statusStyle.Width(m.width - 2).Render(stat))

	return appStyle.Render(b.String())
}

func renderMessage(msg chatMessage, width int) string {
	var header, content string

	switch msg.Role {
	case "user":
		header = userStyle.Render("You:")
	case "assistant":
		header = assistantStyle.Render("Assistant:")
	case "tool":
		header = toolStyle.Render("Tool:")
	case "system":
		return systemStyle.Render("  " + msg.Content)
	case "error":
		header = errorStyle.Render("Error:")
	}

	content = msg.Content
	if len(content) > 500 {
		content = content[:500] + "..."
	}

	roleWidth := width - 6
	if roleWidth < 20 {
		roleWidth = 20
	}
	if roleWidth > 80 {
		roleWidth = 80
	}

	indented := indentContent(content, " ", roleWidth)
	return fmt.Sprintf("%s\n%s", header, indented)
}

func indentContent(content, prefix string, width int) string {
	lines := strings.Split(content, "\n")
	var result []string
	for _, line := range lines {
		wrapped := wrapLine(line, width)
		for _, wl := range wrapped {
			result = append(result, prefix+wl)
		}
	}
	return strings.Join(result, "\n")
}

func wrapLine(line string, width int) []string {
	if len(line) <= width {
		return []string{line}
	}
	var wrapped []string
	for len(line) > 0 {
		if len(line) <= width {
			wrapped = append(wrapped, line)
			break
		}
		wrapped = append(wrapped, line[:width])
		line = line[width:]
	}
	return wrapped
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
