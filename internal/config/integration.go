package config

import (
	"github.com/opencode-go/opencode/internal/provider"
)

func (c *Config) BuildProviders() *provider.Registry {
	reg := provider.NewRegistry()

	if c.Providers.OpenAI != nil {
		p := provider.NewOpenAIProvider(
			c.Providers.OpenAI.APIKey,
			c.Providers.OpenAI.BaseURL,
			nil,
		)
		reg.Register(p)
	}

	if c.Providers.Anthropic != nil {
		p := provider.NewAnthropicProvider(
			c.Providers.Anthropic.APIKey,
			c.Providers.Anthropic.BaseURL,
			nil,
		)
		reg.Register(p)
	}

	for id, cp := range c.Providers.Custom {
		p, err := provider.NewCustomProvider(id, id, provider.Config{
			Type:         cp.Type,
			APIKey:       cp.APIKey,
			BaseURL:      cp.BaseURL,
			DefaultModel: cp.DefaultModel,
			Models:       cp.Models,
		})
		if err != nil {
			continue
		}
		reg.Register(p)
	}

	return reg
}
