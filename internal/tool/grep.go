package tool

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func GrepTool() *Info {
	return Define("grep", func() (*Def, error) {
		return &Def{
			ID:          "grep",
			Description: "Search file contents for a regex pattern. Returns matching lines with line numbers.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{
						"type":        "string",
						"description": "The regex pattern to search for",
					},
					"include": map[string]any{
						"type":        "string",
						"description": "File glob pattern to include (e.g. *.go, *.ts)",
					},
					"path": map[string]any{
						"type":        "string",
						"description": "The directory to search in",
					},
				},
				"required": []string{"pattern"},
			},
			Execute: grepExecute,
		}, nil
	})
}

func grepExecute(args map[string]any, ctx Context) (*ExecuteResult, error) {
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	searchPath, _ := args["path"].(string)
	if searchPath == "" {
		searchPath = "."
	}

	include, _ := args["include"].(string)

	var out strings.Builder
	err = filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if include != "" {
			match, _ := filepath.Match(include, filepath.Base(path))
			if !match {
				return nil
			}
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				out.WriteString(fmt.Sprintf("%s:%d: %s\n", path, lineNum, line))
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return &ExecuteResult{
		Title:  fmt.Sprintf(`Grep "%s"`, pattern),
		Output: out.String(),
	}, nil
}
