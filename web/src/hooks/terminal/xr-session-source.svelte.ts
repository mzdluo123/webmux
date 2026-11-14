import { onDestroy, onMount, tick } from "svelte";
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
import { xrNoticePanel } from "./xr-notice-content";

export function useXrSessionSource(options: {
  id: string;
  terminal: TerminalController;
  hybrid: HybridSession;
  draft: TerminalDraft;
  codePanels(this: void): readonly XrTextPanel[];
  focus(this: void, isCurrent: () => boolean): void | Promise<void>;
  blur(this: void): void;
  editDraft(this: void, input: string): void;
  editChat(this: void, input: string): void;
  sendDraft(this: void): void;
  sendChat(this: void): void;
  toggleDraft(this: void): void;
  showCode(this: void): void;
  showChat(this: void): void;
  showSwitchChoice(this: void): void;
  runQuick(this: void, command: QuickCommand): Promise<void>;
  error(this: void, message: string): void;
  notice(this: void): string;
}) {
  const workspace = findXrSessionWorkspace();
  const t = options.terminal;
  const hybrid = options.hybrid;
  const draft = options.draft;
  let answering = $state(false);
  let immersive = workspace?.active ?? false;
  let alive = true;
  let focusSerial = 0;
  let inputBinding:
    | {
        kind: "terminal" | XrTextPanel["kind"];
        sourceId?: string;
        current: () => boolean;
      }
    | undefined;
  // This runs during owner creation, before useTerminal's async DOM attach.
  t.setImmersive(immersive);

  function canInput(kind: "terminal" | XrTextPanel["kind"], sourceId?: string) {
    if (!alive || hybrid.missing) return false;
    if (kind === "chat" && sourceId === "chat")
      return (
        hybrid.view === "chat" &&
        !!hybrid.chat.state &&
        hybrid.chat.connection === "connected"
      );
    if (kind === "chat" && sourceId === "draft")
      return hybrid.view === "terminal" && draft.enabled && t.ready;
    return (
      kind === "terminal" &&
      (sourceId === undefined || sourceId === "terminal") &&
      hybrid.view === "terminal" &&
      t.ready
    );
  }

  function blur() {
    focusSerial++;
    inputBinding = undefined;
    options.blur();
  }

  async function returnTo2D(reveal: () => void) {
    if (!workspace || !(await workspace.returnTo2D(options.id))) return;
    await tick();
    reveal();
  }

  async function answer(requestId: string, optionId: string) {
    const request = hybrid.chat.state?.requests.find(
      (current) => current.requestId === requestId,
    );
    if (
      answering ||
      hybrid.view !== "chat" ||
      hybrid.chat.connection !== "connected" ||
      request?.kind !== "permission" ||
      !request?.options?.some((option) => option.optionId === optionId)
    )
      return;
    answering = true;
    try {
      await hybrid.actions.answer(requestId, { outcome: "selected", optionId });
    } catch (error) {
      options.error(errorMessage(error));
    } finally {
      answering = false;
    }
  }

  const chatPanel = $derived.by(() => {
    const panel = xrChatPanel({
      model: hybrid.chat.model,
      state: hybrid.view === "chat" ? hybrid.chat.state : undefined,
      draft: draft.text,
      answering,
      answer,
      openForm: () => returnTo2D(options.showChat),
    });
    if (hybrid.view === "chat") return panel;
    return {
      ...panel,
      title: "聊天历史 · 只读",
      lines: [
        {
          text: hybrid.chat.model.entries.length
            ? "当前在终端端运行；此处仅显示已有聊天历史，不能输入。"
            : "当前在终端端运行，暂无可用聊天历史。",
          tone: "muted" as const,
        },
        ...panel.lines,
      ],
      actions: [
        {
          id: "chat-2d",
          label: "返回 2D 切换到聊天",
          run: () => returnTo2D(() => hybrid.request("chat")),
        },
      ],
    };
  });
  const noticePanel = $derived(
    xrNoticePanel({
      error: options.notice(),
      notices: hybrid.notices,
      awaitingChoice: hybrid.awaitingChoice,
      forceSwitchNote: hybrid.chat.state?.forceSwitchNote ?? "",
      openChoice: () => returnTo2D(options.showSwitchChoice),
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
    ...(noticePanel ? [noticePanel] : []),
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
              run: () =>
                returnTo2D(() => hybrid.request(inChat ? "terminal" : "chat")),
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
      ...(noticePanel?.actions ?? []),
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
    get primaryKind() {
      const info = hybrid.info ?? t.info;
      if (!info) return undefined;
      return info.views.includes("chat") ? "chat" : "terminal";
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
      if (
        !immersive ||
        !inputBinding?.current() ||
        !canInput(inputBinding.kind, inputBinding.sourceId)
      )
        return;
      if (hybrid.view === "chat") {
        const enterSubmits = !t.modifiers.shift;
        const input = t.consumeInput(value);
        if (input === "\x03") {
          void hybrid.actions
            .cancel()
            .catch((error: unknown) => options.error(errorMessage(error)));
        } else if (input === "\r" && enterSubmits) options.sendChat();
        else options.editChat(input);
      } else if (draft.enabled) options.editDraft(t.consumeInput(value));
      else t.send(value);
    },
    canInput,
    async focus(kind, sourceId, isCurrent = () => true) {
      const attempt = ++focusSerial;
      const current = () =>
        alive &&
        immersive &&
        attempt === focusSerial &&
        isCurrent() &&
        canInput(kind, sourceId);
      if (!current()) return;
      inputBinding = { kind, sourceId, current };
      await options.focus(current);
      if (!current() && attempt === focusSerial) blur();
    },
    blur,
    latest: () => t.scrollToBottom(),
    scroll: (lines) => t.scrollLines(lines),
    resize: (cols, rows) => t.resizeGrid(cols, rows),
    setImmersive(value) {
      if (immersive !== value) blur();
      immersive = value;
      t.setImmersive(value);
    },
  };
  onMount(() => workspace?.register(source));
  onDestroy(() => {
    alive = false;
    blur();
  });
  return workspace?.header(options.id);
}
