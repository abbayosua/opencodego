package task

import (
	"context"
	"fmt"
	"time"

	"github.com/opencode-go/opencode/internal/bus"
	"github.com/opencode-go/opencode/internal/llm"
	"github.com/opencode-go/opencode/internal/message"
	"github.com/opencode-go/opencode/internal/session"
	"github.com/opencode-go/opencode/internal/storage"
	"github.com/opencode-go/opencode/internal/tool"
)

type Runner struct {
	tools      *tool.Registry
	client     *llm.Client
	model      string
	apiKey     string
	apiURL     string
	workdir    string
	feedbackCh chan string
}

func NewRunner(tools *tool.Registry, apiURL, apiKey, model, workdir string) *Runner {
	client := llm.NewClient(apiURL)
	client.SetAPIKey(apiKey)
	return &Runner{
		tools:      tools,
		client:     client,
		model:      model,
		apiKey:     apiKey,
		apiURL:     apiURL,
		workdir:    workdir,
		feedbackCh: make(chan string, 10),
	}
}

func (r *Runner) SendFeedback(msg string) {
	select {
	case r.feedbackCh <- msg:
	default:
	}
}

func (r *Runner) Run(ctx context.Context, plan *Plan, onProgress func(iteration int, desc string)) error {
	system := "You are an AI developer working autonomously. Follow the plan and complete tasks one by one."

	for {
		select {
		case <-ctx.Done():
			plan.Evaluation = "Task stopped by user"
			SavePlan(r.workdir, plan)
			return nil
		case feedback := <-r.feedbackCh:
			plan.AddTaskAt(0, "[FEEDBACK] "+feedback)
			plan.Evaluation = fmt.Sprintf("Feedback diterima: %s", feedback)
			SavePlan(r.workdir, plan)
			if onProgress != nil {
				onProgress(plan.Iteration, fmt.Sprintf("Feedback: %s", feedback))
			}
		default:
		}

		plan.Iteration++
		next := plan.NextPending()

		if next == nil {
			if plan.IsComplete() {
				if onProgress != nil {
					onProgress(plan.Iteration, "Always-on: menunggu 30 detik...")
				}

				select {
				case <-time.After(30 * time.Second):
					done, newTask := r.evaluateProject(ctx, system, plan)
					if done && newTask == "" {
						continue
					}
					if !done && newTask != "" {
						plan.AddTask(newTask)
						plan.Evaluation = fmt.Sprintf("Menambahkan: %s", newTask)
						SavePlan(r.workdir, plan)
						if onProgress != nil {
							onProgress(plan.Iteration, fmt.Sprintf("Task baru: %s", newTask))
						}
					}
				case feedback := <-r.feedbackCh:
					plan.AddTaskAt(0, "[FEEDBACK] "+feedback)
					plan.Evaluation = fmt.Sprintf("Feedback diterima: %s", feedback)
					SavePlan(r.workdir, plan)
					if onProgress != nil {
						onProgress(plan.Iteration, fmt.Sprintf("Feedback: %s", feedback))
					}
				case <-ctx.Done():
					return nil
				}
				continue
			}

			if onProgress != nil {
				onProgress(plan.Iteration, "Evaluasi hasil...")
			}
			done, newTask := r.evaluateProject(ctx, system, plan)
			if done {
				continue
			}
			plan.AddTask(newTask)
			plan.Evaluation = fmt.Sprintf("Menambahkan: %s", newTask)
			SavePlan(r.workdir, plan)
			if onProgress != nil {
				onProgress(plan.Iteration, fmt.Sprintf("Task baru: %s", newTask))
			}
			continue
		}

		next.Status = StatusRunning
		SavePlan(r.workdir, plan)

		if onProgress != nil {
			onProgress(plan.Iteration, fmt.Sprintf("Mengerjakan: %s", next.Desc))
		}

		prompt := fmt.Sprintf(`Goal: %s

Current task: %s

Project directory: %s

Complete this task. Use tools to explore and modify the project as needed.`, plan.Goal, next.Desc, r.workdir)

		eventBus := bus.New()
		store, err := storage.NewInMemoryStore()
		if err != nil {
			next.Status = StatusFailed
			next.Error = err.Error()
			SavePlan(r.workdir, plan)
			continue
		}

		sessionID := fmt.Sprintf("task_%d_%d", next.ID, time.Now().UnixNano())
		proc := session.NewProcessor(r.tools, r.client, store, eventBus, r.model, system)
		proc.EnableSubAgents()

		ctx2, cancel := context.WithTimeout(ctx, 10*time.Minute)
		result, err := proc.Run(ctx2, prompt, sessionID, "longtask")
		cancel()
		store.Close()

		if err != nil {
			next.Status = StatusFailed
			next.Error = err.Error()
			SavePlan(r.workdir, plan)
			if onProgress != nil {
				onProgress(plan.Iteration, fmt.Sprintf("Gagal: %s", err.Error()))
			}
			time.Sleep(2 * time.Second)
			continue
		}

		if result != nil && result.FinalText != "" {
			next.Result = result.FinalText
		}
		next.Status = StatusCompleted
		SavePlan(r.workdir, plan)

		if onProgress != nil {
			onProgress(plan.Iteration, fmt.Sprintf("Selesai: %s", next.Desc))
		}
	}
}

func (r *Runner) evaluateProject(ctx context.Context, system string, plan *Plan) (bool, string) {
	var progress string
	for _, t := range plan.Tasks {
		if t.Status == StatusCompleted {
			progress += fmt.Sprintf("- %s\n", t.Desc)
		}
	}

	prompt := fmt.Sprintf(`Goal: %s

Completed tasks:
%s

Evaluate: Is the project complete and production-ready? If not, what ONE task should be done next to improve it? 
Answer with a single short task description only (max 80 chars). If complete and nothing needs improvement, answer exactly: "DONE"`, plan.Goal, progress)

	client := llm.NewClient(r.apiURL)
	client.SetAPIKey(r.apiKey)

	req := llm.StreamRequest{
		Model:    r.model,
		System:   system,
		Messages: []message.Message{message.NewTextMessage(message.RoleUser, prompt)},
		Stream:   true,
	}

	events, errs := client.Stream(ctx, req)

	var text string
loop:
	for {
		select {
		case evt, ok := <-events:
			if !ok {
				break loop
			}
			if evt.Type == llm.EventTextDelta {
				text += evt.Text
			}
		case err := <-errs:
			if err != nil {
				return true, ""
			}
			break loop
		case <-ctx.Done():
			return true, ""
		}
	}

	if text == "" || text == "DONE" {
		return true, ""
	}
	if len(text) > 100 {
		text = text[:100]
	}
	return false, text
}
