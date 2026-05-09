package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

func NewCustomProvider(id, name string, cfg Config) (Provider, error) {
	if cfg.Type == "" {
		cfg.Type = "openai-compatible"
	}
	if cfg.DefaultModel == "" && len(cfg.Models) > 0 {
		cfg.DefaultModel = cfg.Models[0]
	}
	if cfg.DefaultModel == "" {
		cfg.DefaultModel = "default"
	}

	baseURL := strings.TrimRight(cfg.BaseURL, "/")

	var modelInfos []ModelInfo
	for _, m := range cfg.Models {
		modelInfos = append(modelInfos, ModelInfo{
			ID:           m,
			ProviderID:   id,
			Name:         m,
			BaseURL:      baseURL,
			Capabilities: []string{"tools", "streaming"},
		})
	}
	if len(modelInfos) == 0 {
		modelInfos = append(modelInfos, ModelInfo{
			ID:           cfg.DefaultModel,
			ProviderID:   id,
			Name:         cfg.DefaultModel,
			BaseURL:      baseURL,
			Capabilities: []string{"tools", "streaming"},
		})
	}

	info := ProviderInfo{
		ID:           id,
		Type:         TypeCustom,
		Name:         name,
		DefaultModel: cfg.DefaultModel,
		Models:       modelInfos,
	}

	compatType := strings.ToLower(cfg.Type)
	switch {
	case strings.Contains(compatType, "anthropic"):
		return &customProvider{
			info:    info,
			apiKey:  cfg.APIKey,
			baseURL: baseURL,
			compat:  "anthropic",
		}, nil

	case strings.Contains(compatType, "openai"):
		return &customProvider{
			info:    info,
			apiKey:  cfg.APIKey,
			baseURL: baseURL,
			compat:  "openai",
		}, nil

	default:
		// Auto-detect by URL pattern
		u, err := url.Parse(baseURL)
		if err == nil {
			if strings.Contains(u.Host, "anthropic") {
				return &customProvider{
					info:    info,
					apiKey:  cfg.APIKey,
					baseURL: baseURL,
					compat:  "anthropic",
				}, nil
			}
		}
		return &customProvider{
			info:    info,
			apiKey:  cfg.APIKey,
			baseURL: baseURL,
			compat:  "openai",
		}, nil
	}
}

type customProvider struct {
	info    ProviderInfo
	apiKey  string
	baseURL string
	compat  string
}

func (p *customProvider) Info() ProviderInfo { return p.info }

func (p *customProvider) Complete(ctx context.Context, req Request) (*Response, error) {
	switch p.compat {
	case "anthropic":
		prov := NewAnthropicProvider(p.apiKey, p.baseURL, nil)
		return prov.Complete(ctx, req)
	default:
		prov := NewOpenAIProvider(p.apiKey, p.baseURL, nil)
		return prov.Complete(ctx, req)
	}
}

func (p *customProvider) Stream(ctx context.Context, req Request) (<-chan StreamEvent, error) {
	switch p.compat {
	case "anthropic":
		prov := NewAnthropicProvider(p.apiKey, p.baseURL, nil)
		return prov.Stream(ctx, req)
	default:
		prov := NewOpenAIProvider(p.apiKey, p.baseURL, nil)
		return prov.Stream(ctx, req)
	}
}

func (p *customProvider) String() string {
	return fmt.Sprintf("CustomProvider(%s, compat=%s)", p.info.Name, p.compat)
}
