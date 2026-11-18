import {
  AgentToolsReportSchema,
  type AgentToolsReport,
} from "../../contracts/schemas";
import { APIError, api } from "../../platform/api";
import { errorMessage } from "../../platform/error";

export type AgentToolsViewState =
  | "loading"
  | "ready"
  | "unavailable"
  | "unsupported";

export type AgentToolsSeed = {
  report?: AgentToolsReport;
  state?: AgentToolsViewState;
  error?: string;
};

export function createAgentToolsInspector(
  getSeed: () => AgentToolsSeed | undefined,
  onRefreshed?: () => void,
) {
  let fetched = $state<{
    report?: AgentToolsReport;
    state: AgentToolsViewState;
    error: string;
  }>({
    state: "loading",
    error: "",
  });
  let overridden = $state(false);
  let checkError = $state("");
  let checking = $state(false);
  let rescanning = $state(false);

  const active = $derived.by(() => {
    const seed = getSeed();
    if (overridden || !seed) return fetched;
    return {
      report: seed.report,
      state: seed.state ?? (seed.report ? "ready" : ("unsupported" as const)),
      error: seed.error ?? "",
    };
  });

  const busy = $derived(active.state === "loading" || checking || rescanning);

  async function load(refresh = false) {
    const previous = active.report;
    if (refresh && busy) return;
    if (refresh && previous) {
      fetched = { ...active };
      rescanning = true;
    } else if (checking || rescanning) {
      return;
    } else {
      fetched = { report: previous, state: "loading", error: "" };
    }
    overridden = true;
    checkError = "";
    try {
      const path = refresh ? "/agent-tools?refresh=1" : "/agent-tools";
      const next = await api(path, AgentToolsReportSchema, "GET");
      fetched = { report: next, state: "ready", error: "" };
      if (refresh) onRefreshed?.();
    } catch (err) {
      const text = errorMessage(err);
      if (previous) {
        fetched = { report: previous, state: "ready", error: "" };
        checkError = text;
      } else if (!refresh && err instanceof APIError && err.status === 404) {
        fetched = { report: undefined, state: "unsupported", error: "" };
      } else {
        fetched = { report: undefined, state: "unavailable", error: text };
      }
    } finally {
      rescanning = false;
    }
  }

  async function checkUpdates() {
    if (busy) return;
    const previous = active.report;
    fetched = { ...active };
    checking = true;
    overridden = true;
    checkError = "";
    try {
      const next = await api(
        "/agent-tools/check",
        AgentToolsReportSchema,
        "POST",
        {
          refreshLocal: false,
        },
      );
      fetched = { report: next, state: "ready", error: "" };
    } catch (err) {
      fetched = {
        report: previous,
        state: previous ? "ready" : "unavailable",
        error: previous ? "" : errorMessage(err),
      };
      if (previous) {
        checkError = errorMessage(err);
      }
    } finally {
      checking = false;
    }
  }

  return {
    get report() {
      return active.report;
    },
    get state() {
      return active.state;
    },
    get error() {
      return active.error;
    },
    get checkError() {
      return checkError;
    },
    get checking() {
      return checking;
    },
    get rescanning() {
      return rescanning;
    },
    get busy() {
      return busy;
    },
    load,
    checkUpdates,
  };
}
