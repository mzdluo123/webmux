import { getContext, setContext } from "svelte";

const key = Symbol("session switcher");
type OpenSwitcher = (invoker: HTMLElement | null) => void;

export function provideSessionSwitcher(open: OpenSwitcher) {
  setContext(key, open);
}

export function findSessionSwitcher() {
  return getContext<OpenSwitcher | undefined>(key);
}

const apple = /Macintosh|iPhone|iPad/.test(navigator.userAgent);
export const sessionSwitcherKeys = apple ? "Meta+K" : "Control+Alt+K";
export const sessionSwitcherKeyLabel = apple ? "⌘ K" : "Ctrl + Alt + K";

function isSessionSwitcherShortcut(event: KeyboardEvent) {
  return (
    !event.repeat &&
    !event.isComposing &&
    event.keyCode !== 229 &&
    !event.getModifierState("AltGraph") &&
    !event.shiftKey &&
    event.key.toLowerCase() === "k" &&
    (apple
      ? event.metaKey && !event.ctrlKey && !event.altKey
      : event.ctrlKey && event.altKey && !event.metaKey)
  );
}

/** Capture the global shortcut without stealing keys from an open overlay. */
export function watchSessionSwitcherShortcut(
  available: () => boolean,
  open: OpenSwitcher,
) {
  const shortcut = (event: KeyboardEvent) => {
    if (
      !available() ||
      !isSessionSwitcherShortcut(event) ||
      document.querySelector(
        '[role="dialog"][data-state="open"], [role="alertdialog"][data-state="open"], [role="menu"][data-state="open"], dialog[open]',
      )
    )
      return;
    // xterm and the pane shortcuts must never receive this chord.
    event.preventDefault();
    event.stopPropagation();
    const active = document.activeElement;
    open(active instanceof HTMLElement ? active : null);
  };
  window.addEventListener("keydown", shortcut, true);
  return () => window.removeEventListener("keydown", shortcut, true);
}
