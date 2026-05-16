package task

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const PlanDir = ".opencode"
const PlanFile = "plan.json"

func PlanPath() string {
	return filepath.Join(PlanDir, PlanFile)
}

func LoadPlan(wd string) (*Plan, error) {
	path := filepath.Join(wd, PlanDir, PlanFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read plan: %w", err)
	}
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("parse plan: %w", err)
	}
	return &plan, nil
}

func SavePlan(wd string, plan *Plan) error {
	dir := filepath.Join(wd, PlanDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create plan dir: %w", err)
	}
	path := filepath.Join(dir, PlanFile)
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write plan: %w", err)
	}
	return nil
}

func NewPlan(goal string) *Plan {
	return &Plan{
		Goal:      goal,
		Iteration: 0,
		Tasks: []Task{
			{ID: 1, Desc: fmt.Sprintf("Analisis kebutuhan untuk: %s", goal), Status: StatusPending},
			{ID: 2, Desc: "Buat struktur project", Status: StatusPending},
		},
	}
}
