package storage_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/opencode-go/opencode/internal/storage"
)

func TestCreateAndGetSession(t *testing.T) {
	store, err := storage.NewInMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	sess, err := store.CreateSession(ctx, storage.CreateSessionInput{
		ID:        "sess-1",
		ProjectID: "proj-1",
		Directory: "/test/dir",
		Title:     "Test Session",
		Agent:     "default",
		Model:     "gpt-4o",
	})
	if err != nil {
		t.Fatal(err)
	}

	if sess.ID != "sess-1" {
		t.Errorf("expected 'sess-1', got %q", sess.ID)
	}
	if sess.ProjectID != "proj-1" {
		t.Errorf("expected 'proj-1', got %q", sess.ProjectID)
	}
	if sess.Agent != "default" {
		t.Errorf("expected 'default', got %q", sess.Agent)
	}
	if sess.Model != "gpt-4o" {
		t.Errorf("expected 'gpt-4o', got %q", sess.Model)
	}
	if sess.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if sess.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}

	// Get by ID
	got, err := store.GetSession(ctx, "sess-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != sess.ID {
		t.Errorf("expected %q, got %q", sess.ID, got.ID)
	}
}

func TestGetNonExistentSession(t *testing.T) {
	store, err := storage.NewInMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	sess, err := store.GetSession(context.Background(), "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if sess != nil {
		t.Error("expected nil for non-existent session")
	}
}

func TestUpdateSession(t *testing.T) {
	store, err := storage.NewInMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	sess, err := store.CreateSession(ctx, storage.CreateSessionInput{
		ID: "sess-upd", Title: "Original",
	})
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(2 * time.Millisecond)

	sess.Title = "Updated"
	sess.Summary = "Summary text"
	if err := store.UpdateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetSession(ctx, "sess-upd")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Updated" {
		t.Errorf("expected 'Updated', got %q", got.Title)
	}
	if got.Summary != "Summary text" {
		t.Errorf("expected 'Summary text', got %q", got.Summary)
	}
	if !got.UpdatedAt.After(got.CreatedAt) && !got.UpdatedAt.Equal(got.CreatedAt) {
		t.Error("expected UpdatedAt >= CreatedAt")
	}
}

