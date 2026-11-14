import {
  clipboardFiles,
  copyText as writeClipboard,
} from "../../platform/clipboard";
import { browserDraftStorage } from "./draft-storage";
import type { SubmissionProgress } from "../../contracts/schemas";
import type {
  DraftOperation,
  TerminalDraft,
  TerminalDrafts,
  TextInsertion,
} from "./terminal-draft.svelte";
import type { HybridView } from "../sessions/hybrid";
import type { SessionInfo } from "../../contracts/schemas";
import type { TerminalController } from "./terminal-search";
import { newSubmissionId } from "./terminal-protocol";
import { hasTouchInput } from "../../platform/touch-input";
import type { Connection } from "../../platform/ws";

export interface SessionHybridView {
  readonly view: HybridView;
  readonly missing: boolean;
  readonly info: SessionInfo | undefined;
  readonly chat: { readonly connection: Connection };
}

export function createHeaderTerminalView(
  hybrid: SessionHybridView,
  t: TerminalController,
  terminalShown: () => boolean,
) {
  return {
    get info() {
      return hybrid.info;
    },
    get connection() {
      return terminalShown() ? t.connection : hybrid.chat.connection;
    },
    get exitCode() {
      return terminalShown() ? t.exitCode : null;
    },
    get restoring() {
      return t.restoring;
    },
    get canFind() {
      return t.canFind;
    },
    scrollToBottom: () => t.scrollToBottom(),
    clear: () => t.clear(),
    resync: () => t.resync(),
  };
}

export function deriveSendUnavailable(t: TerminalController): string {
  return t.connection === "missing"
    ? "会话不存在，无法发送"
    : t.exitCode !== null || t.connection === "exited"
      ? "会话已退出，可编辑或复制草稿"
      : !t.ready
        ? "终端未就绪，草稿不会自动发送"
        : "";
}

export function deriveQuickUnavailable(t: TerminalController): string {
  return t.exitCode !== null || t.connection === "exited"
    ? "会话已退出，无法发送"
    : !t.ready
      ? "终端未就绪，暂不能发送"
      : "";
}

export function deriveChatUploadUnavailable(hybrid: SessionHybridView): string {
  return hybrid.missing
    ? "会话不存在，无法上传文件"
    : hybrid.info?.status === "exited"
      ? "会话已退出，无法上传文件"
      : "";
}

export function createDraftReporter(
  draft: TerminalDraft,
  drafts: TerminalDrafts,
  isMounted: () => boolean,
  setError: (msg: string) => void,
  setNotice: (msg: string) => void,
) {
  return (id: string, announce: boolean) => (state: SubmissionProgress) => {
    drafts.updateSubmission(draft, id, state);
    if (!isMounted()) return;
    if (state === "written") {
      setError("");
      if (announce) setNotice("上次提交的草稿已写入终端。");
    } else if (draftMessages[state]) setError(draftMessages[state]);
  };
}

export function submitSessionDraft(
  draft: TerminalDraft,
  drafts: TerminalDrafts,
  t: TerminalController,
  resend: boolean,
  report: (
    id: string,
    announce: boolean,
  ) => (state: SubmissionProgress) => void,
  setError: (msg: string) => void,
  setNotice: (msg: string) => void,
) {
  if (!draft.enabled || !draft.text || !drafts.valid(draft)) return;
  const pending = draft.submission;
  if (pending && (!resend || pending.state !== "unknown")) return;
  const id = pending?.id ?? newSubmissionId();
  if (!pending) drafts.beginSubmission(draft, id);
  setNotice("");
  if (t.submit(id, draft.text, true, report(id, false))) setError("");
  else {
    if (!pending) drafts.releaseSubmission(draft);
    setError("终端未就绪，草稿未发送；请恢复连接后手动重试。");
  }
}

const draftStorage = browserDraftStorage();

export function savedStorageLabel(
  persisted: boolean,
  oversized: string,
): string {
  return persisted && draftStorage.enabled
    ? "已保存在本机"
    : draftStorage.enabled
      ? oversized
      : "仅当前页面，刷新后丢失";
}

const draftMessages: Partial<Record<SubmissionProgress, string>> = {
  failed: "草稿未写入终端，可修改后重新发送。",
  missing:
    "服务端没有这次提交的记录：可能未送达，或记录已过期。请检查终端后决定是否重新发送。",
  partial: "草稿只有部分写入终端，提交键未写入；请检查终端后再处理草稿。",
};

