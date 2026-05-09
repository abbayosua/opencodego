package llmtest

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestServerText(t *testing.T) {
	s := NewForTest(t)

	s.Text("hello world")

	resp, err := http.Post(s.URL()+"/v1/chat/completions", "application/json",
		strings.NewReader(`{"model":"test","messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var buf strings.Builder
	io.Copy(&buf, resp.Body)
	body := buf.String()

	if !strings.Contains(body, "hello") {
		t.Errorf("expected response to contain 'hello world', got: %s", body)
	}
}

func TestServerTool(t *testing.T) {
	s := NewForTest(t)

	s.Tool("bash", map[string]any{"command": "ls"})

	resp, err := http.Post(s.URL()+"/v1/chat/completions", "application/json",
		strings.NewReader(`{"model":"test","messages":[{"role":"user","content":"run ls"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var buf strings.Builder
	io.Copy(&buf, resp.Body)
	body := buf.String()

	if !strings.Contains(body, "bash") || !strings.Contains(body, "ls") {
		t.Errorf("expected response to contain tool call bash+ls, got: %s", body)
	}
}

func TestServerError(t *testing.T) {
	s := NewForTest(t)

	s.Error(429, map[string]any{"error": "rate limited"})

	resp, err := http.Post(s.URL()+"/v1/chat/completions", "application/json",
		strings.NewReader(`{"model":"test","messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 429 {
		t.Errorf("expected 429, got %d", resp.StatusCode)
	}
}

func TestServerCalls(t *testing.T) {
	s := NewForTest(t)
	s.Text("resp1")
	s.Text("resp2")

	go func() {
		http.Post(s.URL()+"/v1/chat/completions", "application/json",
			strings.NewReader(`{"model":"test","messages":[{"role":"user","content":"msg1"}]}`))
	}()
	go func() {
		http.Post(s.URL()+"/v1/chat/completions", "application/json",
			strings.NewReader(`{"model":"test","messages":[{"role":"user","content":"msg2"}]}`))
	}()

	s.Wait(2)

	if s.Calls() != 2 {
		t.Errorf("expected 2 calls, got %d", s.Calls())
	}

	inputs := s.Inputs()
	if len(inputs) != 2 {
		t.Errorf("expected 2 inputs, got %d", len(inputs))
	}
}

func TestServerReplyBuilder(t *testing.T) {
	s := NewForTest(t)

	s.Reply().Reason("think").Text("answer").Tool("bash", map[string]any{"cmd": "ls"}).Item()

	resp, err := http.Post(s.URL()+"/v1/chat/completions", "application/json",
		strings.NewReader(`{"model":"test","messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var buf strings.Builder
	io.Copy(&buf, resp.Body)
	body := buf.String()

	if !strings.Contains(body, "think") || !strings.Contains(body, "answer") || !strings.Contains(body, "bash") {
		t.Errorf("response missing expected content, got: %s", body)
	}
}

func TestServerReadmeExample(t *testing.T) {
	s := NewForTest(t)
	s.Reply().Reason("thinking...").Text("Hello!").Tool("bash", map[string]any{"cmd": "ls"}).Item()

	data, _ := json.Marshal(map[string]any{
		"model": "test",
		"messages": []map[string]any{
			{"role": "user", "content": "list files"},
		},
	})

	resp, err := http.Post(s.URL()+"/v1/chat/completions", "application/json", strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result strings.Builder
	io.Copy(&result, resp.Body)
	out := result.String()

	if !strings.Contains(out, "thinking...") {
		t.Error("expected reasoning in response")
	}
	if !strings.Contains(out, "Hello!") {
		t.Error("expected text in response")
	}
	if !strings.Contains(out, "bash") {
		t.Error("expected tool call in response")
	}
}
