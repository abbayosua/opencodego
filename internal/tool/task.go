package tool

import (
	"encoding/json"
	"fmt"
)

// SubAgentRunner is a function that runs a sub-agent session.
// It takes a subagent type name and prompt, and returns the result text.
// This is set by the processor so the task tool can create sub-sessions.
var SubAgentRunner func(subagentType, prompt, parentSessionID string) (string, error)

func TaskTool() *Info {
	return Define("task", func() (*Def, error) {
		return &Def{
			ID:          "task",
			Description: "Launch a sub-agent to work on a subtask. Use this for complex tasks that can be parallelized or delegated.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"description": map[string]any{
						"type":        "string",
						"description": "A short 3-5 word description of the task",
					},
					"prompt": map[string]any{
						"type":        "string",
						"description": "The full task prompt for the sub-agent",
					},
					"subagent_type": map[string]any{
						"type":        "string",
						"description": "The agent type to use (e.g. general, explore)",
						"enum":        []string{"general", "explore"},
					},
					"task_id": map[string]any{
						"type":        "string",
						"description": "Optional task ID to resume an existing sub-task",
					},
				},
				"required": []string{"description", "prompt", "subagent_type"},
			},
			Execute: taskExecute,
		}, nil
	})
}

func taskExecute(args map[string]any, ctx Context) (*ExecuteResult, error) {
	desc, _ := args["description"].(string)
	prompt, _ := args["prompt"].(string)
	subagentType, _ := args["subagent_type"].(string)
	taskID, _ := args["task_id"].(string)

	if desc == "" {
		return nil, fmt.Errorf("description is required")
	}
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	if subagentType == "" {
		return nil, fmt.Errorf("subagent_type is required")
	}

	if SubAgentRunner == nil {
		return &ExecuteResult{
			Title:  fmt.Sprintf("Task: %s", desc),
			Output: "Sub-agent runner not configured. Cannot delegate task.",
		}, nil
	}

	finalPrompt := prompt
	if taskID != "" {
		finalPrompt = fmt.Sprintf("[Resuming task %s]\n\n%s", taskID, prompt)
	}

	result, err := SubAgentRunner(subagentType, finalPrompt, ctx.SessionID)
	if err != nil {
		return &ExecuteResult{
			Title: fmt.Sprintf("Task failed: %s", desc),
			Output: fmt.Sprintf("Error running sub-agent '%s': %v", subagentType, err),
			Metadata: map[string]any{
				"error":   err.Error(),
				"task_id": taskID,
			},
		}, nil
	}

	metadata := map[string]any{
		"subagent_type": subagentType,
		"task_id":       taskID,
	}

	metadataJSON, _ := json.Marshal(metadata)

	return &ExecuteResult{
		Title:  fmt.Sprintf("Task: %s", desc),
		Output: result,
		Metadata: map[string]any{
			"subagent_type": subagentType,
			"task_id":       taskID,
			"metadata_json": string(metadataJSON),
		},
	}, nil
}
