package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencode-go/opencode/internal/config"
)

func TestDefaults(t *testing.T) {
	cfg := config.Default()
	if cfg.Agent.Name != "default" {
		t.Errorf("expected 'default', got %q", cfg.Agent.Name)
	}
	if cfg.Agent.MaxTurns != 25 {
		t.Errorf("expected 25, got %d", cfg.Agent.MaxTurns)
	}
	if len(cfg.Permissions.AutoApprove) == 0 {
		t.Error("expected auto-approve permissions")
	}
}

func TestLoadFromFile(t *testing.T) {
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
    local:
      type: openai-compatible
      api_key: sk-local
      base_url: http://localhost:8080/v1
      models: [my-model]

agent:
  name: dev-agent
  mode: auto
  max_turns: 50

permissions:
  auto_approve: [read, write]
`
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(yaml), 0644)

	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Providers.OpenAI == nil {
		t.Fatal("expected OpenAI provider config")
	}
	if cfg.Providers.OpenAI.APIKey != "sk-test-openai" {
		t.Errorf("expected 'sk-test-openai', got %q", cfg.Providers.OpenAI.APIKey)
	}
	if cfg.Providers.OpenAI.DefaultModel != "gpt-4o" {
		t.Errorf("expected 'gpt-4o', got %q", cfg.Providers.OpenAI.DefaultModel)
	}

	if cfg.Providers.Anthropic == nil {
		t.Fatal("expected Anthropic provider config")
	}
	if cfg.Providers.Anthropic.APIKey != "sk-test-anthropic" {
		t.Errorf("expected 'sk-test-anthropic', got %q", cfg.Providers.Anthropic.APIKey)
	}

	if cfg.Providers.Custom == nil {
		t.Fatal("expected custom providers")
	}
	local, ok := cfg.Providers.Custom["local"]
	if !ok {
		t.Fatal("expected 'local' custom provider")
	}
	if local.Type != "openai-compatible" {
		t.Errorf("expected 'openai-compatible', got %q", local.Type)
	}
	if local.BaseURL != "http://localhost:8080/v1" {
		t.Errorf("expected 'http://localhost:8080/v1', got %q", local.BaseURL)
	}
	if len(local.Models) != 1 || local.Models[0] != "my-model" {
		t.Errorf("expected ['my-model'], got %v", local.Models)
	}

	if cfg.Agent.Name != "dev-agent" {
		t.Errorf("expected 'dev-agent', got %q", cfg.Agent.Name)
	}
	if cfg.Agent.MaxTurns != 50 {
		t.Errorf("expected 50, got %d", cfg.Agent.MaxTurns)
	}
}

func TestEnvVarExpansion(t *testing.T) {
	os.Setenv("TEST_OPENAI_KEY", "sk-env-openai")
	os.Setenv("TEST_ANTHROPIC_KEY", "sk-env-anthropic")
	defer os.Unsetenv("TEST_OPENAI_KEY")
	defer os.Unsetenv("TEST_ANTHROPIC_KEY")

	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".opencode")
	os.MkdirAll(cfgDir, 0755)

	yaml := `
providers:
  openai:
    api_key: ${TEST_OPENAI_KEY}
  anthropic:
    api_key: $TEST_ANTHROPIC_KEY
`
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(yaml), 0644)

	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Providers.OpenAI.APIKey != "sk-env-openai" {
		t.Errorf("expected 'sk-env-openai', got %q", cfg.Providers.OpenAI.APIKey)
	}
	if cfg.Providers.Anthropic.APIKey != "sk-env-anthropic" {
		t.Errorf("expected 'sk-env-anthropic', got %q", cfg.Providers.Anthropic.APIKey)
	}
}

func TestMergeDefaultAndFile(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".opencode")
	os.MkdirAll(cfgDir, 0755)

	yaml := `
agent:
  max_turns: 10
`
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(yaml), 0644)

	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Agent.MaxTurns != 10 {
		t.Errorf("expected 10, got %d", cfg.Agent.MaxTurns)
	}
	if cfg.Agent.Name != "default" {
		t.Errorf("expected 'default' (from defaults), got %q", cfg.Agent.Name)
	}
	if cfg.Agent.Mode != "auto" {
		t.Errorf("expected 'auto' (from defaults), got %q", cfg.Agent.Mode)
	}
}

func TestNoConfigFile(t *testing.T) {
	dir := t.TempDir()

	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg == nil {
		t.Fatal("expected default config even without file")
	}
	if cfg.Agent.MaxTurns != 25 {
		t.Errorf("expected default 25, got %d", cfg.Agent.MaxTurns)
	}
}

func TestCustomProviderProvider(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".opencode")
	os.MkdirAll(cfgDir, 0755)

	yaml := `
providers:
  custom:
    my-inference:
      type: anthropic-compatible
      api_key: sk-custom
      base_url: https://my-inference.example.com/v1
      default_model: my-model-v1
      models:
        - my-model-v1
        - my-model-v2
`
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(yaml), 0644)

	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}

	inf := cfg.Providers.Custom["my-inference"]
	if inf.Type != "anthropic-compatible" {
		t.Errorf("expected 'anthropic-compatible', got %q", inf.Type)
	}
	if inf.APIKey != "sk-custom" {
		t.Errorf("expected 'sk-custom', got %q", inf.APIKey)
	}
	if inf.DefaultModel != "my-model-v1" {
		t.Errorf("expected 'my-model-v1', got %q", inf.DefaultModel)
	}
	if len(inf.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(inf.Models))
	}
}

func TestGlobalConfigOverride(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	if oldHome == "" {
		oldHome = os.Getenv("USERPROFILE")
	}
	defer os.Setenv("HOME", oldHome)

	os.Setenv("HOME", home)
	os.Setenv("USERPROFILE", home)

	globalCfgDir := filepath.Join(home, ".opencode")
	os.MkdirAll(globalCfgDir, 0755)
	globalYaml := `
agent:
  name: global-agent
  max_turns: 99
`
	os.WriteFile(filepath.Join(globalCfgDir, "config.yaml"), []byte(globalYaml), 0644)

	// Test in an empty temp dir (no project config)
	emptyDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(emptyDir)
	defer os.Chdir(origWd)

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Agent.Name != "global-agent" {
		t.Errorf("expected 'global-agent', got %q", cfg.Agent.Name)
	}
	if cfg.Agent.MaxTurns != 99 {
		t.Errorf("expected 99, got %d", cfg.Agent.MaxTurns)
	}
}

func TestPermissionConfig(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".opencode")
	os.MkdirAll(cfgDir, 0755)

	yaml := `
permissions:
  auto_approve: [bash, read]
`
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(yaml), 0644)

	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Permissions.AutoApprove) != 2 {
		t.Errorf("expected 2 auto-approve, got %d", len(cfg.Permissions.AutoApprove))
	}
	if cfg.Permissions.AutoApprove[0] != "bash" {
		t.Errorf("expected 'bash', got %q", cfg.Permissions.AutoApprove[0])
	}
}
