package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const manifestFile = "plugin.yaml"
const pluginDir = ".opencode/plugins"

func Load(path string, ctx *Context) (Plugin, error) {
	manifest, err := readManifest(path)
	if err != nil {
		return nil, fmt.Errorf("plugin %s: %w", path, err)
	}

	plugin, err := newPlugin(manifest, path)
	if err != nil {
		return nil, err
	}

	if err := plugin.Init(ctx); err != nil {
		return nil, fmt.Errorf("init %s: %w", manifest.Name, err)
	}

	return plugin, nil
}

func LoadAll(baseDir string, ctx *Context) ([]Plugin, error) {
	dir := filepath.Join(baseDir, pluginDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read plugins dir: %w", err)
	}

	var plugins []Plugin
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pluginPath := filepath.Join(dir, entry.Name())
		p, err := Load(pluginPath, ctx)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", entry.Name(), err)
		}
		plugins = append(plugins, p)
	}

	return plugins, nil
}

func readManifest(dir string) (*Manifest, error) {
	path := filepath.Join(dir, manifestFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	if m.Name == "" {
		return nil, fmt.Errorf("plugin name is required")
	}
	if m.Entrypoint == "" {
		return nil, fmt.Errorf("plugin entrypoint is required")
	}

	return &m, nil
}

func newPlugin(m *Manifest, dir string) (Plugin, error) {
	ext := strings.ToLower(filepath.Ext(m.Entrypoint))

	switch ext {
	case ".exe", ".bat", ".sh", ".py", ".js", "":
		entryPath := filepath.Join(dir, m.Entrypoint)
		return &externalPlugin{manifest: *m, dir: dir}, validateEntrypoint(entryPath)
	default:
		return nil, fmt.Errorf("unsupported plugin entrypoint type: %s", ext)
	}
}

func validateEntrypoint(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("entrypoint not found: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("entrypoint is a directory")
	}
	return nil
}

func writePluginExample(dir string) error {
	pluginDir := filepath.Join(dir, pluginDir, "example")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return err
	}

	manifest := Manifest{
		Name:        "example",
		Version:     "1.0.0",
		Description: "Example plugin for opencode-go",
		Author:      "opencode-go",
		Entrypoint:  "plugin.sh",
		MinVersion:  "1.0.0",
	}
	manifestData, _ := yaml.Marshal(&manifest)
	if err := os.WriteFile(filepath.Join(pluginDir, manifestFile), manifestData, 0644); err != nil {
		return err
	}

	script := `#!/bin/sh
echo '{"result": {"message": "Hello from example plugin!"}}'
`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.sh"), []byte(script), 0755); err != nil {
		return err
	}

	return nil
}
