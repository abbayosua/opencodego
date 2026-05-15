package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/opencode-go/opencode/internal/bus"
	"github.com/opencode-go/opencode/internal/llm"
	"github.com/opencode-go/opencode/internal/message"
	"github.com/opencode-go/opencode/internal/storage"
	"github.com/opencode-go/opencode/internal/tool"
)

type Processor struct {
	tools   *tool.Registry
	llm     *llm.Client
	store   storage.Store
	bus     *bus.Bus
	model   string
	system  string
	maxTurn int
}

func NewProcessor(tools *tool.Registry, llmClient *llm.Client, store storage.Store, eventBus *bus.Bus, model string, system string) *Processor {
	p := &Processor{
		tools:   tools,
		llm:     llmClient,
		store:   store,
		bus:     eventBus,
		model:   model,
		system:  system,
		maxTurn: 25,
	}
	return p
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
	var allLLMEvents []llm.Event
	turnCount := 0

	session := p.createSessionRecord(sessionID, projectID, prompt)
	p.publishAgentStarted("default", sessionID)
	p.publishSessionCreated(sessionID, session.Title, session.Model, "default")

	if err := p.saveUserMessage(sessionID, 0, prompt); err != nil {
		return nil, fmt.Errorf("save user message: %w", err)
	}
	p.publishMessageSent(sessionID, "user", prompt)

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

		llmStartTime := time.Now()
		p.publishLLMStarted(p.model, prompt)

		events, errs := p.llm.Stream(ctx, req)

		var assistantParts []message.Content
		var toolCallsInTurn []llm.Event
		var textBuffer strings.Builder
		var reasoningBuffer strings.Builder
		stepComplete := false

	loop:
		for {
			select {
			case evt, ok := <-events:
				if !ok {
					break loop
				}
				allLLMEvents = append(allLLMEvents, evt)

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

				case llm.EventReasoningDelta:
					reasoningBuffer.WriteString(evt.Text)

				case llm.EventReasoningEnd:
					if reasoningBuffer.Len() > 0 {
						assistantParts = append(assistantParts, message.Content{
							Type:      message.ContentReasoning,
							Reasoning: reasoningBuffer.String(),
						})
						reasoningBuffer.Reset()
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
					p.publishLLMError(p.model, err.Error())
					return nil, fmt.Errorf("llm stream error: %w", err)
				}
				break loop

			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		assistantText := textBuffer.String()
		p.publishLLMCompleted(p.model, assistantText, 0, 0, time.Since(llmStartTime).Milliseconds())

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
		p.publishMessageSent(sessionID, "assistant", assistantText)

		for _, tc := range toolCallsInTurn {
			inputStr := string(tc.Input)
			p.publishToolCalled(sessionID, tc.Name, inputStr)

			toolStart := time.Now()
			result := p.executeTool(ctx, tc)
			durationMs := time.Since(toolStart).Milliseconds()

			if result.isError {
				p.publishToolFailed(sessionID, tc.Name, result.output, durationMs)
			} else {
				p.publishToolCompleted(sessionID, tc.Name, result.output, durationMs)
			}

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
	for _, e := range allLLMEvents {
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
	p.publishSessionUpdated(sessionID, session.Title)

	p.publishAgentCompleted("default", sessionID)

	return &RunResult{
		SessionID: sessionID,
		Messages:  history,
		Events:    allLLMEvents,
		ToolCalls: toolCallCount,
		FinalText: finalText,
	}, nil
}

func (p *Processor) publishSessionCreated(id, title, model, agent string) {
	if p.bus == nil {
		return
	}
	p.bus.Publish(bus.NewSessionCreated(id, title, model, agent))
}

func (p *Processor) publishSessionUpdated(id, title string) {
	if p.bus == nil {
		return
	}
	p.bus.Publish(bus.NewSessionUpdated(id, title))
}

func (p *Processor) publishMessageSent(sessionID, role, content string) {
	if p.bus == nil {
		return
	}
	if len(content) > 200 {
		content = content[:200]
	}
	p.bus.Publish(bus.NewMessageSent(sessionID, role, content))
}

func (p *Processor) publishToolCalled(sessionID, toolName, input string) {
	if p.bus == nil {
		return
	}
	p.bus.Publish(bus.NewToolCalled(sessionID, toolName, input))
}

func (p *Processor) publishToolCompleted(sessionID, toolName, output string, durationMs int64) {
	if p.bus == nil {
		return
	}
	p.bus.Publish(bus.NewToolCompleted(sessionID, toolName, output, durationMs))
}

func (p *Processor) publishToolFailed(sessionID, toolName, err string, durationMs int64) {
	if p.bus == nil {
		return
	}
	p.bus.Publish(bus.NewToolFailed(sessionID, toolName, err, durationMs))
}

func (p *Processor) publishLLMStarted(model, prompt string) {
	if p.bus == nil {
		return
	}
	p.bus.Publish(bus.NewLLMStarted(model, prompt))
}

func (p *Processor) publishLLMCompleted(model, response string, tokensIn, tokensOut int, durationMs int64) {
	if p.bus == nil {
		return
	}
	p.bus.Publish(bus.NewLLMCompleted(model, response, tokensIn, tokensOut, durationMs))
}

func (p *Processor) publishLLMError(model, err string) {
	if p.bus == nil {
		return
	}
	p.bus.Publish(bus.NewLLMError(model, err))
}

func (p *Processor) publishAgentStarted(agentName, sessionID string) {
	if p.bus == nil {
		return
	}
	p.bus.Publish(bus.NewAgentStarted(agentName, sessionID))
}

func (p *Processor) publishAgentCompleted(agentName, sessionID string) {
	if p.bus == nil {
		return
	}
	p.bus.Publish(bus.NewAgentCompleted(agentName, sessionID))
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
	msgID := fmt.Sprintf("msg_%s_u%d_%d", sessionID, turn, time.Now().UnixNano())
	_, err := p.store.CreateMessage(context.Background(), storage.CreateMessageInput{
		ID: msgID, SessionID: sessionID,
		Role:    "user",
		Content: prompt,
	})
	return err
}

func (p *Processor) saveAssistantMessage(sessionID string, turn int, text string, toolCalls []llm.Event) error {
	msgID := fmt.Sprintf("msg_%s_a%d_%d", sessionID, turn, time.Now().UnixNano())
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

func (p *Processor) EnableSubAgents() {
	tool.SubAgentRunner = p.runSubAgent
}

func (p *Processor) runSubAgent(subagentType, prompt, parentSessionID string) (string, error) {
	subSessionID := fmt.Sprintf("sub_%s_%d", subagentType, time.Now().UnixNano())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	system := fmt.Sprintf("You are a %s sub-agent. Focus on your assigned task and report results.", subagentType)

	subProc := NewProcessor(p.tools, p.llm, p.store, p.bus, p.model, system)
	subProc.maxTurn = 10

	result, err := subProc.Run(ctx, prompt, subSessionID, parentSessionID)
	if err != nil {
		return "", fmt.Errorf("sub-agent %s: %w", subagentType, err)
	}

	return result.FinalText, nil
}

func (p *Processor) ExportToolRegistry() *tool.Registry {
	return p.tools
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
