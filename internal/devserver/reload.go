package devserver

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Hub is an SSE (Server-Sent Events) broadcaster. Each GET to Handler
// attaches a long-lived subscriber channel; Broadcast pushes "reload" to
// every subscriber.
type Hub struct {
	mu   sync.Mutex
	subs map[chan string]struct{}
}

func NewHub() *Hub { return &Hub{subs: make(map[chan string]struct{})} }

func (h *Hub) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		ch := make(chan string, 4)
		h.subscribe(ch)
		defer h.unsubscribe(ch)

		heartbeat := time.NewTicker(200 * time.Millisecond)
		defer heartbeat.Stop()

		for {
			select {
			case msg := <-ch:
				if _, err := fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
					return
				}
				fl.Flush()
			case <-heartbeat.C:
				// SSE comment line doubles as keep-alive and dead-client detector.
				if _, err := fmt.Fprintf(w, ": keepalive\n\n"); err != nil {
					return
				}
				fl.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}

// Broadcast sends a message to every live subscriber. Non-blocking per
// subscriber: if a channel is full, that subscriber loses the event.
func (h *Hub) Broadcast(msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- msg:
		default:
		}
	}
}

// Count returns the current subscriber count (test helper).
func (h *Hub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

func (h *Hub) subscribe(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.subs[ch] = struct{}{}
}

func (h *Hub) unsubscribe(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subs, ch)
}
