import { renderHook, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { useServiceStatus } from "@/hooks/useServiceStatus";
import { DEFAULT_API_BASE } from "@/lib/ci-types";

/** Minimal WebSocket surface needed by the hook. */
interface MockWebSocket {
  close: ReturnType<typeof vi.fn>;
  send: ReturnType<typeof vi.fn>;
  readyState: number;
  addEventListener: (type: string, listener: EventListener) => void;
  removeEventListener: (type: string, listener: EventListener) => void;
}

describe("useServiceStatus", () => {
  let mockWs: MockWebSocket & { _handlers: Record<string, EventListener> };

  beforeEach(() => {
    const handlers: Record<string, EventListener> = {};
    mockWs = {
      close: vi.fn(),
      send: vi.fn(),
      readyState: WebSocket.CONNECTING,
      addEventListener: vi.fn((type, listener) => {
        handlers[type] = listener;
      }) as MockWebSocket["addEventListener"],
      removeEventListener: vi.fn() as MockWebSocket["removeEventListener"],
      _handlers: handlers,
    };

    globalThis.WebSocket = vi.fn(function () {
      return mockWs;
    }) as unknown as typeof WebSocket;
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  function mockFetchActiveServices(services: Record<string, boolean>) {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ active_services: services }),
    });
  }

  function expectNoFetch() {
    // If fetch was never mocked, a call would throw — just check it wasn't called.
    if (vi.isMockFunction(globalThis.fetch)) {
      expect(globalThis.fetch).not.toHaveBeenCalled();
    }
  }

  it("connects to the events WebSocket URL", () => {
    renderHook(() => useServiceStatus());
    expect(globalThis.WebSocket).toHaveBeenCalledWith(
      "ws://localhost:2526/api/events",
    );
  });

  it("uses the custom apiBase when provided", () => {
    renderHook(() => useServiceStatus("https://ci.example.com"));
    expect(globalThis.WebSocket).toHaveBeenCalledWith(
      "wss://ci.example.com/api/events",
    );
  });

  it("starts with empty serviceStatus, connected=false, error=null", () => {
    const { result } = renderHook(() => useServiceStatus());
    expect(result.current.serviceStatus).toEqual({});
    expect(result.current.connected).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it("sets connected=true when the WebSocket opens", () => {
    const { result } = renderHook(() => useServiceStatus());
    act(() => {
      mockWs._handlers.open(new Event("open"));
    });
    expect(result.current.connected).toBe(true);
  });

  it("marks service as running on run_started event", () => {
    const { result } = renderHook(() => useServiceStatus());
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", {
          data: JSON.stringify({
            type: "run_started",
            run_id: "run-abc",
            service_name: "ci",
            status: "running",
            timestamp: "2026-06-18T12:00:00Z",
          }),
        }),
      );
    });
    expect(result.current.serviceStatus).toEqual({ ci: true });
  });

  it("marks service as not running on run_finished event", () => {
    const { result } = renderHook(() => useServiceStatus());
    // First start it.
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", {
          data: JSON.stringify({
            type: "run_started",
            run_id: "run-abc",
            service_name: "agentic",
            status: "running",
            timestamp: "2026-06-18T12:00:00Z",
          }),
        }),
      );
    });
    expect(result.current.serviceStatus).toEqual({ agentic: true });

    // Then finish it.
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", {
          data: JSON.stringify({
            type: "run_finished",
            run_id: "run-abc",
            service_name: "agentic",
            status: "passed",
            timestamp: "2026-06-18T12:05:00Z",
          }),
        }),
      );
    });
    expect(result.current.serviceStatus).toEqual({ agentic: false });
  });

  it("tracks multiple services independently", () => {
    const { result } = renderHook(() => useServiceStatus());

    // Start ci.
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", {
          data: JSON.stringify({
            type: "run_started",
            run_id: "run-1",
            service_name: "ci",
            status: "running",
            timestamp: "2026-06-18T12:00:00Z",
          }),
        }),
      );
    });
    expect(result.current.serviceStatus).toEqual({ ci: true });

    // Start agentic.
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", {
          data: JSON.stringify({
            type: "run_started",
            run_id: "run-2",
            service_name: "agentic",
            status: "running",
            timestamp: "2026-06-18T12:01:00Z",
          }),
        }),
      );
    });
    expect(result.current.serviceStatus).toEqual({ ci: true, agentic: true });

    // Finish ci.
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", {
          data: JSON.stringify({
            type: "run_finished",
            run_id: "run-1",
            service_name: "ci",
            status: "passed",
            timestamp: "2026-06-18T12:05:00Z",
          }),
        }),
      );
    });
    expect(result.current.serviceStatus).toEqual({ ci: false, agentic: true });
  });

  it("sets error state on WebSocket error event", () => {
    const { result } = renderHook(() => useServiceStatus());
    act(() => {
      mockWs._handlers.error(new Event("error"));
    });
    expect(result.current.error).toBe("WebSocket connection error");
  });

  it("does not crash on malformed JSON messages", () => {
    const { result } = renderHook(() => useServiceStatus());
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", { data: "not valid json" }),
      );
    });
    expect(result.current.connected).toBe(false);
    expect(result.current.error).toBeNull();
    expect(result.current.serviceStatus).toEqual({});
  });

  it("does not fetch /api/runs/active before WebSocket is connected", () => {
    mockFetchActiveServices({});
    renderHook(() => useServiceStatus());
    // Fetch should not be called yet — WS hasn't opened.
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it("fetches /api/runs/active when WebSocket connects", () => {
    mockFetchActiveServices({ ci: true });
    renderHook(() => useServiceStatus());
    act(() => {
      mockWs._handlers.open(new Event("open"));
    });
    expect(globalThis.fetch).toHaveBeenCalledTimes(1);
    expect(globalThis.fetch).toHaveBeenCalledWith(
      "http://localhost:2526/api/runs/active",
    );
  });

  it("uses custom apiBase when fetching active runs", () => {
    mockFetchActiveServices({});
    renderHook(() => useServiceStatus("https://ci.example.com"));
    act(() => {
      mockWs._handlers.open(new Event("open"));
    });
    expect(globalThis.fetch).toHaveBeenCalledWith(
      "https://ci.example.com/api/runs/active",
    );
  });

  it("merges active_services from fetch into serviceStatus", async () => {
    mockFetchActiveServices({ ci: true, agentic: false });
    const { result } = renderHook(() => useServiceStatus());

    act(() => {
      mockWs._handlers.open(new Event("open"));
    });

    // Wait for fetch promise to resolve.
    await act(async () => {
      await vi.waitFor(() => {
        expect(result.current.serviceStatus).toHaveProperty("ci");
      });
    });

    // ci is running from the fetch response.
    expect(result.current.serviceStatus).toEqual({ ci: true, agentic: false });
  });

  it("fetch only happens once even if multiple events fire", () => {
    mockFetchActiveServices({ ci: true });
    renderHook(() => useServiceStatus());

    act(() => {
      mockWs._handlers.open(new Event("open"));
    });
    expect(globalThis.fetch).toHaveBeenCalledTimes(1);

    // Second open should not trigger another fetch.
    act(() => {
      mockWs._handlers.open(new Event("open"));
    });
    expect(globalThis.fetch).toHaveBeenCalledTimes(1);
  });
});
