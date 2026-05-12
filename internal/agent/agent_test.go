package agent_test

import (
	"testing"

	"github.com/opencode-go/opencode/internal/agent"
)

func TestBuiltInAgents(t *testing.T) {
	svc := agent.NewService()

	if svc.DefaultAgent() != "build" {
		t.Errorf("expected default 'build', got %q", svc.DefaultAgent())
	}

	build, err := svc.Get("build")
	if err != nil {
		t.Fatal(err)
	}
	if build.Mode != agent.ModePrimary {
		t.Errorf("build expected ModePrimary, got %v", build.Mode)
	}
	if !build.Native {
		t.Error("build expected Native=true")
	}

	plan, err := svc.Get("plan")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Mode != agent.ModePrimary {
		t.Errorf("plan expected ModePrimary, got %v", plan.Mode)
	}

	general, err := svc.Get("general")
	if err != nil {
		t.Fatal(err)
	}
	if general.Mode != agent.ModeSubagent {
		t.Errorf("general expected ModeSubagent, got %v", general.Mode)
	}

	explore, err := svc.Get("explore")
	if err != nil {
		t.Fatal(err)
	}
	if explore.Mode != agent.ModeSubagent {
		t.Errorf("explore expected ModeSubagent, got %v", explore.Mode)
	}
}

func TestAgentNotFound(t *testing.T) {
	svc := agent.NewService()
	_, err := svc.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestListAgents(t *testing.T) {
	svc := agent.NewService()
	list := svc.List()

	if len(list) < 4 {
		t.Errorf("expected at least 4 agents, got %d", len(list))
	}

	if list[0].Name != "build" {
		t.Errorf("expected first agent 'build', got %q", list[0].Name)
	}
}

func TestListByMode(t *testing.T) {
	svc := agent.NewService()

	primary := svc.ListByMode(agent.ModePrimary)
	if len(primary) < 2 {
		t.Errorf("expected >=2 primary agents, got %d", len(primary))
	}

	subagent := svc.ListByMode(agent.ModeSubagent)
	if len(subagent) < 2 {
		t.Errorf("expected >=2 subagents, got %d", len(subagent))
	}
}

func TestRegisterAgent(t *testing.T) {
	svc := agent.NewService()

	err := svc.Register(&agent.Info{
		Name: "custom",
		Mode: agent.ModePrimary,
	})
	if err != nil {
		t.Fatal(err)
	}

	custom, err := svc.Get("custom")
	if err != nil {
		t.Fatal(err)
	}
	if custom.Name != "custom" {
		t.Errorf("expected 'custom', got %q", custom.Name)
	}
	if custom.Mode != agent.ModePrimary {
		t.Errorf("expected ModePrimary, got %v", custom.Mode)
	}
}

func TestRegisterAgentNoName(t *testing.T) {
	svc := agent.NewService()
	err := svc.Register(&agent.Info{})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestSetDefaultAgent(t *testing.T) {
	svc := agent.NewService()
	if err := svc.SetDefaultAgent("plan"); err != nil {
		t.Fatal(err)
	}
	if svc.DefaultAgent() != "plan" {
		t.Errorf("expected 'plan', got %q", svc.DefaultAgent())
	}
}

func TestSetDefaultAgentInvalid(t *testing.T) {
	svc := agent.NewService()
	if err := svc.SetDefaultAgent("invalid"); err == nil {
		t.Fatal("expected error for invalid agent")
	}
}

func TestPermissionCheck(t *testing.T) {
	rules := agent.Ruleset{
		{Permission: "read", Pattern: "*", Action: agent.ActionAllow},
		{Permission: "write", Pattern: "*", Action: agent.ActionAsk},
		{Permission: "edit", Pattern: "*.go", Action: agent.ActionDeny},
		{Permission: "edit", Pattern: "*.md", Action: agent.ActionAllow},
	}

	if !rules.IsAllowed("read", "file.go") {
		t.Error("read should be allowed")
	}

	if !rules.IsDenied("edit", "file.go") {
		t.Error("edit *.go should be denied")
	}

	if !rules.IsAllowed("edit", "file.md") {
		t.Error("edit *.md should be allowed")
	}
}

func TestPermissionDisabledTools(t *testing.T) {
	rules := agent.Ruleset{
		{Permission: "write", Pattern: "*", Action: agent.ActionDeny},
		{Permission: "edit", Pattern: "*", Action: agent.ActionDeny},
	}

	disabled := rules.DisabledTools()
	if len(disabled) != 2 {
		t.Errorf("expected 2 disabled tools, got %d", len(disabled))
	}
}

func TestFromConfig(t *testing.T) {
	cfg := map[string]any{
		"read": "allow",
		"edit": map[string]any{
			"*.go": "deny",
			"*.md": "allow",
		},
	}

	rules := agent.FromConfig(cfg)
	if len(rules) != 3 {
		t.Errorf("expected 3 rules, got %d", len(rules))
	}
}

func TestMergeRulesets(t *testing.T) {
	r1 := agent.Ruleset{
		{Permission: "read", Pattern: "*", Action: agent.ActionAllow},
	}
	r2 := agent.Ruleset{
		{Permission: "write", Pattern: "*", Action: agent.ActionDeny},
	}

	merged := agent.Merge(r1, r2)
	if len(merged) != 2 {
		t.Errorf("expected 2 rules, got %d", len(merged))
	}
}

func TestExploreAgentPermissions(t *testing.T) {
	svc := agent.NewService()
	explore, _ := svc.Get("explore")

	if explore.Permission.IsDenied("write", "anything") {
		t.Log("explore correctly denies write")
	}
	if explore.Permission.IsDenied("edit", "anything") {
		t.Log("explore correctly denies edit")
	}
}

func TestDefaultAgentName(t *testing.T) {
	svc := agent.NewService()

	a, err := svc.Get(svc.DefaultAgent())
	if err != nil {
		t.Fatal(err)
	}
	if a.Mode == agent.ModeSubagent {
		t.Error("default agent should not be a subagent")
	}
}
