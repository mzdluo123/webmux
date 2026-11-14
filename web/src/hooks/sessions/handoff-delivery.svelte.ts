import type { HandoffDelivery, HandoffPreview } from "../../contracts/schemas";
import {
  isHandoffComplete,
  sendReviewedHandoff,
} from "../../domains/sessions/handoff-recovery";
import { APIError } from "../../platform/api";
import { errorMessage } from "../../platform/error";
import { newRequestId } from "../../platform/ids";
import { threadAPI } from "../../platform/thread-api";

interface HandoffDeliveryEditor {
  preview: HandoffPreview | null;
  busy: boolean;
  conflict: boolean;
  error: string;
  readonly dirty: boolean;
  readonly choiceChanged: boolean;
  hasSource: () => boolean;
  isDisposed: () => boolean;
  save: () => Promise<boolean>;
  remember: () => void;
  nextTask: () => void;
}

export function createHandoffDelivery(
  sourceId: string,
  editor: HandoffDeliveryEditor,
) {
  let value = $state<HandoffDelivery | null>(null);
  let lastRequestId = "";
  const locked = $derived(
    value !== null && ["pending", "unknown"].includes(value.status),
  );
  const sealed = $derived(isHandoffComplete(value));

  async function refresh(handoffId: string, signal: AbortSignal) {
    try {
      const next = await threadAPI.delivery(handoffId, signal);
      if (!editor.isDisposed() && editor.preview?.id === handoffId) {
        const completed = sealed;
        value = next;
        if (!completed && isHandoffComplete(next)) {
          if (!editor.dirty && !editor.choiceChanged) editor.nextTask();
          else editor.remember();
        }
        if (lastRequestId === next.clientRequestId) lastRequestId = "";
      }
    } catch (cause) {
      if (!(cause instanceof APIError && cause.status === 404)) throw cause;
    }
  }

  async function send() {
    if (
      !editor.preview ||
      !editor.hasSource() ||
      editor.busy ||
      editor.conflict ||
      editor.choiceChanged ||
      (value !== null &&
        ["unknown", "inline", "injected"].includes(value.status))
    )
      return;
    editor.busy = true;
    editor.error = "";
    try {
      if (!(await editor.save())) return;
      if (editor.dirty || editor.choiceChanged || editor.conflict) {
        editor.error = "草稿仍有更改，尚未发送。请检查保存后的预览。";
        return;
      }
      const reviewed = editor.preview;
      const p = await threadAPI.get(reviewed.id);
      if (p.revision !== reviewed.revision) {
        editor.conflict = true;
        editor.error =
          "服务端已有其他版本，尚未发送。请读取服务端版本或保留编辑重新预览。";
        editor.remember();
        return;
      }
      editor.preview = p;
      if (
        (p.stale || p.budget.overBudget) &&
        !["pending", "failed"].includes(value?.status ?? "")
      )
        return;
      // Pending preparations resume their original receipt after refresh;
      // only a known failed attempt receives a new explicit retry command.
      const retry =
        value?.status === "failed" ||
        (value?.status === "pending" && value.retry);
      const requestId =
        value?.status === "pending"
          ? value.clientRequestId
          : (lastRequestId ||= newRequestId());
      value = await sendReviewedHandoff(sourceId, p, requestId, retry);
      if (sealed && !editor.dirty && !editor.choiceChanged) editor.nextTask();
      lastRequestId = "";
    } catch (cause) {
      if (!editor.isDisposed()) editor.error = errorMessage(cause);
    } finally {
      if (!editor.isDisposed()) editor.busy = false;
    }
  }

  return {
    get value() {
      return value;
    },
    set value(next: HandoffDelivery | null) {
      value = next;
    },
    get locked() {
      return locked;
    },
    get sealed() {
      return sealed;
    },
    reset() {
      value = null;
      lastRequestId = "";
    },
    refresh,
    send,
  };
}
