import { onDestroy, onMount, tick } from "svelte";
import { m } from "$lib/paraglide/messages.js";
import type { HybridSession } from "../sessions/hybrid-types";
import type { TerminalController } from "../../domains/terminal/terminal-search";
import type { TerminalDraft } from "../../domains/terminal/terminal-draft.svelte";
import type { QuickCommand } from "../../contracts/schemas";
import type {
  XrAction,
  XrSessionSource,
  XrTextPanel,
} from "../../domains/terminal/xr-workspace";
import { errorMessage } from "../../platform/error";
import { findXrSessionWorkspace } from "./xr-session.svelte";
import type { xrChatPanel } from "./xr-chat-content";
import { xrNoticePanel } from "./xr-notice-content";

export function useXrSessionSource(options: {
  id: string;
  terminal: TerminalController;
  hybrid: HybridSession;
  draft: TerminalDraft;
  chatPanel?: typeof xrChatPanel;
  quickCommands(this: void): readonly QuickCommand[];
  codeAvailable(this: void): boolean;
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

  async function returnTo2D(reveal?: () => void) {
    if (!workspace || !(await workspace.returnTo2D(options.id))) return;
    await tick();
    reveal?.();
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
    if (!hybrid.known || !options.chatPanel) return undefined;
    const panel = options.chatPanel({
      model: hybrid.chat.model,
      state: hybrid.view === "chat" ? hybrid.chat.state : undefined,
      draft: draft.text,
      answering,
      answer,
      openForm: () => returnTo2D(options.showChat),
    });
    const recovering = hybrid.view === "recovery";
    if (hybrid.view === "chat") return panel;
    return {
      ...panel,
      title: recovering
        ? m.s4_xr_recover_session_readonly()
        : m.s4_xr_chat_history_readonly(),
      lines: [
        {
          text: recovering
            ? m.s4_xr_return_2d_to_recover_hint()
            : hybrid.chat.model.entries.length
              ? m.s4_xr_in_terminal_chat_readonly_hint()
              : m.s4_xr_in_terminal_no_chat_hint(),
          tone: "muted" as const,
        },
        ...(recovering && !hybrid.chat.model.entries.length ? [] : panel.lines),
      ],
      actions: [
        {
          id: "chat-2d",
          label: recovering
            ? m.s4_xr_return_2d_recover_btn()
            : m.s4_xr_return_2d_switch_chat_btn(),
          run: () =>
            returnTo2D(recovering ? undefined : () => hybrid.request("chat")),
        },
      ],
    };
  });
  const noticePanel = $derived(
    xrNoticePanel({
      error: options.notice(),
      notices: hybrid.notices,
      awaitingChoice: hybrid.awaitingChoice,
      forceSwitchNote: hybrid.known
        ? (hybrid.chat.state?.forceSwitchNote ?? "")
        : "",
      openChoice: () => returnTo2D(options.showSwitchChoice),
    }),
  );
  const panels = $derived.by((): readonly XrTextPanel[] => [
    ...(chatPanel ? [chatPanel] : []),
    ...(!hybrid.known && draft.enabled
      ? [
          {
            id: "draft",
            kind: "chat" as const,
            title: m.s4_xr_terminal_draft_title(),
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
      ...(options.codeAvailable()
        ? [
            {
              id: "code",
              label: m.s4_xr_open_code_workspace(),
              run: options.showCode,
            },
          ]
        : []),
      ...(!inChat
        ? [
            {
              id: "draft",
              label: draft.enabled
                ? m.s4_exit_draft_mode()
                : m.s4_enable_draft_mode(),
              run: options.toggleDraft,
            },
          ]
        : []),
      ...(inChat || draft.enabled
        ? [
            {
              id: "send-draft",
              label: m.s4_xr_send_draft(),
              disabled:
                !!draft.submission ||
                (!draft.text.trim() && (!inChat || !draft.attachments.length)),
              run: inChat ? options.sendChat : options.sendDraft,
            },
          ]
        : []),
      ...(!t.controlling && !inChat
        ? [
            {
              id: "claim",
              label: m.s4_xr_claim_terminal_control(),
              run: () => t.claim(),
            },
          ]
        : []),
      ...options.quickCommands().map((command) => ({
        id: `quick:${command.id}`,
        label: m.s4_xr_quick_command_label({ name: command.name }),
        run: () => options.runQuick(command),
      })),
      ...(hybrid.known
        ? [
            {
              id: "switch",
              label: inChat ? m.s4_switch_to_terminal() : m.s4_switch_to_chat(),
              run: () =>
                returnTo2D(() => hybrid.request(inChat ? "terminal" : "chat")),
            },
          ]
        : []),
      ...(hybrid.confirming
        ? [
            {
              id: "confirm-switch",
              label: m.s4_xr_confirm_switch_view(),
              confirm: m.s4_xr_confirm_switch_interrupt(),
              run: () => hybrid.confirm(),
            },
            {
              id: "cancel-switch",
              label: m.s4_xr_cancel_switch(),
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
      if (hybrid.view === "recovery") return m.s4_xr_status_recovery_readonly();
      const viewLabel =
        hybrid.view === "chat" ? m.s4_tab_chat() : m.s4_tab_terminal();
      const conn =
        hybrid.view === "chat" ? hybrid.chat.connection : t.connection;
      const ctrl = t.controlling ? ` · ${m.s4_xr_status_controller()}` : "";
      return `${viewLabel} · ${conn}${ctrl}`;
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
