package sse

import (
	"encoding/json"
	"sync"
)

type Message struct {
	Event string
	Data  []byte
}

type Broker struct {
	mu          sync.Mutex
	subscribers map[int]chan Message
	nextID      int

	history     []Message
	historySize int
	closed      bool
}

func NewBroker(historySize int) *Broker {
	if historySize <= 0 {
		historySize = 200
	}
	return &Broker{
		subscribers: map[int]chan Message{},
		historySize: historySize,
	}
}

func (b *Broker) Publish(event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	msg := Message{Event: event, Data: data}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}

	b.history = append(b.history, msg)
	if len(b.history) > b.historySize {
		b.history = b.history[len(b.history)-b.historySize:]
	}

	for _, ch := range b.subscribers {
		select {
		case ch <- msg:
		default:
			// Drop if subscriber is slow.
		}
	}
	b.mu.Unlock()
}

func (b *Broker) Subscribe(buffer int) (<-chan Message, func()) {
	if buffer <= 0 {
		buffer = 100
	}

	ch := make(chan Message, buffer)

	b.mu.Lock()
	if b.closed {
		close(ch)
		b.mu.Unlock()
		return ch, func() {}
	}

	id := b.nextID
	b.nextID++
	b.subscribers[id] = ch
	history := append([]Message(nil), b.history...)
	b.mu.Unlock()

	for _, msg := range history {
		select {
		case ch <- msg:
		default:
			break
		}
	}

	cancel := func() {
		b.mu.Lock()
		c, ok := b.subscribers[id]
		if ok {
			delete(b.subscribers, id)
		}
		b.mu.Unlock()
		if ok {
			close(c)
		}
	}

	return ch, cancel
}

func (b *Broker) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	for id, ch := range b.subscribers {
		delete(b.subscribers, id)
		close(ch)
	}
	b.mu.Unlock()
}
