package tool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteTool(t *testing.T) {
	info := WriteTool()
	def, err := info.Init()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	fpath := filepath.Join(dir, "subdir", "test.txt")

	result, err := def.Execute(map[string]any{
		"file_path": fpath,
		"content":   "hello world",
	}, Context{})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.Output, "Successfully") {
		t.Errorf("unexpected output: %s", result.Output)
	}

	data, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got: %s", string(data))
	}
}
