package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const ConfigDirName = ".opencode"
const ConfigFileName = "config.yaml"

var searchPaths = []string{
	".",
	"..",
	"../..",
	"../../..",
}

func Load() (*Config, error) {
	cfg := Default()

	// 1. Try project-level config (search up from cwd)
	projectCfg, path, err := loadFromProject()
	if err != nil {
		return nil, fmt.Errorf("loading project config: %w", err)
	}
	if projectCfg != nil {
		cfg.Merge(projectCfg)
	}

	// 2. Try global config (~/.opencode/config.yaml)
	globalCfg, err := loadFromGlobal()
	if err != nil {
		return nil, fmt.Errorf("loading global config: %w", err)
	}
	if globalCfg != nil {
		cfg.Merge(globalCfg)
		_ = path // use path
	}

	// 3. Expand env vars in merged config
	expandConfigEnv(cfg)

	return cfg, nil
}

func loadFromProject() (*Config, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}

	dir := cwd
	for {
		cfgPath := filepath.Join(dir, ConfigDirName, ConfigFileName)
		if _, err := os.Stat(cfgPath); err == nil {
			cfg, err := loadFile(cfgPath)
			if err != nil {
				return nil, "", fmt.Errorf("%s: %w", cfgPath, err)
			}
			return cfg, cfgPath, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return nil, "", nil
}

func loadFromGlobal() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil
	}

	cfgPath := filepath.Join(home, ConfigDirName, ConfigFileName)
	if _, err := os.Stat(cfgPath); err != nil {
		return nil, nil
	}

	return loadFile(cfgPath)
}

func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	// Expand env vars after parsing
	expandConfigEnv(&cfg)

	return &cfg, nil
}

func expandConfigEnv(cfg *Config) {
	if cfg.Providers.OpenAI != nil {
		cfg.Providers.OpenAI.APIKey = expandEnv(cfg.Providers.OpenAI.APIKey)
		cfg.Providers.OpenAI.BaseURL = expandEnv(cfg.Providers.OpenAI.BaseURL)
		cfg.Providers.OpenAI.DefaultModel = expandEnv(cfg.Providers.OpenAI.DefaultModel)
	}
	if cfg.Providers.Anthropic != nil {
		cfg.Providers.Anthropic.APIKey = expandEnv(cfg.Providers.Anthropic.APIKey)
		cfg.Providers.Anthropic.BaseURL = expandEnv(cfg.Providers.Anthropic.BaseURL)
		cfg.Providers.Anthropic.DefaultModel = expandEnv(cfg.Providers.Anthropic.DefaultModel)
	}
	for k, v := range cfg.Providers.Custom {
		v.APIKey = expandEnv(v.APIKey)
		v.BaseURL = expandEnv(v.BaseURL)
		v.DefaultModel = expandEnv(v.DefaultModel)
		cfg.Providers.Custom[k] = v
	}
}

func expandEnv(s string) string {
	if s == "" {
		return s
	}

	result := s

	// Match ${VAR_NAME} patterns
	for {
		start := strings.Index(result, "${")
		if start < 0 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end < 0 {
			break
		}
		end = start + end + 1
		varName := result[start+2 : end-1]
		envVal := os.Getenv(varName)
		result = result[:start] + envVal + result[end:]
	}

	// Fallback to $VAR_NAME (simple form, must be at start or after non-alphanum)
	result = os.Expand(result, func(name string) string {
		if name == "" {
			return "$"
		}
		return os.Getenv(name)
	})

	return result
}

func FindConfigPath() string {
	cwd, _ := os.Getwd()
	dir := cwd
	for {
		cfgPath := filepath.Join(dir, ConfigDirName, ConfigFileName)
		if _, err := os.Stat(cfgPath); err == nil {
			return cfgPath
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
