package tool_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-go/opencode/internal/tool"
)

func TestReadTool(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	info := tool.ReadTool()
	def, err := info.Init()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("read full file", func(t *testing.T) {
		result, err := def.Execute(map[string]any{"file_path": fpath}, tool.Context{})
		if err != nil {
			t.Fatal(err)
		}
		want := "1: line1\n2: line2\n3: line3\n4: line4\n5: line5"
		if result.Output != want {
			t.Errorf("mismatch\nwant: %q\ngot:  %q", want, result.Output)
		}
	})

	t.Run("read with offset", func(t *testing.T) {
		result, err := def.Execute(map[string]any{
			"file_path": fpath,
			"offset":    float64(2),
		}, tool.Context{})
		if err != nil {
			t.Fatal(err)
		}
		want := "3: line3\n4: line4\n5: line5"
		if result.Output != want {
			t.Errorf("mismatch\nwant: %q\ngot:  %q", want, result.Output)
		}
	})

	t.Run("read with offset and limit", func(t *testing.T) {
		result, err := def.Execute(map[string]any{
			"file_path": fpath,
			"offset":    float64(1),
			"limit":     float64(2),
		}, tool.Context{})
		if err != nil {
			t.Fatal(err)
		}
		want := "2: line2\n3: line3"
		if result.Output != want {
			t.Errorf("mismatch\nwant: %q\ngot:  %q", want, result.Output)
		}
	})
}

func TestBashTool(t *testing.T) {
	info := tool.BashTool()
	def, err := info.Init()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("echo command", func(t *testing.T) {
		result, err := def.Execute(map[string]any{"command": "echo hello"}, tool.Context{})
		if err != nil {
			t.Fatal(err)
		}
		out := strings.TrimSpace(result.Output)
		if out != "hello" {
			t.Errorf("expected 'hello', got: %s", out)
		}
	})

	t.Run("command with working directory", func(t *testing.T) {
		dir := t.TempDir()
		result, err := def.Execute(map[string]any{
			"command": "echo hello",
			"workdir": dir,
		}, tool.Context{})
		if err != nil {
			t.Fatal(err)
		}
		out := strings.TrimSpace(result.Output)
		if out != "hello" {
			t.Errorf("expected 'hello', got: %s", out)
		}
	})
}

func TestGrepTool(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.ts"), []byte("const x: number = 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	info := tool.GrepTool()
	def, err := info.Init()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("grep all files", func(t *testing.T) {
		result, err := def.Execute(map[string]any{
			"pattern": "func",
			"path":    dir,
		}, tool.Context{})
		if err != nil {
			t.Fatal(err)
		}
		if result.Output == "" {
			t.Error("expected matches, got empty")
		}
	})

	t.Run("grep with include", func(t *testing.T) {
		result, err := def.Execute(map[string]any{
			"pattern": "const",
			"include": "*.ts",
			"path":    dir,
		}, tool.Context{})
		if err != nil {
			t.Fatal(err)
		}
		if result.Output == "" {
			t.Error("expected matches in .ts, got empty")
		}
	})
}

func TestGlobTool(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.go", "b.go", "c.ts"} {
		os.WriteFile(filepath.Join(dir, name), []byte(""), 0644)
	}

	info := tool.GlobTool()
	def, err := info.Init()
	if err != nil {
		t.Fatal(err)
	}

	result, err := def.Execute(map[string]any{
		"pattern": "*.go",
		"path":    dir,
	}, tool.Context{})
	if err != nil {
		t.Fatal(err)
	}

	if result.Output == "" || !strings.Contains(result.Output, "a.go") {
		t.Errorf("expected glob matches containing a.go, got: %s", result.Output)
	}
}

func TestRegistry(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(tool.ReadTool())
	reg.Register(tool.BashTool())

	list := reg.List()
	if len(list) != 2 {
		t.Errorf("expected 2 tools, got %d", len(list))
	}

	defs, err := reg.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 2 {
		t.Errorf("expected 2 defs, got %d", len(defs))
	}
}
