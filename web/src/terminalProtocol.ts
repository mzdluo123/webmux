// Pure helpers for the terminal WebSocket contract in docs/protocol.md.

const encoder = new TextEncoder();

function randomHex(bytes: number) {
  // getRandomValues works in insecure contexts; randomUUID does not.
  return Array.from(crypto.getRandomValues(new Uint8Array(bytes)), (byte) =>
    byte.toString(16).padStart(2, "0"),
  ).join("");
}

export function newSubmissionId() {
  return "s-" + randomHex(16);
}

// Mirrors xterm's paste(): newlines become CR, and the text is wrapped in
// bracketed-paste markers while the application has enabled that mode.
export function preparePaste(text: string, bracketed: boolean) {
  const data = text.replace(/\r?\n/g, "\r");
  return bracketed ? `\x1b[200~${data}\x1b[201~` : data;
}

// A 0x03 payload: one JSON header line, then the body and submit key bytes.
// Terminal bytes stay raw, never JSON strings.
export function encodeSubmission(
  id: string,
  generation: string,
  body: string,
  submit: string,
) {
  const bodyBytes = encoder.encode(body);
  const header = encoder.encode(
    JSON.stringify({ id, generation, bodyBytes: bodyBytes.length }) + "\n",
  );
  const submitBytes = encoder.encode(submit);
  const payload = new Uint8Array(
    header.length + bodyBytes.length + submitBytes.length,
  );
  payload.set(header);
  payload.set(bodyBytes, header.length);
  payload.set(submitBytes, header.length + bodyBytes.length);
  return payload;
}

export function deviceLabel(userAgent: string, touchPoints = 0) {
  const device = /iPhone/.test(userAgent)
    ? "iPhone"
    : /iPad/.test(userAgent) || (/Macintosh/.test(userAgent) && touchPoints > 1)
      ? "iPad"
      : /Android/.test(userAgent)
        ? "Android"
        : /Macintosh|Mac OS X/.test(userAgent)
          ? "Mac"
          : /Windows/.test(userAgent)
            ? "Windows"
            : /Linux|X11/.test(userAgent)
              ? "Linux"
              : "浏览器";
  const browser = /EdgA?\//.test(userAgent)
    ? "Edge"
    : /Firefox\/|FxiOS\//.test(userAgent)
      ? "Firefox"
      : /Chrome\/|CriOS\//.test(userAgent)
        ? "Chrome"
        : /Safari\//.test(userAgent)
          ? "Safari"
          : "";
  return browser ? `${device} · ${browser}` : device;
}

let fallbackClientId: string | undefined;
// One identity per tab: it survives reloads and reconnects of this tab, so
// control and submission acknowledgements follow it, while another tab on
// the same device is a separate client.
export function clientIdentity() {
  let id: string | null = null;
  try {
    id = sessionStorage.getItem("webmux.clientId");
    if (!id || !/^[A-Za-z0-9_-]{8,64}$/.test(id)) {
      id = "c-" + randomHex(12);
      sessionStorage.setItem("webmux.clientId", id);
    }
  } catch {
    id = fallbackClientId ??= "c-" + randomHex(12);
  }
  return {
    id,
    label: deviceLabel(navigator.userAgent, navigator.maxTouchPoints),
  };
}
