package testharness

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
)

type TestCase struct {
	ID       string         `yaml:"id"`
	Prompt   string         `yaml:"prompt"`
	Tools    []ToolExpect   `yaml:"tools"`
	Expected ExpectedResult `yaml:"expected"`
}

type ToolExpect struct {
	Tool string         `yaml:"tool"`
	Args map[string]any `yaml:"args"`
}

type ExpectedResult struct {
	ToolCalls      int      `yaml:"tool_calls"`
	OutputContains []string `yaml:"output_contains,omitempty"`
}

type TestSuite struct {
	Cases []TestCase
}

type CaseResult struct {
	ID     string
	Passed bool
	Error  string
}

type TestResults struct {
	Cases []CaseResult
}

type RunOptions struct {
	WorkDir string
}

type RunOption func(*RunOptions)

func WithWorkDir(dir string) RunOption {
	return func(o *RunOptions) {
		o.WorkDir = dir
	}
}

func (r *TestResults) Summary() string {
	passed, total := 0, len(r.Cases)
	for _, c := range r.Cases {
		if c.Passed {
			passed++
		}
	}
	return fmt.Sprintf("%d/%d passed", passed, total)
}

func LoadTestCases(path string) (*TestSuite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading test cases: %w", err)
	}

	var suite TestSuite
	if err := yaml.Unmarshal(data, &suite.Cases); err != nil {
		return nil, fmt.Errorf("parsing test cases: %w", err)
	}

	return &suite, nil
}

func (suite *TestSuite) RunAll(goBinary string, opts ...RunOption) *TestResults {
	results := &TestResults{}
	for _, tc := range suite.Cases {
		result := suite.runCase(tc, goBinary, opts...)
		results.Cases = append(results.Cases, *result)
	}
	return results
}

func (suite *TestSuite) runCase(tc TestCase, goBinary string, opts ...RunOption) *CaseResult {
	result := &CaseResult{ID: tc.ID}

	var options RunOptions
	for _, o := range opts {
		o(&options)
	}

	wd := options.WorkDir
	if wd == "" {
		wd, _ = os.Getwd()
	}

	for _, t := range tc.Tools {
		argsJSON, _ := json.Marshal(t.Args)
		cmd := exec.Command(goBinary, "execute", t.Tool, string(argsJSON))
		cmd.Dir = wd
		out, err := cmd.CombinedOutput()

		if err != nil {
			result.Error = fmt.Sprintf("tool %s failed: %v\nOutput: %s", t.Tool, err, string(out))
			return result
		}

		for _, substr := range tc.Expected.OutputContains {
			if !strings.Contains(string(out), substr) {
				result.Error = fmt.Sprintf("tool %s: expected %q in output, got: %s", t.Tool, substr, string(out))
				return result
			}
		}
	}

	result.Passed = true
	return result
}
