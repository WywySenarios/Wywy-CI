package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/coder/websocket"
	"wywy-website/ci/server/orchestrator"
)

// Broadcaster manages WebSocket subscriber channels per run.
type Broadcaster struct {
	mu      sync.RWMutex
	streams map[string][]chan orchestrator.LogMessage
}

// NewBroadcaster creates a new Broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		streams: make(map[string][]chan orchestrator.LogMessage),
	}
}

// Subscribe adds a channel for the given run ID and returns it.
func (b *Broadcaster) Subscribe(runID string) chan orchestrator.LogMessage {
	ch := make(chan orchestrator.LogMessage, 64)
	b.mu.Lock()
	b.streams[runID] = append(b.streams[runID], ch)
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a channel from the given run.
func (b *Broadcaster) Unsubscribe(runID string, ch chan orchestrator.LogMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	streams := b.streams[runID]
	for i, c := range streams {
		if c == ch {
			b.streams[runID] = append(streams[:i], streams[i+1:]...)
			close(ch)
			return
		}
	}
}

// Done sends a completion message to all subscribers of the given run and
// closes their channels.
func (b *Broadcaster) Done(runID string, status string) {
	msg := orchestrator.LogMessage{
		Type:   "done",
		RunID:  runID,
		Status: status,
	}
	b.mu.Lock()
	for _, ch := range b.streams[runID] {
		select {
		case ch <- msg:
		default:
		}
		close(ch)
	}
	delete(b.streams, runID)
	b.mu.Unlock()
}

// Send delivers a log message to all subscribers of the given run.
func (b *Broadcaster) Send(runID string, msg orchestrator.LogMessage) {
	b.mu.RLock()
	for _, ch := range b.streams[runID] {
		select {
		case ch <- msg:
		default:
			// drop if buffer full
		}
	}
	b.mu.RUnlock()
}

// handleRunStream upgrades the connection to WebSocket and streams log messages
// for the given run ID via the Broadcaster.
func (h *Handler) handleRunStream(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")

	if h.Broadcaster == nil {
		http.Error(w, "broadcaster not configured", http.StatusInternalServerError)
		return
	}

	ch := h.Broadcaster.Subscribe(runID)
	defer h.Broadcaster.Unsubscribe(runID, ch)

	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	for msg := range ch {
		data, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		if err := c.Write(r.Context(), websocket.MessageText, data); err != nil {
			return
		}
	}
}
