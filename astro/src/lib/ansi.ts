import { AnsiUp } from "ansi_up";
import DOMPurify from "isomorphic-dompurify";

const ansiUp = new AnsiUp();

/**
 * Convert a string containing ANSI escape sequences to sanitized HTML.
 *
 * ANSI SGR codes (colors, bold, etc.) are rendered as `<span>` elements
 * with inline styles via the `ansi_up` library. The result is then
 * sanitized with DOMPurify for safe use with `dangerouslySetInnerHTML`.
 *
 * @param text — raw string that may contain ANSI escape codes
 * @returns HTML string safe to inject via dangerouslySetInnerHTML
 */
export function ansiToHtml(text: string): string {
  const html = ansiUp.ansi_to_html(text);
  return DOMPurify.sanitize(html);
}
