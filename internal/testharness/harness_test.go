package testharness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTestCases(t *testing.T) {
	root := findProjectRoot(t)
	tcDir := filepath.Join(root, "testcases")

	entries, err := os.ReadDir(tcDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		fpath := filepath.Join(tcDir, entry.Name())
		suite, err := LoadTestCases(fpath)
		if err != nil {
			t.Fatalf("loading %s: %v", fpath, err)
		}
		if len(suite.Cases) == 0 {
			t.Errorf("no test cases in %s", fpath)
		}
		for _, tc := range suite.Cases {
			if tc.ID == "" {
				t.Errorf("test case in %s has no ID", fpath)
			}
		}
	}
}

func TestRunAgainstBinary(t *testing.T) {
	root := findProjectRoot(t)
	goBinary := filepath.Join(root, "opencode-go.exe")

	if _, err := os.Stat(goBinary); err != nil {
		t.Skip("opencode-go.exe not found, build with: go build -o opencode-go.exe ./cmd/opencode")
	}

	tcDir := filepath.Join(root, "testcases")
	entries, _ := os.ReadDir(tcDir)

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		fpath := filepath.Join(tcDir, entry.Name())
		suite, err := LoadTestCases(fpath)
		if err != nil {
			t.Fatal(err)
		}

		results := suite.RunAll(goBinary, WithWorkDir(root))
		for _, cr := range results.Cases {
			if !cr.Passed {
				t.Errorf("test %s failed: %s", cr.ID, cr.Error)
			}
		}
	}
}

func findProjectRoot(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("project root (go.mod) not found")
		}
		wd = parent
	}
}
