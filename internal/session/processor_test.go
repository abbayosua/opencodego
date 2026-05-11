package session_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opencode-go/opencode/internal/bus"
	"github.com/opencode-go/opencode/internal/llm"
	"github.com/opencode-go/opencode/internal/llmtest"
	"github.com/opencode-go/opencode/internal/session"
	"github.com/opencode-go/opencode/internal/storage"
	"github.com/opencode-go/opencode/internal/tool"
)

func newTestStore(t *testing.T) storage.Store {
	t.Helper()
	s, err := storage.NewInMemoryStore()
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestProcessorBasicText(t *testing.T) {
	llmSrv := llmtest.NewForTest(t)
	llmSrv.Text("Hello! I am an AI assistant.")

	reg := tool.NewRegistry()
	reg.Register(tool.ReadTool())
	reg.Register(tool.BashTool())

	store := newTestStore(t)
	client := llm.NewClient(llmSrv.URL())
	proc := session.NewProcessor(reg, client, store, bus.New(), "test-model", "You are a helpful assistant.")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := proc.Run(ctx, "Say hello", "", "")
	if err != nil {
		t.Fatal(err)
	}

	if result.FinalText == "" {
		t.Error("expected non-empty final text")
	}
	if !stringsContains(result.FinalText, "Hello") {
		t.Errorf("expected final text to contain 'Hello', got: %s", result.FinalText)
	}
	if result.ToolCalls != 0 {
		t.Errorf("expected 0 tool calls, got %d", result.ToolCalls)
	}
	if result.SessionID == "" {
		t.Error("expected non-empty SessionID")
	}
}

func TestProcessorToolCall(t *testing.T) {
	llmSrv := llmtest.NewForTest(t)
	llmSrv.Reply().Text("Let me check the directory.").Tool("bash", map[string]any{"command": "ls"}).Item()
	llmSrv.Text("I found these files.")

	reg := tool.NewRegistry()
	reg.Register(tool.BashTool())

	store := newTestStore(t)
	client := llm.NewClient(llmSrv.URL())
	proc := session.NewProcessor(reg, client, store, bus.New(), "test-model", "You are a helpful assistant.")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := proc.Run(ctx, "List files", "", "")
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

	store := newTestStore(t)
	client := llm.NewClient(llmSrv.URL())
	proc := session.NewProcessor(reg, client, store, bus.New(), "test-model", "You are a helpful assistant.")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := proc.Run(ctx, "Run two commands", "", "")
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

	store := newTestStore(t)
	client := llm.NewClient(llmSrv.URL())
	proc := session.NewProcessor(reg, client, store, bus.New(), "test-model", "You are a helpful assistant.")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := proc.Run(ctx, "Use unknown tool", "", "")
	if err != nil {
		t.Fatal(err)
	}

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

	store := newTestStore(t)
	client := llm.NewClient(llmSrv.URL())
	proc := session.NewProcessor(reg, client, store, bus.New(), "test-model", "You are a helpful assistant.")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := proc.Run(ctx, "Read go.mod", "", "")
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
	store := newTestStore(t)
	client := llm.NewClient(llmSrv.URL())
	proc := session.NewProcessor(reg, client, store, bus.New(), "test-model", "You are a helpful assistant.")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := proc.Run(ctx, "hang", "", "")
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestProcessorSessionPersistence(t *testing.T) {
	llmSrv := llmtest.NewForTest(t)
	llmSrv.Reply().
		Text("Running a command.").
		Tool("bash", map[string]any{"command": "echo hello"}).
		Item()
	llmSrv.Text("Done!")

	reg := tool.NewRegistry()
	reg.Register(tool.BashTool())

	store := newTestStore(t)
	client := llm.NewClient(llmSrv.URL())
	proc := session.NewProcessor(reg, client, store, bus.New(), "test-model", "You are a helpful assistant.")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sessionID := "test-persist-session"
	result, err := proc.Run(ctx, "Run echo", sessionID, "test-project")
	if err != nil {
		t.Fatal(err)
	}

	if result.SessionID != sessionID {
		t.Errorf("expected SessionID %q, got %q", sessionID, result.SessionID)
	}

	// Verify session was stored
	saved, err := store.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if saved == nil {
		t.Fatal("expected session to be saved in store")
	}
	if saved.ProjectID != "test-project" {
		t.Errorf("expected project 'test-project', got %q", saved.ProjectID)
	}
	if saved.Title == "" {
		t.Error("expected non-empty title")
	}

	// Verify messages were stored
	msgs, err := store.ListMessages(ctx, storage.ListMessagesFilter{SessionID: sessionID, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) < 2 {
		t.Errorf("expected at least 2 messages (user+assistant), got %d", len(msgs))
	}

	hasUser := false
	hasAssistant := false
	for _, m := range msgs {
		if m.Role == "user" {
			hasUser = true
		}
		if m.Role == "assistant" || m.Role == "tool" {
			hasAssistant = true
		}
	}
	if !hasUser {
		t.Error("expected user message")
	}
	if !hasAssistant {
		t.Error("expected assistant/tool messages")
	}
}

func TestProcessorWithExistingStore(t *testing.T) {
	llmSrv := llmtest.NewForTest(t)
	llmSrv.Text("Simple response.")

	reg := tool.NewRegistry()
	store := newTestStore(t)

	// Pre-create a session in the store
	ctx := context.Background()
	_, err := store.CreateSession(ctx, storage.CreateSessionInput{
		ID: "premade-session", ProjectID: "premade",
		Title: "Pre-made Session", Model: "test-model",
	})
	if err != nil {
		t.Fatal(err)
	}

	client := llm.NewClient(llmSrv.URL())
	proc := session.NewProcessor(reg, client, store, bus.New(), "test-model", "You are a helpful assistant.")

	runCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := proc.Run(runCtx, "Test with pre-made session", "premade-session", "premade")
	if err != nil {
		t.Fatal(err)
	}

	if result.SessionID != "premade-session" {
		t.Errorf("expected 'premade-session', got %q", result.SessionID)
	}

	// Verify update worked (title preserved from pre-made)
	saved, _ := store.GetSession(ctx, "premade-session")
	if saved == nil {
		t.Fatal("expected session to exist")
	}
}

func TestProcessorPublishesEvents(t *testing.T) {
	llmSrv := llmtest.NewForTest(t)
	llmSrv.Reply().
		Text("Running tool.").
		Tool("bash", map[string]any{"command": "echo hello"}).
		Item()
	llmSrv.Text("Done!")

	reg := tool.NewRegistry()
	reg.Register(tool.BashTool())

	eventBus := bus.New()
	var sessionEvents, msgEvents, toolEvents, llmEvents, agentEvents int32

	eventBus.Subscribe(bus.TypeSessionCreated, func(e bus.Event) {
		atomic.AddInt32(&sessionEvents, 1)
	})
	eventBus.Subscribe(bus.TypeSessionUpdated, func(e bus.Event) {
		atomic.AddInt32(&sessionEvents, 1)
	})
	eventBus.Subscribe(bus.TypeMessageSent, func(e bus.Event) {
		atomic.AddInt32(&msgEvents, 1)
	})
	eventBus.Subscribe(bus.TypeToolCalled, func(e bus.Event) {
		atomic.AddInt32(&toolEvents, 1)
	})
	eventBus.Subscribe(bus.TypeToolCompleted, func(e bus.Event) {
		atomic.AddInt32(&toolEvents, 1)
	})
	eventBus.Subscribe(bus.TypeLLMStarted, func(e bus.Event) {
		atomic.AddInt32(&llmEvents, 1)
	})
	eventBus.Subscribe(bus.TypeLLMCompleted, func(e bus.Event) {
		atomic.AddInt32(&llmEvents, 1)
	})
	eventBus.Subscribe(bus.TypeAgentStarted, func(e bus.Event) {
		atomic.AddInt32(&agentEvents, 1)
	})
	eventBus.Subscribe(bus.TypeAgentCompleted, func(e bus.Event) {
		atomic.AddInt32(&agentEvents, 1)
	})

	store := newTestStore(t)
	client := llm.NewClient(llmSrv.URL())
	proc := session.NewProcessor(reg, client, store, eventBus, "test-model", "You are a helpful assistant.")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := proc.Run(ctx, "Test events", "", "")
	if err != nil {
		t.Fatal(err)
	}

	if n := atomic.LoadInt32(&sessionEvents); n < 2 {
		t.Errorf("expected >=2 session events (created+updated), got %d", n)
	}
	if n := atomic.LoadInt32(&msgEvents); n < 2 {
		t.Errorf("expected >=2 message events (user+assistant), got %d", n)
	}
	if n := atomic.LoadInt32(&toolEvents); n != 2 {
		t.Errorf("expected 2 tool events (called+completed), got %d", n)
	}
	if n := atomic.LoadInt32(&llmEvents); n < 1 {
		t.Errorf("expected >=1 LLM event, got %d", n)
	}
	if n := atomic.LoadInt32(&agentEvents); n != 2 {
		t.Errorf("expected 2 agent events (started+completed), got %d", n)
	}
}

func stringsContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
