package tool

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func BashTool() *Info {
	return Define("bash", func() (*Def, error) {
		return &Def{
			ID:          "bash",
			Description: "Execute a bash command in the shell.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "The bash command to execute",
					},
					"description": map[string]any{
						"type":        "string",
						"description": "A description of what the command does",
					},
					"workdir": map[string]any{
						"type":        "string",
						"description": "The working directory to run the command in",
					},
				},
				"required": []string{"command"},
			},
			Execute: bashExecute,
		}, nil
	})
}

func bashExecute(args map[string]any, ctx Context) (*ExecuteResult, error) {
	cmdStr, _ := args["command"].(string)
	if cmdStr == "" {
		return nil, fmt.Errorf("command is required")
	}

	shell := "sh"
	flag := "-c"
	if runtime.GOOS == "windows" {
		shell = "cmd.exe"
		flag = "/c"
	}

	c := ctx.Context
	if c == nil {
		c = context.Background()
	}

	var cmd *exec.Cmd
	if wd, ok := args["workdir"].(string); ok && wd != "" {
		cmd = exec.CommandContext(c, shell, flag, cmdStr)
		cmd.Dir = wd
	} else {
		cmd = exec.CommandContext(c, shell, flag, cmdStr)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.Run()

	var out strings.Builder
	if stdout.Len() > 0 {
		out.WriteString(stdout.String())
	}
	if stderr.Len() > 0 {
		if out.Len() > 0 {
			out.WriteString("\n")
		}
		out.WriteString(stderr.String())
	}

	title := fmt.Sprintf("Ran: %s", cmdStr)
	if len(cmdStr) > 60 {
		title = fmt.Sprintf("Ran: %s...", cmdStr[:57])
	}

	return &ExecuteResult{
		Title:  title,
		Output: out.String(),
	}, nil
}
