package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func GlobTool() *Info {
	return Define("glob", func() (*Def, error) {
		return &Def{
			ID:          "glob",
			Description: "Find files matching a glob pattern. Returns file paths sorted by modification time.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{
						"type":        "string",
						"description": "The glob pattern to match (e.g. **/*.go)",
					},
					"path": map[string]any{
						"type":        "string",
						"description": "The directory to search in",
					},
				},
				"required": []string{"pattern"},
			},
			Execute: globExecute,
		}, nil
	})
}

func globExecute(args map[string]any, ctx Context) (*ExecuteResult, error) {
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}

	searchPath, _ := args["path"].(string)
	if searchPath == "" {
		searchPath = "."
	}

	fullPattern := filepath.Join(searchPath, pattern)

	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid glob pattern: %w", err)
	}

	if len(matches) == 0 {
		return &ExecuteResult{
			Title:  fmt.Sprintf("Glob: %s (no matches)", pattern),
			Output: "No files found matching pattern.",
		}, nil
	}

	type fileInfo struct {
		path string
		mod  int64
	}
	var files []fileInfo
	for _, m := range matches {
		info, err := os.Stat(m)
		if err == nil {
			files = append(files, fileInfo{m, info.ModTime().Unix()})
		}
	}

	var out strings.Builder
	for _, f := range files {
		out.WriteString(f.path + "\n")
	}

	title := fmt.Sprintf("Glob: %s (%d matches)", pattern, len(files))
	if len(files) == 1 {
		title = fmt.Sprintf("Glob: %s (1 match)", pattern)
	}

	return &ExecuteResult{
		Title:  title,
		Output: out.String(),
	}, nil
}
