import { tick } from "svelte";
import type { TerminalController } from "../../domains/terminal/terminal-search";
import type {
  DraftOperation,
  TerminalDraft,
  TerminalDrafts,
  TextInsertion,
} from "../../domains/terminal/terminal-draft.svelte";

/** The sole original editor remains the physical-keyboard/IME input target. */
export function createSessionTerminalInput(options: {
  draft: TerminalDraft;
  drafts: TerminalDrafts;
  terminal(): TerminalController;
  mounted(): boolean;
  active(): boolean;
  immersive(): boolean;
  chat(): boolean;
  desktop(): boolean;
  root(): HTMLElement | undefined;
  focusChat(): void;
  focusComposer(): void;
  editComposer(input: string): void;
  hideCode(): void;
  restoreFocus(): void;
  error(message: string): void;
}) {
  const { draft, drafts } = options;
  let xrFocused = $state(false);
  let currentTarget: (() => boolean) | undefined;

  function focusOriginal() {
    if (options.chat()) options.focusChat();
    else if (draft.enabled) options.focusComposer();
    else options.terminal().focus();
  }
  function beginInput(): DraftOperation {
    const operation = drafts.begin(draft);
    return options.chat() ? { ...operation, startedInDraft: true } : operation;
  }
  function insertText(
    text: string,
    operation = drafts.begin(draft),
  ): TextInsertion {
    if (!options.mounted()) return "failed";
    return drafts.insert(draft, text, operation, (value) =>
      options.terminal().paste(value),
    );
  }
  function blurXr() {
    currentTarget = undefined;
    xrFocused = false;
    const target = document.activeElement;
    if (target instanceof HTMLElement && options.root()?.contains(target))
      target.blur();
  }

  return {
    get xrFocused() {
      return xrFocused;
    },
    active() {
      return options.immersive() ? !!currentTarget?.() : options.active();
    },
    focus() {
      if (!options.active() || options.immersive()) return;
      focusOriginal();
    },
    async focusXr(isCurrent: () => boolean) {
      if (!isCurrent()) return;
      currentTarget = isCurrent;
      xrFocused = true;
      await tick();
      if (!options.mounted() || currentTarget !== isCurrent || !isCurrent())
        return;
      focusOriginal();
      if (!isCurrent() && currentTarget === isCurrent) blurXr();
    },
    blurXr,
    guardXrInput(event: Event) {
      if (!options.immersive() || currentTarget?.()) return;
      event.preventDefault();
      event.stopImmediatePropagation();
      blurXr();
    },
    beginInput,
    insertText,
    insertReference(text: string) {
      if (options.chat()) return insertText(text, beginInput());
      const result = insertText(text);
      if (result === "terminal" && !options.desktop()) {
        options.hideCode();
        void tick().then(() => options.terminal().fit());
      }
      return result;
    },
    controlNotice() {
      options.error(
        "终端控制需先退出草稿模式；草稿框内可正常复制、选择和编辑。",
      );
    },
    softInput(input: string) {
      if (draft.enabled) options.editComposer(input);
      else options.terminal().send(input);
    },
    toggleDraft() {
      drafts.setEnabled(draft, !draft.enabled);
      options.terminal().resetModifiers();
      if (draft.enabled) options.restoreFocus();
    },
  };
}
