// End-to-end test for the opencode CLI `run` command.
// Starts a mock LLM server, builds the binary, runs it, and verifies output.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-go/opencode/internal/llmtest"
)

func TestCLIRunBasicText(t *testing.T) {
	llmSrv := llmtest.NewForTest(t)
	llmSrv.Text("Hello from the AI! I'm ready to help.")

	bin := buildBinary(t)

	cmd := exec.Command(bin, "run", "--model", "test-model",
		"--api-url", llmSrv.URL(),
		"--api-key", "test-key",
		"--max-turns", "3",
		"say hello")
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("run failed: %v\nstderr: %s", err, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "Hello") {
		t.Errorf("expected stdout to contain 'Hello', got: %s", out)
	}

	stderrOut := stderr.String()
	if !strings.Contains(stderrOut, "Done in") {
		t.Errorf("expected stderr to contain 'Done in', got: %s", stderrOut)
	}
}

func TestCLIRunWithToolCall(t *testing.T) {
	llmSrv := llmtest.NewForTest(t)
	llmSrv.Reply().
		Text("Let me check the directory.").
		Tool("bash", map[string]any{"command": "echo hello world"}).
		Item()
	llmSrv.Text("Done! The command ran successfully.")

	bin := buildBinary(t)

	cmd := exec.Command(bin, "run", "--model", "test-model",
		"--api-url", llmSrv.URL(),
		"--api-key", "test-key",
		"--max-turns", "5",
		"run echo command")
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("run failed: %v\nstderr: %s", err, stderr.String())
	}

	stderrOut := stderr.String()
	if !strings.Contains(stderrOut, "Tool calls: 1") {
		t.Errorf("expected stderr to contain 'Tool calls: 1', got: %s", stderrOut)
	}
}

func TestCLIRunNoPrompt(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "run", "--model", "test-model",
		"--api-key", "test-key")
	cmd.Env = os.Environ()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}

	if !strings.Contains(stderr.String(), "prompt") {
		t.Errorf("expected error about prompt, got: %s", stderr.String())
	}
}

func TestCLIRunNoAPIKey(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "run", "--model", "test-model",
		"--api-url", "http://localhost:9999",
		"say hi")
	cmd.Env = removeEnv(os.Environ(), "OPENCODE_API_KEY")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error for missing API key")
	}

	if !strings.Contains(stderr.String(), "API_KEY") {
		t.Errorf("expected error about API_KEY, got: %s", stderr.String())
	}
}

func TestCLIRunWithMultipleTools(t *testing.T) {
	llmSrv := llmtest.NewForTest(t)
	llmSrv.Reply().
		Text("Running two commands.").
		Tool("bash", map[string]any{"command": "echo first"}).
		Tool("bash", map[string]any{"command": "echo second"}).
		Item()
	llmSrv.Text("Both done!")

	bin := buildBinary(t)

	cmd := exec.Command(bin, "run", "--model", "test-model",
		"--api-url", llmSrv.URL(),
		"--api-key", "test-key",
		"--max-turns", "5",
		"run two commands")
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("run failed: %v\nstderr: %s", err, stderr.String())
	}

	stderrOut := stderr.String()
	if !strings.Contains(stderrOut, "Tool calls: 2") {
		t.Errorf("expected 'Tool calls: 2', got: %s", stderrOut)
	}
}

func TestCLIRunReadsFile(t *testing.T) {
	llmSrv := llmtest.NewForTest(t)
	llmSrv.Reply().
		Text("Reading go.mod.").
		Tool("read", map[string]any{"file_path": "go.mod"}).
		Item()
	llmSrv.Text("Found the module file!")

	bin := buildBinary(t)

	// Find project root (where go.mod is)
	root := findProjectRoot(t)

	cmd := exec.Command(bin, "run", "--model", "test-model",
		"--api-url", llmSrv.URL(),
		"--api-key", "test-key",
		"--max-turns", "5",
		"read go.mod")
	cmd.Dir = root
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("run failed: %v\nstderr: %s", err, stderr.String())
	}

	stderrOut := stderr.String()
	if !strings.Contains(stderrOut, "Tool calls: 1") {
		t.Errorf("expected 'Tool calls: 1', got: %s", stderrOut)
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	root := findProjectRoot(t)
	bin := filepath.Join(t.TempDir(), "opencode-test.exe")

	cmd := exec.Command("go", "build", "-o", bin, "./cmd/opencode")
	cmd.Dir = root
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\noutput: %s", err, string(out))
	}

	return bin
}

func findProjectRoot(t *testing.T) string {
	t.Helper()
	// Start from test file location and walk up
	// During `go test`, the working dir is the package dir
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("project root (go.mod) not found")
		}
		wd = parent
	}
}

func removeEnv(env []string, key string) []string {
	var filtered []string
	prefix := key + "="
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
