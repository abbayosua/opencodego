package plugin_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencode-go/opencode/internal/plugin"
)

func TestManifestParsing(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "my-plugin")
	os.MkdirAll(pluginDir, 0755)

	yaml := `
name: test-plugin
version: 1.0.0
description: A test plugin
author: tester
entrypoint: plugin.sh
min_version: 1.0.0
`
	os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(yaml), 0644)
	os.WriteFile(filepath.Join(pluginDir, "plugin.sh"), []byte("#!/bin/sh\necho '{}'"), 0755)

	p, err := plugin.Load(pluginDir, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := p.Manifest()
	if m.Name != "test-plugin" {
		t.Errorf("expected 'test-plugin', got %q", m.Name)
	}
	if m.Version != "1.0.0" {
		t.Errorf("expected '1.0.0', got %q", m.Version)
	}
}

func TestManifestMissingName(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir, 0755)

	yaml := `
version: 1.0.0
entrypoint: plugin.sh
`
	os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte(yaml), 0644)
	os.WriteFile(filepath.Join(dir, "plugin.sh"), []byte("#!/bin/sh"), 0755)

	_, err := plugin.Load(dir, nil)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestManifestMissingEntrypoint(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir, 0755)

	yaml := `
name: no-entry
version: 1.0.0
`
	os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte(yaml), 0644)

	_, err := plugin.Load(dir, nil)
	if err == nil {
		t.Fatal("expected error for missing entrypoint")
	}
}

func TestLoadAllNoDir(t *testing.T) {
	dir := t.TempDir()

	plugins, err := plugin.LoadAll(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestLoadAllWithPlugins(t *testing.T) {
	dir := t.TempDir()

	pluginsDir := filepath.Join(dir, ".opencode", "plugins", "p1")
	os.MkdirAll(pluginsDir, 0755)
	os.WriteFile(filepath.Join(pluginsDir, "plugin.yaml"), []byte("name: p1\nversion: 1.0.0\nentrypoint: run.sh"), 0644)
	os.WriteFile(filepath.Join(pluginsDir, "run.sh"), []byte("#!/bin/sh"), 0755)

	plugins, err := plugin.LoadAll(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Manifest().Name != "p1" {
		t.Errorf("expected 'p1', got %q", plugins[0].Manifest().Name)
	}
}

func TestRegistryRegisterAndGet(t *testing.T) {
	dir := t.TempDir()
	plugDir := filepath.Join(dir, "reg-plugin")
	os.MkdirAll(plugDir, 0755)
	os.WriteFile(filepath.Join(plugDir, "plugin.yaml"), []byte("name: reg-test\nversion: 1.0.0\nentrypoint: run.sh"), 0644)
	os.WriteFile(filepath.Join(plugDir, "run.sh"), []byte("#!/bin/sh"), 0755)

	p, err := plugin.Load(plugDir, nil)
	if err != nil {
		t.Fatal(err)
	}

	reg := plugin.NewRegistry()
	if err := reg.Register(p, nil); err != nil {
		t.Fatal(err)
	}

	got := reg.Get("reg-test")
	if got == nil {
		t.Fatal("expected plugin to be found")
	}
	if got.Manifest().Name != "reg-test" {
		t.Errorf("expected 'reg-test', got %q", got.Manifest().Name)
	}

	list := reg.List()
	if len(list) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(list))
	}
}

func TestRegistryHooks(t *testing.T) {
	reg := plugin.NewRegistry()

	hookPlugin := &testHookPlugin{
		name: "hooker",
		hooks: []plugin.Hook{
			{Point: plugin.HookPreTool, Handler: func(ctx *plugin.Context, data map[string]any) error {
				return nil
			}},
			{Point: plugin.HookPostTool, Handler: func(ctx *plugin.Context, data map[string]any) error {
				return nil
			}},
		},
	}

	if err := reg.Register(hookPlugin, nil); err != nil {
		t.Fatal(err)
	}

	if err := reg.RunHooks(plugin.HookPreTool, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := reg.RunHooks(plugin.HookPostTool, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := reg.RunHooks(plugin.HookPreLLM, nil, nil); err != nil {
		// no hooks registered for this point - should be fine
	}
}

func TestRegistryToolDefs(t *testing.T) {
	reg := plugin.NewRegistry()

	toolPlugin := &testToolPlugin{
		name: "tooler",
		defs: []map[string]any{
			{"name": "custom-tool", "description": "A custom tool"},
		},
	}

	if err := reg.Register(toolPlugin, nil); err != nil {
		t.Fatal(err)
	}

	defs := reg.ToolDefs()
	if len(defs) != 1 {
		t.Errorf("expected 1 tool def, got %d", len(defs))
	}
	if defs[0]["name"] != "custom-tool" {
		t.Errorf("expected 'custom-tool', got %v", defs[0]["name"])
	}
}

func TestPluginDirectoryStructure(t *testing.T) {
	dir := t.TempDir()
	pluginsDir := filepath.Join(dir, ".opencode", "plugins", "test")
	os.MkdirAll(pluginsDir, 0755)
	os.WriteFile(filepath.Join(pluginsDir, "plugin.yaml"), []byte("name: dir-test\nversion: 1.0.0\nentrypoint: run.sh"), 0644)
	os.WriteFile(filepath.Join(pluginsDir, "run.sh"), []byte("#!/bin/sh"), 0755)

	plugins, err := plugin.LoadAll(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Manifest().Name != "dir-test" {
		t.Errorf("expected 'dir-test', got %q", plugins[0].Manifest().Name)
	}
}

type testHookPlugin struct {
	name  string
	hooks []plugin.Hook
}

func (p *testHookPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{Name: p.name, Version: "1.0.0"}
}

func (p *testHookPlugin) Init(ctx *plugin.Context) error { return nil }

func (p *testHookPlugin) Hooks() []plugin.Hook { return p.hooks }

type testToolPlugin struct {
	name string
	defs []map[string]any
}

func (p *testToolPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{Name: p.name, Version: "1.0.0"}
}

func (p *testToolPlugin) Init(ctx *plugin.Context) error { return nil }

func (p *testToolPlugin) ToolDefs() []map[string]any { return p.defs }
