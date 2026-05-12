package agent

var DefaultPermissions = Ruleset{
	{Permission: "read", Pattern: "*", Action: ActionAllow},
	{Permission: "write", Pattern: "*", Action: ActionAllow},
	{Permission: "edit", Pattern: "*", Action: ActionAsk},
	{Permission: "bash", Pattern: "*", Action: ActionAsk},
	{Permission: "grep", Pattern: "*", Action: ActionAllow},
	{Permission: "glob", Pattern: "*", Action: ActionAllow},
	{Permission: "question", Pattern: "*", Action: ActionAllow},
}

func BuiltInAgents() map[string]*Info {
	agents := make(map[string]*Info)

	agents["build"] = &Info{
		Name:        "build",
		Description: "The default agent. Executes tools based on configured permissions.",
		Mode:        ModePrimary,
		Native:      true,
		Permission: Merge(DefaultPermissions, FromConfig(map[string]any{
			"question":   "allow",
			"plan_enter": "allow",
		})),
	}

	agents["plan"] = &Info{
		Name:        "plan",
		Description: "Plan mode. Disallows all edit tools except plan files.",
		Mode:        ModePrimary,
		Native:      true,
		Permission: Merge(DefaultPermissions, FromConfig(map[string]any{
			"question": "allow",
			"plan_exit": "allow",
			"edit": map[string]any{
				"*":                          "deny",
				".opencode/plans/*.md":       "allow",
				".opencode/plans/*.json":     "allow",
				".opencode/plans/*.yaml":     "allow",
				".opencode/plans/*.yml":      "allow",
			},
			"write": map[string]any{
				"*":                          "deny",
				".opencode/plans/*.md":       "allow",
				".opencode/plans/*.json":     "allow",
			},
		})),
		Prompt: `You are in plan mode. Create a detailed plan before making any changes.
Focus on understanding the codebase and creating a step-by-step plan.
Save plans to .opencode/plans/ directory.
Do NOT make any edits outside of plan files.`,
	}

	agents["general"] = &Info{
		Name:        "general",
		Description: "A general-purpose subagent for multi-step parallel tasks.",
		Mode:        ModeSubagent,
		Native:      true,
		Permission: Merge(DefaultPermissions, FromConfig(map[string]any{
			"todowrite": "deny",
		})),
	}

	agents["explore"] = &Info{
		Name:        "explore",
		Description: "Fast codebase search agent. Uses grep, glob, read, bash, and webfetch only.",
		Mode:        ModeSubagent,
		Native:      true,
		Permission: Ruleset{
			{Permission: "read",     Pattern: "*", Action: ActionAllow},
			{Permission: "grep",     Pattern: "*", Action: ActionAllow},
			{Permission: "glob",     Pattern: "*", Action: ActionAllow},
			{Permission: "bash",     Pattern: "*", Action: ActionAsk},
			{Permission: "webfetch", Pattern: "*", Action: ActionAllow},
			{Permission: "write",    Pattern: "*", Action: ActionDeny},
			{Permission: "edit",     Pattern: "*", Action: ActionDeny},
		},
		Prompt: `You are a codebase exploration agent. Your job is to search and understand code.
Use grep and glob to find relevant files, read to understand them.
Do NOT make any edits. Only read and explore.`,
	}

	return agents
}
