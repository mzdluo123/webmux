// Validates an OSC 8 terminal hyperlink and returns its normalized href.
// Only http(s) URLs are accepted; IDN hostnames are normalized to Punycode so
// homograph targets remain visible before confirmation.
export function parseTerminalExternalLink(raw: string): string | null {
  const trimmed = raw.trim();
  if (!trimmed) return null;
  try {
    const url = new URL(trimmed);
    if (url.protocol !== "http:" && url.protocol !== "https:") return null;
    return url.href;
  } catch {
    return null;
  }
}

// Opens a validated http(s) terminal hyperlink in a detached tab without
// exposing `window.opener` or leaking the WebMux origin in `Referer`.
export function openTerminalExternalLink(raw: string): boolean {
  const href = parseTerminalExternalLink(raw);
  if (!href) return false;
  window.open(href, "_blank", "noopener,noreferrer");
  return true;
}
