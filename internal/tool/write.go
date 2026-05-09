package tool

import (
	"fmt"
	"os"
	"path/filepath"
)

func WriteTool() *Info {
	return Define("write", func() (*Def, error) {
		return &Def{
			ID:          "write",
			Description: "Write content to a file. Creates the file and any necessary parent directories.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{
						"type":        "string",
						"description": "The path to the file to write",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "The content to write to the file",
					},
				},
				"required": []string{"file_path", "content"},
			},
			Execute: writeExecute,
		}, nil
	})
}

func writeExecute(args map[string]any, ctx Context) (*ExecuteResult, error) {
	filePath, _ := args["file_path"].(string)
	if filePath == "" {
		return nil, fmt.Errorf("file_path is required")
	}

	content, _ := args["content"].(string)

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return &ExecuteResult{
		Title:  fmt.Sprintf("Wrote %s", filepath.Base(filePath)),
		Output: fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), filePath),
	}, nil
}
