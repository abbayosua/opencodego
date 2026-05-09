package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ReadTool() *Info {
	return Define("read", func() (*Def, error) {
		return &Def{
			ID:          "read",
			Description: "Read the contents of a file. Use this when you need to view the contents of a file.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{
						"type":        "string",
						"description": "The path to the file to read",
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "The line number to start reading from",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "The number of lines to read",
					},
				},
				"required": []string{"file_path"},
			},
			Execute: readExecute,
		}, nil
	})
}

func readExecute(args map[string]any, ctx Context) (*ExecuteResult, error) {
	filePath, _ := args["file_path"].(string)
	if filePath == "" {
		return nil, fmt.Errorf("file_path is required")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	content := strings.TrimRight(string(data), "\n")
	lines := strings.Split(content, "\n")

	offset, _ := args["offset"].(float64)
	limit, _ := args["limit"].(float64)

	start := int(offset)
	if start < 0 {
		start = 0
	}
	if start > len(lines) {
		start = len(lines)
	}

	end := len(lines)
	if limit > 0 {
		end = start + int(limit)
		if end > len(lines) {
			end = len(lines)
		}
	}

	var out strings.Builder
	for i := start; i < end; i++ {
		if i > start {
			out.WriteString("\n")
		}
		out.WriteString(fmt.Sprintf("%d: %s", i+1, lines[i]))
	}

	return &ExecuteResult{
		Title:  fmt.Sprintf("Read %s", filepath.Base(filePath)),
		Output: out.String(),
	}, nil
}
