import { api } from "../../platform/api";
import { errorMessage } from "../../platform/error";
import type { LaunchConfigsState } from "../../domains/settings/launch-configs";
import { setLaunchPreviews } from "../../domains/settings/launch-previews.svelte";
import {
  AgentToolsReportSchema,
  LaunchConfigsSchema,
} from "../../contracts/schemas";

/** The saved launch configs, shared by the new-session dialog and quick start. */
export interface LaunchConfigs {
  readonly state: LaunchConfigsState;
  /**
   * Reloads them, e.g. to pick up changes from other devices. `initial`
   * marks the first load, whose failure resets cards to the global preview.
   */
  load(initial?: boolean, refreshTools?: boolean): Promise<void>;
  dispose(): void;
}

export function createLaunchConfigs(): LaunchConfigs {
  let state = $state.raw<LaunchConfigsState>({ status: "loading" });
  let disposed = false;
  let loadSeq = 0;
  const controller = new AbortController();
  return {
    get state() {
      return state;
    },
    async load(initial = false, refreshTools = false) {
      const seq = ++loadSeq;
      const previousTools =
        state.status === "ready" ? state.agentTools : undefined;
      if (state.status !== "ready") state = { status: "loading" };
      const toolsPath = refreshTools
        ? "/agent-tools?refresh=1"
        : "/agent-tools";
      const toolsPromise = api(
        toolsPath,
        AgentToolsReportSchema,
        "GET",
        undefined,
        controller.signal,
      )
        .then((report) => report.tools)
        .catch(() => previousTools);
      try {
        const configs = await api(
          "/launch-configs",
          LaunchConfigsSchema,
          "GET",
          undefined,
          controller.signal,
        );
        if (disposed || seq !== loadSeq) return;
        setLaunchPreviews(configs);
        state = {
          status: "ready",
          configs,
          agentTools: previousTools,
          agentToolsLoading: previousTools === undefined,
        };
        const agentTools = await toolsPromise;
        if (disposed || seq !== loadSeq) return;
        state = {
          status: "ready",
          configs,
          agentTools,
          agentToolsLoading: false,
        };
      } catch (err) {
        if (disposed || seq !== loadSeq) return;
        state = { status: "unavailable", error: errorMessage(err) };
        // Cards keep the last loaded previews while a reload is in flight or
        // fails; only an unavailable store on page load falls back to global.
        if (initial) setLaunchPreviews([]);
      }
    },
    dispose() {
      disposed = true;
      controller.abort();
    },
  };
}
