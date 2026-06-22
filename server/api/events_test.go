package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestEventBroadcasterPublishSubscribe(t *testing.T) {
	eb := &EventBroadcaster{}

	// Subscribe two channels.
	ch1 := eb.Subscribe()
	ch2 := eb.Subscribe()

	// Publish an event.
	event := RunEvent{
		Type:        "run_started",
		RunID:       "run-1",
		ServiceName: "agentic",
		Status:      "running",
		Timestamp:   "2000-01-01T00:00:00Z",
	}
	eb.Publish(event)

	// Both subscribers must receive the event.
	select {
	case got := <-ch1:
		if got != event {
			t.Errorf("ch1: got %+v, want %+v", got, event)
		}
	default:
		t.Error("ch1 did not receive event")
	}

	select {
	case got := <-ch2:
		if got != event {
			t.Errorf("ch2: got %+v, want %+v", got, event)
		}
	default:
		t.Error("ch2 did not receive event")
	}

	// After unsubscribing, ch1 must be closed.
	eb.Unsubscribe(ch1)
	_, ok := <-ch1
	if ok {
		t.Error("ch1 should be closed after Unsubscribe")
	}

	// Publish a second event; only ch2 should receive it.
	event2 := RunEvent{
		Type:        "run_finished",
		RunID:       "run-1",
		ServiceName: "agentic",
		Status:      "passed",
		Timestamp:   "2000-01-01T00:01:00Z",
	}
	eb.Publish(event2)

	select {
	case got := <-ch2:
		if got != event2 {
			t.Errorf("ch2: got %+v, want %+v", got, event2)
		}
	default:
		t.Error("ch2 did not receive event2")
	}
}

func TestEventWebSocketHandler(t *testing.T) {
	eb := NewEventBroadcaster()

	mux := http.NewServeMux()
	h := &Handler{
		Store:            newTestStore(t),
		Broadcaster:      NewBroadcaster(),
		EventBroadcaster: eb,
	}
	h.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	u := "ws" + srv.URL[4:] + "/api/events"
	conn, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatalf("WebSocket dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Publish an event through the EventBroadcaster.
	eb.Publish(RunEvent{
		Type:        "run_started",
		RunID:       "run-1",
		ServiceName: "agentic",
		Status:      "running",
		Timestamp:   "2000-01-01T00:00:00Z",
	})

	_, msg, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("WebSocket read: %v", err)
	}

	var received RunEvent
	if err := json.Unmarshal(msg, &received); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if received.Type != "run_started" {
		t.Errorf("Type: want %q, got %q", "run_started", received.Type)
	}
	if received.RunID != "run-1" {
		t.Errorf("RunID: want %q, got %q", "run-1", received.RunID)
	}
	if received.ServiceName != "agentic" {
		t.Errorf("ServiceName: want %q, got %q", "agentic", received.ServiceName)
	}
	if received.Status != "running" {
		t.Errorf("Status: want %q, got %q", "running", received.Status)
	}
}

func TestEventWebSocketHandlerAcceptsCrossOrigin(t *testing.T) {
	eb := NewEventBroadcaster()

	mux := http.NewServeMux()
	h := &Handler{
		Store:            newTestStore(t),
		Broadcaster:      NewBroadcaster(),
		EventBroadcaster: eb,
	}
	h.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	u := "ws" + srv.URL[4:] + "/api/events"

	// Dial with an Origin header that differs from the server host.
	// Currently fails because websocket.Accept is called with nil options
	// which rejects cross-origin requests by default.
	conn, _, err := websocket.Dial(ctx, u, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Origin": {"http://frontend.localhost:3001"},
		},
	})
	if err != nil {
		t.Fatalf("WebSocket dial with cross-origin header: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Verify the connection works by publishing and receiving an event.
	eb.Publish(RunEvent{
		Type:        "run_started",
		RunID:       "cross-origin-run",
		ServiceName: "agentic",
		Status:      "running",
		Timestamp:   "2000-01-01T00:00:00Z",
	})

	_, msg, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("WebSocket read after cross-origin dial: %v", err)
	}

	var received RunEvent
	if err := json.Unmarshal(msg, &received); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if received.Type != "run_started" {
		t.Errorf("Type: want %q, got %q", "run_started", received.Type)
	}
	if received.RunID != "cross-origin-run" {
		t.Errorf("RunID: want %q, got %q", "cross-origin-run", received.RunID)
	}
}
