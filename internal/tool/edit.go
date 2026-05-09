// EditTool provides file editing capabilities with search-and-replace semantics.
// This mirrors the original opencode apply_patch tool.
package tool

import (
	"fmt"
	"os"
	"strings"
)

func EditTool() *Info {
	return Define("edit", func() (*Def, error) {
		return &Def{
			ID:          "edit",
			Description: "Edit a file by replacing exact text matches. Use this to make targeted changes to files.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{
						"type":        "string",
						"description": "The path to the file to edit",
					},
					"old_string": map[string]any{
						"type":        "string",
						"description": "The exact text to find and replace (must match exactly)",
					},
					"new_string": map[string]any{
						"type":        "string",
						"description": "The new text to replace with",
					},
				},
				"required": []string{"file_path", "old_string", "new_string"},
			},
			Execute: editExecute,
		}, nil
	})
}

func editExecute(args map[string]any, ctx Context) (*ExecuteResult, error) {
	filePath, _ := args["file_path"].(string)
	if filePath == "" {
		return nil, fmt.Errorf("file_path is required")
	}
	oldStr, _ := args["old_string"].(string)
	if oldStr == "" {
		return nil, fmt.Errorf("old_string is required")
	}
	newStr, _ := args["new_string"].(string)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	content := string(data)

	idx := strings.Index(content, oldStr)
	if idx < 0 {
		return nil, fmt.Errorf("old_string not found in file content")
	}

	// Check for multiple matches
	secondIdx := strings.Index(content[idx+len(oldStr):], oldStr)
	if secondIdx >= 0 {
		return nil, fmt.Errorf("found multiple matches for old_string. Provide more surrounding context to identify the correct match")
	}

	modified := strings.Replace(content, oldStr, newStr, 1)

	if err := os.WriteFile(filePath, []byte(modified), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return &ExecuteResult{
		Title:  fmt.Sprintf("Edited %s", filePath),
		Output: fmt.Sprintf("Successfully replaced text in %s", filePath),
	}, nil
}
