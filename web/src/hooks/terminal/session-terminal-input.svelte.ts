import { tick } from "svelte";
import type { TerminalController } from "../../domains/terminal/terminal-search";
import type {
  TerminalDraft,
  TerminalDrafts,
} from "../../domains/terminal/terminal-draft.svelte";
import { createSessionTerminalDraft } from "./session-terminal-draft";

/** The sole original editor remains the physical-keyboard/IME input target. */
export function createSessionTerminalInput(options: {
  draft: TerminalDraft;
  drafts: TerminalDrafts;
  terminal: () => TerminalController;
  mounted: () => boolean;
  active: () => boolean;
  immersive: () => boolean;
  chat: () => boolean;
  desktop: () => boolean;
  root: () => HTMLElement | undefined;
  focusChat: () => void;
  focusComposer: () => void;
  editComposer: (input: string) => void;
  hideCode: () => void;
  restoreFocus: () => void;
  error: (message: string) => void;
  notice: (message: string) => void;
}) {
  const { draft, drafts } = options;
  let xrFocused = $state(false);
  let currentTarget: (() => boolean) | undefined;
  const draftInput = createSessionTerminalDraft({
    draft,
    drafts,
    terminal: options.terminal,
    isMounted: options.mounted,
    isChatView: options.chat,
    editComposerKey: options.editComposer,
    restoreInputFocus: options.restoreFocus,
    setError: options.error,
    setNotice: options.notice,
  });

  function focusOriginal() {
    if (options.chat()) options.focusChat();
    else if (draft.enabled) options.focusComposer();
    else options.terminal().focus();
  }
  function blurXr() {
    currentTarget = undefined;
    xrFocused = false;
    const target = document.activeElement;
    if (target instanceof HTMLElement && options.root()?.contains(target))
      target.blur();
  }

  return {
    beginInput: draftInput.beginInput,
    insertText: draftInput.insertText,
    controlNotice: draftInput.controlNotice,
    softInput: draftInput.softInput,
    toggleDraft: draftInput.toggleDraft,
    sendDraft: draftInput.sendDraft,
    releaseDraftSubmission: draftInput.releaseDraftSubmission,
    resumeDraftSubmission: draftInput.resumeDraftSubmission,
    get draftSaved() {
      return draftInput.draftSaved;
    },
    get scratchSaved() {
      return draftInput.scratchSaved;
    },
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
    insertReference(text: string) {
      if (options.chat())
        return draftInput.insertText(text, draftInput.beginInput());
      const result = draftInput.insertText(text);
      if (result === "terminal" && !options.desktop()) {
        options.hideCode();
        void tick().then(() => options.terminal().fit());
      }
      return result;
    },
  };
}
