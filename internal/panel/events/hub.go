package events

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type Event struct {
	Name string
	Data any
}

type Hub struct {
	mu          sync.Mutex
	subscribers map[chan Event]struct{}
}

func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[chan Event]struct{}),
	}
}

func (h *Hub) Publish(name string, data any) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for ch := range h.subscribers {
		select {
		case ch <- Event{Name: name, Data: data}:
		default:
		}
	}
}

func (h *Hub) Subscribe() (chan Event, func()) {
	ch := make(chan Event, 16)

	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()

	cancel := func() {
		h.mu.Lock()
		if _, ok := h.subscribers[ch]; ok {
			delete(h.subscribers, ch)
			close(ch)
		}
		h.mu.Unlock()
	}

	return ch, cancel
}

func WriteSSE(w http.ResponseWriter, event Event) error {
	payload := []byte("{}")
	if event.Data != nil {
		data, err := json.Marshal(event.Data)
		if err != nil {
			return fmt.Errorf("failed to encode event data: %w", err)
		}
		payload = data
	}

	if _, err := fmt.Fprintf(w, "event: %s\n", event.Name); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
		return err
	}

	return nil
}
