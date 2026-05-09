package tool

import "context"

type ExecuteResult struct {
	Title      string
	Output     string
	Metadata   map[string]any
}

type Context struct {
	context.Context
	SessionID string
	MessageID string
	CallID    string
	Agent     string
}

type Def struct {
	ID          string
	Description string
	Parameters  map[string]any
	Execute     func(args map[string]any, ctx Context) (*ExecuteResult, error)
}

type Info struct {
	ID   string
	Init func() (*Def, error)
}

func Define(id string, init func() (*Def, error)) *Info {
	return &Info{ID: id, Init: init}
}
