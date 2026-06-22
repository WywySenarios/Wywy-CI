import { describe, it, expect } from "vitest";
import { ansiToHtml } from "@/lib/ansi";

describe("ansiToHtml", () => {
  it("passes through plain text unchanged", () => {
    expect(ansiToHtml("hello world")).toBe("hello world");
  });

  it("converts yellow bold ANSI codes to spans", () => {
    const input = "\x1b[1;33m[INFO]\x1b[0m Running Go tests...";
    const expected =
      '<span style="font-weight:bold;color:rgb(187,187,0)">[INFO]</span> Running Go tests...';
    expect(ansiToHtml(input)).toBe(expected);
  });

  it("converts green ANSI codes to spans", () => {
    const input = "\x1b[0;32m[PASS]\x1b[0m go vet";
    const expected =
      '<span style="color:rgb(0,187,0)">[PASS]</span> go vet';
    expect(ansiToHtml(input)).toBe(expected);
  });

  it("converts red ANSI codes to spans", () => {
    const input = "\x1b[0;31m[FAIL]\x1b[0m Astro tests";
    const expected =
      '<span style="color:rgb(187,0,0)">[FAIL]</span> Astro tests';
    expect(ansiToHtml(input)).toBe(expected);
  });

  it("handles multiple ANSI sequences in one string", () => {
    const input =
      "\x1b[1;33m[INFO]\x1b[0m \x1b[0;32m[PASS]\x1b[0m done";
    const expected =
      '<span style="font-weight:bold;color:rgb(187,187,0)">[INFO]</span> <span style="color:rgb(0,187,0)">[PASS]</span> done';
    expect(ansiToHtml(input)).toBe(expected);
  });

  it("HTML-escapes special characters in non-ANSI text", () => {
    const input = "a < b && b > c";
    expect(ansiToHtml(input)).toBe("a &lt; b &amp;&amp; b &gt; c");
  });

  it("strips unknown ANSI codes silently", () => {
    // \x1b[?25l (hide cursor) is not a standard SGR code
    const input = "\x1b[?25lhidden";
    expect(ansiToHtml(input)).toBe("hidden");
  });
});
