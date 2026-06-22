package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"wywy-website/ci/server/orchestrator"

	"github.com/coder/websocket"
)

// defaultAcceptOptions allows cross-origin WebSocket connections from any origin.
// This matches the permissive CORS policy already set by the CORS middleware.
var defaultAcceptOptions = &websocket.AcceptOptions{
	InsecureSkipVerify: true,
}

// RunEvent represents a run lifecycle event broadcast to all connected clients.
type RunEvent struct {
	Type        string `json:"type"` // "run_started" | "run_finished"
	RunID       string `json:"run_id"`
	ServiceName string `json:"service_name"`
	Status      string `json:"status,omitempty"` // "running" | "passed" | "failed" | "cancelled"
	Timestamp   string `json:"timestamp"`
}

// EventBroadcaster delivers run lifecycle events to all connected WebSocket clients.
type EventBroadcaster struct {
	mu          sync.RWMutex
	subscribers []chan RunEvent
}

// NewEventBroadcaster creates a new EventBroadcaster.
func NewEventBroadcaster() *EventBroadcaster {
	return &EventBroadcaster{}
}

// Subscribe adds a channel that receives all broadcast events.
func (eb *EventBroadcaster) Subscribe() chan RunEvent {
	ch := make(chan RunEvent, 64)
	eb.mu.Lock()
	eb.subscribers = append(eb.subscribers, ch)
	eb.mu.Unlock()
	return ch
}

// Unsubscribe removes a channel and closes it.
func (eb *EventBroadcaster) Unsubscribe(ch chan RunEvent) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	for i, c := range eb.subscribers {
		if c == ch {
			eb.subscribers = append(eb.subscribers[:i], eb.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// Publish sends an event to all subscribers. Slow readers may miss events.
func (eb *EventBroadcaster) Publish(event RunEvent) {
	eb.mu.RLock()
	for _, ch := range eb.subscribers {
		select {
		case ch <- event:
		default:
			// drop if buffer full
		}
	}
	eb.mu.RUnlock()
}

// handleEvents upgrades the connection to WebSocket and streams all
// RunEvents from the EventBroadcaster to the client.
func (h *Handler) handleEvents(w http.ResponseWriter, r *http.Request) {
	if h.EventBroadcaster == nil {
		http.Error(w, "event broadcaster not configured", http.StatusInternalServerError)
		return
	}

	ch := h.EventBroadcaster.Subscribe()
	defer h.EventBroadcaster.Unsubscribe(ch)

	c, err := websocket.Accept(w, r, defaultAcceptOptions)
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

// eventBroadcasterAdapter wraps an api.EventBroadcaster to satisfy the
// orchestrator.EventBroadcaster interface.
type eventBroadcasterAdapter struct {
	inner *EventBroadcaster
}

// Publish converts an orchestrator.LifecycleEvent to a RunEvent and
// forwards it to the underlying api.EventBroadcaster.
func (a *eventBroadcasterAdapter) Publish(event orchestrator.LifecycleEvent) {
	a.inner.Publish(RunEvent{
		Type:        event.Type,
		RunID:       event.RunID,
		ServiceName: event.ServiceName,
		Status:      event.Status,
		Timestamp:   event.Timestamp,
	})
}

// NewEventBroadcasterAdapter wraps an api.EventBroadcaster to satisfy the
// orchestrator.EventBroadcaster interface for wiring into the Runner.
func NewEventBroadcasterAdapter(eb *EventBroadcaster) orchestrator.EventBroadcaster {
	return &eventBroadcasterAdapter{inner: eb}
}
