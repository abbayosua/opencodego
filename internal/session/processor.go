package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/opencode-go/opencode/internal/llm"
	"github.com/opencode-go/opencode/internal/message"
	"github.com/opencode-go/opencode/internal/storage"
	"github.com/opencode-go/opencode/internal/tool"
)

type Processor struct {
	tools   *tool.Registry
	llm     *llm.Client
	store   storage.Store
	model   string
	system  string
	maxTurn int
}

func NewProcessor(tools *tool.Registry, llmClient *llm.Client, store storage.Store, model string, system string) *Processor {
	return &Processor{
		tools:   tools,
		llm:     llmClient,
		store:   store,
		model:   model,
		system:  system,
		maxTurn: 25,
	}
}

type RunResult struct {
	SessionID  string
	Messages   []message.Message
	Events     []llm.Event
	ToolCalls  int
	FinalText  string
}

func (p *Processor) Run(ctx context.Context, prompt, sessionID, projectID string) (*RunResult, error) {
	if sessionID == "" {
		sessionID = fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}
	if projectID == "" {
		projectID = "default"
	}

	history := []message.Message{
		message.NewTextMessage(message.RoleUser, prompt),
	}

	toolDefs := p.loadToolDefs()
	var allEvents []llm.Event
	turnCount := 0

	session := p.createSessionRecord(sessionID, projectID, prompt)

	if err := p.saveUserMessage(sessionID, 0, prompt); err != nil {
		return nil, fmt.Errorf("save user message: %w", err)
	}

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

		assistantText := textBuffer.String()

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

		if err := p.saveAssistantMessage(sessionID, turnCount, assistantText, toolCallsInTurn); err != nil {
			return nil, fmt.Errorf("save assistant message: %w", err)
		}

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

			if err := p.saveToolResult(sessionID, turnCount, tc, result); err != nil {
				return nil, fmt.Errorf("save tool result: %w", err)
			}
		}

		if len(toolCallsInTurn) == 0 && stepComplete {
			break
		}
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

	session.Title = truncate(prompt, 80)
	if finalText != "" {
		session.Summary = truncate(finalText, 200)
	}
	if err := p.store.UpdateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("update session: %w", err)
	}

	return &RunResult{
		SessionID: sessionID,
		Messages:  history,
		Events:    allEvents,
		ToolCalls: toolCallCount,
		FinalText: finalText,
	}, nil
}

func (p *Processor) createSessionRecord(id, projectID, prompt string) *storage.Session {
	ctx := context.Background()
	s, err := p.store.CreateSession(ctx, storage.CreateSessionInput{
		ID:        id,
		ProjectID: projectID,
		Title:     truncate(prompt, 80),
		Model:     p.model,
		Agent:     "default",
	})
	if err != nil {
		return &storage.Session{
			ID: id, ProjectID: projectID,
			Title: truncate(prompt, 80), Model: p.model, Agent: "default",
		}
	}
	return s
}

func (p *Processor) saveUserMessage(sessionID string, turn int, prompt string) error {
	msgID := fmt.Sprintf("msg_%s_u%d", sessionID, turn)
	_, err := p.store.CreateMessage(context.Background(), storage.CreateMessageInput{
		ID: msgID, SessionID: sessionID,
		Role:    "user",
		Content: prompt,
	})
	return err
}

func (p *Processor) saveAssistantMessage(sessionID string, turn int, text string, toolCalls []llm.Event) error {
	msgID := fmt.Sprintf("msg_%s_a%d", sessionID, turn)
	content := text

	if len(toolCalls) > 0 {
		tcNames := make([]string, len(toolCalls))
		for i, tc := range toolCalls {
			tcNames[i] = tc.Name
		}
		tcJSON, _ := json.Marshal(tcNames)
		content = text + "\n[TOOL_CALLS: " + string(tcJSON) + "]"
	}

	_, err := p.store.CreateMessage(context.Background(), storage.CreateMessageInput{
		ID: msgID, SessionID: sessionID,
		Role:    "assistant",
		Content: content,
	})
	if err != nil {
		return err
	}

	if text != "" {
		_, err = p.store.CreatePart(context.Background(), storage.CreatePartInput{
			ID: msgID + "_text", MessageID: msgID,
			SessionID: sessionID, Type: "text",
			Content: text,
		})
		if err != nil {
			return err
		}
	}

	for i, tc := range toolCalls {
		inputStr := string(tc.Input)
		if inputStr == "" {
			inputStr = "{}"
		}
		partID := fmt.Sprintf("%s_tc_%s_%d", msgID, tc.Name, i)
		_, err = p.store.CreatePart(context.Background(), storage.CreatePartInput{
			ID: partID, MessageID: msgID,
			SessionID: sessionID, Type: "tool_use",
			Content:  fmt.Sprintf(`{"name":"%s","input":%s}`, tc.Name, inputStr),
			Metadata: `{"status":"executing"}`,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Processor) saveToolResult(sessionID string, turn int, tc llm.Event, result toolExecResult) error {
	msgID := fmt.Sprintf("msg_%s_r%d_%s_%d", sessionID, turn, tc.Name, time.Now().UnixNano())
	status := "completed"
	if result.isError {
		status = "error"
	}
	metadata, _ := json.Marshal(map[string]string{"status": status})

	_, err := p.store.CreateMessage(context.Background(), storage.CreateMessageInput{
		ID: msgID, SessionID: sessionID,
		Role:    "tool",
		Content: result.output,
	})
	if err != nil {
		return err
	}

	_, err = p.store.CreatePart(context.Background(), storage.CreatePartInput{
		ID: msgID + "_part", MessageID: msgID,
		SessionID: sessionID, Type: "tool_result",
		Content:  result.output,
		Metadata: string(metadata),
	})

	return err
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

type toolExecResult struct {
	output  string
	isError bool
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
