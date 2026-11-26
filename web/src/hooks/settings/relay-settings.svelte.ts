import { m } from "$lib/paraglide/messages.js";
import { onMount } from "svelte";
import { api, APIError } from "../../platform/api";
import {
  RelayStatusSchema,
  RelayPairingSchema,
  RelayEnrollmentRequestSchema,
  RelayUpdateSchema,
  RelayTransportRequestSchema,
  type RelayDataProtocol,
  type RelayStatus,
  type RelayPairing,
} from "../../contracts/relay-schemas";
import { clearRelayAuth, relayAuthStatus } from "../../platform/relay-auth";
import { LatestRequest } from "../shared/remote-request.svelte";

export interface RelaySettingsView {
  status: RelayStatus | undefined;
  pairing: RelayPairing | undefined;
  loading: boolean;
  busy: boolean;
  phase:
    | "idle"
    | "loading"
    | "submitting"
    | "verifying"
    | "failed"
    | "completed";
  error: string;
  notice: string;
}

function failureMessage(cause: unknown) {
  if (cause === "auth") return m.relay_error_auth();
  if (!(cause instanceof APIError)) return m.relay_error_host();
  if (cause.notPersisted) return m.relay_error_persist();
  switch (cause.status) {
    case 401:
      return m.relay_error_auth();
    case 403:
      return m.relay_error_denied();
    case 409:
      return m.relay_error_conflict();
    case 410:
      return m.relay_error_expired();
    case 429:
      return m.relay_error_rate();
    case 503:
      return m.relay_error_unavailable();
    default:
      return m.relay_error_unknown();
  }
}

