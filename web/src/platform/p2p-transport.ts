import { m } from "$lib/paraglide/messages.js";
import {
  P2P_CHUNK_BYTES,
  StreamFrameReassembler,
  encodeStreamCloseFrame,
  encodeStreamFrames,
} from "./p2p-framing";
import {
  P2PRPCRequestHeaderSchema,
  P2PRPCResponseHeaderSchema,
} from "../contracts/schemas";
import {
  SOCKET_CLOSED,
  SOCKET_CONNECTING,
  SOCKET_OPEN,
  interactiveStreamKind,
  TransportUploadError,
  type ClientTransport,
  type PeerTransportPath,
  type TransportSocket,
  type UploadResult,
} from "./transport";

export interface P2PTransportOptions {
  pc: RTCPeerConnection;
  fallback: ClientTransport;
  path: PeerTransportPath;
  interactiveOnly: boolean;
  onChannelFailure?: () => void;
}

function stripCompressionQuery(path: string): string {
  const [base = "", search = ""] = path.split("?");
  const params = new URLSearchParams(search);
  params.delete("compress");
  if (interactiveStreamKind(path) === "terminal") params.set("zreplay", "1");
  const query = params.toString();
  return query ? `${base}?${query}` : base;
}

function normalizeHeaders(headers?: HeadersInit): Record<string, string> {
  const result: Record<string, string> = {};
  new Headers(headers).forEach((value, key) => {
    result[key] = value;
  });
  return result;
}

async function serializeBody(
  body: BodyInit | null | undefined,
  headers: Record<string, string>,
) {
  if (body === undefined || body === null) return new Uint8Array(0);
  if (typeof body === "string") return new TextEncoder().encode(body);
  if (body instanceof Uint8Array) return new Uint8Array(body);
  if (body instanceof ArrayBuffer) return new Uint8Array(body.slice(0));
  const response = new Response(body);
  const contentType = response.headers.get("Content-Type");
  if (
    contentType &&
    !Object.keys(headers).some((key) => key.toLowerCase() === "content-type")
  ) {
    headers["Content-Type"] = contentType;
  }
  return new Uint8Array(await response.arrayBuffer());
}

async function waitBufferedLow(
  dc: RTCDataChannel,
  signal?: AbortSignal | null,
) {
  while (dc.readyState === "open" && dc.bufferedAmount > 2 * 1024 * 1024) {
    signal?.throwIfAborted();
    const { promise, resolve, reject } = Promise.withResolvers<void>();
    const cleanup = () => {
      dc.removeEventListener("bufferedamountlow", low);
      dc.removeEventListener("close", closed);
      dc.removeEventListener("error", closed);
      signal?.removeEventListener("abort", aborted);
    };
    const low = () => {
      cleanup();
      resolve();
    };
    const closed = () => {
      cleanup();
      reject(new Error("P2P DataChannel closed"));
    };
    const aborted = () => {
      cleanup();
      reject(new DOMException("Aborted", "AbortError"));
    };
    dc.addEventListener("bufferedamountlow", low, { once: true });
    dc.addEventListener("close", closed, { once: true });
    dc.addEventListener("error", closed, { once: true });
    signal?.addEventListener("abort", aborted, { once: true });
    await promise;
  }
}

interface RPCResponse {
  status: number;
  statusText: string;
  headers: Record<string, string>;
  body: Uint8Array<ArrayBuffer>;
}

