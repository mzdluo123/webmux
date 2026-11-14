import { onMount, untrack } from "svelte";
import type { Terminal } from "@xterm/xterm";
import { SearchAddon } from "@xterm/addon-search";
import {
  executeTerminalFind,
  terminalFindError,
  terminalFindLimit,
  type TerminalFindOptions,
  type TerminalFindResults,
} from "../../domains/terminal/terminal-search";
import { terminalFont } from "./terminal-font-preference.svelte";
import {
  modifiedInput,
  type Modifiers,
} from "../../domains/terminal/terminal-keys";
import { loadTerminalFont } from "../../platform/fonts";
import { hasTouchInput } from "../../platform/touch-input";
import { attachTouchGestures } from "./touch";
import {
  createTerminalRenderer,
  type TerminalRenderer,
} from "./terminal-renderer";
import {
  attachTerminalInput,
  bindTerminalViewport,
  fitTerminalGrid,
  resizeTerminalGrid,
  toggleTerminalKeyboard,
  type TerminalClipboard,
  type TerminalUserInput,
} from "./terminal-input";
import { createTerminalSocket } from "./terminal-socket.svelte";
export type { TerminalController } from "../../domains/terminal/terminal-search";
import type { TerminalController } from "../../domains/terminal/terminal-search";

// A single parsed model and socket survive every DOM/XR presentation change.
export function useTerminal(
  getContainer: () => HTMLDivElement,
  id: string,
  clipboard: TerminalClipboard,
  isVisible: () => boolean = () => true,
  userInput?: TerminalUserInput,
  isKeyboardDismissed: () => boolean = () => false,
  isSizeLocked: () => boolean = () => false,
  isActive: () => boolean = () => true,
  manageChrome = true,
  // A soft keyboard opened from the keyboard button: keep the grid while
  // only the host height changes. Width and font changes still refit.
  isHeightHeld: () => boolean = () => false,
): TerminalController {
  let terminal = $state.raw<Terminal>();
  let renderer: TerminalRenderer | undefined;
  let immersive = false;
  let presentationChanged = () => {};
  let scrollOnFit = false;
  let sized = false;
  let fitWidth = 0;
  let fitFont = 0;
  let follow = true;
  const gridFixed = () => isSizeLocked() || isHeightHeld();
  const stream = createTerminalSocket(id, manageChrome);
  const fontSize = $derived(terminalFont.size);
  const setFontSize = terminalFont.set;
  let modifiers = $state<Modifiers>({
    ctrl: false,
    alt: false,
    shift: false,
  });
  let shiftLocked = $state(false);
  let selection = $state("");
  let search: SearchAddon | undefined;
  let canFind = $state(false);
  let findResults = $state<TerminalFindResults | null>(null);
  let findReplayed = $state(false);
  let bufferType = $state<"normal" | "alternate">("normal");
  let scrollback = $state(10000);
  let findActive = false;
  function clearFind() {
    findActive = false;
    search?.clearDecorations();
    terminal?.clearSelection();
    // clearDecorations does not emit a result-reset event.
    findResults = null;
    findReplayed = false;
  }
  function find(
    query: string,
    options: TerminalFindOptions,
    previous: boolean,
    incremental = false,
  ) {
    if (!query) {
      clearFind();
      return;
    }
    if (
      !terminal ||
      !search ||
      stream.restoring ||
      terminalFindError(query, options.regex)
    )
      return;
    const wasActive = findActive;
    findActive = true;
    findReplayed = false;
    executeTerminalFind(
      terminal,
      search,
      query,
      options,
      previous,
      incremental,
      wasActive,
    );
  }
  let atBottom = $state(true);
  let pasteResult = false;
  // Literal paste bypasses one-shot modifiers in xterm's onData callback.
  let pasting = false;
  let keyboardReopenAttempted = false;
  function anchorBottom(parent: HTMLElement) {
    terminal?.scrollToBottom();
    parent.scrollTop = parent.scrollHeight;
    // Anchoring is an explicit request to follow the tail, not a user scroll.
    follow = true;
  }
  function safeFit() {
    if (immersive) {
      const grid = stream.control;
      if (
        terminal &&
        (stream.observing || !sized) &&
        grid &&
        (terminal.cols !== grid.cols || terminal.rows !== grid.rows)
      )
        terminal.resize(grid.cols, grid.rows);
      if (grid) sized = true;
      updateBottom();
      return;
    }
    const parent = getContainer();
    if (isVisible() && parent?.clientWidth > 0 && parent?.clientHeight > 0) {
      const held =
        isHeightHeld() &&
        sized &&
        parent.offsetWidth === fitWidth &&
        terminal?.options.fontSize === fitFont;
      ({ sized, fitWidth, fitFont } = fitTerminalGrid(
        parent,
        terminal,
        renderer,
        stream.control,
        stream.observing,
        isSizeLocked(),
        held,
        sized,
        fitWidth,
        fitFont,
      ));
      if ((scrollOnFit || (gridFixed() && follow)) && terminal) {
        anchorBottom(parent);
        scrollOnFit = false;
      }
      updateBottom();
    } else {
      sized = false;
    }
  }
  function updateBottom(userScrolled = false) {
    const parent = getContainer();
    if (!terminal || !parent) return;
    const buffer = terminal.buffer.active;
    // Observers and locked grids scroll their host past the xterm viewport.
    const scrollsHost =
      !immersive && (parent.classList.contains("observing") || gridFixed());
    atBottom =
      buffer.viewportY >= buffer.baseY &&
      (!scrollsHost ||
        parent.scrollTop + parent.clientHeight >= parent.scrollHeight - 1);
    // Output and host resizes can change the geometry without moving the
    // scroll position; only an actual user scroll changes the intent.
    if (userScrolled) follow = atBottom;
  }
  function scrollToBottom() {
    if (immersive) {
      terminal?.scrollToBottom();
      follow = true;
      updateBottom();
      return;
    }
    scrollOnFit = true;
    follow = true;
    safeFit();
  }
  function releaseModifiers() {
    modifiers = { ctrl: false, alt: false, shift: shiftLocked };
  }
  function clearModifiers() {
    shiftLocked = false;
    modifiers = { ctrl: false, alt: false, shift: false };
  }
  function consumeInput(input: string) {
    const value = pasting ? input : modifiedInput(input, modifiers);
    releaseModifiers();
    return value;
  }
  function send(input: string) {
    pasteResult = stream.sendInput(consumeInput(input));
    return pasteResult;
  }

  $effect(() => {
    const value = fontSize;
    untrack(() => {
      if (!terminal) return;
      terminal.options.fontSize = value;
      const lockedGrid = isSizeLocked() && sized;
      safeFit();
      if (lockedGrid && follow) {
        requestAnimationFrame(() => {
          if (terminal && isSizeLocked() && follow) {
            anchorBottom(getContainer());
            updateBottom();
          }
        });
      }
    });
  });

  onMount(() => {
    const parent = getContainer();
    let disposed = false;
    let release = () => {};
    void loadTerminalFont(fontSize).then(() => {
      if (!disposed) release = mountTerminal();
    });

    function mountTerminal() {
      const view = createTerminalRenderer(parent, fontSize, {
        layout: safeFit,
        // Called only by the theme observer, once `attachment` exists.
        themeChanged: () => attachment.themeChanged(),
      });
      const term = view.term;
      renderer = view;
      terminal = term;
      view.setImmersive(immersive);
      search = new SearchAddon({ highlightLimit: terminalFindLimit });
      term.loadAddon(search);
      canFind = true;
      scrollback = term.options.scrollback ?? 10000;
      bufferType = term.buffer.active.type;
      const resultsChanged = search.onDidChangeResults((results) => {
        if (findActive) {
          // The addon clears then reselects on its delayed output refresh.
          // xterm omits the second selection event for unchanged coordinates,
          // so read the completed search's selection rather than its clear.
          selection = term.getSelection();
          findResults = results;
        }
      });
      const bufferChanged = term.buffer.onBufferChange((buffer) => {
        clearFind();
        bufferType = buffer.type;
        // Replayed TUI entry/exit must not erase the full-replay notice.
        findReplayed = stream.restoring;
      });
      const attachment = stream.attach(term, {
        fit: safeFit,
        scrollToBottom,
        isVisible,
        isImmersive: () => immersive,
        copy: (text) => clipboard.copySelection(text),
        resetFind: () => {
          clearFind();
          findReplayed = true;
        },
        preserveViewport: () => findActive || !follow,
      });
      presentationChanged = attachment.presentationChanged;
      const detachInput = attachTerminalInput(term, parent, {
        userInput,
        clipboard,
        onData: (data, user) => {
          if (user) send(data);
          else stream.sendReply(data);
        },
      });
      const detachViewport = bindTerminalViewport(
        term,
        parent,
        () => (selection = term.getSelection()),
        () => updateBottom(true),
        () => updateBottom(),
        () => {
          pasting = true;
          queueMicrotask(() => (pasting = false));
        },
      );
      const detachTouch = attachTouchGestures(term, parent, {
        hasSelection: () => Boolean(selection),
        keyboardDismissed: isKeyboardDismissed,
        hostScrolled: () => updateBottom(true),
      });
      safeFit();
      if (!hasTouchInput() && isVisible() && isActive()) term.focus();
      return () => {
        disposed = true;
        view.dispose();
        detachTouch();
        detachInput();
        detachViewport();
        resultsChanged.dispose();
        bufferChanged.dispose();
        clearFind();
        search?.dispose();
        search = undefined;
        canFind = false;
        attachment.dispose();
        terminal = undefined;
        renderer = undefined;
        parent.classList.remove("observing");
        atBottom = true;
        follow = true;
        scrollOnFit = false;
      };
    }

    return () => {
      disposed = true;
      release();
    };
  });

  return {
    get emulator() {
      return terminal;
    },
    get graphicsUnsupported() {
      return stream.graphicsUnsupported;
    },
    setImmersive(active) {
      if (immersive === active) return;
      immersive = active;
      renderer?.setImmersive(active);
      presentationChanged();
      if (!active) safeFit();
    },
    scrollLines(lines) {
      if (!Number.isFinite(lines)) return;
      terminal?.scrollLines(Math.trunc(lines));
      updateBottom(true);
    },
    resizeGrid(cols, rows) {
      if (!resizeTerminalGrid(terminal, stream.controlling, cols, rows))
        return false;
      updateBottom();
      return true;
    },
    get ready() {
      return !!terminal && stream.accepting;
    },
    get info() {
      return stream.info;
    },
    get connection() {
      return stream.connection;
    },
    get restoring() {
      return stream.restoring;
    },
    get drawn() {
      return stream.drawn;
    },
    get exitCode() {
      return stream.exitCode;
    },
    get fontSize() {
      return fontSize;
    },
    get modifiers() {
      return modifiers;
    },
    get shiftLocked() {
      return shiftLocked;
    },
    get error() {
      return stream.error;
    },
    get selection() {
      return selection;
    },
    get atBottom() {
      return atBottom;
    },
    get historyTruncated() {
      return stream.historyTruncated;
    },
    get control() {
      return stream.control;
    },
    get controlling() {
      return stream.controlling;
    },
    get clientId() {
      return stream.clientId;
    },
    get canFind() {
      return canFind;
    },
    get findResults() {
      return findResults;
    },
    get findReplayed() {
      return findReplayed;
    },
    get bufferType() {
      return bufferType;
    },
    get scrollback() {
      return scrollback;
    },
    findNext(query, options, incremental = false) {
      find(query, options, false, incremental);
    },
    findPrevious(query, options) {
      find(query, options, true);
    },
    clearFind,
    claim: stream.claim,
    submit(submissionId, text, enter, report) {
      if (!stream.submittable) return false;
      releaseModifiers();
      return stream.submit(submissionId, text, enter, report);
    },
    track: stream.track,
    untrack: stream.untrack,
    copySelection() {
      if (selection) clipboard.copySelection(selection, true);
    },
    clearSelection() {
      terminal?.clearSelection();
    },
    send,
    consumeInput,
    setFontSize,
    resetModifiers: clearModifiers,
    toggleModifier(key: keyof Modifiers) {
      if (key === "shift" && shiftLocked) {
        shiftLocked = false;
        modifiers = { ...modifiers, shift: false };
        return;
      }
      modifiers = { ...modifiers, [key]: !modifiers[key] };
    },
    lockModifier(key: "shift") {
      shiftLocked = true;
      modifiers = { ...modifiers, [key]: true };
    },
    paste(text: string) {
      if (!terminal || !stream.accepting) return false;
      pasteResult = false;
      releaseModifiers();
      pasting = true;
      try {
        terminal.paste(text);
      } finally {
        pasting = false;
      }
      return pasteResult;
    },
    keyboard(beforeOpen) {
      keyboardReopenAttempted = toggleTerminalKeyboard(
        terminal,
        isKeyboardDismissed,
        keyboardReopenAttempted,
        stream.claim,
        beforeOpen,
      );
    },
    clear() {
      clearFind();
      terminal?.clear();
    },
    resync: stream.resync,
    reconnect: stream.reconnect,
    fit: safeFit,
    scrollToBottom,
    focus() {
      terminal?.focus();
    },
  };
}
