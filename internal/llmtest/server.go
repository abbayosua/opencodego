package llmtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
)

type response struct {
	kind string
	data map[string]any
}

type callRecord struct {
	Body map[string]any
	URL  string
}

type Server struct {
	srv    *httptest.Server
	mu     sync.Mutex
	cond   *sync.Cond

	queue   []response
	calls   int64
	inputs  []callRecord

	closed bool
}

func NewServer() *Server {
	s := &Server{}
	s.cond = sync.NewCond(&s.mu)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", s.handleChat)
	mux.HandleFunc("/chat/completions", s.handleChat)
	mux.HandleFunc("/v1/responses", s.handleResponses)
	mux.HandleFunc("/responses", s.handleResponses)
	s.srv = httptest.NewServer(mux)
	return s
}

func (s *Server) URL() string {
	return s.srv.URL
}

func (s *Server) Close() {
	s.mu.Lock()
	s.closed = true
	s.cond.Broadcast()
	s.mu.Unlock()
	s.srv.Close()
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	r.Body.Close()

	var reqBody map[string]any
	json.Unmarshal(body, &reqBody)

	isStream := false
	if v, ok := reqBody["stream"].(bool); ok {
		isStream = v
	}

	s.mu.Lock()
	s.inputs = append(s.inputs, callRecord{Body: reqBody, URL: r.URL.String()})
	callNum := atomic.AddInt64(&s.calls, 1)
	s.cond.Broadcast()
	s.mu.Unlock()

	resp := s.nextResponse()
	if isStream {
		s.writeStreamingResponse(w, resp, callNum, r.Context())
	} else {
		s.writeJSONResponse(w, resp, callNum)
	}
}

func (s *Server) handleResponses(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	r.Body.Close()

	var reqBody map[string]any
	json.Unmarshal(body, &reqBody)

	s.mu.Lock()
	s.inputs = append(s.inputs, callRecord{Body: reqBody, URL: r.URL.String()})
	callNum := atomic.AddInt64(&s.calls, 1)
	s.cond.Broadcast()
	s.mu.Unlock()

	resp := s.nextResponse()
	s.writeStreamingResponse(w, resp, callNum, r.Context())
}

func (s *Server) nextResponse() response {
	s.mu.Lock()
	defer s.mu.Unlock()

	for len(s.queue) == 0 && !s.closed {
		s.cond.Wait()
	}

	if len(s.queue) == 0 {
		return response{kind: "hang"}
	}

	resp := s.queue[0]
	s.queue = s.queue[1:]
	return resp
}

func (s *Server) writeStreamingResponse(w http.ResponseWriter, resp response, callNum int64, reqCtx context.Context) {
	if resp.kind == "hang" {
		<-reqCtx.Done()
		return
	}
	if resp.kind == "error" {
		code := 500
		if c, ok := resp.data["status"].(float64); ok {
			code = int(c)
		}
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(resp.data)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(200)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	model := resp.data["model"]
	if model == nil {
		model = "test-model"
	}

	sendEvent := func(event map[string]any) {
		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	// Default streaming format matching OpenAI
	sendEvent(map[string]any{
		"id":      fmt.Sprintf("chatcmpl-%d", callNum),
		"object":  "chat.completion.chunk",
		"created": callNum,
		"model":   model,
		"choices": []map[string]any{
			{
				"index": 0,
				"delta": map[string]any{
					"role": "assistant",
				},
				"finish_reason": nil,
			},
		},
	})

	rawParts, _ := resp.data["parts"]

	var partList []map[string]any
	switch v := rawParts.(type) {
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				partList = append(partList, m)
			}
		}
	case []map[string]any:
		partList = v
	case nil:
		if text, ok := resp.data["text"].(string); ok {
			partList = []map[string]any{{"type": "text", "text": text}}
		}
	}

	stopReason := "stop"
	for _, part := range partList {
		switch part["type"] {
		case "reasoning":
			reasoning, _ := part["reasoning"].(string)
			sendEvent(map[string]any{
				"id":      fmt.Sprintf("chatcmpl-%d", callNum),
				"object":  "chat.completion.chunk",
				"model":   model,
				"choices": []map[string]any{
					{
						"index": 0,
						"delta": map[string]any{
							"reasoning_content": reasoning,
						},
					},
				},
			})
		case "text":
			text, _ := part["text"].(string)
			sendEvent(map[string]any{
				"id":      fmt.Sprintf("chatcmpl-%d", callNum),
				"object":  "chat.completion.chunk",
				"model":   model,
				"choices": []map[string]any{
					{
						"index": 0,
						"delta": map[string]any{
							"content": text,
						},
					},
				},
			})
		case "tool_use":
			id, _ := part["id"].(string)
			name, _ := part["name"].(string)
			arguments := part["arguments"]

			var argsJSON string
			switch a := arguments.(type) {
			case string:
				argsJSON = a
			default:
				b, _ := json.Marshal(a)
				argsJSON = string(b)
			}

			toolCall := map[string]any{
				"id":   id,
				"type": "function",
				"function": map[string]any{
					"name":      name,
					"arguments": argsJSON,
				},
			}
			sendEvent(map[string]any{
				"id":      fmt.Sprintf("chatcmpl-%d", callNum),
				"object":  "chat.completion.chunk",
				"model":   model,
				"choices": []map[string]any{
					{
						"index": 0,
						"delta": map[string]any{
							"tool_calls": []map[string]any{toolCall},
						},
					},
				},
			})
		}
	}

	sendEvent(map[string]any{
		"id":      fmt.Sprintf("chatcmpl-%d", callNum),
		"object":  "chat.completion.chunk",
		"model":   model,
		"choices": []map[string]any{
			{
				"index":         0,
				"delta":         map[string]any{},
				"finish_reason": stopReason,
			},
		},
	})

	sendEvent(map[string]any{
		"id":      fmt.Sprintf("chatcmpl-%d", callNum),
		"object":  "chat.completion.chunk",
		"model":   model,
		"choices": []map[string]any{
			{
				"index":         0,
				"delta":         map[string]any{},
				"finish_reason": stopReason,
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     50,
			"completion_tokens": 50,
			"total_tokens":      100,
		},
	})
}

func (s *Server) writeJSONResponse(w http.ResponseWriter, resp response, callNum int64) {
	if resp.kind == "hang" {
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{}})
		return
	}
	if resp.kind == "error" {
		code := 500
		if c, ok := resp.data["status"].(float64); ok {
			code = int(c)
		}
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(resp.data)
		return
	}

	text, _ := resp.data["text"].(string)
	parts, _ := resp.data["parts"].([]map[string]any)

	var msg map[string]any
	if len(parts) > 0 {
		var content []map[string]any
		var toolCalls []map[string]any

		for _, part := range parts {
			switch part["type"] {
			case "text":
				content = append(content, map[string]any{"type": "text", "text": part["text"]})
			case "tool_use":
				args, _ := json.Marshal(part["arguments"])
				toolCalls = append(toolCalls, map[string]any{
					"id":   part["id"],
					"type": "function",
					"function": map[string]any{
						"name":      part["name"],
						"arguments": string(args),
					},
				})
			}
		}

		if text == "" && len(content) > 0 {
			var b strings.Builder
			for _, c := range content {
				if t, ok := c["text"].(string); ok {
					b.WriteString(t)
				}
			}
			text = b.String()
		}

		msg = map[string]any{"role": "assistant", "content": text}
		if len(toolCalls) > 0 {
			msg["tool_calls"] = toolCalls
		}
	} else {
		msg = map[string]any{"role": "assistant", "content": text}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":      fmt.Sprintf("chatcmpl-%d", callNum),
		"object":  "chat.completion",
		"created": callNum,
		"model":   resp.data["model"],
		"choices": []map[string]any{
			{
				"index":         0,
				"message":       msg,
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     10,
			"completion_tokens": 10,
			"total_tokens":      20,
		},
	})
}

