package bus_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/opencode-go/opencode/internal/bus"
)

func TestPublishSubscribe(t *testing.T) {
	b := bus.New()
	received := make([]bus.Event, 0)
	var mu sync.Mutex

	id := b.Subscribe(bus.TypeSessionCreated, func(e bus.Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
	})
	defer b.Unsubscribe(bus.TypeSessionCreated, id)

	event := bus.NewSessionCreated("sess-1", "Test", "gpt-4o", "dev")
	b.Publish(event)

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].Type() != bus.TypeSessionCreated {
		t.Errorf("expected TypeSessionCreated, got %q", received[0].Type())
	}

	se, ok := received[0].(bus.SessionEvent)
	if !ok {
		t.Fatal("expected SessionEvent")
	}
	if se.SessionID != "sess-1" {
		t.Errorf("expected 'sess-1', got %q", se.SessionID)
	}
	if se.Title != "Test" {
		t.Errorf("expected 'Test', got %q", se.Title)
	}
}

func TestSubscribeAll(t *testing.T) {
	b := bus.New()
	var received int32

	id := b.SubscribeAll(func(e bus.Event) {
		atomic.AddInt32(&received, 1)
	})
	defer b.UnsubscribeAll(id)

	b.Publish(bus.NewSessionCreated("s1", "", "", ""))
	b.Publish(bus.NewMessageSent("s1", "user", "hi"))
	b.Publish(bus.NewToolCalled("s1", "bash", "ls"))

	if n := atomic.LoadInt32(&received); n != 3 {
		t.Errorf("expected 3 events, got %d", n)
	}
}

func TestUnsubscribe(t *testing.T) {
	b := bus.New()
	var count int32

	id := b.Subscribe(bus.TypeToolCalled, func(e bus.Event) {
		atomic.AddInt32(&count, 1)
	})

	b.Publish(bus.NewToolCalled("s1", "bash", "ls"))
	b.Unsubscribe(bus.TypeToolCalled, id)
	b.Publish(bus.NewToolCalled("s1", "bash", "pwd"))

	if n := atomic.LoadInt32(&count); n != 1 {
		t.Errorf("expected 1 event after unsubscribe, got %d", n)
	}
}

