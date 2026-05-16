package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencode-go/opencode/internal/bus"
	"github.com/opencode-go/opencode/internal/llm"
	"github.com/opencode-go/opencode/internal/log"
	"github.com/opencode-go/opencode/internal/session"
	"github.com/opencode-go/opencode/internal/storage"
	"github.com/opencode-go/opencode/internal/tool"
	"github.com/opencode-go/opencode/internal/tui"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: opencode <command> [args...]\n")
		os.Exit(1)
	}

	reg := tool.NewRegistry()
	reg.Register(tool.ReadTool())
	reg.Register(tool.WriteTool())
	reg.Register(tool.EditTool())
	reg.Register(tool.BashTool())
	reg.Register(tool.GrepTool())
	reg.Register(tool.GlobTool())
	reg.Register(tool.TaskTool())
	initLogging()

	ctx := context.Background()
	toolCtx := tool.Context{Context: ctx}

	switch os.Args[1] {
	case "run":
		runCmd(flag.NewFlagSet("run", flag.ContinueOnError), reg)

	case "tools":
		for _, id := range reg.List() {
			def, err := reg.Get(id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				continue
			}
			if def != nil {
				fmt.Printf("- %s: %s\n", def.ID, def.Description)
			}
		}

	case "tui":
		tuiCmd()

	case "session":
		sessionCmd(os.Args[2:])

	case "execute":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: opencode execute <tool> <json-args>\n")
			os.Exit(1)
		}
		toolName := os.Args[2]
		jsonArgs := os.Args[3]

		parsed, err := decodeArg(jsonArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid JSON args: %v\n", err)
			os.Exit(1)
		}

		def, err := reg.Get(toolName)
		if err != nil || def == nil {
			fmt.Fprintf(os.Stderr, "Tool not found: %s\n", toolName)
			os.Exit(1)
		}

		result, err := def.Execute(parsed, toolCtx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

type runConfig struct {
	model   string
	apiURL  string
	apiKey  string
	maxTurn int
}

func runCmd(fs *flag.FlagSet, reg *tool.Registry) {
	cfg := runConfig{
		model:   envOr("OPENCODE_MODEL", "big-pickle"),
		apiURL:  envOr("OPENCODE_API_URL", "https://opencode.ai/zen/v1"),
		apiKey:  envOr("OPENCODE_API_KEY", "public"),
		maxTurn: 25,
	}

	fs.StringVar(&cfg.model, "model", cfg.model, "LLM model name")
	fs.StringVar(&cfg.apiURL, "api-url", cfg.apiURL, "LLM API base URL")
	fs.StringVar(&cfg.apiKey, "api-key", cfg.apiKey, "LLM API key")
	fs.IntVar(&cfg.maxTurn, "max-turns", cfg.maxTurn, "max conversation turns")

	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	prompt := strings.Join(fs.Args(), " ")
	if prompt == "" {
		prompt = envOr("OPENCODE_PROMPT", "")
	}
	if prompt == "" {
		fmt.Fprintf(os.Stderr, "Usage: opencode run [flags] <prompt>\n")
		fs.PrintDefaults()
		os.Exit(1)
	}

	if cfg.apiKey == "" {
		fmt.Fprintf(os.Stderr, "Warning: OPENCODE_API_KEY not set. Running without LLM authentication.\n")
	}

	client := llm.NewClient(cfg.apiURL)
	client.SetAPIKey(cfg.apiKey)

	system := "You are a helpful AI assistant with access to tools. Use them to accomplish tasks accurately."
	store, err := storage.NewSQLiteStore(filepath.Join(os.TempDir(), "opencode.db"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	proc := session.NewProcessor(reg, client, store, bus.New(), cfg.model, system)
	proc.EnableSubAgents()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	fmt.Fprintf(os.Stderr, "🧠 Running: %s\n", prompt)
	fmt.Fprintf(os.Stderr, "   Model:   %s\n", cfg.model)
	fmt.Fprintf(os.Stderr, "   API:     %s\n", cfg.apiURL)
	fmt.Fprintf(os.Stderr, "\n")

	start := time.Now()
	wd, _ := os.Getwd()
	sessionID := fmt.Sprintf("sess_%d", time.Now().UnixNano())
	projectID := filepath.Base(wd)
	result, err := proc.Run(ctx, prompt, sessionID, projectID)
	elapsed := time.Since(start)

	if err != nil {
		if ctx.Err() != nil {
			fmt.Fprintf(os.Stderr, "\n⏱️  Timeout after %v\n", elapsed)
		} else {
			fmt.Fprintf(os.Stderr, "\n❌ Error: %v\n", err)
		}
		os.Exit(1)
	}

	if result.FinalText != "" {
		fmt.Println(result.FinalText)
	}

	if result.ToolCalls > 0 {
		fmt.Fprintf(os.Stderr, "\n🔧 Tool calls: %d\n", result.ToolCalls)
	}

	fmt.Fprintf(os.Stderr, "✅ Done in %v\n", elapsed.Round(time.Millisecond))
}

func initLogging() {
	// Parse global log flags before other commands
	for i, arg := range os.Args {
		if arg == "--log-level" && i+1 < len(os.Args) {
			if err := log.SetLevelString(os.Args[i+1]); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			}
		}
		if arg == "--log-file" && i+1 < len(os.Args) {
			f, err := os.Create(os.Args[i+1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: cannot create log file: %v\n", err)
			} else {
				log.SetOutput(io.MultiWriter(os.Stderr, f))
			}
		}
		if arg == "--log-json" {
			log.SetJSON(true)
		}
	}
}

func subscribeBusToLog(eventBus *bus.Bus) {
	eventBus.SubscribeAll(func(e bus.Event) {
		log.Debug("bus."+e.Type(),
			"type", e.Type(),
		)
	})
}

func tuiCmd() {
	reg := tool.NewRegistry()
	reg.Register(tool.ReadTool())
	reg.Register(tool.WriteTool())
	reg.Register(tool.EditTool())
	reg.Register(tool.BashTool())
	reg.Register(tool.GrepTool())
	reg.Register(tool.GlobTool())
	reg.Register(tool.TaskTool())

	model := envOr("OPENCODE_MODEL", "big-pickle")
	apiURL := envOr("OPENCODE_API_URL", "https://opencode.ai/zen/v1")
	apiKey := os.Getenv("OPENCODE_API_KEY")
	if apiKey == "" {
		apiKey = "public"
	}

	if err := tui.Run(reg, model, apiURL, apiKey); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}

func decodeArg(raw string) (map[string]any, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err == nil {
		return args, nil
	}
	if len(raw) > 1 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		inner := raw[1 : len(raw)-1]
		inner = strings.ReplaceAll(inner, `""`, `"`)
		if err := json.Unmarshal([]byte(inner), &args); err == nil {
			return args, nil
		}
	}
	return nil, fmt.Errorf("cannot parse JSON: %s", raw)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
