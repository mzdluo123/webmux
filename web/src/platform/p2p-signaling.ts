import {
  P2PPingResponseSchema,
  P2PSignalResponseSchema,
  type P2PICECandidate,
} from "../contracts/schemas";
import { BrowserICEEndpointSchema } from "../contracts/p2p-schemas";
import {
  authenticationHeaders,
  relayOwnerAuthenticated,
  relayAuthRevision,
} from "./relay-auth";
import { defaultBrowserTransport } from "./transport";

export const PING_TIMEOUT_MS = 4_000;
export const NEGOTIATION_TIMEOUT_MS = 30_000;

let pingSeq = 0;

export function nextP2PPingSequence() {
  return ++pingSeq;
}

async function withRequestTimeout<T>(
  signal: AbortSignal | undefined,
  timeoutMs: number,
  request: (signal: AbortSignal) => Promise<T>,
): Promise<T> {
  const controller = new AbortController();
  const timer = setTimeout(
    () => controller.abort(new Error("P2P request timeout")),
    timeoutMs,
  );
  const abort = () => {
    clearTimeout(timer);
    controller.abort(signal?.reason);
  };
  if (signal?.aborted) abort();
  else signal?.addEventListener("abort", abort, { once: true });
  try {
    controller.signal.throwIfAborted();
    return await request(controller.signal);
  } finally {
    clearTimeout(timer);
    signal?.removeEventListener("abort", abort);
  }
}

export async function sendSignalHttp(
  payload: {
    action: "offer" | "candidate" | "close";
    peerId?: string;
    sdp?: string;
    candidates?: P2PICECandidate[];
    peerConnectionId?: string;
  },
  signal?: AbortSignal,
) {
  return withRequestTimeout(
    signal,
    payload.action === "close" ? PING_TIMEOUT_MS : NEGOTIATION_TIMEOUT_MS,
    async (requestSignal) => {
      const res = await defaultBrowserTransport().fetch("/p2p/signal", {
        method: "POST",
        headers: {
          ...authenticationHeaders(),
          "Content-Type": "application/json",
        },
        body: JSON.stringify(payload),
        signal: requestSignal,
        credentials: "same-origin",
        cache: "no-store",
      });
      if (res.status === 401) window.dispatchEvent(new Event("webmux:auth"));
      if (!res.ok) {
        throw new Error(`P2P signal failed (${res.status})`);
      }
      return P2PSignalResponseSchema.parse(await res.json());
    },
  );
}

export async function measureBaselineHttpRtt(
  count = 3,
  signal?: AbortSignal,
): Promise<number[]> {
  const samples: number[] = [];
  for (let i = 0; i < count; i++) {
    const started = performance.now();
    const seq = nextP2PPingSequence();
    await withRequestTimeout(signal, PING_TIMEOUT_MS, async (requestSignal) => {
      const res = await defaultBrowserTransport().fetch(
        `/p2p/ping?seq=${seq}`,
        {
          method: "GET",
          headers: authenticationHeaders(),
          credentials: "same-origin",
          cache: "no-store",
          signal: requestSignal,
        },
      );
      if (res.status === 401) window.dispatchEvent(new Event("webmux:auth"));
      if (!res.ok) throw new Error(`HTTP ping failed (${res.status})`);
      const pong = P2PPingResponseSchema.parse(await res.json());
      if (pong.seq !== seq) throw new Error("Mismatched baseline pong");
      requestSignal.throwIfAborted();
    });
    samples.push(Math.max(0, Math.round(performance.now() - started)));
  }
  return samples;
}

export async function requestOwnerICE(
  peerConnectionId: string,
  authorizationRevision: number,
  signal?: AbortSignal,
) {
  const endpoint = await withRequestTimeout(
    signal,
    PING_TIMEOUT_MS,
    async (requestSignal) => {
      const res = await defaultBrowserTransport().fetch("/relay/ice", {
        method: "POST",
        credentials: "same-origin",
        cache: "no-store",
        headers: {
          ...authenticationHeaders(),
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ peerConnectionId }),
        signal: requestSignal,
      });
      if (res.status === 401) window.dispatchEvent(new Event("webmux:auth"));
      if (!res.ok) throw new Error(`ICE authorization failed (${res.status})`);
      return BrowserICEEndpointSchema.parse(await res.json());
    },
  );
  signal?.throwIfAborted();
  if (
    !relayOwnerAuthenticated() ||
    relayAuthRevision() !== authorizationRevision ||
    endpoint.expiresAt * 1000 <= Date.now()
  ) {
    throw new Error("ICE authorization expired");
  }
  return endpoint;
}