func TestUnsubscribeAll(t *testing.T) {
	b := bus.New()
	var count int32

	mu := sync.Mutex{}

	id1 := b.Subscribe(bus.TypeToolCalled, func(e bus.Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})
	id2 := b.Subscribe(bus.TypeSessionCreated, func(e bus.Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	b.Publish(bus.NewToolCalled("s1", "bash", "ls"))  // +1 (id1)
	b.Publish(bus.NewSessionCreated("s1", "", "", "")) // +1 (id2)

	b.UnsubscribeAll(id1)

	b.Publish(bus.NewToolCalled("s1", "bash", "pwd")) // +0 (id1 unsubscribed)

	mu.Lock()
	if count != 2 {
		t.Errorf("expected 2 events (tool + session), got %d", count)
	}
	mu.Unlock()

	b.UnsubscribeAll(id2)

	b.Publish(bus.NewToolCalled("s1", "bash", "x")) // +0

	mu.Lock()
	if count != 2 {
		t.Errorf("expected still 2 events after full unsubscribe, got %d", count)
	}
	mu.Unlock()
}

func TestSubscribeOnce(t *testing.T) {
	b := bus.New()
	var count int32

	b.SubscribeOnce(bus.TypeLLMError, func(e bus.Event) {
		atomic.AddInt32(&count, 1)
	})

	b.Publish(bus.NewLLMError("gpt-4o", "timeout"))
	b.Publish(bus.NewLLMError("gpt-4o", "rate-limit"))

	if n := atomic.LoadInt32(&count); n != 1 {
		t.Errorf("expected 1 event from SubscribeOnce, got %d", n)
	}
}

func TestNoCrossTalk(t *testing.T) {
	b := bus.New()
	var sessionCount, toolCount int32
	var mu sync.Mutex

	b.Subscribe(bus.TypeSessionCreated, func(e bus.Event) {
		mu.Lock()
		sessionCount++
		mu.Unlock()
	})
	b.Subscribe(bus.TypeToolCalled, func(e bus.Event) {
		mu.Lock()
		toolCount++
		mu.Unlock()
	})

	b.Publish(bus.NewSessionCreated("s1", "", "", ""))
	b.Publish(bus.NewToolCalled("s1", "bash", "ls"))
	b.Publish(bus.NewSessionCreated("s2", "", "", ""))

	mu.Lock()
	if sessionCount != 2 {
		t.Errorf("expected 2 session events, got %d", sessionCount)
	}
	if toolCount != 1 {
		t.Errorf("expected 1 tool event, got %d", toolCount)
	}
	mu.Unlock()
}

func TestHasSubscribers(t *testing.T) {
	b := bus.New()

	if b.HasSubscribers(bus.TypeSessionCreated) {
		t.Error("expected no subscribers initially")
	}

	id := b.Subscribe(bus.TypeSessionCreated, func(e bus.Event) {})
	defer b.Unsubscribe(bus.TypeSessionCreated, id)

	if !b.HasSubscribers(bus.TypeSessionCreated) {
		t.Error("expected subscriber after Subscribe")
	}
}

func TestSubscriberCount(t *testing.T) {
	b := bus.New()

	b.Subscribe(bus.TypeSessionCreated, func(e bus.Event) {})
	b.Subscribe(bus.TypeToolCalled, func(e bus.Event) {})
	b.Subscribe(bus.TypeSessionCreated, func(e bus.Event) {})
	b.SubscribeAll(func(e bus.Event) {})

	if n := b.SubscriberCount(); n != 4 {
		t.Errorf("expected 4 subscribers, got %d", n)
	}

	if n := b.SubscriberCountFor(bus.TypeSessionCreated); n != 2 {
		t.Errorf("expected 2 session subscribers, got %d", n)
	}
	if n := b.SubscriberCountFor(bus.TypeToolCalled); n != 1 {
		t.Errorf("expected 1 tool subscriber, got %d", n)
	}
}

func TestMultipleEvents(t *testing.T) {
	b := bus.New()
	var received int32

	sub := b.SubscribeAll(func(e bus.Event) {
		atomic.AddInt32(&received, 1)
	})
	defer b.UnsubscribeAll(sub)

	events := []bus.Event{
		bus.NewSessionCreated("s1", "A", "gpt-4o", "dev"),
		bus.NewSessionUpdated("s1", "A Updated"),
		bus.NewMessageSent("s1", "user", "hello"),
		bus.NewMessageSent("s1", "assistant", "hi there"),
		bus.NewToolCalled("s1", "bash", "ls"),
		bus.NewToolCompleted("s1", "bash", "file1\nfile2", 150),
		bus.NewToolFailed("s1", "bash", "permission denied", 50),
		bus.NewLLMStarted("gpt-4o", "test prompt"),
		bus.NewLLMCompleted("gpt-4o", "response", 10, 20, 500),
		bus.NewLLMError("gpt-4o", "timeout"),
		bus.NewAgentStarted("default", "s1"),
		bus.NewAgentCompleted("default", "s1"),
	}

	for _, e := range events {
		b.Publish(e)
	}

	if n := atomic.LoadInt32(&received); n != 12 {
		t.Errorf("expected 12 events, got %d", n)
	}
}

func TestPublishAsync(t *testing.T) {
	b := bus.New()
	var received int32

	b.SubscribeAll(func(e bus.Event) {
		atomic.AddInt32(&received, 1)
	})

	b.PublishAsync(bus.NewSessionCreated("s1", "", "", ""))
	b.PublishAsync(bus.NewSessionCreated("s2", "", "", ""))

	// Wait for async publishes
	for atomic.LoadInt32(&received) < 2 {
	}
}

func TestReset(t *testing.T) {
	b := bus.New()

	id1 := b.Subscribe(bus.TypeSessionCreated, func(e bus.Event) {})
	id2 := b.SubscribeAll(func(e bus.Event) {})
	_ = id1
	_ = id2

	b.Reset()

	if n := b.SubscriberCount(); n != 0 {
		t.Errorf("expected 0 subscribers after reset, got %d", n)
	}
}

func TestHandlerPanicSafety(t *testing.T) {
	b := bus.New()

	var received int32
	b.Subscribe(bus.TypeSessionCreated, func(e bus.Event) {
		atomic.AddInt32(&received, 1)
	})
	b.Subscribe(bus.TypeSessionCreated, func(e bus.Event) {
		panic("test panic")
	})
	b.Subscribe(bus.TypeSessionCreated, func(e bus.Event) {
		atomic.AddInt32(&received, 1)
	})

	b.Publish(bus.NewSessionCreated("s1", "", "", ""))

	if n := atomic.LoadInt32(&received); n != 2 {
		t.Errorf("expected 2 successful handlers, got %d", n)
	}
}

func TestConcurrentPublish(t *testing.T) {
	b := bus.New()
	var received int32

	b.SubscribeAll(func(e bus.Event) {
		atomic.AddInt32(&received, 1)
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Publish(bus.NewSessionCreated("s1", "", "", ""))
		}()
	}
	wg.Wait()

	if n := atomic.LoadInt32(&received); n != 20 {
		t.Errorf("expected 20 events, got %d", n)
	}
}

func TestConcurrentSubscribeAndPublish(t *testing.T) {
	b := bus.New()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := b.Subscribe(bus.TypeSessionCreated, func(e bus.Event) {})
			b.Unsubscribe(bus.TypeSessionCreated, id)
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Publish(bus.NewSessionCreated("s1", "", "", ""))
		}()
	}

	wg.Wait()
}

func TestEventTypes(t *testing.T) {
	tests := []struct {
		event bus.Event
		want  string
	}{
		{bus.NewSessionCreated("", "", "", ""), bus.TypeSessionCreated},
		{bus.NewSessionUpdated("", ""), bus.TypeSessionUpdated},
		{bus.NewMessageSent("", "", ""), bus.TypeMessageSent},
		{bus.NewToolCalled("", "", ""), bus.TypeToolCalled},
		{bus.NewToolCompleted("", "", "", 0), bus.TypeToolCompleted},
		{bus.NewToolFailed("", "", "", 0), bus.TypeToolFailed},
		{bus.NewLLMStarted("", ""), bus.TypeLLMStarted},
		{bus.NewLLMCompleted("", "", 0, 0, 0), bus.TypeLLMCompleted},
		{bus.NewLLMError("", ""), bus.TypeLLMError},
		{bus.NewAgentStarted("", ""), bus.TypeAgentStarted},
		{bus.NewAgentCompleted("", ""), bus.TypeAgentCompleted},
	}

	for _, tt := range tests {
		if tt.event.Type() != tt.want {
			t.Errorf("expected type %q, got %q", tt.want, tt.event.Type())
		}
	}
}
