package agent

import "strings"

type Mode string

const (
	ModePrimary   Mode = "primary"
	ModeSubagent  Mode = "subagent"
	ModeAll       Mode = "all"
)

type Action string

const (
	ActionAllow Action = "allow"
	ActionDeny  Action = "deny"
	ActionAsk   Action = "ask"
)

type Rule struct {
	Permission string `yaml:"permission" json:"permission"`
	Pattern    string `yaml:"pattern" json:"pattern"`
	Action     Action `yaml:"action" json:"action"`
}

type Ruleset []Rule

type ModelRef struct {
	ProviderID string `yaml:"provider_id" json:"provider_id"`
	ModelID    string `yaml:"model_id" json:"model_id"`
}

type Info struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Mode        Mode              `yaml:"mode" json:"mode"`
	Native      bool              `yaml:"native,omitempty" json:"native,omitempty"`
	Hidden      bool              `yaml:"hidden,omitempty" json:"hidden,omitempty"`
	Permission  Ruleset           `yaml:"permission,omitempty" json:"permission,omitempty"`
	Model       *ModelRef         `yaml:"model,omitempty" json:"model,omitempty"`
	Prompt      string            `yaml:"prompt,omitempty" json:"prompt,omitempty"`
	Options     map[string]any    `yaml:"options,omitempty" json:"options,omitempty"`
	Steps       int               `yaml:"steps,omitempty" json:"steps,omitempty"`
	TopP        float64           `yaml:"top_p,omitempty" json:"top_p,omitempty"`
	Temperature float64           `yaml:"temperature,omitempty" json:"temperature,omitempty"`
	Color       string            `yaml:"color,omitempty" json:"color,omitempty"`
}

func (r Ruleset) Check(permission, pattern string) Action {
	for i := len(r) - 1; i >= 0; i-- {
		rule := r[i]
		if rule.Permission == permission && matchPattern(rule.Pattern, pattern) {
			return rule.Action
		}
	}
	return ActionAsk
}

func (r Ruleset) IsAllowed(permission, pattern string) bool {
	return r.Check(permission, pattern) == ActionAllow
}

func (r Ruleset) IsDenied(permission, pattern string) bool {
	return r.Check(permission, pattern) == ActionDeny
}

func (r Ruleset) DisabledTools() []string {
	var disabled []string
	seen := make(map[string]bool)
	for _, rule := range r {
		if rule.Action == ActionDeny && rule.Pattern == "*" && !seen[rule.Permission] {
			disabled = append(disabled, rule.Permission)
			seen[rule.Permission] = true
		}
	}
	return disabled
}

func matchPattern(pattern, input string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(input, suffix)
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(input, prefix)
	}
	return pattern == input
}

func Merge(rulesets ...Ruleset) Ruleset {
	var result Ruleset
	for _, rs := range rulesets {
		result = append(result, rs...)
	}
	return result
}

func FromConfig(cfg map[string]any) Ruleset {
	var rs Ruleset
	for key, val := range cfg {
		switch v := val.(type) {
		case string:
			rs = append(rs, Rule{Permission: key, Action: Action(v), Pattern: "*"})
		case map[string]any:
			for pattern, action := range v {
				if a, ok := action.(string); ok {
					rs = append(rs, Rule{Permission: key, Action: Action(a), Pattern: pattern})
				}
			}
		}
	}
	return rs
}