func TestListSessions(t *testing.T) {
	store, err := storage.NewInMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, err := store.CreateSession(ctx, storage.CreateSessionInput{
			ID: fmt.Sprintf("sess-l%d", i), Title: fmt.Sprintf("Session %d", i),
			ProjectID: "proj-list",
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	sessions, err := store.ListSessions(ctx, storage.ListSessionsFilter{
		ProjectID: "proj-list",
		Limit:     10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 5 {
		t.Errorf("expected 5 sessions, got %d", len(sessions))
	}
}

func TestDeleteSession(t *testing.T) {
	store, err := storage.NewInMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	store.CreateSession(ctx, storage.CreateSessionInput{ID: "sess-del"})

	if err := store.DeleteSession(ctx, "sess-del"); err != nil {
		t.Fatal(err)
	}

	sess, _ := store.GetSession(ctx, "sess-del")
	if sess != nil {
		t.Error("expected nil after delete")
	}
}

func TestCreateAndGetMessage(t *testing.T) {
	store, err := storage.NewInMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	store.CreateSession(ctx, storage.CreateSessionInput{ID: "sess-msg"})

	msg, err := store.CreateMessage(ctx, storage.CreateMessageInput{
		ID: "msg-1", SessionID: "sess-msg", Role: "user",
		Content: "Hello!",
	})
	if err != nil {
		t.Fatal(err)
	}

	if msg.Role != "user" {
		t.Errorf("expected 'user', got %q", msg.Role)
	}
	if msg.Content != "Hello!" {
		t.Errorf("expected 'Hello!', got %q", msg.Content)
	}

	got, err := store.GetMessage(ctx, "msg-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "msg-1" {
		t.Errorf("expected 'msg-1', got %q", got.ID)
	}
}

func TestListMessages(t *testing.T) {
	store, err := storage.NewInMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	store.CreateSession(ctx, storage.CreateSessionInput{ID: "sess-list-msg"})

	for i := 0; i < 3; i++ {
		store.CreateMessage(ctx, storage.CreateMessageInput{
			ID: fmt.Sprintf("msg-%d", i), SessionID: "sess-list-msg",
			Role: "user", Content: fmt.Sprintf("msg %d", i),
		})
	}

	messages, err := store.ListMessages(ctx, storage.ListMessagesFilter{
		SessionID: "sess-list-msg", Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(messages))
	}
}

func TestCreateAndGetPart(t *testing.T) {
	store, err := storage.NewInMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	store.CreateSession(ctx, storage.CreateSessionInput{ID: "sess-part"})
	store.CreateMessage(ctx, storage.CreateMessageInput{ID: "msg-part", SessionID: "sess-part", Role: "assistant"})

	part, err := store.CreatePart(ctx, storage.CreatePartInput{
		ID: "part-1", MessageID: "msg-part", SessionID: "sess-part",
		Type: "text", Content: "Hello!", Metadata: `{"key":"val"}`,
	})
	if err != nil {
		t.Fatal(err)
	}

	if part.Type != "text" {
		t.Errorf("expected 'text', got %q", part.Type)
	}
	if part.Content != "Hello!" {
		t.Errorf("expected 'Hello!', got %q", part.Content)
	}
	if part.Metadata != `{"key":"val"}` {
		t.Errorf("expected metadata, got %q", part.Metadata)
	}

	got, err := store.GetPart(ctx, "part-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "part-1" {
		t.Errorf("expected 'part-1', got %q", got.ID)
	}
}

func TestListParts(t *testing.T) {
	store, err := storage.NewInMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	store.CreateSession(ctx, storage.CreateSessionInput{ID: "sess-parts"})
	store.CreateMessage(ctx, storage.CreateMessageInput{ID: "msg-parts", SessionID: "sess-parts", Role: "assistant"})

	for i := 0; i < 3; i++ {
		store.CreatePart(ctx, storage.CreatePartInput{
			ID: fmt.Sprintf("part-%d", i), MessageID: "msg-parts",
			SessionID: "sess-parts", Type: "text", Content: fmt.Sprintf("part %d", i),
		})
	}

	parts, err := store.ListParts(ctx, "msg-parts")
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 3 {
		t.Errorf("expected 3 parts, got %d", len(parts))
	}
}

func TestGetSessionWithMessages(t *testing.T) {
	store, err := storage.NewInMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	store.CreateSession(ctx, storage.CreateSessionInput{ID: "sess-tree", Title: "Tree Test"})
	store.CreateMessage(ctx, storage.CreateMessageInput{ID: "m1", SessionID: "sess-tree", Role: "user", Content: "hi"})
	store.CreateMessage(ctx, storage.CreateMessageInput{ID: "m2", SessionID: "sess-tree", Role: "assistant", Content: "hello!"})

	sess, msgs, err := store.GetSessionWithMessages(ctx, "sess-tree")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Title != "Tree Test" {
		t.Errorf("expected 'Tree Test', got %q", sess.Title)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}
}

func TestCascadeDeleteSessionDeletesMessages(t *testing.T) {
	store, err := storage.NewInMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	store.CreateSession(ctx, storage.CreateSessionInput{ID: "sess-cas"})
	store.CreateMessage(ctx, storage.CreateMessageInput{ID: "m-cas", SessionID: "sess-cas", Role: "user"})
	store.CreatePart(ctx, storage.CreatePartInput{ID: "p-cas", MessageID: "m-cas", SessionID: "sess-cas", Type: "text"})

	store.DeleteSession(ctx, "sess-cas")

	msgs, _ := store.ListMessages(ctx, storage.ListMessagesFilter{SessionID: "sess-cas", Limit: 10})
	if len(msgs) != 0 {
		t.Error("expected messages to be cascade deleted")
	}
}

func TestSessionWithAllRelations(t *testing.T) {
	store, err := storage.NewInMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()

	store.CreateSession(ctx, storage.CreateSessionInput{
		ID: "full-sess", ProjectID: "proj-full",
		Title: "Full Test", Agent: "dev", Model: "gpt-4o",
	})

	store.CreateMessage(ctx, storage.CreateMessageInput{
		ID: "full-msg", SessionID: "full-sess", Role: "user",
		Content: "What files are here?",
	})

	store.CreatePart(ctx, storage.CreatePartInput{
		ID: "full-part-1", MessageID: "full-msg",
		SessionID: "full-sess", Type: "text",
		Content: "What files are here?",
	})

	store.CreatePart(ctx, storage.CreatePartInput{
		ID: "full-part-2", MessageID: "full-msg",
		SessionID: "full-sess", Type: "tool",
		Content: `{"tool":"bash","cmd":"ls"}`,
		Metadata: `{"status":"completed"}`,
	})

	sess, err := store.GetSession(ctx, "full-sess")
	if err != nil {
		t.Fatal(err)
	}
	if sess.ProjectID != "proj-full" || sess.Title != "Full Test" {
		t.Error("session fields mismatch")
	}

	msg, err := store.GetMessage(ctx, "full-msg")
	if err != nil {
		t.Fatal(err)
	}
	if msg.Role != "user" || msg.Content != "What files are here?" {
		t.Error("message fields mismatch")
	}

	parts, err := store.ListParts(ctx, "full-msg")
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}

	if parts[0].Type != "text" {
		t.Errorf("expected first part type 'text', got %q", parts[0].Type)
	}
	if parts[1].Type != "tool" {
		t.Errorf("expected second part type 'tool', got %q", parts[1].Type)
	}
}

func TestMultipleSessions(t *testing.T) {
	store, err := storage.NewInMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()

	for i := 0; i < 3; i++ {
		sid := fmt.Sprintf("multi-sess-%d", i)
		store.CreateSession(ctx, storage.CreateSessionInput{
			ID: sid, ProjectID: "multi", Title: fmt.Sprintf("Session %d", i),
		})
		for j := 0; j < 2; j++ {
			mid := fmt.Sprintf("multi-msg-%d-%d", i, j)
			store.CreateMessage(ctx, storage.CreateMessageInput{
				ID: mid, SessionID: sid, Role: "user",
				Content: fmt.Sprintf("message %d-%d", i, j),
			})
		}
	}

	sessions, _ := store.ListSessions(ctx, storage.ListSessionsFilter{
		ProjectID: "multi", Limit: 10,
	})
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	for _, s := range sessions {
		_, msgs, err := store.GetSessionWithMessages(ctx, s.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(msgs) != 2 {
			t.Errorf("session %s: expected 2 messages, got %d", s.ID, len(msgs))
		}
	}
}

func TestInMemoryStore(t *testing.T) {
	store, err := storage.NewInMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	store.CreateSession(ctx, storage.CreateSessionInput{ID: "mem-sess"})

	sess, err := store.GetSession(ctx, "mem-sess")
	if err != nil {
		t.Fatal(err)
	}
	if sess.ID != "mem-sess" {
		t.Errorf("expected 'mem-sess', got %q", sess.ID)
	}
}


