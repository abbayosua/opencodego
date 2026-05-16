package log_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/opencode-go/opencode/internal/log"
)

func TestDefaultLevel(t *testing.T) {
	if !log.Default.Enabled(log.LevelInfo) {
		t.Error("INFO should be enabled by default")
	}
	if log.Default.Enabled(log.LevelDebug) {
		t.Error("DEBUG should NOT be enabled by default")
	}
}

func TestSetLevel(t *testing.T) {
	log.SetLevel(log.LevelDebug)
	defer log.SetLevel(log.LevelInfo)

	if !log.Default.Enabled(log.LevelDebug) {
		t.Error("DEBUG should be enabled after SetLevel(Debug)")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  log.Level
		ok    bool
	}{
		{"DEBUG", log.LevelDebug, true},
		{"INFO", log.LevelInfo, true},
		{"WARN", log.LevelWarn, true},
		{"ERROR", log.LevelError, true},
		{"debug", log.LevelDebug, true},
		{"Debug", log.LevelDebug, true},
		{"INVALID", log.LevelInfo, false},
	}

	for _, tt := range tests {
		got, err := log.ParseLevel(tt.input)
		if tt.ok && err != nil {
			t.Errorf("ParseLevel(%q): unexpected error: %v", tt.input, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("ParseLevel(%q): expected error, got %v", tt.input, got)
		}
		if tt.ok && got != tt.want {
			t.Errorf("ParseLevel(%q): got %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestInfoOutput(t *testing.T) {
	var buf bytes.Buffer
	l := log.New(log.LevelDebug, &buf)

	l.Info("test message")

	out := buf.String()
	if !strings.Contains(out, "INFO") {
		t.Errorf("expected INFO level, got: %s", out)
	}
	if !strings.Contains(out, "test message") {
		t.Errorf("expected message, got: %s", out)
	}
}

func TestDebugNotShownByDefault(t *testing.T) {
	var buf bytes.Buffer
	l := log.New(log.LevelInfo, &buf)

	l.Debug("should not appear")

	if buf.Len() > 0 {
		t.Errorf("expected no output for debug at info level, got: %s", buf.String())
	}
}

func TestDebugShownAtDebugLevel(t *testing.T) {
	var buf bytes.Buffer
	l := log.New(log.LevelDebug, &buf)

	l.Debug("debug message")
	l.Info("info message")
	l.Warn("warn message")
	l.Error("error message")

	out := buf.String()
	if !strings.Contains(out, "DEBUG") {
		t.Errorf("expected DEBUG, got: %s", out)
	}
	if !strings.Contains(out, "INFO") {
		t.Errorf("expected INFO, got: %s", out)
	}
	if !strings.Contains(out, "WARN") {
		t.Errorf("expected WARN, got: %s", out)
	}
	if !strings.Contains(out, "ERROR") {
		t.Errorf("expected ERROR, got: %s", out)
	}
}

func TestStructuredFields(t *testing.T) {
	var buf bytes.Buffer
	l := log.New(log.LevelDebug, &buf)

	l.Info("user login", "user_id", "123", "ip", "192.168.1.1")

	out := buf.String()
	if !strings.Contains(out, "user_id=123") {
		t.Errorf("expected user_id=123, got: %s", out)
	}
	if !strings.Contains(out, "ip=192.168.1.1") {
		t.Errorf("expected ip=192.168.1.1, got: %s", out)
	}
}

func TestJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	l := log.NewJSON(log.LevelInfo, &buf)

	l.Info("json test", "key", "val")

	out := buf.String()
	if !strings.HasPrefix(out, "{") {
		t.Errorf("expected JSON object, got: %s", out)
	}
	if !strings.Contains(out, `"message":"json test"`) {
		t.Errorf("expected message field, got: %s", out)
	}
	if !strings.Contains(out, `"fields"`) {
		t.Errorf("expected fields, got: %s", out)
	}
}

func TestWithPrefix(t *testing.T) {
	var buf bytes.Buffer
	l := log.New(log.LevelDebug, &buf).WithPrefix("[svc] ")

	l.Info("hello")

	out := buf.String()
	if !strings.Contains(out, "[svc] hello") {
		t.Errorf("expected prefix, got: %s", out)
	}
}

func TestPackageLevelFunctions(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetLevel(log.LevelDebug)
	defer log.SetLevel(log.LevelInfo)

	log.Info("package info")
	log.Debug("package debug")

	out := buf.String()
	if !strings.Contains(out, "package info") {
		t.Errorf("expected package info, got: %s", out)
	}
	if !strings.Contains(out, "package debug") {
		t.Errorf("expected package debug, got: %s", out)
	}
}
