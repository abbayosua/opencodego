package llmtest

import "testing"

func NewForTest(t testing.TB) *Server {
	s := NewServer()
	t.Cleanup(s.Close)
	return s
}
