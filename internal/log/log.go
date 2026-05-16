package log

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

func ParseLevel(s string) (Level, error) {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return LevelDebug, nil
	case "INFO":
		return LevelInfo, nil
	case "WARN":
		return LevelWarn, nil
	case "ERROR":
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf("invalid log level: %s", s)
	}
}

type entry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
	Fields  any    `json:"fields,omitempty"`
}

type Logger struct {
	mu      sync.Mutex
	level   Level
	output  io.Writer
	jsonFmt bool
	prefix  string
}

func New(level Level, output io.Writer) *Logger {
	return &Logger{
		level:  level,
		output: output,
	}
}

func NewJSON(level Level, output io.Writer) *Logger {
	return &Logger{
		level:   level,
		output:  output,
		jsonFmt: true,
	}
}

var Default = New(LevelInfo, os.Stderr)

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
}

func (l *Logger) SetJSON(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.jsonFmt = enabled
}

func (l *Logger) WithPrefix(prefix string) *Logger {
	return &Logger{
		level:   l.level,
		output:  l.output,
		jsonFmt: l.jsonFmt,
		prefix:  l.prefix + prefix,
	}
}

func (l *Logger) Enabled(level Level) bool {
	return level >= l.level
}

func (l *Logger) Debug(msg string, keysAndValues ...string) {
	l.log(LevelDebug, msg, keysAndValues...)
}

func (l *Logger) Info(msg string, keysAndValues ...string) {
	l.log(LevelInfo, msg, keysAndValues...)
}

func (l *Logger) Warn(msg string, keysAndValues ...string) {
	l.log(LevelWarn, msg, keysAndValues...)
}

func (l *Logger) Error(msg string, keysAndValues ...string) {
	l.log(LevelError, msg, keysAndValues...)
}

func (l *Logger) log(level Level, msg string, keysAndValues ...string) {
	if !l.Enabled(level) {
		return
	}

	var fields map[string]string
	if len(keysAndValues) > 0 {
		fields = make(map[string]string)
		for i := 0; i < len(keysAndValues)-1; i += 2 {
			fields[keysAndValues[i]] = keysAndValues[i+1]
		}
		if len(keysAndValues)%2 == 1 {
			fields[keysAndValues[len(keysAndValues)-1]] = ""
		}
	}

	prefix := l.prefix
	msg = prefix + msg

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.jsonFmt {
		e := entry{
			Time:    time.Now().UTC().Format(time.RFC3339Nano),
			Level:   level.String(),
			Message: msg,
			Fields:  fields,
		}
		data, _ := json.Marshal(e)
		fmt.Fprintln(l.output, string(data))
	} else {
		t := time.Now().Format("15:04:05.000")
		if fields != nil {
			var fstr []string
			for k, v := range fields {
				fstr = append(fstr, fmt.Sprintf("%s=%s", k, v))
			}
			fmt.Fprintf(l.output, "%s [%s] %s  %s\n", t, level.String(), msg, strings.Join(fstr, " "))
		} else {
			fmt.Fprintf(l.output, "%s [%s] %s\n", t, level.String(), msg)
		}
	}
}

// Package-level convenience functions
func Debug(msg string, keysAndValues ...string)  { Default.Debug(msg, keysAndValues...) }
func Info(msg string, keysAndValues ...string)   { Default.Info(msg, keysAndValues...) }
func Warn(msg string, keysAndValues ...string)   { Default.Warn(msg, keysAndValues...) }
func Error(msg string, keysAndValues ...string)  { Default.Error(msg, keysAndValues...) }
func SetLevel(level Level)                       { Default.SetLevel(level) }
func SetOutput(w io.Writer)                      { Default.SetOutput(w) }
func SetJSON(enabled bool)                       { Default.SetJSON(enabled) }
func SetLevelString(s string) error {
	level, err := ParseLevel(s)
	if err != nil {
		return err
	}
	SetLevel(level)
	return nil
}
