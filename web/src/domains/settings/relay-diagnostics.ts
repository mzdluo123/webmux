import { m } from "$lib/paraglide/messages.js";

const reasons: Record<string, () => string> = {
  "authorization-revoked": () => m.relay_diag_revoked(),
  "control-bootstrap-unavailable": () => m.relay_diag_bootstrap(),
  "storage-unavailable": () => m.relay_diag_storage(),
  "bulk-unavailable": () => m.relay_diag_bulk(),
  "turn-unavailable": () => m.relay_diag_turn(),
  "p2p-plugin-unavailable": () => m.relay_diag_plugin(),
  "p2p-build-unavailable": () => m.relay_diag_build(),
  "stable-latency-advantage": () => m.relay_diag_advantage(),
};

// Status prose is not a channel for transport credentials or ICE material.
export function relayDiagnostic(value: string) {
  if (!value) return "";
  if (Object.hasOwn(reasons, value)) return reasons[value]!();
  if (
    value.length > 240 ||
    /[\r\n\0]|[A-Za-z0-9_+/=-]{32,}|https?:|wss?:|turns?:|\b\d{1,3}(?:\.\d{1,3}){3}\b|[a-f0-9]{0,4}:[a-f0-9:]+:|candidate:|fingerprint|sdp|cookie|authorization|private.key|certificate|credential|\b(?:token|secret|password|passwd|code|verifier|nonce|signature|key)\s*[:=]/i.test(
      value,
    )
  ) {
    return m.relay_diag_hidden();
  }
  return value;
}

export function relayEndpoint(value: string) {
  if (!value) return m.relay_unassigned();
  try {
    const url = new URL(value);
    if (
      !["https:", "wss:"].includes(url.protocol) ||
      url.username ||
      url.password ||
      url.search ||
      url.hash ||
      url.hostname.includes(":") ||
      /^\d+(\.\d+){3}$/.test(url.hostname)
    )
      return m.relay_endpoint_hidden();
    return url.origin + (url.pathname === "/" ? "" : url.pathname);
  } catch {
    return m.relay_endpoint_hidden();
  }
}
