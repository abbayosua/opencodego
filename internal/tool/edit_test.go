package tool

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEditTool(t *testing.T) {
	info := EditTool()
	def, err := info.Init()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.txt")
	original := "The quick brown fox jumps over the lazy dog.\n"
	if err := os.WriteFile(fpath, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("basic replace", func(t *testing.T) {
		result, err := def.Execute(map[string]any{
			"file_path":  fpath,
			"old_string": "brown fox",
			"new_string": "red cat",
		}, Context{})
		if err != nil {
			t.Fatal(err)
		}

		data, _ := os.ReadFile(fpath)
		expected := "The quick red cat jumps over the lazy dog.\n"
		if string(data) != expected {
			t.Errorf("expected %q, got %q", expected, string(data))
		}
		_ = result
	})

	t.Run("not found", func(t *testing.T) {
		_, err := def.Execute(map[string]any{
			"file_path":  fpath,
			"old_string": "nonexistent",
			"new_string": "whatever",
		}, Context{})
		if err == nil {
			t.Error("expected error for non-existent string")
		}
	})
}