export function useRelaySettings() {
  let failure = $state<unknown>();
  const ui = $state<RelaySettingsView>({
    status: undefined,
    pairing: undefined,
    loading: true,
    phase: "loading",
    get busy() {
      return this.phase === "submitting" || this.phase === "verifying";
    },
    get error() {
      return failure === undefined ? "" : failureMessage(failure);
    },
    get notice() {
      if (this.phase === "submitting") return "";
      if (this.status?.operation.phase === "completed") {
        if (this.status.operation.kind !== "delete")
          return m.relay_notice_completed();
        if (!this.status.paired)
          return this.status.operation.accessClosed
            ? m.relay_deleted_closed()
            : m.relay_deleted_unclosed();
      }
      if (this.pairing?.phase === "paired") return m.relay_notice_paired();
      if (this.pairing?.phase === "cancelled")
        return m.relay_notice_cancelled();
      return "";
    },
  });
  const requests = new LatestRequest();
  let timer: ReturnType<typeof setTimeout> | undefined;
  let mounted = false;
  let reading = false;
  let failures = 0;
  let authInvalid = false;
  let rejected = false;

  function accept(status: RelayStatus) {
    const previous = ui.status;
    ui.status = status;
    if (previous && (!status.paired || !status.enabled)) clearRelayAuth();
    else if (previous?.controlReady && !status.controlReady) {
      clearRelayAuth("reconnecting");
      if (relayAuthStatus().relay)
        window.dispatchEvent(new Event("webmux:relay-check"));
    }
  }

  function schedule() {
    clearTimeout(timer);
    if (mounted && !authInvalid)
      timer = setTimeout(
        () => void refresh(),
        Math.min(15_000, 2000 * 2 ** failures),
      );
  }

  async function refresh() {
    if (ui.phase === "submitting" || reading || requests.disposed) return;
    clearTimeout(timer);
    reading = true;
    const ticket = requests.begin();
    const signal = AbortSignal.any([
      ticket.signal,
      AbortSignal.timeout(10_000),
    ]);
    try {
      const [status, pairing] = await Promise.all([
        api("/relay", RelayStatusSchema, "GET", undefined, signal),
        api("/relay/enrollments", RelayPairingSchema, "GET", undefined, signal),
      ]);
      if (!ticket.current()) return;
      const completed =
        status.operation.phase === "completed" &&
        (ui.status?.operation.phase !== "completed" ||
          ui.status.operation.kind !== status.operation.kind ||
          ui.status.operation.deleted !== status.operation.deleted ||
          ui.status.operation.accessClosed !== status.operation.accessClosed ||
          ui.status.enabled !== status.enabled);
      if (
        (status.paired && !ui.status?.paired) ||
        (["paired", "cancelled"].includes(pairing.phase) &&
          pairing.phase !== ui.pairing?.phase) ||
        completed
      )
        rejected = false;
      accept(status);
      ui.pairing = pairing;
      if (!rejected) failure = undefined;
      failures = 0;
      authInvalid = false;
      ui.phase = rejected
        ? "failed"
        : (pairing.phase === "paired" && !status.paired) ||
            status.operation.phase === "submitting"
          ? "verifying"
          : status.operation.phase === "failed" ||
              status.operation.phase === "uncertain"
            ? "failed"
            : status.operation.phase === "completed"
              ? "completed"
              : "idle";
    } catch (cause) {
      if (ticket.reportable()) {
        failure = cause;
        failures = Math.min(failures + 1, 3);
        ui.phase = ui.phase === "verifying" ? "verifying" : "failed";
      }
    } finally {
      if (ticket.current()) {
        reading = false;
        ui.loading = false;
        schedule();
      }
    }
  }

  async function submit<T>(
    request: (signal: AbortSignal) => Promise<T>,
    apply: (value: T) => void,
    timeoutMs = 10_000,
  ) {
    clearTimeout(timer);
    reading = false;
    ui.phase = "submitting";
    failure = undefined;
    rejected = false;
    const ticket = requests.begin();
    try {
      const result = await request(
        AbortSignal.any([ticket.signal, AbortSignal.timeout(timeoutMs)]),
      );
      if (ticket.current()) apply(result);
    } catch (cause) {
      if (ticket.reportable()) {
        failure = cause;
        rejected =
          cause instanceof APIError &&
          cause.status < 500 &&
          cause.status !== 429;
      }
    } finally {
      if (ticket.current()) {
        ui.phase = "verifying";
        await refresh();
      }
    }
  }

  async function pairingMutation(method: "POST" | "DELETE") {
    if (ui.phase === "submitting" || ui.loading || requests.disposed) return;
    const active =
      ui.pairing &&
      ["preparing", "waiting", "retrying", "uncertain", "cancelling"].includes(
        ui.pairing.phase,
      );
    if (
      method === "POST" &&
      (ui.busy || active || !ui.pairing || ui.status?.paired)
    )
      return;
    await submit(
      (signal) =>
        api(
          "/relay/enrollments",
          RelayPairingSchema,
          method,
          method === "POST"
            ? RelayEnrollmentRequestSchema.parse({ confirmPublic: true })
            : {},
          signal,
        ),
      (pairing) => {
        ui.pairing = pairing;
      },
    );
  }

  async function mutate(path: string, method: string, body: unknown) {
    if (ui.busy || ui.loading || requests.disposed) return;
    window.dispatchEvent(new Event("webmux:relay-revoked"));
    await submit(
      (signal) => api(path, RelayStatusSchema, method, body, signal),
      accept,
    );
  }

  onMount(() => {
    mounted = true;
    const invalidate = () => {
      authInvalid = true;
      requests.cancel();
      reading = false;
      clearTimeout(timer);
      ui.status = undefined;
      ui.pairing = undefined;
      ui.phase = "failed";
      ui.loading = false;
      failure = "auth";
    };
    const restored = () => {
      if (authInvalid) void refresh();
    };
    const visible = () => {
      if (!document.hidden) void refresh();
    };
    window.addEventListener("webmux:auth", invalidate);
    window.addEventListener("webmux:relay-restored", restored);
    document.addEventListener("visibilitychange", visible);
    void refresh();
    return () => {
      mounted = false;
      window.removeEventListener("webmux:auth", invalidate);
      window.removeEventListener("webmux:relay-restored", restored);
      document.removeEventListener("visibilitychange", visible);
      clearTimeout(timer);
      requests.dispose();
    };
  });

  return {
    ui,
    refresh,
    pair: () => pairingMutation("POST"),
    cancelPairing: () => pairingMutation("DELETE"),
    setEnabled: (enabled: boolean) =>
      mutate(
        "/relay",
        "PUT",
        RelayUpdateSchema.parse({ enabled, confirmPublic: enabled }),
      ),
    selectTransport: async (protocol: RelayDataProtocol) => {
      if (ui.busy || ui.loading || requests.disposed) return;
      // Probing a parallel route leaves the current browser grant intact.
      await submit(
        (signal) =>
          api(
            "/relay/transport",
            RelayStatusSchema,
            "POST",
            RelayTransportRequestSchema.parse({ protocol }),
            signal,
          ),
        accept,
        60_000,
      );
    },
    disconnect: () => mutate("/relay/disconnect", "POST", {}),
    reconnect: () => mutate("/relay/reconnect", "POST", {}),
    deleteBinding: () => mutate("/relay/binding", "DELETE", {}),
  };
}
