package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"wywy-ci/server/orchestrator"
)

func TestWebSocketStream(t *testing.T) {
	s := newTestStore(t)
	b := NewBroadcaster()

	mux := http.NewServeMux()
	h := &Handler{
		Store:       s,
		Broadcaster: b,
	}
	h.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Replace http scheme with ws for WebSocket dial.
	u := "ws" + srv.URL[4:] + "/api/runs/r1/stream"
	conn, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatalf("WebSocket dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Send a log entry through the broadcaster.
	b.Send("r1", orchestrator.LogMessage{
		Type:        "log",
		RunID:       "r1",
		ServiceName: "agentic",
		Level:       "ERROR",
		Content:     "test",
	})

	// Read the message from WebSocket.
	_, msg, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("WebSocket read: %v", err)
	}

	var received orchestrator.LogMessage
	if err := json.Unmarshal(msg, &received); err != nil {
		t.Fatalf("unmarshal message: %v", err)
	}

	if received.Type != "log" {
		t.Errorf("type: want %q, got %q", "log", received.Type)
	}
	if received.RunID != "r1" {
		t.Errorf("run_id: want %q, got %q", "r1", received.RunID)
	}
	if received.ServiceName != "agentic" {
		t.Errorf("service_name: want %q, got %q", "agentic", received.ServiceName)
	}
	if received.Level != "ERROR" {
		t.Errorf("level: want %q, got %q", "ERROR", received.Level)
	}
	if received.Content != "test" {
		t.Errorf("content: want %q, got %q", "test", received.Content)
	}
}

func TestWebSocketMultipleClients(t *testing.T) {
	s := newTestStore(t)
	b := NewBroadcaster()

	mux := http.NewServeMux()
	h := &Handler{
		Store:       s,
		Broadcaster: b,
	}
	h.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	u := "ws" + srv.URL[4:] + "/api/runs/r1/stream"

	// Connect client 1
	conn1, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatalf("client 1 dial: %v", err)
	}
	defer conn1.Close(websocket.StatusNormalClosure, "")

	// Connect client 2
	conn2, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatalf("client 2 dial: %v", err)
	}
	defer conn2.Close(websocket.StatusNormalClosure, "")

	// Send 3 messages
	for i := 0; i < 3; i++ {
		b.Send("r1", orchestrator.LogMessage{
			Type:    "log",
			RunID:   "r1",
			Content: fmt.Sprintf("msg %d", i),
		})
	}

	// Both clients should receive all 3 messages
	for i := 0; i < 3; i++ {
		_, msg, err := conn1.Read(ctx)
		if err != nil {
			t.Fatalf("client 1 read %d: %v", i, err)
		}
		var received orchestrator.LogMessage
		if err := json.Unmarshal(msg, &received); err != nil {
			t.Fatalf("client 1 unmarshal %d: %v", i, err)
		}
		if received.Content != fmt.Sprintf("msg %d", i) {
			t.Errorf("client 1 msg %d: want %q, got %q", i, fmt.Sprintf("msg %d", i), received.Content)
		}
	}

	for i := 0; i < 3; i++ {
		_, msg, err := conn2.Read(ctx)
		if err != nil {
			t.Fatalf("client 2 read %d: %v", i, err)
		}
		var received orchestrator.LogMessage
		if err := json.Unmarshal(msg, &received); err != nil {
			t.Fatalf("client 2 unmarshal %d: %v", i, err)
		}
		if received.Content != fmt.Sprintf("msg %d", i) {
			t.Errorf("client 2 msg %d: want %q, got %q", i, fmt.Sprintf("msg %d", i), received.Content)
		}
	}
}

func TestWebSocketCompletion(t *testing.T) {
	s := newTestStore(t)
	b := NewBroadcaster()

	mux := http.NewServeMux()
	h := &Handler{
		Store:       s,
		Broadcaster: b,
	}
	h.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	u := "ws" + srv.URL[4:] + "/api/runs/r1/stream"
	conn, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Trigger completion
	b.Done("r1", "passed")

	// Read done message
	_, msg, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read done message: %v", err)
	}
	var received orchestrator.LogMessage
	if err := json.Unmarshal(msg, &received); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if received.Type != "done" {
		t.Errorf("type: want %q, got %q", "done", received.Type)
	}
	if received.RunID != "r1" {
		t.Errorf("run_id: want %q, got %q", "r1", received.RunID)
	}
	if received.Status != "passed" {
		t.Errorf("status: want %q, got %q", "passed", received.Status)
	}

	// Connection should be closed by server after done message
	_, _, err = conn.Read(ctx)
	if err == nil {
		t.Error("expected connection close after done message, but read succeeded")
	}
}

func TestRunStreamClientConnectsAfterDone(t *testing.T) {
	s := newTestStore(t)
	b := NewBroadcaster()

	// Call Done before any client connects — simulates a run that already finished.
	b.Done("r1", "passed")

	mux := http.NewServeMux()
	h := &Handler{
		Store:       s,
		Broadcaster: b,
	}
	h.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	u := "ws" + srv.URL[4:] + "/api/runs/r1/stream"
	conn, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Should receive a "done" message because the run already completed.
	_, msg, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read message after Done: %v", err)
	}
	var received orchestrator.LogMessage
	if err := json.Unmarshal(msg, &received); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if received.Type != "done" {
		t.Errorf("type: want %q, got %q", "done", received.Type)
	}
	if received.RunID != "r1" {
		t.Errorf("run_id: want %q, got %q", "r1", received.RunID)
	}
	if received.Status != "passed" {
		t.Errorf("status: want %q, got %q", "passed", received.Status)
	}
}
