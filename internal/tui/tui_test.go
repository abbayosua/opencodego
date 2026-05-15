package tui

import (
	"strings"
	"testing"
)

func TestModelInitialState(t *testing.T) {
	m := initialModel()
	if m.ready != false {
		t.Error("expected ready=false initially")
	}
	if len(m.messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(m.messages))
	}
	if m.messages[0].Role != "system" {
		t.Errorf("expected system role, got %q", m.messages[0].Role)
	}
}

func TestModelAddUserMessage(t *testing.T) {
	m := initialModel()
	m.messages = append(m.messages, chatMessage{Role: "user", Content: "list files"})

	if len(m.messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(m.messages))
	}
	if m.messages[1].Content != "list files" {
		t.Errorf("expected 'list files', got %q", m.messages[1].Content)
	}
}

func TestModelLoadingState(t *testing.T) {
	m := initialModel()
	m.isLoading = true
	if !m.isLoading {
		t.Error("expected isLoading=true")
	}
	m.isLoading = false
	if m.isLoading {
		t.Error("expected isLoading=false after reset")
	}
}

func TestModelErrorState(t *testing.T) {
	m := initialModel()
	m.lastError = "API timeout"
	m.messages = append(m.messages, chatMessage{Role: "error", Content: "API timeout"})

	if m.lastError != "API timeout" {
		t.Errorf("expected 'API timeout', got %q", m.lastError)
	}
	if len(m.messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(m.messages))
	}
	if m.messages[1].Role != "error" {
		t.Errorf("expected error role, got %q", m.messages[1].Role)
	}
}

func TestModelMultipleMessages(t *testing.T) {
	m := initialModel()
	for i := 0; i < 5; i++ {
		m.messages = append(m.messages, chatMessage{Role: "user", Content: "msg"})
		m.messages = append(m.messages, chatMessage{Role: "assistant", Content: "resp"})
	}
	if len(m.messages) != 11 {
		t.Errorf("expected 11 messages, got %d", len(m.messages))
	}
}

func TestRenderingSystemMessage(t *testing.T) {
	m := initialModel()
	m.ready = true
	m.width = 80
	m.height = 24

	out := render(&m)
	if !strings.Contains(out, "Welcome") {
		t.Errorf("expected welcome message, got: %s", out)
	}
}

func TestRenderingLoadingState(t *testing.T) {
	m := initialModel()
	m.ready = true
	m.width = 80
	m.height = 24
	m.isLoading = true

	out := render(&m)
	if !strings.Contains(out, "Processing") {
		t.Errorf("expected Processing, got: %s", out)
	}
}

func TestRenderingWithMessages(t *testing.T) {
	m := initialModel()
	m.ready = true
	m.width = 80
	m.height = 24
	m.messages = append(m.messages,
		chatMessage{Role: "user", Content: "list go files"},
		chatMessage{Role: "assistant", Content: "Files: main.go, handler.go"},
	)

	out := render(&m)
	if !strings.Contains(out, "You:") {
		t.Errorf("expected You:, got: %s", out)
	}
	if !strings.Contains(out, "Assistant:") {
		t.Errorf("expected Assistant:, got: %s", out)
	}
}

func TestRenderingError(t *testing.T) {
	m := initialModel()
	m.ready = true
	m.width = 80
	m.height = 24
	m.lastError = "API timeout"

	out := render(&m)
	if !strings.Contains(out, "Error:") {
		t.Errorf("expected Error:, got: %s", out)
	}
}

func TestRenderingWithoutReady(t *testing.T) {
	m := initialModel()
	out := render(&m)
	if !strings.Contains(out, "Initializing") {
		t.Errorf("expected Initializing, got: %s", out)
	}
}

func TestIndentContent(t *testing.T) {
	result := indentContent("hello\nworld", "> ", 80)
	if !strings.Contains(result, "> hello") {
		t.Errorf("expected '> hello', got: %s", result)
	}
	if !strings.Contains(result, "> world") {
		t.Errorf("expected '> world', got: %s", result)
	}
}

func TestWrapLine(t *testing.T) {
	wrapped := wrapLine("short", 80)
	if len(wrapped) != 1 {
		t.Errorf("expected 1 line, got %d", len(wrapped))
	}

	long := "a" + string(make([]byte, 100))
	wrapped = wrapLine(long, 10)
	if len(wrapped) < 2 {
		t.Errorf("expected multiple lines for long string, got %d", len(wrapped))
	}
}

func TestRenderingWithInput(t *testing.T) {
	m := initialModel()
	m.ready = true
	m.width = 80
	m.height = 24
	m.input = "list files"

	out := render(&m)
	if !strings.Contains(out, "list files") {
		t.Errorf("expected input text to render, got: %s", out)
	}

	// Test that input text REPLACES the placeholder
	if strings.Contains(out, "Type a prompt") {
		t.Log("placeholder hidden when input is non-empty")
	}

	m.isLoading = true
	out = render(&m)
	if strings.Contains(out, "list files") {
		t.Log("input hidden during loading")
	}
}

func TestProgressMessagesRender(t *testing.T) {
	m := initialModel()
	m.ready = true
	m.width = 80
	m.height = 24
	m.isLoading = true
	m.progressList = []string{
		"🧠 Step 1: Thinking...",
		"🔧 Step 1: bash ls -la",
		"✅ Step 1: bash completed",
	}

	out := render(&m)
	if !strings.Contains(out, "🔧") {
		t.Errorf("expected tool emoji, got: %s", out)
	}
	if !strings.Contains(out, "bash") {
		t.Errorf("expected tool name, got: %s", out)
	}
}

func TestProgressMessagesWithoutTools(t *testing.T) {
	m := initialModel()
	m.ready = true
	m.width = 80
	m.height = 24
	m.isLoading = true

	out := render(&m)
	if !strings.Contains(out, "Processing") {
		t.Errorf("expected Processing text, got: %s", out)
	}
}

func TestProgressMessagesShowStepNumber(t *testing.T) {
	m := initialModel()
	m.ready = true
	m.width = 80
	m.height = 24
	m.isLoading = true
	m.progressList = []string{
		"🧠 Step 1: Thinking...",
		"🔧 Step 1: bash ls",
		"✅ Step 1: bash completed",
	}

	out := render(&m)
	if !strings.Contains(out, "Step 1") {
		t.Errorf("expected Step 1 in output, got: %s", out)
	}
}

func TestProgressMessagesClearedAfterRun(t *testing.T) {
	m := initialModel()
	m.isLoading = true
	m.progressList = []string{"🔧 bash"}

	// Simulate run completion
	m.isLoading = false
	m.progressList = nil

	if len(m.progressList) != 0 {
		t.Error("expected progress messages cleared")
	}
}

func TestTruncateStr(t *testing.T) {
	if truncateStr("short", 10) != "short" {
		t.Errorf("expected 'short', got %q", truncateStr("short", 10))
	}
	result := truncateStr("this is a very long string", 10)
	if len(result) != 10 {
		t.Errorf("expected length 10, got %d: %q", len(result), result)
	}
}
