package plugin

const (
	HookPreTool   = "pre_tool"
	HookPostTool  = "post_tool"
	HookPreLLM    = "pre_llm"
	HookPostLLM   = "post_llm"
	HookPreRun    = "pre_run"
	HookPostRun   = "post_run"
)

type Manifest struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
	Author      string `yaml:"author"`
	Entrypoint  string `yaml:"entrypoint"`
	MinVersion  string `yaml:"min_version"`
}

type Hook struct {
	Point   string
	Handler func(ctx *Context, data map[string]any) error
}

type Context struct {
	SessionID string
	Agent     string
	Model     string
	Config    map[string]any
}

type Plugin interface {
	Manifest() Manifest
	Init(ctx *Context) error
}

type WithHooks interface {
	Plugin
	Hooks() []Hook
}

type WithTools interface {
	Plugin
	ToolDefs() []map[string]any
}

type ToolResult struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

type ExternalRequest struct {
	ID     string         `json:"id"`
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

type ExternalResponse struct {
	ID     string `json:"id"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type externalPlugin struct {
	manifest Manifest
	dir      string
}

func (p *externalPlugin) Manifest() Manifest { return p.manifest }

func (p *externalPlugin) Init(ctx *Context) error {
	return nil
}
