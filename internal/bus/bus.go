package bus

import (
	"fmt"
	"sync"
)

type Handler func(Event)

type Bus struct {
	mu           sync.RWMutex
	subsByType   map[string]map[int]Handler
	wildcardSubs map[int]Handler
	nextID       int
}

func New() *Bus {
	return &Bus{
		subsByType:   make(map[string]map[int]Handler),
		wildcardSubs: make(map[int]Handler),
	}
}

func (b *Bus) Publish(event Event) {
	handlers := b.gatherHandlers(event.Type())
	for _, h := range handlers {
		b.safeCall(h, event)
	}
}

func (b *Bus) PublishAsync(event Event) {
	go b.Publish(event)
}

func (b *Bus) gatherHandlers(eventType string) []Handler {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var handlers []Handler
	if h, ok := b.subsByType[eventType]; ok {
		for _, handler := range h {
			handlers = append(handlers, handler)
		}
	}
	for _, handler := range b.wildcardSubs {
		handlers = append(handlers, handler)
	}
	return handlers
}

func (b *Bus) Subscribe(eventType string, handler Handler) int {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	id := b.nextID

	if b.subsByType[eventType] == nil {
		b.subsByType[eventType] = make(map[int]Handler)
	}
	b.subsByType[eventType][id] = handler

	return id
}

func (b *Bus) SubscribeAll(handler Handler) int {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	id := b.nextID
	b.wildcardSubs[id] = handler

	return id
}

func (b *Bus) Unsubscribe(eventType string, id int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if handlers, ok := b.subsByType[eventType]; ok {
		delete(handlers, id)
		if len(handlers) == 0 {
			delete(b.subsByType, eventType)
		}
	}
}

func (b *Bus) UnsubscribeAll(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.wildcardSubs, id)
	for _, handlers := range b.subsByType {
		delete(handlers, id)
	}
}

func (b *Bus) SubscribeOnce(eventType string, handler Handler) int {
	var id int
	var wrapped Handler
	wrapped = func(e Event) {
		if e.Type() == eventType {
			b.Unsubscribe(eventType, id)
			handler(e)
		}
	}
	id = b.Subscribe(eventType, wrapped)
	return id
}

func (b *Bus) HasSubscribers(eventType string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.wildcardSubs) > 0 {
		return true
	}
	handlers, ok := b.subsByType[eventType]
	return ok && len(handlers) > 0
}

func (b *Bus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	count := len(b.wildcardSubs)
	for _, handlers := range b.subsByType {
		count += len(handlers)
	}
	return count
}

func (b *Bus) SubscriberCountFor(eventType string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if handlers, ok := b.subsByType[eventType]; ok {
		return len(handlers)
	}
	return 0
}

func (b *Bus) safeCall(handler Handler, event Event) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("bus: handler panicked for event %s: %v\n", event.Type(), r)
		}
	}()
	handler(event)
}

func (b *Bus) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subsByType = make(map[string]map[int]Handler)
	b.wildcardSubs = make(map[int]Handler)
}
