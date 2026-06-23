import { renderHook, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { useServiceStatus } from "@/hooks/useServiceStatus";

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

  const runStartedEvent = (overrides: Record<string, string> = {}) => ({
    type: "run_started",
    run_id: "run-abc",
    service_name: "ci",
    suite: "test",
    status: "running",
    timestamp: "2026-06-18T12:00:00Z",
    ...overrides,
  });

  const runFinishedEvent = (overrides: Record<string, string> = {}) => ({
    type: "run_finished",
    run_id: "run-abc",
    service_name: "ci",
    suite: "test",
    status: "passed",
    timestamp: "2026-06-18T12:05:00Z",
    ...overrides,
  });

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

  function mockFetchActiveSuites(suites: Record<string, Record<string, boolean>>) {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ active_suites: suites }),
    });
  }

  function expectNoFetch() {
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

  it("starts with empty status, connected=false, error=null", () => {
    const { result } = renderHook(() => useServiceStatus());
    expect(result.current.suiteStatus).toEqual({});
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

  it("marks suite and service as running on run_started event", () => {
    const { result } = renderHook(() => useServiceStatus());
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", {
          data: JSON.stringify(runStartedEvent()),
        }),
      );
    });
    expect(result.current.suiteStatus).toEqual({ ci: { test: true } });
    expect(result.current.serviceStatus).toEqual({ ci: true });
  });

  it("clears suite and service on run_finished event", () => {
    const { result } = renderHook(() => useServiceStatus());

    // Start it.
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", {
          data: JSON.stringify(runStartedEvent()),
        }),
      );
    });
    expect(result.current.suiteStatus).toEqual({ ci: { test: true } });
    expect(result.current.serviceStatus).toEqual({ ci: true });

    // Finish it.
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", {
          data: JSON.stringify(runFinishedEvent()),
        }),
      );
    });
    expect(result.current.suiteStatus).toEqual({ ci: { test: false } });
    expect(result.current.serviceStatus).toEqual({ ci: false });
  });

  it("tracks multiple suites for the same service independently", () => {
    const { result } = renderHook(() => useServiceStatus());

    // Start test suite.
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", {
          data: JSON.stringify(runStartedEvent({ service_name: "ci", suite: "test", run_id: "run-1" })),
        }),
      );
    });
    expect(result.current.suiteStatus).toEqual({ ci: { test: true } });
    expect(result.current.serviceStatus).toEqual({ ci: true });

    // Start e2e suite on same service.
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", {
          data: JSON.stringify(runStartedEvent({ service_name: "ci", suite: "e2e", run_id: "run-2" })),
        }),
      );
    });
    expect(result.current.suiteStatus).toEqual({ ci: { test: true, e2e: true } });
    expect(result.current.serviceStatus).toEqual({ ci: true });

    // Finish test suite only.
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", {
          data: JSON.stringify(runFinishedEvent({ service_name: "ci", suite: "test", run_id: "run-1" })),
        }),
      );
    });
    expect(result.current.suiteStatus).toEqual({ ci: { test: false, e2e: true } });
    // Service still running because e2e is still active.
    expect(result.current.serviceStatus).toEqual({ ci: true });

    // Finish e2e suite.
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", {
          data: JSON.stringify(runFinishedEvent({ service_name: "ci", suite: "e2e", run_id: "run-2" })),
        }),
      );
    });
    expect(result.current.suiteStatus).toEqual({ ci: { test: false, e2e: false } });
    expect(result.current.serviceStatus).toEqual({ ci: false });
  });

  it("tracks multiple services independently", () => {
    const { result } = renderHook(() => useServiceStatus());

    // Start ci test.
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", {
          data: JSON.stringify(runStartedEvent({ service_name: "ci", suite: "test", run_id: "run-1" })),
        }),
      );
    });
    expect(result.current.suiteStatus).toEqual({ ci: { test: true } });
    expect(result.current.serviceStatus).toEqual({ ci: true });

    // Start agentic test.
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", {
          data: JSON.stringify(runStartedEvent({ service_name: "agentic", suite: "test", run_id: "run-2" })),
        }),
      );
    });
    expect(result.current.suiteStatus).toEqual({
      ci: { test: true },
      agentic: { test: true },
    });
    expect(result.current.serviceStatus).toEqual({ ci: true, agentic: true });

    // Finish ci test.
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", {
          data: JSON.stringify(runFinishedEvent({ service_name: "ci", suite: "test", run_id: "run-1" })),
        }),
      );
    });
    expect(result.current.suiteStatus).toEqual({
      ci: { test: false },
      agentic: { test: true },
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
    expect(result.current.suiteStatus).toEqual({});
    expect(result.current.serviceStatus).toEqual({});
  });

  it("does not fetch /api/runs/active before WebSocket is connected", () => {
    mockFetchActiveSuites({});
    renderHook(() => useServiceStatus());
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it("fetches /api/runs/active when WebSocket connects", () => {
    mockFetchActiveSuites({});
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
    mockFetchActiveSuites({});
    renderHook(() => useServiceStatus("https://ci.example.com"));
    act(() => {
      mockWs._handlers.open(new Event("open"));
    });
    expect(globalThis.fetch).toHaveBeenCalledWith(
      "https://ci.example.com/api/runs/active",
    );
  });

  it("initializes suiteStatus from active_suites fetch response", async () => {
    mockFetchActiveSuites({
      ci: { test: true, e2e: false },
      agentic: { test: false },
    });
    const { result } = renderHook(() => useServiceStatus());

    act(() => {
      mockWs._handlers.open(new Event("open"));
    });

    await act(async () => {
      await vi.waitFor(() => {
        expect(result.current.suiteStatus).toHaveProperty("ci");
      });
    });

    expect(result.current.suiteStatus).toEqual({
      ci: { test: true, e2e: false },
      agentic: { test: false },
    });
    expect(result.current.serviceStatus).toEqual({ ci: true, agentic: false });
  });

  it("fetch only happens once even if multiple events fire", () => {
    mockFetchActiveSuites({});
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
