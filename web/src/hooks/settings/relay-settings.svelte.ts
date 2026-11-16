import { m } from "$lib/paraglide/messages.js";
import { onMount } from "svelte";
import { api, APIError } from "../../platform/api";
import { NoContentSchema } from "../../contracts/schemas";
import {
  RelayStatusSchema,
  RelayEnrollmentSchema,
  RelayEnrollmentRequestSchema,
  RelayPollRequestSchema,
  RelayPollResultSchema,
  RelayUpdateSchema,
  type RelayStatus,
  type RelayEnrollment,
} from "../../contracts/relay-schemas";
import { clearRelayAuth } from "../../platform/relay-auth";
import { LatestRequest } from "../shared/remote-request.svelte";

export interface RelaySettingsView {
  status: RelayStatus | undefined;
  enrollment: RelayEnrollment | undefined;
  loading: boolean;
  busy: boolean;
  error: string;
  notice: string;
}

function failureMessage(cause: unknown) {
  if (cause === "expired") return m.relay_error_expired();
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

function definitivePairingFailure(cause: unknown) {
  return (
    cause instanceof APIError &&
    [400, 401, 403, 404, 409, 410, 426].includes(cause.status)
  );
}

export function useRelaySettings() {
  let failure = $state<unknown>();
  let notice = $state<"paired" | "cancelled" | "completed" | "pending">();
  const ui = $state<RelaySettingsView>({
    status: undefined,
    enrollment: undefined,
    loading: true,
    busy: false,
    get error() {
      return failure === undefined ? "" : failureMessage(failure);
    },
    get notice() {
      switch (notice) {
        case "paired":
          return m.relay_notice_paired();
        case "cancelled":
          return m.relay_notice_cancelled();
        case "completed":
          return m.relay_notice_completed();
        case "pending":
          return m.relay_notice_pending();
        default:
          return "";
      }
    },
  });
  const requests = new LatestRequest();
  let timer: ReturnType<typeof setTimeout> | undefined;
  let pairingTimer: ReturnType<typeof setTimeout> | undefined;
  let pairingRevision = 0;
  let mounted = false;

  function accept(status: RelayStatus) {
    const previous = ui.status;
    ui.status = status;
    if (
      previous &&
      (!status.paired ||
        !status.enabled ||
        (previous.controlReady && !status.controlReady))
    )
      clearRelayAuth();
  }

  async function refresh() {
    if (ui.busy || requests.disposed) return;
    const ticket = requests.begin();
    try {
      const status = await api(
        "/relay",
        RelayStatusSchema,
        "GET",
        undefined,
        ticket.signal,
      );
      if (ticket.current()) accept(status);
    } catch (cause) {
      if (ticket.reportable()) failure = cause;
    } finally {
      if (ticket.current()) ui.loading = false;
    }
  }

  function clearPairing() {
    pairingRevision++;
    clearTimeout(pairingTimer);
    ui.enrollment = undefined;
  }

  async function pollEnrollment(enrollment: RelayEnrollment, revision: number) {
    if (!mounted || revision !== pairingRevision) return;
    if (Date.now() >= enrollment.expiresAt * 1000) {
      clearPairing();
      failure = "expired";
      return;
    }
    const ticket = requests.begin();
    try {
      const result = await api(
        "/relay/enrollments/poll",
        RelayPollResultSchema,
        "POST",
        { handle: enrollment.handle },
        ticket.signal,
        RelayPollRequestSchema,
      );
      if (!ticket.current() || revision !== pairingRevision) return;
      if ("enabled" in result) {
        clearPairing();
        accept(result);
        notice = "paired";
        return;
      }
      pairingTimer = setTimeout(
        () => void pollEnrollment(enrollment, revision),
        enrollment.pollIntervalSeconds * 1000,
      );
    } catch (cause) {
      if (!ticket.reportable() || revision !== pairingRevision) return;
      failure = cause;
      if (definitivePairingFailure(cause)) {
        clearPairing();
        void refresh();
      }
    }
  }

  async function pair(rebind: boolean) {
    if (ui.busy || ui.enrollment || requests.disposed) return;
    ui.busy = true;
    failure = undefined;
    notice = undefined;
    const revision = ++pairingRevision;
    const ticket = requests.begin();
    try {
      const enrollment = await api(
        "/relay/enrollments",
        RelayEnrollmentSchema,
        "POST",
        { rebind, confirmPublic: true },
        ticket.signal,
        RelayEnrollmentRequestSchema,
      );
      if (!ticket.current() || revision !== pairingRevision) return;
      if (rebind) clearRelayAuth();
      ui.enrollment = enrollment;
      pairingTimer = setTimeout(
        () => void pollEnrollment(enrollment, revision),
        enrollment.pollIntervalSeconds * 1000,
      );
    } catch (cause) {
      if (ticket.reportable()) failure = cause;
    } finally {
      if (ticket.current()) ui.busy = false;
    }
  }

  async function cancelPairing() {
    const enrollment = ui.enrollment;
    pairingRevision++;
    clearTimeout(pairingTimer);
    requests.cancel();
    if (!enrollment || requests.disposed) return;
    ui.busy = true;
    failure = undefined;
    const ticket = requests.begin();
    try {
      await api(
        "/relay/enrollments",
        NoContentSchema,
        "DELETE",
        { handle: enrollment.handle },
        ticket.signal,
        RelayPollRequestSchema,
      );
      if (ticket.current()) {
        clearPairing();
        notice = "cancelled";
      }
    } catch (cause) {
      if (ticket.reportable()) {
        failure = cause;
        if (definitivePairingFailure(cause)) clearPairing();
      }
    } finally {
      if (ticket.current()) {
        ui.busy = false;
        if (!ui.enrollment) void refresh();
      }
    }
  }

  async function mutate(path: string, method: string, body: unknown) {
    if (ui.busy || requests.disposed) return;
    clearPairing();
    ui.busy = true;
    failure = undefined;
    notice = undefined;
    window.dispatchEvent(new Event("webmux:relay-revoked"));
    const ticket = requests.begin();
    try {
      const result = await api(
        path,
        RelayPollResultSchema,
        method,
        body,
        ticket.signal,
      );
      if (!ticket.current()) return;
      if ("enabled" in result) {
        accept(result);
        notice = "completed";
      } else {
        notice = "pending";
      }
    } catch (cause) {
      if (ticket.reportable()) failure = cause;
    } finally {
      if (ticket.current()) {
        ui.busy = false;
        await refresh();
      }
    }
  }

  onMount(() => {
    mounted = true;
    const invalidate = () => {
      clearPairing();
      requests.cancel();
      ui.busy = false;
      ui.status = undefined;
      failure = "auth";
    };
    window.addEventListener("webmux:auth", invalidate);
    async function poll() {
      if (!ui.enrollment && !document.hidden) await refresh();
      if (mounted) timer = setTimeout(() => void poll(), 2000);
    }
    void poll();
    return () => {
      mounted = false;
      window.removeEventListener("webmux:auth", invalidate);
      const enrollment = ui.enrollment;
      clearPairing();
      clearTimeout(timer);
      requests.dispose();
      if (enrollment)
        void api(
          "/relay/enrollments",
          NoContentSchema,
          "DELETE",
          { handle: enrollment.handle },
          undefined,
          RelayPollRequestSchema,
        ).catch(() => undefined);
    };
  });

  return {
    ui,
    refresh,
    pair,
    cancelPairing,
    setEnabled: (enabled: boolean) =>
      mutate(
        "/relay",
        "PUT",
        RelayUpdateSchema.parse({ enabled, confirmPublic: enabled }),
      ),
    disconnect: () => mutate("/relay/disconnect", "POST", {}),
    reconnect: () => mutate("/relay/reconnect", "POST", {}),
    deleteBinding: () => mutate("/relay/binding", "DELETE", {}),
  };
}
