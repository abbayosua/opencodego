package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/opencode-go/opencode/internal/llm"
	"github.com/opencode-go/opencode/internal/message"
	"github.com/opencode-go/opencode/internal/tool"
)

type Result string

const (
	ResultCompact  Result = "compact"
	ResultStop     Result = "stop"
	ResultContinue Result = "continue"
)

type Handle struct {
	Events   []llm.Event
	Messages []message.Message
}

type Processor struct {
	tools    *tool.Registry
	llm      *llm.Client
	model    string
	system   string
	maxTurn  int
}

func NewProcessor(tools *tool.Registry, llmClient *llm.Client, model string, system string) *Processor {
	return &Processor{
		tools:   tools,
		llm:     llmClient,
		model:   model,
		system:  system,
		maxTurn: 25,
	}
}

type RunResult struct {
	Messages   []message.Message
	Events     []llm.Event
	ToolCalls  int
	FinalText  string
}

func (p *Processor) Run(ctx context.Context, prompt string) (*RunResult, error) {
	history := []message.Message{
		message.NewTextMessage(message.RoleUser, prompt),
	}

	toolDefs := p.loadToolDefs()
	var allEvents []llm.Event
	turnCount := 0

	for turnCount < p.maxTurn {
		turnCount++

		req := llm.StreamRequest{
			Model:    p.model,
			System:   p.system,
			Messages: history,
			Stream:   true,
		}

		if len(toolDefs) > 0 {
			req.Tools = toolDefs
		}

		events, errs := p.llm.Stream(ctx, req)

		var assistantParts []message.Content
		var toolCallsInTurn []llm.Event
		var textBuffer strings.Builder
		stepComplete := false

	loop:
		for {
			select {
			case evt, ok := <-events:
				if !ok {
					break loop
				}
				allEvents = append(allEvents, evt)

				switch evt.Type {
				case llm.EventTextDelta:
					textBuffer.WriteString(evt.Text)

				case llm.EventTextEnd:
					if textBuffer.Len() > 0 {
						assistantParts = append(assistantParts, message.Content{
							Type: message.ContentText,
							Text: textBuffer.String(),
						})
					}

				case llm.EventToolCall:
					toolCallsInTurn = append(toolCallsInTurn, evt)

				case llm.EventFinishStep:
					stepComplete = true

				case llm.EventFinish:
					if evt.FinishReason == "stop" || evt.FinishReason == "end_turn" {
						break loop
					}
				}

			case err, ok := <-errs:
				if ok && err != nil {
					return nil, fmt.Errorf("llm stream error: %w", err)
				}
				break loop

			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// Append assistant message
		if textBuffer.Len() > 0 {
			assistantParts = append(assistantParts, message.Content{
				Type: message.ContentText,
				Text: textBuffer.String(),
			})
		}

		// Add tool call content parts
		for _, tc := range toolCallsInTurn {
			assistantParts = append(assistantParts, message.Content{
				Type:      message.ContentToolUse,
				ToolUseID: tc.ToolCallID,
				ToolName:  tc.Name,
				ToolInput: tc.Input,
			})
		}

		if len(assistantParts) > 0 {
			assistantMsg := message.Message{
				Role:    message.RoleAssistant,
				Content: assistantParts,
			}
			history = append(history, assistantMsg)
		}

		// Execute tool calls and add results
		for _, tc := range toolCallsInTurn {
			result := p.executeTool(ctx, tc)
			toolResultContent := message.Content{
				Type:       message.ContentToolResult,
				ToolUseID:  tc.ToolCallID,
				ToolName:   tc.Name,
				ToolOutput: result.output,
				IsError:    result.isError,
			}

			toolResultMsg := message.Message{
				Role:    message.RoleTool,
				Content: []message.Content{toolResultContent},
			}
			history = append(history, toolResultMsg)
		}

		// If no tool calls and step completed, we're done
		if len(toolCallsInTurn) == 0 && stepComplete {
			break
		}

		// Limit check
		if turnCount >= p.maxTurn {
			break
		}
	}

	var finalText string
	for _, m := range history {
		if m.Role == message.RoleAssistant {
			for _, c := range m.Content {
				if c.Type == message.ContentText {
					finalText += c.Text
				}
			}
		}
	}

	toolCallCount := 0
	for _, e := range allEvents {
		if e.Type == llm.EventToolCall {
			toolCallCount++
		}
	}

	return &RunResult{
		Messages:  history,
		Events:    allEvents,
		ToolCalls: toolCallCount,
		FinalText: finalText,
	}, nil
}

type toolExecResult struct {
	output  string
	isError bool
}

func (p *Processor) executeTool(ctx context.Context, evt llm.Event) toolExecResult {
	toolName := evt.Name

	def, err := p.tools.Get(toolName)
	if err != nil || def == nil {
		return toolExecResult{
			output:  fmt.Sprintf("Unknown tool: %s", toolName),
			isError: true,
		}
	}

	var args map[string]any
	if len(evt.Input) > 0 {
		if err := json.Unmarshal(evt.Input, &args); err != nil {
			return toolExecResult{
				output:  fmt.Sprintf("Invalid arguments: %v", err),
				isError: true,
			}
		}
	}

	toolCtx := tool.Context{
		Context: ctx,
		CallID:  evt.ToolCallID,
	}

	start := time.Now()
	result, err := def.Execute(args, toolCtx)
	elapsed := time.Since(start)

	if err != nil {
		return toolExecResult{
			output:  fmt.Sprintf("Error (%v): %v", elapsed, err),
			isError: true,
		}
	}

	return toolExecResult{
		output:  result.Output,
		isError: false,
	}
}

func (p *Processor) loadToolDefs() []llm.ToolDef {
	defs, err := p.tools.All()
	if err != nil {
		return nil
	}

	var toolDefs []llm.ToolDef
	for _, d := range defs {
		toolDefs = append(toolDefs, llm.ToolDef{
			Name:        d.ID,
			Description: d.Description,
			Parameters:  d.Parameters,
		})
	}
	return toolDefs
}