export function createP2PTransport(
  options: P2PTransportOptions,
): ClientTransport {
  const { pc, fallback, path: selectedPath, onChannelFailure } = options;
  let rpcCounter = 0;
  // The existing Token/local path retains its RPC protocol. Owner grants never
  // enter this code: their policy is selected before serializing any bulk body.
  async function executeRPC(
    path: string,
    method: string,
    headers: Record<string, string>,
    body: Uint8Array<ArrayBuffer>,
    signal?: AbortSignal | null,
    onProgress?: (percent: number) => void,
  ): Promise<RPCResponse> {
    signal?.throwIfAborted();
    if (
      pc.connectionState !== "connected" &&
      pc.connectionState !== "connecting"
    ) {
      throw new Error("PeerConnection not active");
    }
    const dc = pc.createDataChannel(`rpc:${++rpcCounter}`, { ordered: true });
    dc.binaryType = "arraybuffer";
    dc.bufferedAmountLowThreshold = 256 * 1024;
    const { promise, resolve, reject } = Promise.withResolvers<RPCResponse>();
    let settled = false;
    let header: Omit<RPCResponse, "body"> | null = null;
    const chunks: Uint8Array[] = [];
    let bytes = 0;
    const finish = (done: () => void) => {
      if (settled) return;
      settled = true;
      signal?.removeEventListener("abort", aborted);
      dc.close();
      done();
    };
    const aborted = () =>
      finish(() => reject(new DOMException("Aborted", "AbortError")));
    signal?.addEventListener("abort", aborted, { once: true });
    dc.onerror = () => {
      onChannelFailure?.();
      finish(() => reject(new Error("P2P RPC DataChannel error")));
    };
    dc.onclose = () => {
      if (settled) return;
      onChannelFailure?.();
      finish(() => reject(new Error("P2P RPC DataChannel closed early")));
    };
    dc.onmessage = (event) => {
      if (settled) return;
      if (!header) {
        try {
          if (typeof event.data !== "string")
            throw new Error("Invalid P2P RPC header");
          header = P2PRPCResponseHeaderSchema.parse(JSON.parse(event.data));
        } catch (error) {
          finish(() => reject(error));
        }
        return;
      }
      const chunk =
        event.data instanceof ArrayBuffer
          ? new Uint8Array(event.data)
          : new TextEncoder().encode(String(event.data));
      if (chunk.byteLength === 0) {
        const body = new Uint8Array(bytes);
        let offset = 0;
        for (const part of chunks) {
          body.set(part, offset);
          offset += part.byteLength;
        }
        const received = header;
        finish(() => resolve({ ...received, body }));
        return;
      }
      chunks.push(chunk);
      bytes += chunk.byteLength;
    };
    dc.onopen = () => {
      void (async () => {
        try {
          dc.send(
            JSON.stringify(
              P2PRPCRequestHeaderSchema.parse({
                method,
                path: `/api${path}`,
                headers,
                bodyLength: body.byteLength,
              }),
            ),
          );
          for (
            let offset = 0;
            offset < body.byteLength;
            offset += P2P_CHUNK_BYTES
          ) {
            await waitBufferedLow(dc, signal);
            const end = Math.min(offset + P2P_CHUNK_BYTES, body.byteLength);
            dc.send(body.subarray(offset, end));
            onProgress?.(
              Math.min(
                99,
                Math.round(
                  (Math.max(0, end - dc.bufferedAmount) / body.byteLength) *
                    100,
                ),
              ),
            );
          }
          await waitBufferedLow(dc, signal);
          dc.send(new Uint8Array(0));
          onProgress?.(100);
        } catch (error) {
          finish(() => reject(error));
        }
      })();
    };
    return promise;
  }
  return {
    get origin() {
      return fallback.origin;
    },
    async fetch(path, init) {
      if (
        options.interactiveOnly ||
        path === "/p2p" ||
        path.startsWith("/p2p/")
      ) {
        return fallback.fetch(path, init);
      }
      const method = (init.method ?? "GET").toUpperCase();
      const headers = normalizeHeaders(init.headers);
      const body = await serializeBody(init.body, headers);
      try {
        const response = await executeRPC(
          path,
          method,
          headers,
          body,
          init.signal,
        );
        return new Response(
          [204, 205, 304].includes(response.status) ? null : response.body,
          {
            status: response.status,
            statusText: response.statusText,
            headers: response.headers,
          },
        );
      } catch (error) {
        if (error instanceof DOMException && error.name === "AbortError")
          throw error;
        if (method === "GET" || method === "HEAD")
          return fallback.fetch(path, init);
        throw error;
      }
    },
    async upload(request): Promise<UploadResult> {
      if (options.interactiveOnly) return fallback.upload(request);
      if (request.signal.aborted) {
        throw new DOMException(m.s1_upload_cancelled(), "AbortError");
      }
      const headers: Record<string, string> = {
        Authorization: "Bearer " + request.token,
      };
      const body = await serializeBody(request.form, headers);
      if (request.signal.aborted) {
        throw new DOMException(m.s1_upload_cancelled(), "AbortError");
      }
      const controller = new AbortController();
      let timedOut = false;
      const timer = setTimeout(() => {
        timedOut = true;
        controller.abort();
      }, request.timeoutMs);
      const aborted = () => controller.abort();
      request.signal.addEventListener("abort", aborted, { once: true });
      try {
        const response = await executeRPC(
          request.path,
          "POST",
          headers,
          body,
          controller.signal,
          request.onProgress,
        );
        let parsed: unknown = null;
        if (response.body.byteLength > 0) {
          const text = new TextDecoder().decode(response.body);
          try {
            parsed = JSON.parse(text);
          } catch {
            parsed = text;
          }
        }
        return {
          status: response.status,
          statusText: response.statusText,
          body: parsed,
        };
      } catch {
        if (request.signal.aborted) {
          throw new DOMException(m.s1_upload_cancelled(), "AbortError");
        }
        if (timedOut) {
          throw new TransportUploadError("timeout");
        }
        throw new TransportUploadError("network");
      } finally {
        clearTimeout(timer);
        request.signal.removeEventListener("abort", aborted);
      }
    },
    openStream(path, streamOptions) {
      if (options.interactiveOnly && !interactiveStreamKind(path))
        return fallback.openStream(path, streamOptions);
      if (pc.connectionState !== "connected") {
        onChannelFailure?.();
        return fallback.openStream(path, streamOptions);
      }
      try {
        const dc = pc.createDataChannel(
          `stream:${stripCompressionQuery(path)}`,
          { ordered: true },
        );
        dc.binaryType = "arraybuffer";
        return adaptStreamDataChannel(dc, selectedPath, onChannelFailure);
      } catch {
        onChannelFailure?.();
        return fallback.openStream(path, streamOptions);
      }
    },
  };
}

