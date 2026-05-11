package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/opencode-go/opencode/internal/storage"
)

var (
	buildOnce  sync.Once
	binaryPath string
	binaryDir  string
)

func testBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		var err error
		binaryDir, err = os.MkdirTemp("", "opencode-bin-*")
		if err != nil {
			t.Fatalf("mkdir temp: %v", err)
		}
		root := findProjectRoot(t)
		bin := filepath.Join(binaryDir, "opencode-test.exe")
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/opencode")
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("build failed: %v\n%s", err, string(out))
		}
		binaryPath = bin
	})
	return binaryPath
}

func openTestDB(t *testing.T) (string, storage.Store) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "opencode-test.db")
	store, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return dbPath, store
}

func TestSessionListEmpty(t *testing.T) {
	bin := testBinary(t)
	dbPath, _ := openTestDB(t)

	cmd := exec.Command(bin, "session", "list")
	cmd.Env = append(os.Environ(), "OPENCODE_DB_PATH="+dbPath)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("list failed: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "No sessions") {
		t.Errorf("expected 'No sessions', got: %s", stdout.String())
	}
}

func TestSessionListWithData(t *testing.T) {
	bin := testBinary(t)
	dbPath, store := openTestDB(t)

	for i := 0; i < 3; i++ {
		store.CreateSession(context.Background(), storage.CreateSessionInput{
			ID: fmt.Sprintf("sess-test-%d", i),
			Title: fmt.Sprintf("Test Session %d", i),
			Model: "gpt-4o", Agent: "default",
		})
		store.CreateMessage(context.Background(), storage.CreateMessageInput{
			ID: fmt.Sprintf("msg-%d-1", i), SessionID: fmt.Sprintf("sess-test-%d", i),
			Role: "user", Content: "hello",
		})
	}

	cmd := exec.Command(bin, "session", "list")
	cmd.Env = append(os.Environ(), "OPENCODE_DB_PATH="+dbPath)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "sess-test-0") {
		t.Errorf("expected session ID, got: %s", out)
	}
}

func TestSessionReadExisting(t *testing.T) {
	bin := testBinary(t)
	dbPath, store := openTestDB(t)

	store.CreateSession(context.Background(), storage.CreateSessionInput{
		ID: "sess-read-1", Title: "Read Test",
		Model: "claude-3", Agent: "dev",
	})
	store.CreateMessage(context.Background(), storage.CreateMessageInput{
		ID: "msg-r-1", SessionID: "sess-read-1",
		Role: "user", Content: "What files?",
	})
	store.CreateMessage(context.Background(), storage.CreateMessageInput{
		ID: "msg-r-2", SessionID: "sess-read-1",
		Role: "assistant", Content: "Found files.",
	})

	cmd := exec.Command(bin, "session", "read", "sess-read-1")
	cmd.Env = append(os.Environ(), "OPENCODE_DB_PATH="+dbPath)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("read failed: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Read Test") {
		t.Errorf("expected title, got: %s", out)
	}
	if !strings.Contains(out, "[user]") {
		t.Errorf("expected user role, got: %s", out)
	}
}

func TestSessionReadNotFound(t *testing.T) {
	bin := testBinary(t)
	dbPath, store := openTestDB(t)
	store.CreateSession(context.Background(), storage.CreateSessionInput{ID: "other-sess"})

	cmd := exec.Command(bin, "session", "read", "nonexistent")
	cmd.Env = append(os.Environ(), "OPENCODE_DB_PATH="+dbPath)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Errorf("expected 'not found', got: %s", stderr.String())
	}
}

func TestSessionSubcommandHelp(t *testing.T) {
	bin := testBinary(t)

	cmd := exec.Command(bin, "session")
	cmd.Env = os.Environ()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), "list") || !strings.Contains(stderr.String(), "read") {
		t.Errorf("expected usage, got: %s", stderr.String())
	}
}

func TestSessionReadShowsTimestamps(t *testing.T) {
	bin := testBinary(t)
	dbPath, store := openTestDB(t)

	store.CreateSession(context.Background(), storage.CreateSessionInput{
		ID: "sess-time-1", Title: "Time Test", Model: "test",
	})
	store.Close()

	time.Sleep(1 * time.Millisecond)

	cmd := exec.Command(bin, "session", "read", "sess-time-1")
	cmd.Env = append(os.Environ(), "OPENCODE_DB_PATH="+dbPath)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Created:") {
		t.Errorf("expected Created timestamp, got: %s", stdout.String())
	}
}
