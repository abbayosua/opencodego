package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/opencode-go/opencode/internal/tool"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: opencode <command> [args...]\n")
		os.Exit(1)
	}

	reg := tool.NewRegistry()
	reg.Register(tool.ReadTool())
	reg.Register(tool.WriteTool())
	reg.Register(tool.EditTool())
	reg.Register(tool.BashTool())
	reg.Register(tool.GrepTool())
	reg.Register(tool.GlobTool())

	ctx := context.Background()
	toolCtx := tool.Context{
		Context: ctx,
	}

	switch os.Args[1] {
	case "run":
		prompt := "No prompt provided"
		if len(os.Args) > 2 {
			prompt = os.Args[2]
		}
		fmt.Fprintf(os.Stderr, "Prompt: %s\n\n", prompt)

		defs, err := reg.All()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading tools: %v\n", err)
			os.Exit(1)
		}
		data, _ := json.MarshalIndent(defs, "", "  ")
		fmt.Println(string(data))

	case "tools":
		for _, id := range reg.List() {
			def, err := reg.Get(id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				continue
			}
			if def != nil {
				fmt.Printf("- %s: %s\n", def.ID, def.Description)
			}
		}

	case "execute":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: opencode execute <tool> <json-args>\n")
			os.Exit(1)
		}
		toolName := os.Args[2]
		jsonArgs := os.Args[3]

		parsed, err := decodeArg(jsonArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid JSON args: %v\n", err)
			os.Exit(1)
		}

		def, err := reg.Get(toolName)
		if err != nil || def == nil {
			fmt.Fprintf(os.Stderr, "Tool not found: %s\n", toolName)
			os.Exit(1)
		}

		result, err := def.Execute(parsed, toolCtx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func decodeArg(raw string) (map[string]any, error) {
	// On Windows, cmd.exe and PowerShell may strip outer quotes.
	// Try to parse as-is first, then fall back to quote-stripped.
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err == nil {
		return args, nil
	}
	if len(raw) > 1 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		inner := raw[1 : len(raw)-1]
		inner = strings.ReplaceAll(inner, `""`, `"`)
		if err := json.Unmarshal([]byte(inner), &args); err == nil {
			return args, nil
		}
	}
	return nil, fmt.Errorf("cannot parse JSON: %s", raw)
}
