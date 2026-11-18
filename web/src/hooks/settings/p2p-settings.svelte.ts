import { onMount } from "svelte";
import { m } from "$lib/paraglide/messages.js";
import { api } from "../../platform/api";
import { errorMessage } from "../../platform/error";
import { requestFeatureHostRefresh } from "../../platform/feature-host.svelte";
import {
  refreshP2PAfterSettingsChange,
  refreshP2PStatus,
} from "../../platform/p2p.svelte";
import {
  P2PSettingsUpdateSchema,
  P2PStatusSchema,
  type P2PSettingsUpdate,
  type P2PStatus,
} from "../../contracts/schemas";
import { LatestRequest } from "../shared/remote-request.svelte";

const PORTMAP_POLL_MS = 2000;

export interface P2PSettingsController {
  status: P2PStatus | undefined;
  saving: boolean;
  failure: string;
  feedback: string;
  revision: number;
}

export function useP2PSettings() {
  const ui = $state<P2PSettingsController>({
    status: undefined,
    saving: false,
    failure: "",
    feedback: "",
    revision: 0,
  });
  const requests = new LatestRequest();
  let mutationFailed = false;
  let pollTimer: ReturnType<typeof setTimeout> | undefined;

  function schedulePortMapPoll(next: P2PStatus | undefined) {
    clearTimeout(pollTimer);
    if (next?.portMap.state === "probing" && !requests.disposed) {
      pollTimer = setTimeout(() => {
        if (document.hidden) {
          schedulePortMapPoll(ui.status);
          return;
        }
        void refresh();
      }, PORTMAP_POLL_MS);
    }
  }

  async function refresh() {
    if (ui.saving || requests.disposed) return;
    clearTimeout(pollTimer);
    const ticket = requests.begin();
    try {
      const next = await api(
        "/p2p",
        P2PStatusSchema,
        "GET",
        undefined,
        ticket.signal,
      );
      if (!ticket.current()) return;
      if (!ui.status) ui.revision++;
      ui.status = next;
      if (!mutationFailed) ui.failure = "";
      refreshP2PStatus(next);
    } catch (cause) {
      if (ticket.reportable() && !mutationFailed)
        ui.failure = errorMessage(cause);
    } finally {
      if (ticket.current()) schedulePortMapPoll(ui.status);
    }
  }

  async function applyChange(
    method: "PUT" | "DELETE",
    payload?: P2PSettingsUpdate,
  ) {
    if (ui.saving || requests.disposed) return false;
    clearTimeout(pollTimer);
    ui.saving = true;
    mutationFailed = false;
    ui.failure = "";
    ui.feedback = "";
    const ticket = requests.begin();
    try {
      const next = await api(
        "/p2p/settings",
        P2PStatusSchema,
        method,
        payload,
        ticket.signal,
      );
      if (!ticket.current()) return false;
      ui.status = next;
      ui.revision++;
      refreshP2PAfterSettingsChange(next);
      await requestFeatureHostRefresh();
      if (!ticket.current()) return false;
      ui.feedback =
        method === "DELETE"
          ? m.s2_p2p_feedback_reset()
          : m.s2_p2p_feedback_saved();
      return true;
    } catch (cause) {
      if (ticket.reportable()) {
        mutationFailed = true;
        ui.failure = errorMessage(cause);
      }
      return false;
    } finally {
      if (ticket.current()) {
        ui.saving = false;
        schedulePortMapPoll(ui.status);
      }
    }
  }

  onMount(() => {
    const handleVisibility = () => {
      if (
        !document.hidden &&
        !requests.disposed &&
        (!ui.status || ui.status.portMap.state === "probing")
      ) {
        void refresh();
      }
    };
    document.addEventListener("visibilitychange", handleVisibility);
    if (!document.hidden) void refresh();
    return () => {
      document.removeEventListener("visibilitychange", handleVisibility);
      requests.dispose();
      clearTimeout(pollTimer);
    };
  });

  return {
    ui,
    refresh,
    save: (draft: P2PSettingsUpdate) =>
      applyChange("PUT", P2PSettingsUpdateSchema.parse(draft)),
    reset: () => applyChange("DELETE"),
  };
}
