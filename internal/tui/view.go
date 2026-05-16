package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	appStyle       = lipgloss.NewStyle().Padding(0, 1)
	inputStyle     = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("62")).Padding(0, 1)
	statusStyle    = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("240"))
	headerStyle    = lipgloss.NewStyle().Padding(0, 1).Background(lipgloss.Color("62")).Foreground(lipgloss.Color("229")).Bold(true)
	userStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	assistantStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
	toolStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	systemStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	loadingStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	separatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true)
)

func renderMessages(m *tuiModel) string {
	var b strings.Builder
	width := m.width - 4
	if width < 20 {
		width = 20
	}

	for _, msg := range m.messages {
		b.WriteString(renderMessage(msg, width))
		b.WriteString("\n")
	}

	if m.isLoading && len(m.progressList) > 0 {
		start := 0
		if len(m.progressList) > 8 {
			start = len(m.progressList) - 8
			b.WriteString(systemStyle.Render(fmt.Sprintf("  ... %d more steps", start)))
			b.WriteString("\n")
		}
		for _, v := range m.progressList[start:] {
			b.WriteString(toolStyle.Render("  " + v))
			b.WriteString("\n")
		}
	} else if m.isLoading {
		b.WriteString(loadingStyle.Render("\n  🤔 Processing...\n"))
	}

	return b.String()
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
		if strings.HasPrefix(msg.Content, "─────") {
			return separatorStyle.Render("  " + msg.Content)
		}
		return systemStyle.Render("  " + msg.Content)
	case "error":
		header = errorStyle.Render("Error:")
	}
	content = msg.Content
	if len(content) > 2000 {
		content = content[:2000] + "..."
	}
	wrapWidth := width - 2
	if wrapWidth < 10 {
		wrapWidth = 10
	}
	indented := indentContent(content, " ", wrapWidth)
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
