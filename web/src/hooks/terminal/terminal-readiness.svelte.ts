import type { Terminal } from "@xterm/xterm";

// Mounting xterm does not mean its socket and replay can accept user input.
// Keep the native editable honest without disabling automatic device replies.
export function useTerminalReadiness(
  getTerminal: () => Terminal | undefined,
  canInput: () => boolean,
) {
  $effect(() => {
    const textarea = getTerminal()?.textarea;
    const ready = canInput();
    if (!textarea) return;
    textarea.readOnly = !ready;
    textarea.setAttribute("aria-disabled", String(!ready));
    const blockInput = (event: Event) => {
      if (!textarea.readOnly) return;
      // An exited or reconnecting terminal still permits copying its output.
      if (
        event instanceof KeyboardEvent &&
        (event.ctrlKey || event.metaKey) &&
        event.key.toLowerCase() === "c"
      )
        return;
      event.preventDefault();
      event.stopImmediatePropagation();
    };
    for (const type of ["keydown", "beforeinput", "paste"])
      textarea.addEventListener(type, blockInput, true);
    return () => {
      for (const type of ["keydown", "beforeinput", "paste"])
        textarea.removeEventListener(type, blockInput, true);
    };
  });
}