function adaptStreamDataChannel(
  dc: RTCDataChannel,
  path: PeerTransportPath,
  onChannelFailure?: () => void,
): TransportSocket {
  const reassembler = new StreamFrameReassembler();
  let closed = false;
  let closeCode = 1006;
  let closeReason = "";
  const socket: TransportSocket = {
    transportPath: path,
    get readyState() {
      if (closed) return SOCKET_CLOSED;
      switch (dc.readyState) {
        case "connecting":
          return SOCKET_CONNECTING;
        case "open":
          return SOCKET_OPEN;
        default:
          return SOCKET_CLOSED;
      }
    },
    get bufferedAmount() {
      return dc.bufferedAmount;
    },
    onopen: null,
    onmessage: null,
    onclose: null,
    onerror: null,
    send(data) {
      if (closed || dc.readyState !== "open")
        throw new Error("P2P stream closed");
      if (dc.bufferedAmount + data.length > 1024 * 1024) {
        throw new Error("P2P stream backpressure");
      }
      for (const frame of encodeStreamFrames(data)) {
        dc.send(frame);
      }
    },
    close(code = 1000, reason = "") {
      if (closed) return;
      if (dc.readyState === "open") {
        try {
          dc.send(encodeStreamCloseFrame(code, reason));
        } catch {
          // ignore send errors during close
        }
      }
      emitClose(code, reason);
    },
  };

  function emitClose(code: number, reason: string) {
    if (closed) return;
    closed = true;
    closeCode = code;
    closeReason = reason;
    try {
      dc.close();
    } catch {
      // ignore
    }
    socket.onclose?.({ code: closeCode, reason: closeReason });
  }

  dc.onopen = () => {
    if (!closed) socket.onopen?.();
  };
  dc.onmessage = (event) => {
    if (closed) return;
    const raw =
      event.data instanceof ArrayBuffer
        ? event.data
        : new TextEncoder().encode(String(event.data)).buffer;
    const decoded = reassembler.feed(raw);
    if (!decoded) return;
    if (decoded.kind === "message") {
      socket.onmessage?.({ data: decoded.data });
    } else if (decoded.kind === "close") {
      emitClose(decoded.code, decoded.reason);
    } else {
      socket.onerror?.();
      emitClose(1002, decoded.message);
    }
  };
  dc.onerror = () => {
    onChannelFailure?.();
    if (!closed) socket.onerror?.();
  };
  dc.onclose = () => {
    if (!closed && closeCode === 1006) onChannelFailure?.();
    emitClose(closeCode, closeReason);
  };

  return socket;
}