export interface SessionClipboardContext {
  id: string;
  isMounted: () => boolean;
  isActiveAndVisible: () => boolean;
  isTerminalVisible: () => boolean;
  draft: TerminalDraft;
  drafts: TerminalDrafts;
  insertText: (text: string, operation?: DraftOperation) => TextInsertion;
  uploadFiles: (
    files: FileList | File[],
    operation?: DraftOperation,
  ) => Promise<void>;
  focusComposer: () => void;
  setPendingCopy: (text: string | null) => void;
  openCopyDialog: (text: string) => void;
  openPasteDialog: (operation: DraftOperation) => void;
  setError: (msg: string) => void;
}

export async function copySessionText(
  text: string,
  userGesture: boolean,
  ctx: SessionClipboardContext,
): Promise<void> {
  if (!ctx.isMounted()) return;
  // Keep only the latest background request. It must neither replace the
  // clipboard nor open a focus-trapped dialog in another pane.
  if (!ctx.isActiveAndVisible() || (!userGesture && !ctx.isTerminalVisible())) {
    ctx.setPendingCopy(text);
    return;
  }
  if (window.isSecureContext && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return;
    } catch {
      // Permission denied or a missing transient activation.
    }
  }
  // A toolbar click or Ctrl/Cmd+C is a gesture. The legacy execCommand path
  // only fills the gap on plain-HTTP LAN/VPN origins; a secure origin that
  // denied the Clipboard API keeps the manual text box fallback.
  if (userGesture && !window.isSecureContext && (await writeClipboard(text)))
    return;
  if (!ctx.isMounted()) return;
  if (!ctx.isActiveAndVisible() || (!userGesture && !ctx.isTerminalVisible())) {
    ctx.setPendingCopy(text);
    return;
  }
  ctx.openCopyDialog(text);
}

export async function pasteFromSessionClipboard(
  ctx: SessionClipboardContext,
): Promise<void> {
  const operation = ctx.drafts.begin(ctx.draft);
  try {
    if (
      matchMedia("(pointer: fine)").matches &&
      typeof navigator.clipboard?.read === "function"
    ) {
      const items = await navigator.clipboard.read();
      if (!ctx.isMounted()) return;
      const files = await clipboardFiles(items);
      if (files.length) {
        if (ctx.isMounted()) await ctx.uploadFiles(files, operation);
        return;
      }
      const textItem = items.find((item) => item.types.includes("text/plain"));
      if (textItem) {
        const text = await (await textItem.getType("text/plain")).text();
        if (ctx.isMounted()) ctx.insertText(text, operation);
        return;
      }
    }
    const text = await navigator.clipboard.readText();
    if (ctx.isMounted()) ctx.insertText(text, operation);
  } catch {
    if (ctx.isMounted()) {
      if (ctx.drafts.destination(ctx.draft, operation) === "draft") {
        ctx.setError(
          ctx.draft.enabled
            ? "无法读取剪贴板，请在现有草稿框内手动粘贴。"
            : "无法读取剪贴板，请重新开启草稿模式后在草稿框内手动粘贴。",
        );
        if (ctx.draft.enabled && ctx.isActiveAndVisible() && !hasTouchInput())
          ctx.focusComposer();
      } else if (ctx.drafts.destination(ctx.draft, operation) !== "failed") {
        ctx.openPasteDialog(operation);
      }
    }
  }
}

export function captureFindReturnFocus(): HTMLElement | null {
  const active = document.activeElement;
  return active instanceof HTMLElement &&
    (active.matches(".xterm-helper-textarea") || active.closest(".draft-pane"))
    ? active
    : null;
}

export function isEditableActiveElement(): boolean {
  const active = document.activeElement;
  return (
    active instanceof HTMLInputElement ||
    active instanceof HTMLTextAreaElement ||
    (active instanceof HTMLElement && active.isContentEditable)
  );
}

export function clearFindFocus(
  clearFind: () => void,
  hasTouch: boolean,
  canFocus: () => boolean,
  returnFocus: HTMLElement | null,
  fallbackFocus: () => void,
  afterTick: (fn: () => void) => void,
) {
  clearFind();
  if (document.activeElement instanceof HTMLElement)
    document.activeElement.blur();
  if (!hasTouch)
    afterTick(() => {
      if (!canFocus()) return;
      if (returnFocus?.isConnected) returnFocus.focus();
      else fallbackFocus();
    });
}

export function createSessionDraftSender(
  draft: TerminalDraft,
  drafts: TerminalDrafts,
  terminal: () => TerminalController,
  report: Parameters<typeof submitSessionDraft>[4],
  setError: (message: string) => void,
  setNotice: (message: string) => void,
) {
  let sending = false;
  return (resend = false) => {
    if (sending) return;
    sending = true;
    try {
      submitSessionDraft(
        draft,
        drafts,
        terminal(),
        resend,
        report,
        setError,
        setNotice,
      );
    } finally {
      sending = false;
    }
  };
}
