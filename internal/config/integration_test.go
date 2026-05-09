package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencode-go/opencode/internal/config"
)

func TestBuildProvidersFromConfig(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".opencode")
	os.MkdirAll(cfgDir, 0755)

	yaml := `
providers:
  openai:
    api_key: sk-test-openai
    default_model: gpt-4o
  anthropic:
    api_key: sk-test-anthropic
    default_model: claude-sonnet-4-20250514
  custom:
    my-inf:
      type: openai-compatible
      api_key: sk-custom
      base_url: http://localhost:8080/v1
      models: [model-x]
`
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(yaml), 0644)

	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}

	reg := cfg.BuildProviders()
	infos := reg.List()

	if len(infos) != 3 {
		t.Fatalf("expected 3 providers, got %d", len(infos))
	}

	ids := make(map[string]bool)
	for _, info := range infos {
		ids[info.ID] = true
	}

	if !ids["openai"] {
		t.Error("expected 'openai' provider")
	}
	if !ids["anthropic"] {
		t.Error("expected 'anthropic' provider")
	}
	if !ids["my-inf"] {
		t.Error("expected 'my-inf' custom provider")
	}
}

func TestBuildProvidersPartial(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".opencode")
	os.MkdirAll(cfgDir, 0755)

	yaml := `
providers:
  custom:
    local:
      type: anthropic-compatible
      api_key: sk-local
      base_url: http://localhost:9000/v1
      default_model: local-model
`
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(yaml), 0644)

	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}

	reg := cfg.BuildProviders()
	infos := reg.List()

	if len(infos) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(infos))
	}

	if infos[0].ID != "local" {
		t.Errorf("expected 'local', got %q", infos[0].ID)
	}
	if infos[0].Type != "custom" {
		t.Errorf("expected 'custom', got %q", infos[0].Type)
	}
}

func TestConfigToProvidersEmpty(t *testing.T) {
	cfg := config.Default()
	reg := cfg.BuildProviders()

	infos := reg.List()
	if len(infos) != 0 {
		t.Errorf("expected 0 providers, got %d", len(infos))
	}
}
