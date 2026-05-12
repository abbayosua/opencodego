package tool_test

import (
	"strings"
	"testing"

	"github.com/opencode-go/opencode/internal/tool"
)

func TestTaskToolDefinition(t *testing.T) {
	info := tool.TaskTool()
	def, err := info.Init()
	if err != nil {
		t.Fatal(err)
	}

	if def.ID != "task" {
		t.Errorf("expected 'task', got %q", def.ID)
	}

	params := def.Parameters
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties")
	}

	for _, required := range []string{"description", "prompt", "subagent_type"} {
		if _, ok := props[required]; !ok {
			t.Errorf("expected required param %q", required)
		}
	}
}

func TestTaskToolWithoutRunner(t *testing.T) {
	info := tool.TaskTool()
	def, _ := info.Init()

	// Save and restore runner
	oldRunner := tool.SubAgentRunner
	tool.SubAgentRunner = nil
	defer func() { tool.SubAgentRunner = oldRunner }()

	result, err := def.Execute(map[string]any{
		"description":   "list files",
		"prompt":        "list all go files",
		"subagent_type": "explore",
	}, tool.Context{})
	if err != nil {
		t.Fatal(err)
	}

	if result.Title != "Task: list files" {
		t.Errorf("expected title 'Task: list files', got %q", result.Title)
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}
}

func TestTaskToolWithRunner(t *testing.T) {
	info := tool.TaskTool()
	def, _ := info.Init()

	// Set mock runner
	oldRunner := tool.SubAgentRunner
	tool.SubAgentRunner = func(subagentType, prompt, parentSessionID string) (string, error) {
		if subagentType != "explore" {
			t.Errorf("expected 'explore', got %q", subagentType)
		}
		return "Found 3 files: main.go, handler.go, util.go", nil
	}
	defer func() { tool.SubAgentRunner = oldRunner }()

	result, err := def.Execute(map[string]any{
		"description":   "search files",
		"prompt":        "find all go files",
		"subagent_type": "explore",
	}, tool.Context{SessionID: "parent-123"})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.Output, "Found 3 files") {
		t.Errorf("expected mock result, got: %s", result.Output)
	}
	if result.Metadata["subagent_type"] != "explore" {
		t.Errorf("expected subagent_type in metadata")
	}
}

func TestTaskToolMissingArgs(t *testing.T) {
	info := tool.TaskTool()
	def, _ := info.Init()

	_, err := def.Execute(map[string]any{}, tool.Context{})
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}

func TestTaskToolResume(t *testing.T) {
	info := tool.TaskTool()
	def, _ := info.Init()

	oldRunner := tool.SubAgentRunner
	tool.SubAgentRunner = func(subagentType, prompt, parentSessionID string) (string, error) {
		return "Resumed result", nil
	}
	defer func() { tool.SubAgentRunner = oldRunner }()

	result, err := def.Execute(map[string]any{
		"description":   "continue search",
		"prompt":        "continue finding files",
		"subagent_type": "general",
		"task_id":       "task-abc-123",
	}, tool.Context{SessionID: "parent-456"})
	if err != nil {
		t.Fatal(err)
	}

	if result.Metadata["task_id"] != "task-abc-123" {
		t.Errorf("expected task_id in metadata, got: %v", result.Metadata["task_id"])
	}
}


