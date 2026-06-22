import { describe, it, expect, vi, beforeEach } from "vitest";
import { DEFAULT_API_BASE } from "@/lib/ci-types";

describe("DEFAULT_API_BASE", () => {
  beforeEach(() => {
    vi.unstubAllEnvs();
  });

  it("defaults to http://localhost:2526 when env vars are absent", () => {
    expect(DEFAULT_API_BASE).toBe("http://localhost:2526");
  });

  it("reads PUBLIC_CI_API_HOST and PUBLIC_CI_API_PORT from import.meta.env", async () => {
    vi.resetModules();
    // Set env vars that should override the default.
    import.meta.env.PUBLIC_CI_API_HOST = "ci.example.com";
    import.meta.env.PUBLIC_CI_API_PORT = "8080";

    const mod = await import("@/lib/ci-types");
    expect(mod.DEFAULT_API_BASE).toBe("http://ci.example.com:8080");
  });
});
