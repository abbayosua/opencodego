package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/opencode-go/opencode/internal/llm"
	"github.com/opencode-go/opencode/internal/llmtest"
	"github.com/opencode-go/opencode/internal/session"
	"github.com/opencode-go/opencode/internal/tool"
)

func TestProcessorBasicText(t *testing.T) {
	llmSrv := llmtest.NewForTest(t)
	llmSrv.Text("Hello! I am an AI assistant.")

	reg := tool.NewRegistry()
	reg.Register(tool.ReadTool())
	reg.Register(tool.BashTool())

	client := llm.NewClient(llmSrv.URL())
	proc := session.NewProcessor(reg, client, "test-model", "You are a helpful assistant.")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := proc.Run(ctx, "Say hello")
	if err != nil {
		t.Fatal(err)
	}

	if result.FinalText == "" {
		t.Error("expected non-empty final text")
	}
	if !contains(result.FinalText, "Hello") {
		t.Errorf("expected final text to contain 'Hello', got: %s", result.FinalText)
	}
	if result.ToolCalls != 0 {
		t.Errorf("expected 0 tool calls, got %d", result.ToolCalls)
	}
}

func TestProcessorToolCall(t *testing.T) {
	llmSrv := llmtest.NewForTest(t)
	llmSrv.Reply().Text("Let me check the directory.").Tool("bash", map[string]any{"command": "ls"}).Item()
	llmSrv.Text("I found these files.")

	reg := tool.NewRegistry()
	reg.Register(tool.BashTool())

	client := llm.NewClient(llmSrv.URL())
	proc := session.NewProcessor(reg, client, "test-model", "You are a helpful assistant.")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := proc.Run(ctx, "List files")
	if err != nil {
		t.Fatal(err)
	}

	if result.ToolCalls != 1 {
		t.Errorf("expected 1 tool call, got %d", result.ToolCalls)
	}

	history := result.Messages
	hasToolCall := false
	hasToolResult := false
	for _, m := range history {
		if m.Role == "tool" {
			hasToolResult = true
		}
		for _, c := range m.Content {
			if c.Type == "tool_use" {
				hasToolCall = true
			}
		}
	}
	if !hasToolCall {
		t.Error("expected tool_use content in history")
	}
	if !hasToolResult {
		t.Error("expected tool role message in history")
	}
}

func TestProcessorMultiTurn(t *testing.T) {
	llmSrv := llmtest.NewForTest(t)
	llmSrv.Reply().
		Text("Let me check multiple things.").
		Tool("bash", map[string]any{"command": "echo hello"}).
		Tool("bash", map[string]any{"command": "echo world"}).
		Item()
	llmSrv.Text("Done with both commands.")

	reg := tool.NewRegistry()
	reg.Register(tool.BashTool())

	client := llm.NewClient(llmSrv.URL())
	proc := session.NewProcessor(reg, client, "test-model", "You are a helpful assistant.")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := proc.Run(ctx, "Run two commands")
	if err != nil {
		t.Fatal(err)
	}

	if result.ToolCalls != 2 {
		t.Errorf("expected 2 tool calls, got %d", result.ToolCalls)
	}
}

func TestProcessorErrorHandling(t *testing.T) {
	llmSrv := llmtest.NewForTest(t)
	llmSrv.Tool("unknown_tool", map[string]any{"arg": "val"})
	llmSrv.Text("I see the tool failed.")

	reg := tool.NewRegistry()
	reg.Register(tool.BashTool())

	client := llm.NewClient(llmSrv.URL())
	proc := session.NewProcessor(reg, client, "test-model", "You are a helpful assistant.")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := proc.Run(ctx, "Use unknown tool")
	if err != nil {
		t.Fatal(err)
	}

	// The processor should handle unknown tools gracefully
	if result.ToolCalls != 1 {
		t.Errorf("expected 1 tool call (unknown), got %d", result.ToolCalls)
	}
}

func TestProcessorWithTools(t *testing.T) {
	llmSrv := llmtest.NewForTest(t)
	llmSrv.Reply().
		Text("Let me read the test file.").
		Tool("read", map[string]any{"file_path": "go.mod"}).
		Item()
	llmSrv.Text("I found the module info.")

	reg := tool.NewRegistry()
	reg.Register(tool.ReadTool())
	reg.Register(tool.BashTool())
	reg.Register(tool.GrepTool())

	client := llm.NewClient(llmSrv.URL())
	proc := session.NewProcessor(reg, client, "test-model", "You are a helpful assistant.")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := proc.Run(ctx, "Read go.mod")
	if err != nil {
		t.Fatal(err)
	}

	if result.ToolCalls != 1 {
		t.Errorf("expected 1 tool call, got %d", result.ToolCalls)
	}
}

func TestProcessorCancellation(t *testing.T) {
	llmSrv := llmtest.NewForTest(t)
	llmSrv.Hang()

	reg := tool.NewRegistry()
	client := llm.NewClient(llmSrv.URL())
	proc := session.NewProcessor(reg, client, "test-model", "You are a helpful assistant.")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := proc.Run(ctx, "hang")
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && stringsContains(s, substr)
}

func stringsContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
