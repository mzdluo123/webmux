import { onMount } from "svelte";
import type { HybridSession } from "../sessions/hybrid-types";
import type { TerminalController } from "../../domains/terminal/terminal-search";
import type { TerminalDraft } from "../../domains/terminal/terminal-draft.svelte";
import type { QuickCommand } from "../../contracts/schemas";
import type {
  XrAction,
  XrSessionSource,
  XrTextPanel,
} from "../../domains/terminal/xr-workspace";
import { quickCommands } from "../../domains/settings/quick-commands.svelte";
import { errorMessage } from "../../platform/error";
import { findXrSessionWorkspace } from "./xr-session.svelte";
import { xrChatPanel } from "./xr-chat-content";

export function useXrSessionSource(options: {
  id: string;
  terminal: TerminalController;
  hybrid: HybridSession;
  draft: TerminalDraft;
  codePanels(this: void): readonly XrTextPanel[];
  focus(this: void): void;
  editDraft(this: void, input: string): void;
  editChat(this: void, input: string): void;
  sendDraft(this: void): void;
  sendChat(this: void): void;
  toggleDraft(this: void): void;
  showCode(this: void): void;
  showChat(this: void): void;
  runQuick(this: void, command: QuickCommand): Promise<void>;
  error(this: void, message: string): void;
  notice(this: void): string;
}) {
  const workspace = findXrSessionWorkspace();
  const t = options.terminal;
  const hybrid = options.hybrid;
  const draft = options.draft;
  let answering = $state(false);

  async function answer(requestId: string, optionId: string) {
    if (answering) return;
    answering = true;
    try {
      await hybrid.actions.answer(requestId, { outcome: "selected", optionId });
    } catch (error) {
      options.error(errorMessage(error));
    } finally {
      answering = false;
    }
  }

  const chatPanel = $derived(
    xrChatPanel({
      model: hybrid.chat.model,
      state: hybrid.chat.state,
      draft: draft.text,
      answering,
      answer,
      openForm: () => {
        options.showChat();
        workspace?.header(options.id).exit();
      },
    }),
  );
  const panels = $derived.by((): readonly XrTextPanel[] => [
    ...(hybrid.known ? [chatPanel] : []),
    ...(!hybrid.known && draft.enabled
      ? [
          {
            id: "draft",
            kind: "chat" as const,
            title: "终端草稿",
            lines: draft.text.split("\n").map((text) => ({ text })),
          },
        ]
      : []),
    ...options.codePanels(),
    ...(options.notice()
      ? [
          {
            id: "notice",
            kind: "chat" as const,
            title: "会话提示",
            lines: options
              .notice()
              .split("\n")
              .map((text) => ({ text, tone: "error" as const })),
          },
        ]
      : []),
  ]);
  const actions = $derived.by((): readonly XrAction[] => {
    const inChat = hybrid.view === "chat";
    return [
      { id: "code", label: "打开代码工作区", run: options.showCode },
      ...(!inChat
        ? [
            {
              id: "draft",
              label: draft.enabled ? "退出草稿模式" : "开启草稿模式",
              run: options.toggleDraft,
            },
          ]
        : []),
      ...(inChat || draft.enabled
        ? [
            {
              id: "send-draft",
              label: "发送草稿",
              disabled:
                !!draft.submission ||
                (!draft.text.trim() && (!inChat || !draft.attachments.length)),
              run: inChat ? options.sendChat : options.sendDraft,
            },
          ]
        : []),
      ...(!t.controlling && !inChat
        ? [{ id: "claim", label: "取得终端控制", run: () => t.claim() }]
        : []),
      ...quickCommands.list.map((command) => ({
        id: `quick:${command.id}`,
        label: `快捷指令：${command.name}`,
        run: () => options.runQuick(command),
      })),
      ...(hybrid.known
        ? [
            {
              id: "switch",
              label: inChat ? "切换到终端" : "切换到聊天",
              run: () => hybrid.request(inChat ? "terminal" : "chat"),
            },
          ]
        : []),
      ...(hybrid.confirming
        ? [
            {
              id: "confirm-switch",
              label: "确认切换视图",
              confirm: "切换会中断当前任务。确认继续？",
              run: () => hybrid.confirm(),
            },
            {
              id: "cancel-switch",
              label: "取消切换",
              run: () => hybrid.cancelConfirm(),
            },
          ]
        : []),
    ];
  });
  const source: XrSessionSource = {
    id: options.id,
    get title() {
      return hybrid.info?.title || t.info?.title || options.id;
    },
    get status() {
      return `${hybrid.view === "chat" ? "聊天" : "终端"} · ${hybrid.view === "chat" ? hybrid.chat.connection : t.connection}${t.controlling ? " · 控制端" : ""}`;
    },
    get terminal() {
      return t.emulator;
    },
    get graphicsUnsupported() {
      return t.graphicsUnsupported;
    },
    get modifiers() {
      return t.modifiers;
    },
    get shiftLocked() {
      return t.shiftLocked;
    },
    toggleModifier: (key) => t.toggleModifier(key),
    resetModifiers: () => t.resetModifiers(),
    lockModifier: (key) => t.lockModifier(key),
    panels: () => panels,
    actions: () => actions,
    input(value) {
      if (hybrid.view === "chat") {
        const input = t.consumeInput(value);
        if (input === "\x03") {
          void hybrid.actions
            .cancel()
            .catch((error: unknown) => options.error(errorMessage(error)));
        } else options.editChat(input);
      } else if (draft.enabled) options.editDraft(t.consumeInput(value));
      else t.send(value);
    },
    focus: options.focus,
    latest: () => t.scrollToBottom(),
    scroll: (lines) => t.scrollLines(lines),
    resize: (cols, rows) => t.resizeGrid(cols, rows),
    setImmersive: (value) => t.setImmersive(value),
  };
  onMount(() => workspace?.register(source));
  return workspace?.header(options.id);
}