func (s *Server) Queue(kind string, data map[string]any) {
	s.mu.Lock()
	s.queue = append(s.queue, response{kind: kind, data: data})
	s.cond.Signal()
	s.mu.Unlock()
}

func (s *Server) Text(text string) {
	s.Queue("text", map[string]any{"text": text, "parts": []map[string]any{{"type": "text", "text": text}}})
}

func (s *Server) Tool(name string, args map[string]any) {
	s.Queue("tool", map[string]any{
		"parts": []map[string]any{
			{
				"type":      "tool_use",
				"id":        fmt.Sprintf("call_%s", name),
				"name":      name,
				"arguments": args,
			},
		},
	})
}

func (s *Server) Error(status int, data map[string]any) {
	if data == nil {
		data = make(map[string]any)
	}
	data["status"] = float64(status)
	s.Queue("error", data)
}

func (s *Server) Hang() {
	s.Queue("hang", nil)
}

func (s *Server) Reason(text string) {
	s.Queue("reason", map[string]any{
		"parts": []map[string]any{
			{"type": "reasoning", "reasoning": text},
		},
	})
}

func (s *Server) Reply() *ReplyBuilder {
	return &ReplyBuilder{server: s}
}

func (s *Server) Calls() int64 {
	return atomic.LoadInt64(&s.calls)
}

func (s *Server) Inputs() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]map[string]any, len(s.inputs))
	for i, c := range s.inputs {
		result[i] = c.Body
	}
	return result
}

func (s *Server) Wait(n int64) {
	s.mu.Lock()
	for atomic.LoadInt64(&s.calls) < n && !s.closed {
		s.cond.Wait()
	}
	s.mu.Unlock()
}

type ReplyBuilder struct {
	server *Server
	parts  []map[string]any
}

func (b *ReplyBuilder) Text(text string) *ReplyBuilder {
	b.parts = append(b.parts, map[string]any{"type": "text", "text": text})
	return b
}

func (b *ReplyBuilder) Tool(name string, args map[string]any) *ReplyBuilder {
	b.parts = append(b.parts, map[string]any{
		"type":      "tool_use",
		"id":        fmt.Sprintf("call_%s", name),
		"name":      name,
		"arguments": args,
	})
	return b
}

func (b *ReplyBuilder) Reason(text string) *ReplyBuilder {
	b.parts = append(b.parts, map[string]any{"type": "reasoning", "reasoning": text})
	return b
}

func (b *ReplyBuilder) Item() {
	b.server.Queue("composite", map[string]any{"parts": b.parts})
	b.parts = nil
}
