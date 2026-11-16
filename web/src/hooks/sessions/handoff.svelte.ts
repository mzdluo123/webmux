import { m } from "$lib/paraglide/messages.js";
import { APIError } from "../../platform/api";
import { errorMessage } from "../../platform/error";
import { threadAPI } from "../../platform/thread-api";
import type {
  HandoffLocation,
  HandoffPreview,
  HandoffOperation,
  SessionInfo,
} from "../../contracts/schemas";
import type { TargetKind } from "../../domains/sessions/new-session-defaults";
export const emptyHandoffLocation = (): HandoffLocation => ({
  cwd: "",
  repositoryId: "",
  worktreeId: "",
  newWorktreeBase: "",
  newWorktreeBranch: "",
});
export function createHandoffEditor(sourceId: string, fork: boolean) {
  let source = $state<SessionInfo | null>(null);
  let configId = $state("");
  let targetSessionId = $state("");
  let model = $state("");
  let task = $state("");
  let location = $state<HandoffLocation>(emptyHandoffLocation());
  let locationKind = $state<TargetKind>("directory");
  let preview = $state<HandoffPreview | null>(null);
  let reviewedRequestKey = $state("");
  let operation = $state<HandoffOperation | null>(null);
  let uncertainOperationId = $state("");
  let uncertainInstanceId = $state("");
  let loading = $state(true);
  let busy = $state(false);
  let error = $state("");
  let disposed = false;
  const lifetime = new AbortController();
  const request = $derived({
    configId: targetSessionId ? "" : configId,
    targetSessionId,
    agentConfig: (model ? { model } : {}) as Record<string, string>,
    userText: task,
    location: targetSessionId ? emptyHandoffLocation() : location,
  });
  const choiceChanged = $derived(
    preview !== null && reviewedRequestKey !== JSON.stringify(request),
  );
  const sealed = $derived(operation !== null || uncertainOperationId !== "");
  const active = $derived(
    operation?.state === "preparing" || operation?.state === "sending",
  );
  $effect(() => {
    void threadAPI
      .source(sourceId, lifetime.signal)
      .then((value) => {
        if (disposed) return;
        source = value;
        location = { ...emptyHandoffLocation(), cwd: value.cwd };
      })
      .catch((e) => {
        if (!disposed) error = errorMessage(e);
      })
      .finally(() => {
        if (!disposed) loading = false;
      });
    return () => {
      disposed = true;
      lifetime.abort();
    };
  });
  async function generate() {
    if (busy || active) return;
    busy = true;
    error = "";
    try {
      const key = JSON.stringify(request);
      preview = await threadAPI.preview(sourceId, request, lifetime.signal);
      reviewedRequestKey = key;
      operation = null;
      uncertainOperationId = "";
      uncertainInstanceId = "";
    } catch (e) {
      if (!disposed) error = errorMessage(e);
    } finally {
      busy = false;
    }
  }
  async function refresh() {
    const id = operation?.id || uncertainOperationId;
    if (!id || busy) return;
    busy = true;
    try {
      const next = await threadAPI.delivery(id, lifetime.signal);
      if (
        (operation?.instanceId || uncertainInstanceId) &&
        next.instanceId !== (operation?.instanceId || uncertainInstanceId)
      ) {
        preview = null;
        operation = null;
        error = m.handoff_scope_expired();
      } else {
        operation = next;
        uncertainOperationId = "";
        uncertainInstanceId = "";
        error = "";
      }
    } catch (e) {
      if (!disposed) {
        error = errorMessage(e);
        if (e instanceof APIError && e.status === 410) {
          preview = null;
          operation = null;
          uncertainOperationId = "";
          uncertainInstanceId = "";
        }
      }
    } finally {
      busy = false;
    }
  }
  async function send() {
    if (!preview || busy || sealed || choiceChanged) return;
    const reviewed = preview;
    let attempted = false;
    busy = true;
    error = "";
    // One request starts the operation. Reconnection only observes its handle.
    try {
      await threadAPI.get(reviewed.id, lifetime.signal);
      attempted = true;
      operation = await threadAPI.send(
        reviewed.id,
        reviewed.operationId,
        lifetime.signal,
      );
    } catch (e) {
      if (!disposed) {
        error = attempted
          ? m.handoff_unknown_result() + " " + errorMessage(e)
          : errorMessage(e);
        if (attempted) {
          uncertainOperationId = reviewed.operationId;
          uncertainInstanceId = reviewed.instanceId;
        }
        preview = null;
      }
    } finally {
      busy = false;
    }
  }
  async function cancel() {
    if (!operation || busy) return;
    busy = true;
    try {
      operation = await threadAPI.cancel(operation.id, lifetime.signal);
    } catch (e) {
      if (!disposed) error = errorMessage(e);
    } finally {
      busy = false;
    }
  }
  return {
    get source() {
      return source;
    },
    get loading() {
      return loading;
    },
    get busy() {
      return busy;
    },
    get active() {
      return active;
    },
    get error() {
      return error;
    },
    get configId() {
      return configId;
    },
    set configId(v: string) {
      configId = v;
    },
    get targetSessionId() {
      return targetSessionId;
    },
    set targetSessionId(v: string) {
      targetSessionId = v;
      model = "";
    },
    get model() {
      return model;
    },
    set model(v: string) {
      model = v;
    },
    get task() {
      return task;
    },
    set task(v: string) {
      task = v;
    },
    get location() {
      return location;
    },
    set location(v: HandoffLocation) {
      location = v;
    },
    get locationKind() {
      return locationKind;
    },
    set locationKind(v: TargetKind) {
      locationKind = v;
    },
    get preview() {
      return preview;
    },
    get uncertain() {
      return uncertainOperationId !== "";
    },
    get operation() {
      return operation;
    },
    get choiceChanged() {
      return choiceChanged;
    },
    get sealed() {
      return sealed;
    },
    fork,
    generate,
    send,
    refresh,
    cancel,
  };
}
export type HandoffEditor = ReturnType<typeof createHandoffEditor>;
