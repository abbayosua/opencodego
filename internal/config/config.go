package config

type Config struct {
	Providers   ProviderConfigs      `yaml:"providers"`
	Models      map[string]ModelConf `yaml:"models"`
	Agent       AgentConf            `yaml:"agent"`
	Permissions PermissionConf       `yaml:"permissions"`
}

type ProviderConfigs struct {
	OpenAI    *ProviderEntry                `yaml:"openai"`
	Anthropic *ProviderEntry                `yaml:"anthropic"`
	Custom    map[string]CustomProviderConf `yaml:"custom"`
}

type ProviderEntry struct {
	APIKey       string `yaml:"api_key"`
	DefaultModel string `yaml:"default_model"`
	BaseURL      string `yaml:"base_url"`
}

type CustomProviderConf struct {
	Type         string   `yaml:"type"`
	APIKey       string   `yaml:"api_key"`
	BaseURL      string   `yaml:"base_url"`
	DefaultModel string   `yaml:"default_model"`
	Models       []string `yaml:"models"`
}

type ModelConf struct {
	Provider  string `yaml:"provider"`
	MaxTokens int    `yaml:"max_tokens"`
}

type AgentConf struct {
	Name     string `yaml:"name"`
	Mode     string `yaml:"mode"`
	MaxTurns int    `yaml:"max_turns"`
}

type PermissionConf struct {
	AutoApprove []string `yaml:"auto_approve"`
}

func Default() *Config {
	return &Config{
		Providers: ProviderConfigs{},
		Models:    map[string]ModelConf{},
		Agent: AgentConf{
			Name:     "default",
			Mode:     "auto",
			MaxTurns: 25,
		},
		Permissions: PermissionConf{
			AutoApprove: []string{"read", "write", "edit", "glob", "grep"},
		},
	}
}

func (c *Config) Merge(other *Config) {
	if other == nil {
		return
	}
	if other.Providers.OpenAI != nil {
		c.Providers.OpenAI = other.Providers.OpenAI
	}
	if other.Providers.Anthropic != nil {
		c.Providers.Anthropic = other.Providers.Anthropic
	}
	for k, v := range other.Providers.Custom {
		if c.Providers.Custom == nil {
			c.Providers.Custom = make(map[string]CustomProviderConf)
		}
		c.Providers.Custom[k] = v
	}
	for k, v := range other.Models {
		c.Models[k] = v
	}
	if other.Agent.Name != "" {
		c.Agent.Name = other.Agent.Name
	}
	if other.Agent.Mode != "" {
		c.Agent.Mode = other.Agent.Mode
	}
	if other.Agent.MaxTurns > 0 {
		c.Agent.MaxTurns = other.Agent.MaxTurns
	}
	if len(other.Permissions.AutoApprove) > 0 {
		c.Permissions.AutoApprove = other.Permissions.AutoApprove
	}
}
