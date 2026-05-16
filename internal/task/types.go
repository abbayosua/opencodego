package task

import "time"

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusStopped   Status = "stopped"
)

type Task struct {
	ID       int       `json:"id"`
	Desc     string    `json:"desc"`
	Status   Status    `json:"status"`
	Result   string    `json:"result,omitempty"`
	Error    string    `json:"error,omitempty"`
}

type Plan struct {
	Goal       string `json:"goal"`
	Iteration  int    `json:"iteration"`
	Tasks      []Task `json:"tasks"`
	Evaluation string `json:"evaluation,omitempty"`
}

type LongTask struct {
	ID          string    `json:"id"`
	Goal        string    `json:"goal"`
	Status      Status    `json:"status"`
	Iteration   int       `json:"iteration"`
	TotalTasks  int       `json:"total_tasks"`
	DoneTasks   int       `json:"done_tasks"`
	Result      string    `json:"result,omitempty"`
	Error       string    `json:"error,omitempty"`
	SessionID   string    `json:"session_id"`
	ChatID      int64     `json:"chat_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (p *Plan) NextPending() *Task {
	for i := range p.Tasks {
		if p.Tasks[i].Status == StatusPending {
			return &p.Tasks[i]
		}
	}
	return nil
}

func (p *Plan) NewEvaluateTask(desc string) Task {
	return Task{
		ID:     len(p.Tasks) + 1,
		Desc:   desc,
		Status: StatusPending,
	}
}

func (p *Plan) UpdateTask(id int, status Status, result string) {
	for i := range p.Tasks {
		if p.Tasks[i].ID == id {
			p.Tasks[i].Status = status
			p.Tasks[i].Result = result
			return
		}
	}
}

func (p *Plan) AddTask(desc string) {
	p.Tasks = append(p.Tasks, Task{
		ID:     len(p.Tasks) + 1,
		Desc:   desc,
		Status: StatusPending,
	})
}

func (p *Plan) Progress() int {
	if len(p.Tasks) == 0 {
		return 0
	}
	done := 0
	for _, t := range p.Tasks {
		if t.Status == StatusCompleted {
			done++
		}
	}
	return done * 100 / len(p.Tasks)
}
