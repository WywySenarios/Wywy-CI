import { renderHook, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { useRunStream } from "@/hooks/useRunStream";
import { DEFAULT_API_BASE } from "@/lib/ci-types";

/** Minimal WebSocket surface needed by the hook. */
interface MockWebSocket {
  close: ReturnType<typeof vi.fn>;
  send: ReturnType<typeof vi.fn>;
  readyState: number;
  addEventListener: (
    type: string,
    listener: EventListener,
  ) => void;
  removeEventListener: (
    type: string,
    listener: EventListener,
  ) => void;
}

describe("useRunStream", () => {
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

    // Use a regular function so that new WebSocket(url) works
    // (arrow functions cannot be used as constructors).
    globalThis.WebSocket = vi.fn(function () {
      return mockWs;
    }) as unknown as typeof WebSocket;
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("connects to the WebSocket URL for the given run ID", () => {
    renderHook(() => useRunStream("run-abc"));
    expect(globalThis.WebSocket).toHaveBeenCalledWith(
      "ws://localhost:2526/api/runs/run-abc/stream",
    );
  });

  it("uses the custom apiBase when provided, converting http to ws", () => {
    renderHook(() => useRunStream("run-xyz", "https://ci.example.com"));
    expect(globalThis.WebSocket).toHaveBeenCalledWith(
      "wss://ci.example.com/api/runs/run-xyz/stream",
    );
  });

  it("starts with connected=false, done=false, error=null, logEntries=[]", () => {
    const { result } = renderHook(() => useRunStream("run-abc"));
    expect(result.current.connected).toBe(false);
    expect(result.current.done).toBe(false);
    expect(result.current.error).toBeNull();
    expect(result.current.finalStatus).toBeNull();
    expect(result.current.logEntries).toEqual([]);
  });

  it("sets connected=true when the WebSocket opens", () => {
    const { result } = renderHook(() => useRunStream("run-abc"));
    act(() => {
      mockWs._handlers.open(new Event("open"));
    });
    expect(result.current.connected).toBe(true);
  });

  it("parses incoming log messages and appends them to logEntries", () => {
    const { result } = renderHook(() => useRunStream("run-abc"));
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", {
          data: JSON.stringify({
            type: "log",
            run_id: "run-abc",
            service_name: "frontend",
            level: "info",
            content: "Build started",
          }),
        }),
      );
    });
    expect(result.current.logEntries).toHaveLength(1);
    expect(result.current.logEntries[0].content).toBe("Build started");
  });

  it("marks stream done and sets finalStatus on a 'done' message", () => {
    const { result } = renderHook(() => useRunStream("run-abc"));
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", {
          data: JSON.stringify({
            type: "done",
            run_id: "run-abc",
            status: "success",
          }),
        }),
      );
    });
    expect(result.current.done).toBe(true);
    expect(result.current.finalStatus).toBe("success");
  });

  it("closes the WebSocket on unmount", () => {
    const { unmount } = renderHook(() => useRunStream("run-abc"));
    unmount();
    expect(mockWs.close).toHaveBeenCalledWith(1000, "component unmounted");
  });

  it("sets error state on WebSocket error event", () => {
    const { result } = renderHook(() => useRunStream("run-abc"));
    act(() => {
      mockWs._handlers.error(new Event("error"));
    });
    expect(result.current.error).toBe("WebSocket connection error");
  });

  it("does not crash on malformed JSON messages", () => {
    const { result } = renderHook(() => useRunStream("run-abc"));
    act(() => {
      mockWs._handlers.message(
        new MessageEvent("message", { data: "not valid json" }),
      );
    });
    expect(result.current.logEntries).toHaveLength(0);
    expect(result.current.error).toBeNull();
  });
});
